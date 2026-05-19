//go:build !private_distribution

package settlement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	ethWal "github.com/mobazha/mobazha3.0/internal/chains/evm"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/relay"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ── EscrowOperations port: ReleaseCancelableFunds ───────────────────────

// ReleaseCancelableFunds encapsulates chain-specific logic for releasing CANCELABLE
// payment funds. Uses the payment registry to determine the release mechanism
// based on PaymentModel rather than chain type checks.
//
// Returns (txid, actualPayoutAddress, error). If the payment is not CANCELABLE or the
// chain doesn't support backend release, txid is empty and payoutAddress is unchanged.
func (s *SettlementService) ReleaseCancelableFunds(order *models.Order, payoutAddress string) (iwallet.TransactionID, string, error) {
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return "", payoutAddress, nil
	}
	if paymentSent.Method != pb.PaymentSent_CANCELABLE {
		return "", payoutAddress, nil
	}

	coinType, err := order.GetPaymentCoinType()
	if err != nil {
		return "", payoutAddress, err
	}

	if s.paymentRegistry == nil {
		return "", payoutAddress, fmt.Errorf("payment registry not initialized")
	}

	strategyV2, err := s.paymentRegistry.ForCoinV2(coinType)
	if err != nil {
		return "", payoutAddress, fmt.Errorf("no chain escrow for coin %s: %w", coinType, err)
	}

	switch strategyV2.Model() {
	case payment.PaymentModelMonitored:
		coinInfo, cerr := iwallet.CoinInfoFromCoinType(coinType)
		if cerr != nil {
			return "", payoutAddress, cerr
		}
		// ManagedEscrow-backed EVM orders no longer share the legacy seller-confirm path.
		// They require the unified settlement-action flow so the caller can
		// obtain a pollable ActionID instead of forcing an implicit relay.
		if coinInfo.IsEthTypeChain() {
			return "", payoutAddress, fmt.Errorf("%w: ManagedEscrow-backed EVM orders must use POST /v1/orders/{orderID}/settlement-actions/confirm",
				coreiface.ErrBadRequest)
		}
		return s.releaseMonitoredCancelableFunds(order, payoutAddress)
	case payment.PaymentModelClientSigned:
		coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
		if err != nil {
			return "", payoutAddress, err
		}
		if coinInfo.IsEthTypeChain() && s.IsEVMRelayAvailable() {
			return s.releaseViaRelay(order, &coinInfo, payoutAddress)
		}
		if coinInfo.Chain == iwallet.ChainSolana && s.IsSolanaRelayAvailable() {
			return s.releaseSolanaViaRelay(order, payoutAddress)
		}
		return "", payoutAddress, nil
	default:
		return "", payoutAddress, nil
	}
}

func (s *SettlementService) releaseMonitoredCancelableFunds(order *models.Order, payoutAddress string) (iwallet.TransactionID, string, error) {
	logger.LogInfoWithIDf(log, s.nodeID, "Releasing monitored CANCELABLE funds for order %s via backend confirm", order.ID)

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return "", "", err
	}
	if paymentSent.Method != pb.PaymentSent_CANCELABLE {
		return "", "", errors.New("order payment method is not CANCELABLE")
	}

	coinType := iwallet.CoinType(paymentSent.Coin)
	var toAddress iwallet.Address

	if payoutAddress != "" {
		toAddress = iwallet.NewAddress(payoutAddress, coinType)
		wallet, err := s.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
		if err != nil {
			return "", "", fmt.Errorf("failed to get wallet for %s: %w", paymentSent.Coin, err)
		}
		if err := wallet.ValidateAddress(toAddress); err != nil {
			return "", "", fmt.Errorf("invalid payout address %s: %w", payoutAddress, err)
		}
	} else {
		toAddress, err = s.GetPayoutAddress(paymentSent.Coin)
		if err != nil {
			return "", "", err
		}
	}

	params := contracts.ReleaseFromCancelableParams{
		CoinCode:       paymentSent.Coin,
		PaymentAddress: paymentSent.ToAddress,
		ScriptHex:      paymentSent.Script,
		ChaincodeHex:   paymentSent.Chaincode,
		ToAddress:      toAddress,
		FinishType:     iwallet.ORDER_FINISH_COMPLETE,
	}

	wTx, tx, err := s.ReleaseFromCancelableAddressWithParams(order, params)
	if err != nil {
		return "", "", fmt.Errorf("failed to release CANCELABLE payment: %w", err)
	}

	if err := wTx.Commit(); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to commit wallet transaction for order %s: %v (on-chain tx already broadcast)", order.ID, err)
	}

	actualAddr := toAddress.String()
	var txid iwallet.TransactionID

	if tx != nil {
		txid = tx.ID
		if err := order.PutTransaction(*tx); err != nil && !models.IsDuplicateTransactionError(err) {
			logger.LogWarningWithIDf(log, s.nodeID, "Failed to save release transaction for order %s: %v", order.ID, err)
		}
		if err := s.db.Update(func(dbtx database.Tx) error {
			return dbtx.Save(order)
		}); err != nil {
			logger.LogWarningWithIDf(log, s.nodeID, "Failed to persist order with release transaction: %v", err)
		}
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Successfully released monitored CANCELABLE payment for order %s (txid=%s)", order.ID, txid)
	return txid, actualAddr, nil
}

func (s *SettlementService) releaseViaRelay(order *models.Order, coinInfo *iwallet.CoinInfo, payoutAddress string) (iwallet.TransactionID, string, error) {
	logger.LogInfoWithIDf(log, s.nodeID, "Releasing client-signed CANCELABLE funds for order %s via relay (chain=%s)", order.ID, coinInfo.Chain)

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return "", "", fmt.Errorf("relay release: failed to get PaymentSent message for order %s: %w", order.ID, err)
	}

	if payoutAddress == "" {
		addr, err := s.GetPayoutAddress(paymentSent.Coin)
		if err != nil {
			return "", "", fmt.Errorf("failed to get payout address: %w", err)
		}
		payoutAddress = addr.String()
	}

	_, instructions, err := s.GetConfirmOrderInstructions(models.OrderID(order.ID), "", payoutAddress)
	if err != nil {
		return "", "", fmt.Errorf("failed to get confirm order instructions: %w", err)
	}
	if instructions == nil {
		return "", "", errors.New("no instructions returned for client-signed CANCELABLE order")
	}

	txHashStr, err := s.RelayInstructions(string(order.ID), iwallet.CoinType(paymentSent.Coin), instructions)
	if err != nil {
		return "", "", fmt.Errorf("failed to relay transaction: %w", err)
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Successfully released client-signed CANCELABLE payment for order %s via relay (txid=%s)", order.ID, txHashStr)
	return iwallet.TransactionID(txHashStr), payoutAddress, nil
}

func (s *SettlementService) releaseSolanaViaRelay(order *models.Order, payoutAddress string) (iwallet.TransactionID, string, error) {
	logger.LogInfoWithIDf(log, s.nodeID, "Releasing Solana CANCELABLE funds for order %s via relay", order.ID)

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return "", "", fmt.Errorf("solana relay release: failed to get PaymentSent message for order %s: %w", order.ID, err)
	}
	if payoutAddress == "" {
		addr, err := s.GetPayoutAddress(paymentSent.Coin)
		if err != nil {
			return "", "", fmt.Errorf("failed to get payout address: %w", err)
		}
		payoutAddress = addr.String()
	}

	_, instructions, err := s.GetConfirmOrderInstructions(order.ID, "", payoutAddress)
	if err != nil {
		return "", "", fmt.Errorf("failed to get confirm order instructions: %w", err)
	}
	if instructions == nil {
		return "", "", fmt.Errorf("no instructions returned for Solana CANCELABLE order")
	}

	txSig, err := s.RelaySolanaTransaction(context.Background(), string(order.ID), instructions)
	if err != nil {
		return "", "", fmt.Errorf("failed to relay Solana transaction: %w", err)
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Successfully released Solana CANCELABLE payment for order %s via relay (sig=%s)", order.ID, txSig)
	return iwallet.TransactionID(txSig), payoutAddress, nil
}

// ── EscrowOperations port: RelayInstructions ────────────────────────────

// RelayInstructions dispatches instructions to the appropriate relay service.
func (s *SettlementService) RelayInstructions(orderID string, coinType iwallet.CoinType, instructions any) (string, error) {
	coinInfo, err := coinType.CoinInfo()
	if err != nil {
		return "", fmt.Errorf("failed to get coin info: %w", err)
	}

	if coinInfo.IsEthTypeChain() {
		if !s.IsEVMRelayAvailable() {
			return "", fmt.Errorf("EVM relay service not available")
		}
		txData, ok := instructions.(*ethWal.TransactionData)
		if !ok {
			if _, isMap := instructions.(map[string]any); isMap {
				return "", fmt.Errorf("%w: ManagedEscrow-backed EVM instructions require the unified settlement-action submit flow", coreiface.ErrBadRequest)
			}
			return "", fmt.Errorf("invalid EVM transaction data type: %T", instructions)
		}
		return s.RelayEVMTransactionWithRetry(context.Background(), orderID, string(coinInfo.Chain), string(coinType), txData)
	}

	if coinInfo.Chain == iwallet.ChainSolana {
		if !s.IsSolanaRelayAvailable() {
			return "", fmt.Errorf("Solana relay service not available")
		}
		return s.RelaySolanaTransaction(context.Background(), orderID, instructions)
	}

	return "", fmt.Errorf("unsupported chain for relay: %s", coinInfo.Chain)
}

// ── Confirm Order Instructions ──────────────────────────────────────────

// GetConfirmOrderInstructions generates chain-specific instructions for confirming a CANCELABLE order.
func (s *SettlementService) GetConfirmOrderInstructions(orderID models.OrderID, initiatorAddress string, payoutAddress string) (coinType iwallet.CoinType, instructions any, err error) {
	var order models.Order
	err = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return "", nil, err
	}

	if !order.CanConfirm() {
		return "", nil, fmt.Errorf("order is not in a state where it can be confirmed")
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return "", nil, err
	}

	if paymentSent.Method != pb.PaymentSent_CANCELABLE {
		logger.LogInfoWithIDf(log, s.nodeID, "%s not a cancelable payment, no instructions needed", orderID)
		return "", nil, nil
	}

	coinType = iwallet.CoinType(paymentSent.Coin)
	if !payment.UsesClientSignedPayMode(&order, paymentSent) {
		logger.LogInfoWithIDf(log, s.nodeID, "%s uses address-monitored settlement flow, no instructions needed", orderID)
		return coinType, nil, nil
	}

	if len(payoutAddress) == 0 {
		toAddress, err := s.GetPayoutAddress(paymentSent.Coin)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get payout address: %w", err)
		}
		payoutAddress = toAddress.String()
	}

	if s.paymentRegistry == nil {
		return coinType, nil, fmt.Errorf("payment registry not initialized")
	}
	strategy, err := s.paymentRegistry.ForCoin(coinType)
	if err != nil {
		return coinType, nil, fmt.Errorf("no chain escrow for coin %s: %w", paymentSent.Coin, err)
	}

	result, err := strategy.GetConfirmInstructions(context.Background(), payment.InstructionParams{
		OrderID:       orderID.String(),
		InitiatorAddr: initiatorAddress,
		PayoutAddr:    payoutAddress,
		PaymentCoin:   paymentSent.Coin,
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     &order,
	})
	if err != nil {
		return coinType, nil, err
	}

	instructions = nil
	if result != nil {
		instructions = result.Instructions
	}
	return coinType, instructions, nil
}

// ── EVM Relay Infrastructure ────────────────────────────────────────────

// RelayExecuteRequest is the request structure for platform relay API.
type RelayExecuteRequest struct {
	ChainType string `json:"chainType"`
	To        string `json:"to"`
	Data      string `json:"data"`
	OrderID   string `json:"orderId"`
}

// RelayExecuteResponse is the inner payload of response.Success for platform relay.
type RelayExecuteResponse struct {
	Success bool   `json:"success"`
	TxHash  string `json:"txHash,omitempty"`
	Error   string `json:"error,omitempty"`
}

type relayExecuteSuccessEnvelope struct {
	Data RelayExecuteResponse `json:"data"`
}

func (s *SettlementService) RelayEVMTransaction(orderID, chainType string, txData *ethWal.TransactionData) (string, error) {
	if s.evmRelayService != nil && s.evmRelayService.IsAvailable() {
		logger.LogInfoWithIDf(log, s.nodeID, "Using HostService relay for order %s (direct call)", orderID)
		resp, err := s.evmRelayService.Execute(context.Background(), &relay.EVMRelayRequest{
			ChainType: chainType,
			To:        txData.To,
			Data:      txData.Data,
			OrderID:   orderID,
		})
		if err != nil {
			return "", fmt.Errorf("hostservice relay failed: %w", err)
		}
		return resp.TxHash, nil
	}

	if s.relayAPIURL == "" {
		return "", fmt.Errorf("relay service not available: no HostService relay and no relayAPIURL configured")
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Using HTTP relay for order %s (fallback)", orderID)
	return s.relayEVMTransactionViaHTTP(orderID, chainType, txData)
}

func (s *SettlementService) relayEVMTransactionViaHTTP(orderID, chainType string, txData *ethWal.TransactionData) (string, error) {
	req := RelayExecuteRequest{
		ChainType: chainType,
		To:        txData.To,
		Data:      txData.Data,
		OrderID:   orderID,
	}

	base := strings.TrimRight(strings.TrimSpace(s.relayAPIURL), "/")
	url := base + "/platform/v1/relay/execute"

	rc := resty.New().SetTimeout(30*time.Second).R().
		SetHeader("Content-Type", "application/json").
		SetBody(req)
	if b := relay.BearerFromConfigOrEnv(s.relayAPIBearer); b != "" {
		rc.SetHeader("Authorization", "Bearer "+b)
	}

	resp, err := rc.Post(url)
	if err != nil {
		return "", fmt.Errorf("failed to send relay request: %w", err)
	}

	if resp.IsError() {
		return "", fmt.Errorf("relay request failed with status %d", resp.StatusCode())
	}

	var wrap relayExecuteSuccessEnvelope
	if err := json.Unmarshal(resp.Body(), &wrap); err != nil {
		return "", fmt.Errorf("relay response: %w", err)
	}
	if !wrap.Data.Success || wrap.Data.TxHash == "" {
		if wrap.Data.Error != "" {
			return "", fmt.Errorf("relay failed: %s", wrap.Data.Error)
		}
		return "", fmt.Errorf("relay failed: empty txHash")
	}

	return wrap.Data.TxHash, nil
}

// VerifyEVMConfirmReceipt delegates to the injected ReceiptVerifier.
func (s *SettlementService) VerifyEVMConfirmReceipt(coinCode string, txHash string) error {
	if s.receiptVerifier == nil {
		return nil
	}
	return s.receiptVerifier.VerifyTransactionReceipt(context.Background(), coinCode, txHash)
}

const (
	relayMaxRetries  = 3
	relayBaseBackoff = 2 * time.Second
)

// RelayEVMTransactionWithReceipt broadcasts via relay then waits for the
// on-chain receipt, returning the txHash only after confirming receipt.Status == 1.
func (s *SettlementService) RelayEVMTransactionWithReceipt(ctx context.Context, orderID, chainType, coinCode string, txData *ethWal.TransactionData) (string, error) {
	txHash, err := s.RelayEVMTransaction(orderID, chainType, txData)
	if err != nil {
		return "", err
	}

	if s.receiptVerifier == nil {
		logger.LogWarningWithIDf(log, s.nodeID, "No receipt verifier configured for order %s (returning txHash as-is)", orderID)
		return txHash, nil
	}

	if err := s.receiptVerifier.WaitAndVerifyReceipt(ctx, coinCode, txHash); err != nil {
		return txHash, fmt.Errorf("receipt verification failed for tx %s: %w", txHash, err)
	}

	return txHash, nil
}

// RelayEVMTransactionWithRetry wraps RelayEVMTransactionWithReceipt with
// exponential backoff retry. Only transient errors trigger retries;
// ErrTransactionReverted is fatal and returned immediately.
func (s *SettlementService) RelayEVMTransactionWithRetry(ctx context.Context, orderID, chainType, coinCode string, txData *ethWal.TransactionData) (string, error) {
	var lastErr error

	for attempt := 0; attempt < relayMaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * relayBaseBackoff
			logger.LogWarningWithIDf(log, s.nodeID, "Relay retry %d/%d for order %s after %v (last error: %v)", attempt+1, relayMaxRetries, orderID, backoff, lastErr)

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		txHash, err := s.RelayEVMTransactionWithReceipt(ctx, orderID, chainType, coinCode, txData)
		if err == nil {
			return txHash, nil
		}

		if errors.Is(err, payment.ErrTransactionReverted) {
			return txHash, err
		}

		lastErr = err
	}

	return "", fmt.Errorf("relay failed after %d attempts for order %s: %w", relayMaxRetries, orderID, lastErr)
}

// ── Solana Relay Infrastructure ─────────────────────────────────────────

// RelaySolanaTransaction sends Solana instructions to the platform relay service
// for fee-payer signing and broadcasting.
func (s *SettlementService) RelaySolanaTransaction(ctx context.Context, orderID string, instructions any) (string, error) {
	if !s.IsSolanaRelayAvailable() {
		return "", fmt.Errorf("Solana relay service not available")
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Relaying Solana transaction for order %s via HostService relay", orderID)
	resp, err := s.solanaRelayService.Execute(ctx, &relay.SolanaRelayRequest{
		Instructions: instructions,
		OrderID:      orderID,
	})
	if err != nil {
		return "", fmt.Errorf("solana relay failed: %w", err)
	}
	return resp.TxSignature, nil
}

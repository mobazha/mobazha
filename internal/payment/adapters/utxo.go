package adapters

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// UTXOMonitorQuerier provides the subset of UTXOMonitorService needed for
// chain-level UTXO verification. Defined here to avoid importing the full
// monitor interface into the adapter.
type UTXOMonitorQuerier interface {
	GetWatchedAddress(address string) *utxo.WatchedAddress
	GetAddressTransactions(chainType iwallet.ChainType, address string, scriptPubKey []byte) ([]iwallet.Transaction, error)
}

// UTXOAutoConfirmAdapter wraps UTXO auto-confirm logic as a PaymentStrategy.
// This is a thin adapter — complex business logic (wallet, DB, messenger)
// is accessed via injected callbacks, not a direct MobazhaNode reference.
type UTXOAutoConfirmAdapter struct {
	Multiwallet    contracts.WalletOperator
	Keys           contracts.KeyProvider
	MonitorQuerier UTXOMonitorQuerier
	OnAutoConfirm  func(event *events.CancelablePaymentReady)
	GetPaymentInfo func(ctx context.Context, orderID, moderator string, coinType iwallet.CoinType) (*models.PaymentData, error)
}

// Model returns the payment model (Monitored for UTXO).
func (a *UTXOAutoConfirmAdapter) Model() payment.PaymentModel {
	return payment.PaymentModelMonitored
}

// Capabilities returns UTXO chain capabilities.
func (a *UTXOAutoConfirmAdapter) Capabilities() payment.ChainCapabilities {
	return payment.ChainCapabilities{
		HasReceiptVerification: false,
		HasClientSignedEscrow:  false,
		EscrowType:             "multisig",
	}
}

// AutoConfirm invokes the OnAutoConfirm callback.
func (a *UTXOAutoConfirmAdapter) AutoConfirm(_ context.Context, event *events.CancelablePaymentReady) error {
	a.OnAutoConfirm(event)
	return nil
}

// ── UTXO Escrow Operations ──────────────────────────────────────

// SignEscrowRelease signs the multisig escrow release transaction.
func (a *UTXOAutoConfirmAdapter) SignEscrowRelease(_ context.Context, params payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	wallet, err := a.Multiwallet.WalletForCurrencyCode(params.CoinCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet for %s: %w", params.CoinCode, err)
	}

	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return nil, fmt.Errorf("wallet for %s does not support escrow", params.CoinCode)
	}

	escrowMasterKey, err := a.Keys.EscrowMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get escrow master key: %w", err)
	}

	key, err := utils.GenerateEscrowPrivateKey(escrowMasterKey, params.ChainCode)
	if err != nil {
		return nil, fmt.Errorf("failed to generate escrow private key: %w", err)
	}

	sigs, err := escrowWallet.SignMultisigTransaction(params.Transaction, *key, params.Script)
	if err != nil {
		return nil, fmt.Errorf("failed to sign multisig transaction: %w", err)
	}

	return sigs, nil
}

// EstimateEscrowFee estimates the escrow fee for the given parameters.
func (a *UTXOAutoConfirmAdapter) EstimateEscrowFee(coinCode string, nIn, nOut int, feeLevel iwallet.FeeLevel) (iwallet.Amount, error) {
	wallet, err := a.Multiwallet.WalletForCurrencyCode(coinCode)
	if err != nil {
		return iwallet.Amount{}, fmt.Errorf("failed to get wallet for %s: %w", coinCode, err)
	}

	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return iwallet.Amount{}, fmt.Errorf("wallet for %s does not support escrow", coinCode)
	}

	return escrowWallet.EstimateEscrowFee(nIn, nOut, feeLevel)
}

// ── UTXO Payment Setup ──────────────────────────────────────────

// GeneratePaymentInstructions returns payment setup result via GetPaymentInfo.
func (a *UTXOAutoConfirmAdapter) GeneratePaymentInstructions(ctx context.Context, params payment.PaymentSetupParams) (*payment.PaymentSetupResult, error) {
	paymentData, err := a.GetPaymentInfo(ctx, params.OrderID, params.Moderator, params.CoinType)
	result := &payment.PaymentSetupResult{
		PaymentModel: payment.PaymentModelMonitored,
		PaymentData:  paymentData,
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

// ── UTXO Instruction Methods ────────────────────────────────────

// UTXO chains are backend-monitored — the backend handles signing and
// broadcasting. All instruction methods return nil Instructions, meaning
// the frontend should call the action API directly without chain interaction.

// GetConfirmInstructions returns nil instructions (backend-monitored).
func (a *UTXOAutoConfirmAdapter) GetConfirmInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{Instructions: nil}, nil
}

// GetCancelInstructions returns nil instructions (backend-monitored).
func (a *UTXOAutoConfirmAdapter) GetCancelInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{Instructions: nil}, nil
}

// GetCompleteInstructions returns nil instructions (backend-monitored).
func (a *UTXOAutoConfirmAdapter) GetCompleteInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{Instructions: nil}, nil
}

// GetDisputeReleaseInstructions returns nil instructions (backend-monitored).
func (a *UTXOAutoConfirmAdapter) GetDisputeReleaseInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{Instructions: nil}, nil
}

// VerifyDeposit is a noop for UTXO — Electrum handles deposit detection.
func (a *UTXOAutoConfirmAdapter) VerifyDeposit(_ context.Context, _ payment.DepositVerifyParams) error {
	return nil
}

// ValidatePaymentMessage validates UTXO PaymentSent against OrderOpen:
// escrow pubkey derivation, multisig script reconstruction, address match,
// and payment amount verification.
func (a *UTXOAutoConfirmAdapter) ValidatePaymentMessage(params payment.PaymentMessageParams) error {
	orderOpen, paymentSent, err := assertPaymentMessageParams(params)
	if err != nil {
		return err
	}

	wallet, err := a.Multiwallet.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		return fmt.Errorf("wallet for %s: %w", paymentSent.Coin, err)
	}

	if err := utils.ValidatePayment(orderOpen, paymentSent, params.EscrowTimeoutHours, wallet); err != nil {
		return err
	}

	if paymentSent.Method != pb.PaymentSent_DIRECT {
		if err := utils.ValidatePaymentAmount(orderOpen.Amount, paymentSent.Amount); err != nil {
			return err
		}
	}

	return nil
}

// ErrUTXOAlreadySpent indicates a UTXO has been spent on-chain.
var ErrUTXOAlreadySpent = errors.New("UTXO already spent on chain")

// VerifyPreRelease queries the chain to verify that expected UTXOs are still
// unspent before signing an escrow release. Best-effort: if the monitor is
// unavailable or the address is not watched, verification is skipped.
func (a *UTXOAutoConfirmAdapter) VerifyPreRelease(_ context.Context, params payment.PreReleaseParams) error {
	if a.MonitorQuerier == nil {
		return nil
	}

	coinInfo, err := params.CoinType.CoinInfo()
	if err != nil || !coinInfo.Chain.IsUTXOChain() {
		return nil
	}

	wa := a.MonitorQuerier.GetWatchedAddress(params.PaymentAddress)
	if wa == nil || len(wa.ScriptPubKey) == 0 {
		log.Warningf("Cannot verify UTXOs: address %s not watched, skipping chain verification", params.PaymentAddress)
		return nil
	}

	chainTxs, err := a.MonitorQuerier.GetAddressTransactions(coinInfo.Chain, params.PaymentAddress, wa.ScriptPubKey)
	if err != nil {
		log.Warningf("Chain UTXO verification query failed for %s: %v, proceeding with local data", params.PaymentAddress, err)
		return nil
	}

	chainSpent := make(map[string]bool)
	for _, tx := range chainTxs {
		for _, from := range tx.From {
			chainSpent[hex.EncodeToString(from.ID)] = true
		}
	}
	chainUnspent := make(map[string]bool)
	for _, tx := range chainTxs {
		for _, to := range tx.To {
			id := hex.EncodeToString(to.ID)
			if !chainSpent[id] && to.Address.String() == params.PaymentAddress {
				chainUnspent[id] = true
			}
		}
	}

	for _, utxo := range params.ExpectedUTXOs {
		id := hex.EncodeToString(utxo.ID)
		if !chainUnspent[id] {
			return fmt.Errorf("%w: outpoint %s not found in unspent set for address %s", ErrUTXOAlreadySpent, id, params.PaymentAddress)
		}
	}

	log.Infof("Chain UTXO verification passed: %d UTXOs confirmed unspent at %s", len(params.ExpectedUTXOs), params.PaymentAddress)
	return nil
}

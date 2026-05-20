//go:build !private_distribution

package settlement

import (
	"context"

	ethWal "github.com/mobazha/mobazha3.0/internal/chains/evm"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ── UTXO Auto-Confirm ───────────────────────────────────────────────────

// HandleCancelablePaymentForUTXO handles CancelablePaymentReady event for UTXO chains.
// Fetches the order, acquires the auto-confirm lock, and emits OrderAutoConfirmRequest.
func (s *SettlementService) HandleCancelablePaymentForUTXO(event *events.CancelablePaymentReady) {
	logger.LogInfoWithIDf(log, s.nodeID, "Handling UTXO CANCELABLE payment ready event for order %s", event.OrderID)

	order, err := s.fetchOrderByID(event.OrderID)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get order %s for UTXO CANCELABLE auto-confirm: %v", event.OrderID, err)
		return
	}

	unlock := s.TryLockAutoConfirm(order.ID.String())
	if unlock == nil {
		return
	}
	defer unlock()

	logger.LogInfoWithIDf(log, s.nodeID, "Auto-confirming UTXO CANCELABLE payment for order %s", order.ID)

	s.eventBus.Emit(&events.OrderAutoConfirmRequest{
		OrderID: order.ID.String(),
	})

	logger.LogInfoWithIDf(log, s.nodeID, "Emitted OrderAutoConfirmRequest for UTXO CANCELABLE order %s", order.ID)
}

// ── EVM Auto-Confirm ────────────────────────────────────────────────────

// HandleCancelablePaymentForEVM handles CancelablePaymentReady event for EVM chains via relay.
func (s *SettlementService) HandleCancelablePaymentForEVM(event *events.CancelablePaymentReady, chainType string) {
	logger.LogInfoWithIDf(log, s.nodeID, "Handling EVM CANCELABLE payment ready event for order %s (chain=%s)", event.OrderID, chainType)

	if !s.IsEVMRelayAvailable() {
		logger.LogWarningWithIDf(log, s.nodeID, "EVM Relay service not available, cannot auto-confirm EVM CANCELABLE payment for order %s", event.OrderID)
		return
	}

	order, err := s.fetchOrderByID(event.OrderID)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get order %s for EVM CANCELABLE auto-confirm: %v", event.OrderID, err)
		return
	}

	s.autoConfirmEVMCancelablePayment(order, chainType)
}

func (s *SettlementService) autoConfirmEVMCancelablePayment(order *models.Order, chainType string) {
	unlock := s.TryLockAutoConfirm(order.ID.String())
	if unlock == nil {
		return
	}
	defer unlock()

	logger.LogInfoWithIDf(log, s.nodeID, "Auto-confirming EVM CANCELABLE payment for order %s via platform relay", order.ID)

	if !order.CanConfirm() {
		logger.LogWarningWithIDf(log, s.nodeID, "Order %s cannot be confirmed (state=%s)", order.ID, order.State)
		return
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get PaymentSent for order %s: %v", order.ID, err)
		return
	}
	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to resolve payment coin for order %s: %v", order.ID, err)
		return
	}

	payoutAddress, err := s.GetPayoutAddress(string(coinType))
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get payout address for order %s: %v", order.ID, err)
		return
	}

	coinType, instructions, err := s.GetConfirmOrderInstructions(order.ID, "", payoutAddress.String())
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get confirm order instructions for order %s: %v", order.ID, err)
		return
	}

	if instructions == nil {
		logger.LogWarningWithIDf(log, s.nodeID, "No instructions returned for order %s (coinType=%s)", order.ID, coinType)
		return
	}

	coinInfo, err := coinType.CoinInfo()
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get coin info for %s: %v", coinType, err)
		return
	}

	var txHash string

	if coinInfo.IsEthTypeChain() {
		txData, ok := instructions.(*ethWal.TransactionData)
		if !ok {
			logger.LogErrorWithIDf(log, s.nodeID, "Invalid transaction data type for EVM chain order %s", order.ID)
			return
		}
		txHash, err = s.RelayEVMTransactionWithRetry(context.Background(), order.ID.String(), chainType, string(coinType), txData)
		if err != nil {
			logger.LogErrorWithIDf(log, s.nodeID, "Failed to relay EVM transaction for order %s: %v", order.ID, err)
			return
		}
	} else if coinInfo.Chain == iwallet.ChainSolana {
		txHash, err = s.RelaySolanaTransaction(context.Background(), order.ID.String(), instructions)
		if err != nil {
			logger.LogErrorWithIDf(log, s.nodeID, "Failed to relay Solana transaction for order %s: %v", order.ID, err)
			return
		}
	} else {
		logger.LogErrorWithIDf(log, s.nodeID, "Unsupported chain type for relay: %s", coinInfo.Chain)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Successfully relayed transaction for order %s, txHash=%s", order.ID, txHash)

	s.eventBus.Emit(&events.OrderAutoConfirmRequest{
		OrderID:       order.ID.String(),
		TxID:          txHash,
		PayoutAddress: payoutAddress.String(),
	})

	logger.LogInfoWithIDf(log, s.nodeID, "Emitted OrderAutoConfirmRequest for EVM CANCELABLE order %s", order.ID)
}

// ── Solana Auto-Confirm ─────────────────────────────────────────────────

// HandleCancelablePaymentForSolana handles CancelablePaymentReady event for Solana
// chains via relay.
func (s *SettlementService) HandleCancelablePaymentForSolana(event *events.CancelablePaymentReady) {
	logger.LogInfoWithIDf(log, s.nodeID, "Handling Solana CANCELABLE payment ready event for order %s", event.OrderID)

	if !s.IsSolanaRelayAvailable() {
		logger.LogWarningWithIDf(log, s.nodeID, "Solana Relay service not available, cannot auto-confirm Solana CANCELABLE payment for order %s", event.OrderID)
		return
	}

	order, err := s.fetchOrderByID(event.OrderID)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get order %s for Solana CANCELABLE auto-confirm: %v", event.OrderID, err)
		return
	}

	s.autoConfirmSolanaCancelablePayment(order)
}

func (s *SettlementService) autoConfirmSolanaCancelablePayment(order *models.Order) {
	unlock := s.TryLockAutoConfirm(order.ID.String())
	if unlock == nil {
		return
	}
	defer unlock()

	logger.LogInfoWithIDf(log, s.nodeID, "Auto-confirming Solana CANCELABLE payment for order %s via platform relay", order.ID)

	if !order.CanConfirm() {
		logger.LogWarningWithIDf(log, s.nodeID, "Order %s cannot be confirmed (state=%s)", order.ID, order.State)
		return
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get PaymentSent for order %s: %v", order.ID, err)
		return
	}
	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to resolve payment coin for order %s: %v", order.ID, err)
		return
	}

	payoutAddress, err := s.GetPayoutAddress(string(coinType))
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get payout address for order %s: %v", order.ID, err)
		return
	}

	_, instructions, err := s.GetConfirmOrderInstructions(order.ID, "", payoutAddress.String())
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get confirm order instructions for order %s: %v", order.ID, err)
		return
	}

	if instructions == nil {
		logger.LogWarningWithIDf(log, s.nodeID, "No instructions returned for Solana order %s", order.ID)
		return
	}

	txSig, err := s.RelaySolanaTransaction(context.Background(), order.ID.String(), instructions)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to relay Solana transaction for order %s: %v", order.ID, err)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Successfully relayed Solana transaction for order %s, sig=%s", order.ID, txSig)

	s.eventBus.Emit(&events.OrderAutoConfirmRequest{
		OrderID:       order.ID.String(),
		TxID:          txSig,
		PayoutAddress: payoutAddress.String(),
	})

	logger.LogInfoWithIDf(log, s.nodeID, "Emitted OrderAutoConfirmRequest for Solana CANCELABLE order %s", order.ID)
}

// ── TRON Auto-Confirm ───────────────────────────────────────────────────

// HandleCancelablePaymentForTRON handles CancelablePaymentReady event for TRON.
// TRON relay integration will be implemented in a future sprint;
// for now this logs and returns.
func (s *SettlementService) HandleCancelablePaymentForTRON(event *events.CancelablePaymentReady) {
	logger.LogInfoWithIDf(log, s.nodeID, "Handling TRON CANCELABLE payment ready event for order %s (relay not yet implemented)", event.OrderID)
}

// ── FIAT Auto-Confirm ───────────────────────────────────────────────────

// HandleFiatPaymentReady handles FiatPaymentReady event emitted after a fiat
// payment (Stripe/PayPal) is successfully captured. No on-chain escrow involved,
// so the handler simply emits OrderAutoConfirmRequest to trigger ConfirmOrder.
func (s *SettlementService) HandleFiatPaymentReady(event *events.FiatPaymentReady) {
	logger.LogInfoWithIDf(log, s.nodeID, "Handling fiat payment ready for order %s (provider=%s)", event.OrderID, event.ProviderID)

	unlock := s.TryLockAutoConfirm(event.OrderID)
	if unlock == nil {
		return
	}
	defer unlock()

	s.eventBus.Emit(&events.OrderAutoConfirmRequest{
		OrderID: event.OrderID,
	})

	logger.LogInfoWithIDf(log, s.nodeID, "Emitted OrderAutoConfirmRequest for fiat order %s", event.OrderID)
}

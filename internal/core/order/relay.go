//go:build !private_distribution

package order

import (
	"fmt"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// relayOrDirect is a shared helper for ViaRelay methods.
// If instructions is nil (UTXO), calls directAction directly.
// Otherwise, relays the instructions and calls relayedAction with the resulting txHash.
func (s *OrderAppService) relayOrDirect(
	orderID models.OrderID,
	action string,
	coinType iwallet.CoinType,
	instructions any,
	directAction func() error,
	relayedAction func(txid iwallet.TransactionID) error,
) error {
	// Fiat refund "instructions" are informational payloads for the API layer.
	// The actual provider refund must happen in the direct order action.
	if coinType.IsFiatPayment() {
		return directAction()
	}
	if instructions == nil {
		return directAction()
	}
	if s.escrow == nil {
		return fmt.Errorf("relay service not configured")
	}
	txHash, err := s.escrow.RelayInstructions(orderID.String(), coinType, instructions)
	if err != nil {
		return fmt.Errorf("failed to relay %s transaction: %w", action, err)
	}
	logger.LogInfoWithIDf(log, s.nodeID, "%s transaction relayed for order %s, txHash=%s", action, orderID, txHash)
	return relayedAction(iwallet.TransactionID(txHash))
}

// RefundOrderViaRelay refunds an order using the relay service.
func (s *OrderAppService) RefundOrderViaRelay(orderID models.OrderID, done chan struct{}) error {
	coinType, instructions, err := s.GetRefundOrderInstructions(orderID, "")
	if err != nil {
		return fmt.Errorf("failed to get refund instructions: %w", err)
	}
	return s.relayOrDirect(orderID, "refund", coinType, instructions,
		func() error { return s.RefundOrder(orderID, "", done) },
		func(txid iwallet.TransactionID) error { return s.RefundOrder(orderID, txid, done) },
	)
}

// DeclineOrderViaRelay declines an order using the relay service.
func (s *OrderAppService) DeclineOrderViaRelay(orderID models.OrderID, reason string, done chan struct{}) error {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return err
	}

	funded, err := order.IsFunded()
	if err != nil {
		return err
	}

	if !funded {
		return s.DeclineOrder(orderID, "", reason, done)
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return err
	}

	if payment.MethodIsCancelable(payment.ResolvedPaymentMethod(&order, paymentSent)) {
		return s.DeclineOrder(orderID, "", reason, done)
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return err
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil {
		return err
	}

	if coinInfo.Chain.IsUTXOChain() {
		return s.DeclineOrder(orderID, "", reason, done)
	}

	_, instructions, err := s.GetRefundOrderInstructions(orderID, "")
	if err != nil {
		return fmt.Errorf("failed to get refund instructions for decline: %w", err)
	}
	return s.relayOrDirect(orderID, "decline", coinType, instructions,
		func() error { return s.DeclineOrder(orderID, "", reason, done) },
		func(txid iwallet.TransactionID) error { return s.DeclineOrder(orderID, txid, reason, done) },
	)
}

// CancelOrderViaRelay cancels a CANCELABLE order using the relay service.
func (s *OrderAppService) CancelOrderViaRelay(orderID models.OrderID, done chan struct{}) error {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return err
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return err
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return err
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil {
		return err
	}

	if coinInfo.Chain.IsUTXOChain() {
		return s.CancelOrder(orderID, "", done)
	}

	_, instructions, err := s.GetEscrowReleaseInstructions(orderID, "", paymentSent.PayerAddress)
	if err != nil {
		return fmt.Errorf("failed to get cancel instructions: %w", err)
	}
	return s.relayOrDirect(orderID, "cancel", coinType, instructions,
		func() error { return s.CancelOrder(orderID, "", done) },
		func(txid iwallet.TransactionID) error { return s.CancelOrder(orderID, txid, done) },
	)
}

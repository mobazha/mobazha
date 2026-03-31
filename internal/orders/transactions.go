package orders

import (
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// RecordOutgoingTransaction records a payment coming out of an order's payment address.
// Called by the orchestration layer (OrderAppService) after ProcessMessage to persist
// chain transaction data that the handler no longer fetches.
func (op *OrderProcessor) RecordOutgoingTransaction(dbtx database.Tx, order *models.Order, tx iwallet.Transaction) error {
	err := order.PutTransaction(tx)
	if models.IsDuplicateTransactionError(err) {
		logger.LogInfoWithIDf(log, op.nodeID, "Received duplicate transaction %s", tx.ID.String())
		return nil
	}
	dbtx.RegisterCommitHook(func() {
		op.bus.Emit(&events.SpendFromPaymentAddress{Transaction: tx})
	})
	return err
}

// ProcessOrderPayment processes a payment for an order (called from API for EVM payments)
func (op *OrderProcessor) ProcessOrderPayment(dbtx database.Tx, order *models.Order, orderMessage *npb.OrderMessage, tx iwallet.Transaction) error {
	// Record the transaction
	err := order.PutTransaction(tx)
	if models.IsDuplicateTransactionError(err) {
		logger.LogInfoWithIDf(log, op.nodeID, "Received duplicate transaction %s", tx.ID.String())
		// Continue to process message even if transaction is duplicate
		// This handles the case where transaction was recorded but message wasn't processed
		// (e.g., node crashed between recording transaction and processing message for UTXO)
	} else if err != nil {
		return err
	}

	// Process the order message
	_, err = op.processMessage(dbtx, order, orderMessage)
	if err != nil {
		return err
	}

	// If PAYMENT_SENT handling already marked payment as verified
	// (fiat pre-verified, or crypto tx already confirmed in local tx set),
	// promote to PENDING now so confirm/auto-confirm is not blocked by an
	// intermediate stale AWAITING_PAYMENT_VERIFICATION state.
	paymentSent := new(pb.PaymentSent)
	if unmarshalErr := orderMessage.Message.UnmarshalTo(paymentSent); unmarshalErr == nil {
		if order.IsPaymentVerified() {
			op.advanceToPendingAfterVerification(order)
		}
	}

	return nil
}

package orders

import (
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// processOutgoingPayment processes payments coming out of an order's payment address.
func (op *OrderProcessor) processOutgoingPayment(dbtx database.Tx, order *models.Order, tx iwallet.Transaction) error {
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
		return nil
	} else if err != nil {
		return err
	}

	// Process the order message
	_, err = op.processMessage(dbtx, order, op.identity, orderMessage)
	return err
}

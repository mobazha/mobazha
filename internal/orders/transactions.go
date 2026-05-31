package orders

import (
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
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

// ProcessOrderPayment records the transaction and processes the PAYMENT_SENT
// message. It does NOT perform payment verification (wallet/chain I/O) — that
// responsibility belongs to the orchestration layer which calls
// RecordVerifiedPayment after a successful FetchAndVerify.
func (op *OrderProcessor) ProcessOrderPayment(dbtx database.Tx, order *models.Order, orderMessage *npb.OrderMessage, tx iwallet.Transaction) error {
	normalizeAwaitingPaymentBeforePaymentSent(order)

	err := order.PutTransaction(tx)
	if models.IsDuplicateTransactionError(err) {
		logger.LogInfoWithIDf(log, op.nodeID, "Received duplicate transaction %s", tx.ID.String())
	} else if err != nil {
		return err
	}

	_, err = op.processMessage(dbtx, order, orderMessage)
	return err
}

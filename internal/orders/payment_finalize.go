package orders

import (
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

func (op *OrderProcessor) processPaymentFinalizeMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	paymentFinalized := new(pb.PaymentFinalized)
	if err := message.Message.UnmarshalTo(paymentFinalized); err != nil {
		return nil, err
	}
	dup, err := isDuplicate(paymentFinalized, order.SerializedPaymentFinalized)
	if err != nil {
		return nil, err
	}
	if order.SerializedPaymentFinalized != nil && !dup {
		logger.LogInfoWithIDf(log, op.nodeID, "Duplicate PAYMENT_FINALIZE message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	if order.SerializedDisputeClosed != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received PAYMENT_FINALIZE message for order %s after DISPUTE_CLOSE", order.ID)
		return nil, ErrUnexpectedMessage
	}

	event := &events.VendorFinalizedPayment{
		OrderID: order.ID.String(),
	}

	if op.identity.String() == message.SenderPeerID {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own PAYMENT_FINALIZE for orderID: %s", order.ID)
	} else {
		logger.LogInfoWithIDf(log, op.nodeID, "Received PAYMENT_FINALIZE message for order %s", order.ID)
	}

	order.Open = false

	return event, order.PutMessage(message)
}

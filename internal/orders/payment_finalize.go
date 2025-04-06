package orders

import (
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/events"
	"github.com/mobazha/mobazha3.0/internal/models"
	npb "github.com/mobazha/mobazha3.0/internal/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/internal/orders/mbzpb"
	"github.com/libp2p/go-libp2p/core/peer"
)

func (op *OrderProcessor) processPaymentFinalizeMessage(dbtx database.Tx, order *models.Order, pid peer.ID, message *npb.OrderMessage) (interface{}, error) {
	paymentFinalized := new(pb.PaymentFinalized)
	if err := message.Message.UnmarshalTo(paymentFinalized); err != nil {
		return nil, err
	}
	dup, err := isDuplicate(paymentFinalized, order.SerializedPaymentFinalized)
	if err != nil {
		return nil, err
	}
	if order.SerializedPaymentFinalized != nil && !dup {
		log.Errorf("Duplicate PAYMENT_FINALIZE message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	if order.SerializedDisputeClosed != nil {
		log.Errorf("Received PAYMENT_FINALIZE message for order %s after DISPUTE_CLOSE", order.ID)
		return nil, ErrUnexpectedMessage
	}

	event := &events.VendorFinalizedPayment{
		OrderID: order.ID.String(),
	}

	if op.identity == pid {
		log.Infof("Processed own PAYMENT_FINALIZE for orderID: %s", order.ID)
	} else {
		log.Infof("Received PAYMENT_FINALIZE message for order %s", order.ID)
	}

	return event, order.PutMessage(message)
}

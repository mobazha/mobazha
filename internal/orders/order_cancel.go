package orders

import (
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

func (op *OrderProcessor) processOrderCancelMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	orderCancel := new(pb.OrderCancel)
	if err := message.Message.UnmarshalTo(orderCancel); err != nil {
		return nil, err
	}
	dup, err := isDuplicate(orderCancel, order.SerializedOrderCancel)
	if err != nil {
		return nil, err
	}
	if order.SerializedOrderCancel != nil && !dup {
		logger.LogInfoWithIDf(log, op.nodeID, "Duplicate ORDER_CANCEL message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	if order.SerializedOrderDecline != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Possible race: Received ORDER_CANCEL message for order %s after ORDER_DECLINE", order.ID)
	}

	if order.SerializedOrderConfirmation != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Rejected ORDER_CANCEL message for order %s: already confirmed", order.ID)
		return nil, ErrUnexpectedMessage
	}

	orderOpen, err := order.OrderOpenMessage()
	if models.IsMessageNotExistError(err) {
		if parkErr := order.ParkMessage(message); parkErr != nil {
			return nil, parkErr
		}
		return nil, ErrMessageParked
	}
	if err != nil {
		return nil, err
	}

	paymentSent, err := order.PaymentSentMessage()
	if models.IsMessageNotExistError(err) {
		if parkErr := order.ParkMessage(message); parkErr != nil {
			return nil, parkErr
		}
		return nil, ErrMessageParked
	}
	if err != nil {
		return nil, err
	}

	// Wallet I/O (GetTransaction + processOutgoingPayment) has been moved to the
	// orchestration layer (preProcess/postProcess in OrderAppService).
	_ = paymentSent

	event := &events.OrderCancel{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		BuyerHandle: orderOpen.BuyerID.Handle,
		BuyerID:     orderOpen.BuyerID.PeerID,
	}

	if order.Role() == models.RoleBuyer {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own ORDER_CANCEL for orderID: %s", order.ID)
	} else if order.Role() == models.RoleVendor {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_CANCEL message for order %s", order.ID)
	}

	order.Open = false

	return event, order.PutMessage(message)
}

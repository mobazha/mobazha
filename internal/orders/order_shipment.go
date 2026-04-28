package orders

import (
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

func (op *OrderProcessor) processOrderShipmentMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	shipment := new(pb.OrderShipment)
	if err := message.Message.UnmarshalTo(shipment); err != nil {
		return nil, err
	}

	if order.SerializedOrderDecline != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_SHIPMENT message for order %s after ORDER_DECLINE", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedOrderCancel != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Possible race: Received ORDER_SHIPMENT message for order %s after ORDER_CANCEL", order.ID)
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

	_, err = order.OrderConfirmationMessage()
	if models.IsMessageNotExistError(err) {
		if parkErr := order.ParkMessage(message); parkErr != nil {
			return nil, parkErr
		}
		return nil, ErrMessageParked
	}
	if err != nil {
		return nil, err
	}

	event := &events.OrderShipment{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		VendorName:   orderOpen.Listings[0].Listing.VendorID.DisplayName(),
		VendorAvatar: orderOpen.Listings[0].Listing.VendorID.DisplayAvatar(),
		VendorID:     orderOpen.Listings[0].Listing.VendorID.PeerID,
	}

	if order.Role() == models.RoleBuyer {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_SHIPMENT message for order %s", order.ID)
	} else if order.Role() == models.RoleVendor {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own ORDER_SHIPMENT for order %s", order.ID)
	}

	if err := order.PutMessage(message); err != nil {
		return nil, err
	}

	if order.ShippedAt == nil {
		if shipped, sErr := order.IsShipped(); sErr == nil && shipped {
			now := time.Now()
			order.ShippedAt = &now
		}
	}

	return event, nil
}

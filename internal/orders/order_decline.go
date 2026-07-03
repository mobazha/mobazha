package orders

import (
	"strings"

	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

func (op *OrderProcessor) processOrderDeclineMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	orderDecline := new(pb.OrderDecline)
	if err := message.Message.UnmarshalTo(orderDecline); err != nil {
		return nil, err
	}
	dup, err := isDuplicate(orderDecline, order.SerializedOrderDecline)
	if err != nil {
		return nil, err
	}
	if order.SerializedOrderDecline != nil && !dup {
		logger.LogInfoWithIDf(log, op.nodeID, "Duplicate ORDER_DECLINE message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	confirmedDeclineRefund := order.SerializedOrderConfirmation != nil &&
		strings.TrimSpace(orderDecline.TransactionID) != "" &&
		order.SerializedOrderShipments == nil &&
		order.SerializedOrderComplete == nil &&
		order.SerializedDisputeOpen == nil &&
		order.SerializedDisputeUpdate == nil &&
		order.SerializedDisputeClosed == nil &&
		order.SerializedRefunds == nil &&
		order.SerializedPaymentFinalized == nil
	// Confirmed but unfulfilled monitored orders may still receive a seller
	// decline that carries the already-submitted refund txid.
	if order.SerializedOrderConfirmation != nil && !confirmedDeclineRefund {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_DECLINE message for order %s after ORDER_CONFIRMATION", order.ID)
		return nil, ErrUnexpectedMessage
	}

	// FSM-covered: if the order is in CANCELED state, the FSM rejects EventVendorDecline.
	if order.SerializedOrderCancel != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Possible race: Received ORDER_DECLINE message for order %s after ORDER_CANCEL", order.ID)
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

	unfunded := order.State == models.OrderState_AWAITING_PAYMENT

	if !unfunded {
		_, err = order.PaymentSentMessage()
		if models.IsMessageNotExistError(err) {
			if parkErr := order.ParkMessage(message); parkErr != nil {
				return nil, parkErr
			}
			return nil, ErrMessageParked
		}
		if err != nil {
			return nil, err
		}
	}

	event := &events.OrderDeclined{
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
		if unfunded {
			logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_DECLINE for unfunded order %s (AWAITING_PAYMENT)", order.ID)
		} else {
			logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_DECLINE message for order %s", order.ID)
			// Fiat refund and UTXO CANCELABLE escrow release have been moved to
			// preProcessOrderDecline in the orchestration layer (OrderAppService).
		}
	} else if order.Role() == models.RoleVendor {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own ORDER_DECLINE for orderID: %s", order.ID)
	}

	order.Open = false

	return event, order.PutMessage(message)
}

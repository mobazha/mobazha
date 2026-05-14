package orders

import (
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

func (op *OrderProcessor) processOrderConfirmationMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	orderConfirmation := new(pb.OrderConfirmation)
	if err := message.Message.UnmarshalTo(orderConfirmation); err != nil {
		return nil, err
	}
	dup, err := isDuplicate(orderConfirmation, order.SerializedOrderConfirmation)
	if err != nil {
		return nil, err
	}
	if order.SerializedOrderConfirmation != nil && !dup {
		logger.LogInfoWithIDf(log, op.nodeID, "Duplicate ORDER_CONFIRMATION message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	// FSM-covered: if the order is in DECLINED state, the FSM rejects EventVendorConfirm.
	if order.SerializedOrderDecline != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_CONFIRMATION message for order %s after ORDER_DECLINE", order.ID)
		return nil, ErrUnexpectedMessage
	}

	// FSM-covered: if the order is in CANCELED state, the FSM rejects EventVendorConfirm.
	if order.SerializedOrderCancel != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Possible race: Received ORDER_CONFIRMATION message for order %s after ORDER_CANCEL", order.ID)
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

	// Park if payment has been submitted but not yet chain/provider-verified.
	// The async verification worker will replay parked messages after
	// verification via RecordVerifiedPayment.
	if !order.IsPaymentVerified() {
		if parkErr := order.ParkMessage(message); parkErr != nil {
			return nil, parkErr
		}
		return nil, ErrMessageParked
	}

	// Wallet I/O (GetTransaction, verifyConfirmReceipt, processOutgoingPayment)
	// has been moved to the orchestration layer:
	//   preProcessOrderConfirmation  — fetches chain tx + verifies EVM receipt
	//   postProcessOrderConfirmationInTx — records outgoing tx on the order
	// This keeps the handler deterministic for both inbound and outbound paths.
	_ = paymentSent

	event := &events.OrderConfirmation{
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
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_CONFIRMATION message for order %s", order.ID)
	} else if order.Role() == models.RoleVendor {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own ORDER_CONFIRMATION for order %s", order.ID)
	}

	return event, order.PutMessage(message)
}

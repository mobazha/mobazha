package orders

import (
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

func (op *OrderProcessor) processRefundMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	refund := new(pb.Refund)
	if err := message.Message.UnmarshalTo(refund); err != nil {
		return nil, err
	}

	if order.SerializedOrderCancel != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received REFUND message for order %s after ORDER_CANCEL", order.ID)
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

	order.Open = false

	if err := order.PutMessage(message); err != nil {
		if models.IsDuplicateTransactionError(err) {
			return nil, nil
		}
		return nil, err
	}

	// Wallet I/O (GetTransaction + processOutgoingPayment for DIRECT, and
	// releaseRefundEscrowFunds for MODERATED) has been moved to the orchestration
	// layer (preProcessRefund / postProcessRefundInTx in OrderAppService).
	_ = paymentSent

	if order.Role() == models.RoleBuyer {
		logger.LogInfoWithIDf(log, op.nodeID, "Received REFUND message for order %s", order.ID)
	} else if order.Role() == models.RoleVendor {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own REFUND for order %s", order.ID)
	}

	event := &events.Refund{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		VendorHandle: orderOpen.Listings[0].Listing.VendorID.Handle,
		VendorID:     orderOpen.Listings[0].Listing.VendorID.PeerID,
	}
	return event, nil
}

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

	if order.SerializedOrderReject != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_CONFIRMATION message for order %s after ORDER_REJECT", order.ID)
		return nil, ErrUnexpectedMessage
	}

	if order.SerializedOrderCancel != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Possible race: Received ORDER_CONFIRMATION message for order %s after ORDER_CANCEL", order.ID)
	}

	orderOpen, err := order.OrderOpenMessage()
	if models.IsMessageNotExistError(err) {
		return nil, order.ParkMessage(message)
	}
	if err != nil {
		return nil, err
	}

	paymentSent, err := order.PaymentSentMessage()
	if models.IsMessageNotExistError(err) {
		return nil, order.ParkMessage(message)
	}
	if err != nil {
		return nil, err
	}

	wallet, err := op.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		return nil, err
	}

	if orderConfirmation.TransactionID != "" && paymentSent.Method == pb.PaymentSent_CANCELABLE {
		// If this fails it's OK as the processor's unfunded order checking loop will
		// retry at it's next interval.
		tx, err := wallet.GetTransaction(iwallet.TransactionID(orderConfirmation.TransactionID), iwallet.CoinType(paymentSent.Coin))
		if err == nil && tx != nil {
			for _, from := range tx.From {
				if from.Address.String() == order.PaymentAddress {
					if err := op.processOutgoingPayment(dbtx, order, *tx); err != nil {
						return nil, err
					}
				}
			}
		}
	}

	event := &events.OrderConfirmation{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		VendorHandle: orderOpen.Listings[0].Listing.VendorID.Handle,
		VendorID:     orderOpen.Listings[0].Listing.VendorID.PeerID,
	}

	if order.Role() == models.RoleBuyer {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_CONFIRMATION message for order %s", order.ID)
	} else if order.Role() == models.RoleVendor {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own ORDER_CONFIRMATION for order %s", order.ID)
	}

	return event, order.PutMessage(message)
}

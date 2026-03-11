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

	// FSM-covered: if the order is in AWAITING_FULFILLMENT (confirmed), the FSM rejects EventVendorDecline.
	if order.SerializedOrderConfirmation != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_DECLINE message for order %s after ORDER_CONFIRMATION", order.ID)
		return nil, ErrUnexpectedMessage
	}

	// FSM-covered: if the order is in CANCELED state, the FSM rejects EventVendorDecline.
	if order.SerializedOrderCancel != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Possible race: Received ORDER_DECLINE message for order %s after ORDER_CANCEL", order.ID)
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

	event := &events.OrderDeclined{
		OrderID: order.ID.String(),
		Thumbnail: events.Thumbnail{
			Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
			Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
		},
		VendorHandle: orderOpen.Listings[0].Listing.VendorID.Handle,
		VendorID:     orderOpen.Listings[0].Listing.VendorID.PeerID,
	}

	if order.Role() == models.RoleBuyer {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_DECLINE message for order %s", order.ID)

		coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(paymentSent.Coin))
		if err != nil {
			return nil, err
		}

		// For UTXO chains with CANCELABLE method, release funds here
		// For MODERATED orders, the REFUND message handles the escrow release
		// For ETH/Solana, the order is handled on-chain
		if coinInfo.Chain != iwallet.ChainSolana && !coinInfo.IsEthTypeChain() {
			if order.CanCancel() && paymentSent.Method == pb.PaymentSent_CANCELABLE {
				wTx, _, err := op.releaseFromCancelableAddress(dbtx, order)
				if err != nil {
					return nil, err
				}
				wTx.Commit()
			}
			// For MODERATED orders, the accompanying REFUND message will handle the escrow release
		}
	} else if order.Role() == models.RoleVendor {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own ORDER_DECLINE for orderID: %s", order.ID)
	}

	order.Open = false

	return event, order.PutMessage(message)
}

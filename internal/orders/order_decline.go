package orders

import (
	"fmt"

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

	unfunded := order.State == models.OrderState_AWAITING_PAYMENT

	var paymentSent *pb.PaymentSent
	if !unfunded {
		paymentSent, err = order.PaymentSentMessage()
		if models.IsMessageNotExistError(err) {
			return nil, order.ParkMessage(message)
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
		VendorHandle: orderOpen.Listings[0].Listing.VendorID.Handle,
		VendorID:     orderOpen.Listings[0].Listing.VendorID.PeerID,
	}

	if order.Role() == models.RoleBuyer {
		if unfunded {
			logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_DECLINE for unfunded order %s (AWAITING_PAYMENT)", order.ID)
		} else {
			logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_DECLINE message for order %s", order.ID)

			coinType := iwallet.CoinType(paymentSent.Coin)

			if coinType.IsFiatPayment() || coinType.IsStripeChain() {
				if op.fiatRefundOnDeclineFunc != nil {
					providerID := orderOpen.FiatProvider
					if providerID == "" {
						providerID = "stripe"
					}
					if err := op.fiatRefundOnDeclineFunc(
						order.ID.String(),
						paymentSent.TransactionID,
						providerID,
						orderOpen.PricingCoin,
					); err != nil {
						logger.LogErrorWithIDf(log, op.nodeID, "Fiat auto-refund on decline failed for order %s: %v", order.ID, err)
						return nil, fmt.Errorf("fiat refund failed for order %s, decline aborted: %w", order.ID, err)
					}
				}
			} else {
				coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
				if err != nil {
					return nil, err
				}

				if coinInfo.Chain != iwallet.ChainSolana && !coinInfo.IsEthTypeChain() {
					if order.CanCancel() && paymentSent.Method == pb.PaymentSent_CANCELABLE {
						wTx, _, err := op.releaseFromCancelableAddress(dbtx, order)
						if err != nil {
							return nil, err
						}
						wTx.Commit()
					}
				}
			}
		}
	} else if order.Role() == models.RoleVendor {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own ORDER_DECLINE for orderID: %s", order.ID)
	}

	order.Open = false

	return event, order.PutMessage(message)
}

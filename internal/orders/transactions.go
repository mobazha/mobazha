package orders

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/anypb"
	"gorm.io/gorm"
)

// processWalletTransaction scan's through a transaction's inputs and outputs and attempts
// to load the order for that address from the database. If an order is found, the transaction
// is handed off to the appropriate handler for further processing.
func (op *OrderProcessor) processWalletTransaction(transaction iwallet.Transaction) {
	err := op.db.Update(func(tx database.Tx) error {
		for _, to := range transaction.To {
			var order models.Order
			err := tx.Read().Where("payment_address = ?", to.Address.String()).First(&order).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			} else if err != nil {
				return err
			}

			if err := op.processIncomingPayment(tx, &order, transaction); err != nil {
				return err
			}

			if err := tx.Save(&order); err != nil {
				return err
			}
		}
		for _, from := range transaction.From {
			var order models.Order
			err := tx.Read().Where("payment_address = ?", from.Address.String()).First(&order).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			} else if err != nil {
				return err
			}

			if err := op.processOutgoingPayment(tx, &order, transaction); err != nil {
				return err
			}

			if err := tx.Save(&order); err != nil {
				return err
			}

		}
		return nil
	})
	if err != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Error handling incoming order transaction %s: %s", transaction.ID, err)
	}
}

func (op *OrderProcessor) ProcessOrderPayment(dbtx database.Tx, order *models.Order, message *npb.OrderMessage, realTx iwallet.Transaction) error {
	paymentSent := new(pb.PaymentSent)
	if err := message.Message.UnmarshalTo(paymentSent); err != nil {
		return err
	}

	err := order.PutTransaction(realTx)
	if models.IsDuplicateTransactionError(err) {
		logger.LogInfoWithIDf(log, op.nodeID, "Received duplicate transaction %s", realTx.ID.String())
		return nil
	} else if err != nil {
		return err
	}
	order.PaymentAddress = paymentSent.ToAddress

	op.bus.Emit(&events.TransactionReceived{
		Transaction:  realTx,
		CurrencyCode: paymentSent.Coin,
	})

	if order.Role() == models.RoleBuyer {
		if err := order.PutMessage(message); err != nil {
			return err
		}
	}

	funded, err := order.IsFunded()
	if err != nil {
		return err
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return err
	}

	switch order.Role() {
	case models.RoleBuyer:
		payload := &anypb.Any{}
		if err := payload.MarshalFrom(message); err != nil {
			return err
		}

		messageID := make([]byte, 20)
		if _, err := rand.Read(messageID); err != nil {
			return err
		}

		msg := npb.Message{
			MessageType: npb.Message_ORDER,
			MessageID:   hex.EncodeToString(messageID),
			Payload:     payload,
		}

		vendor, err := peer.Decode(orderOpen.Listings[0].Listing.VendorID.PeerID)
		if err != nil {
			return err
		}

		if err := op.messenger.ReliablySendMessage(dbtx, vendor, &msg, nil); err != nil {
			return err
		}

		if funded {
			fundingTotal, err := order.FundingTotal()
			if err != nil {
				return err
			}
			dbtx.RegisterCommitHook(func() {
				op.bus.Emit(&events.OrderPaymentReceived{
					OrderID:      order.ID.String(),
					FundingTotal: fundingTotal.String(),
					CoinType:     paymentSent.Coin,
				})
			})
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected: Order %s fully funded", order.ID)
		} else {
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected: Order %s partially funded", order.ID)
		}

	case models.RoleVendor:
		if funded {
			// TODO: mark vendor inventory downwards is not wasFunded.

			if err := op.sendRatingSignatures(dbtx, order, orderOpen); err != nil {
				logger.LogInfoWithIDf(log, op.nodeID, "Error sending rating signature message: %s", err)
			}

			dbtx.RegisterCommitHook(func() {
				op.bus.Emit(&events.OrderFunded{
					BuyerHandle: orderOpen.BuyerID.Handle,
					BuyerID:     orderOpen.BuyerID.PeerID,
					ListingType: orderOpen.Listings[0].Listing.Metadata.ContractType.String(),
					OrderID:     order.ID.String(),
					Price: events.ListingPrice{
						Amount:        paymentSent.Amount,
						CurrencyCode:  paymentSent.Coin,
						PriceModifier: orderOpen.Listings[0].Listing.Item.CryptoListingPriceModifier,
					},
					Slug: orderOpen.Listings[0].Listing.Slug,
					Thumbnail: events.Thumbnail{
						Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
						Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
					},
					Title: orderOpen.Listings[0].Listing.Item.Title,
				})
			})
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected: Order %s fully funded", order.ID)
		} else {
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected: Order %s partially funded", order.ID)
		}
	}
	return nil
}

// processIncomingPayment processes payments into an order's payment address.
func (op *OrderProcessor) processIncomingPayment(dbtx database.Tx, order *models.Order, tx iwallet.Transaction) error {
	return nil
}

// processOutgoingPayment processes payments coming out of an order's payment address.
func (op *OrderProcessor) processOutgoingPayment(dbtx database.Tx, order *models.Order, tx iwallet.Transaction) error {
	err := order.PutTransaction(tx)
	if models.IsDuplicateTransactionError(err) {
		logger.LogInfoWithIDf(log, op.nodeID, "Received duplicate transaction %s", tx.ID.String())
		return nil
	}
	dbtx.RegisterCommitHook(func() {
		op.bus.Emit(&events.SpendFromPaymentAddress{Transaction: tx})
	})
	return err
}

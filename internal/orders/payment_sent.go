package orders

import (
	"fmt"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func (op *OrderProcessor) processPaymentSentMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	paymentSent := new(pb.PaymentSent)
	if err := message.Message.UnmarshalTo(paymentSent); err != nil {
		return nil, err
	}

	dup, err := isDuplicate(paymentSent, order.SerializedPaymentSent)
	if err != nil {
		return nil, err
	}
	if order.SerializedPaymentSent != nil && !dup {
		logger.LogInfoWithIDf(log, op.nodeID, "Duplicate PAYMENT_SENT message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	orderOpen, err := order.OrderOpenMessage()
	if models.IsMessageNotExistError(err) {
		return nil, order.ParkMessage(message)
	}
	if err != nil {
		return nil, err
	}

	wallet, err := op.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		return nil, fmt.Errorf("cannot validate paymentSent. coin not supported. %w", err)
	}

	if err := utils.ValidatePayment(orderOpen, paymentSent, paymentSent.EscrowTimeoutHours, wallet); err != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Failed to validate payment sent message: %s", err)
		return nil, err
	}

	// Set payment address from PaymentSent message for future reference
	// This is needed by refund, cancel, and other operations that need to match addresses
	order.PaymentAddress = paymentSent.ToAddress

	err = order.PutMessage(message)
	if models.IsDuplicateTransactionError(err) {
		return nil, nil
	}

	txs, err := order.GetTransactions()
	if err != nil && !models.IsMessageNotExistError(err) {
		return nil, err
	}

	for _, tx := range txs {
		if tx.ID.String() == paymentSent.TransactionID {
			logger.LogInfoWithIDf(log, op.nodeID, "Received PAYMENT_SENT message for order %s but already know about transaction", order.ID)
			return nil, nil
		}
	}

	// If this fails it's OK as the processor's unfunded order checking loop will
	// retry at it's next interval.
	var tx *iwallet.Transaction
	if iwallet.CoinType(paymentSent.Coin).IsStripeChain() {
		tx, err = op.getStripeTransactionFunc(iwallet.TransactionID(paymentSent.TransactionID), iwallet.CoinType(paymentSent.Coin))
	} else {
		tx, err = wallet.GetTransaction(iwallet.TransactionID(paymentSent.TransactionID), iwallet.CoinType(paymentSent.Coin))
		if err != nil {
			logger.LogErrorWithIDf(log, op.nodeID, "Failed to get transaction from txid: %s, error: %s", paymentSent.TransactionID, err)

			logger.LogInfoWithIDf(log, op.nodeID, "building transaction from payment sent message, txid: %s", paymentSent.TransactionID)
			tx = utils.BuildPaymentSentTransaction(paymentSent)
			err = nil
		}
	}
	if err == nil && tx != nil {
		paymentAddress, err := order.GetPaymentAddress()
		if err != nil {
			return nil, err
		}
		for _, to := range tx.To {
			if to.Address.String() == paymentAddress {
				if err := op.ProcessOrderPayment(dbtx, order, message, *tx); err != nil {
					return nil, err
				}
			}
		}
	} else {
		logger.LogErrorWithIDf(log, op.nodeID, "Failed to get transaction from id: %s, error: %s", paymentSent.TransactionID, err)
	}

	// Check if order is funded and handle role-specific logic
	funded, _ := order.IsFunded()

	switch order.Role() {
	case models.RoleBuyer:
		// Buyer: emit OrderPaymentReceived event
		if funded {
			fundingTotal, err := order.FundingTotal()
			if err == nil {
				dbtx.RegisterCommitHook(func() {
					op.bus.Emit(&events.OrderPaymentReceived{
						OrderID:      order.ID.String(),
						FundingTotal: fundingTotal.String(),
						CoinType:     orderOpen.PricingCoin,
					})
				})
			}
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected: Order %s fully funded", order.ID)
		}

	case models.RoleVendor:
		// Vendor: send rating signatures and emit OrderFunded event
		if funded {
			if err := op.sendRatingSignatures(dbtx, order, orderOpen); err != nil {
				logger.LogInfoWithIDf(log, op.nodeID, "Error sending rating signatures: %s", err)
			}

			dbtx.RegisterCommitHook(func() {
				op.bus.Emit(&events.OrderFunded{
					BuyerHandle: orderOpen.BuyerID.Handle,
					BuyerID:     orderOpen.BuyerID.PeerID,
					ListingType: orderOpen.Listings[0].Listing.Metadata.ContractType.String(),
					OrderID:     order.ID.String(),
					Price: events.ListingPrice{
						Amount:        orderOpen.Amount,
						CurrencyCode:  orderOpen.PricingCoin,
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
		}

		// For CANCELABLE UTXO payments, emit event to trigger auto-confirm in core layer
		// This decouples message processing (orders layer) from wallet operations (core layer)
		if paymentSent.Method == pb.PaymentSent_CANCELABLE {
			coinType := iwallet.CoinType(paymentSent.Coin)
			if coinInfo, err := coinType.CoinInfo(); err == nil && coinInfo.Chain.IsUTXOChain() {
				// Parse amount from string
				var amount uint64
				if tx != nil {
					amount = uint64(tx.Value.Int64())
				}
				dbtx.RegisterCommitHook(func() {
					op.bus.Emit(&events.UTXOCancelablePaymentReady{
						OrderID:       order.ID.String(),
						TransactionID: paymentSent.TransactionID,
						Coin:          paymentSent.Coin,
						Amount:        amount,
					})
				})
				logger.LogInfoWithIDf(log, op.nodeID, "CANCELABLE UTXO payment ready for auto-confirm: order %s", order.ID)
			}
		}
	}

	logger.LogInfoWithIDf(log, op.nodeID, "Received PAYMENT_SENT message for order %s", order.ID)

	event := &events.PaymentSentReceived{
		OrderID: order.ID.String(),
		Txid:    paymentSent.TransactionID,
	}
	return event, nil
}

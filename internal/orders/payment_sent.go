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

	coinType := iwallet.CoinType(paymentSent.Coin)

	if err := op.validatePaymentSent(coinType, orderOpen, paymentSent); err != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Failed to validate payment sent message: %s", err)
		return nil, err
	}

	order.PaymentAddress = paymentSent.ToAddress

	err = order.PutMessage(message)
	if models.IsDuplicateTransactionError(err) {
		return nil, nil
	}

	txs, err := order.GetTransactions()
	if err != nil && !models.IsMessageNotExistError(err) {
		return nil, err
	}

	transactionKnown := false
	for _, tx := range txs {
		if tx.ID.String() == paymentSent.TransactionID {
			logger.LogInfoWithIDf(log, op.nodeID, "Received PAYMENT_SENT message for order %s but already know about transaction", order.ID)
			transactionKnown = true
			break
		}
	}

	if transactionKnown {
		order.PaymentVerified = true
	}
	// Sync verification (FetchAndVerify) is now handled by the orchestration
	// layer: preProcessPaymentSent + postProcessPaymentSentInTx. When the tx
	// is not yet on-chain, the async verification loop retries later.

	op.emitPaymentSentEvents(dbtx, order, orderOpen, paymentSent, nil)

	logger.LogInfoWithIDf(log, op.nodeID, "Received PAYMENT_SENT message for order %s", order.ID)

	return &events.PaymentSentReceived{
		OrderID: order.ID.String(),
		Txid:    paymentSent.TransactionID,
	}, nil
}

// validatePaymentSent validates a PaymentSent message against the OrderOpen.
// Primary validation is now done by PVS in preProcessPaymentSent; when multiwallet
// is nil (pure mode), this is a no-op since the orchestration layer has already validated.
func (op *OrderProcessor) validatePaymentSent(coinType iwallet.CoinType, orderOpen *pb.OrderOpen, paymentSent *pb.PaymentSent) error {
	if op.multiwallet == nil {
		return nil
	}
	wallet, err := op.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		return fmt.Errorf("cannot validate paymentSent. coin not supported. %w", err)
	}
	return utils.ValidatePayment(orderOpen, paymentSent, paymentSent.EscrowTimeoutHours, wallet)
}

// emitPaymentSentEvents registers commit hooks for payment-related events.
func (op *OrderProcessor) emitPaymentSentEvents(
	dbtx database.Tx,
	order *models.Order,
	orderOpen *pb.OrderOpen,
	paymentSent *pb.PaymentSent,
	verifiedTx *iwallet.Transaction,
) {
	funded, _ := order.IsFunded()

	switch order.Role() {
	case models.RoleBuyer:
		if funded {
			fundingTotal, err := order.FundingTotal()
			if err == nil {
				dbtx.RegisterCommitHook(func() {
					op.bus.Emit(&events.OrderPaymentReceived{
						OrderID:      order.ID.String(),
						FundingTotal: fundingTotal.String(),
						CoinType:     paymentSent.Coin,
					})
				})
			}
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected: Order %s fully funded", order.ID)
		}

	case models.RoleVendor:
		if funded && order.PaymentVerified {
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
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected and chain-verified: Order %s fully funded", order.ID)
		}

		if paymentSent.Method == pb.PaymentSent_CANCELABLE && order.PaymentVerified {
			var amount uint64
			if verifiedTx != nil {
				amount = uint64(verifiedTx.Value.Int64())
			}
			dbtx.RegisterCommitHook(func() {
				op.bus.Emit(&events.CancelablePaymentReady{
					OrderID:       order.ID.String(),
					TransactionID: paymentSent.TransactionID,
					Coin:          paymentSent.Coin,
					Amount:        amount,
				})
			})
			logger.LogInfoWithIDf(log, op.nodeID, "CANCELABLE payment chain-verified, ready for auto-confirm: order %s (coin=%s)", order.ID, paymentSent.Coin)
		}

		if paymentSent.Method == pb.PaymentSent_RWA_INSTANT && order.PaymentVerified {
			dbtx.RegisterCommitHook(func() {
				op.bus.Emit(&events.RwaInstantBuyCompleted{
					OrderID:       order.ID.String(),
					TransactionID: paymentSent.TransactionID,
					Coin:          paymentSent.Coin,
				})
			})
			logger.LogInfoWithIDf(log, op.nodeID, "RWA instant buy chain-verified, ready for auto-confirm: order %s", order.ID)
		}

		if !order.PaymentVerified {
			logger.LogInfoWithIDf(log, op.nodeID, "Order %s: PaymentSent received but not yet chain-verified, financial events deferred", order.ID)
		}
	}
}

// RecordVerifiedPayment records a pre-verified payment transaction into the
// order. Called by postProcessInTx (sync path) and the async verification loop
// after FetchAndVerify succeeds. Pure DB + event emission — no network I/O.
//
// Unlike ProcessOrderPayment, this does NOT call processMessage (which would
// panic on nil message). Instead it directly puts the transaction and emits events.
// The caller must ensure this runs within a DB transaction.
func (op *OrderProcessor) RecordVerifiedPayment(
	dbtx database.Tx,
	order *models.Order,
	tx iwallet.Transaction,
) error {
	if err := order.PutTransaction(tx); err != nil {
		if !models.IsDuplicateTransactionError(err) {
			return err
		}
	}
	order.PaymentVerified = true

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return nil
	}

	op.emitPaymentSentEvents(dbtx, order, orderOpen, paymentSent, &tx)
	return dbtx.Save(order)
}

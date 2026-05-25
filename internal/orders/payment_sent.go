package orders

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	coreorders "github.com/mobazha/mobazha3.0/pkg/orders"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
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
		if parkErr := order.ParkMessage(message); parkErr != nil {
			return nil, parkErr
		}
		return nil, ErrMessageParked
	}
	if err != nil {
		return nil, err
	}

	coinType := iwallet.CoinType(paymentSent.Coin)
	if err := coinType.ValidateCanonicalPaymentCoin(); err != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Invalid payment coin: %v", err)
		return nil, err
	}

	if err := op.validatePaymentSent(coinType, orderOpen, paymentSent); err != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Failed to validate payment sent message: %s", err)
		return nil, err
	}

	order.PaymentAddress = paymentSent.ToAddress
	if paymentSent.CancelFeeAmount != "" {
		order.CancelFeeAmount = paymentSent.CancelFeeAmount
	}
	order.MarkPaymentVerificationPending()

	err = order.PutMessage(message)
	if models.IsDuplicateTransactionError(err) {
		return nil, nil
	}

	txs, err := order.GetTransactions()
	if err != nil && !models.IsMessageNotExistError(err) {
		return nil, err
	}

	transactionKnown := false
	var knownTx *iwallet.Transaction
	for i := range txs {
		tx := &txs[i]
		if tx.ID.String() == paymentSent.TransactionID {
			logger.LogInfoWithIDf(log, op.nodeID, "Received PAYMENT_SENT message for order %s but already know about transaction", order.ID)
			transactionKnown = true
			knownTx = tx
			break
		}
	}

	if transactionKnown {
		method, methodOK := payment.ResolvedPaymentMethod(order, paymentSent)
		preVerifiedFiat := methodOK && payment.IsFiatPaymentRoute(method, coinType)
		// For crypto, a transaction already persisted on this order with
		// block height means local on-chain verification has already happened.
		knownConfirmedCrypto := !preVerifiedFiat && isKnownTxConfirmed(knownTx)
		if preVerifiedFiat || knownConfirmedCrypto {
			order.MarkPaymentVerified()
			// Keep FSM and verification gate semantically aligned:
			// once payment is verified, order should leave
			// AWAITING_PAYMENT_VERIFICATION immediately.
			op.advanceToPendingAfterVerification(order)
			if order.PaidAt == nil {
				now := time.Now()
				order.PaidAt = &now
			}
		}
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
	specProto := paymentSent.GetSettlementSpec()
	if specProto != nil && specProto.GetEscrowType() == string(payment.EscrowTypeManagedEscrow) {
		return nil
	}
	if op.multiwallet == nil {
		return nil
	}
	if specProto == nil {
		return fmt.Errorf("payment_sent missing settlement spec")
	}
	if payment.IsFiatPaymentRoute(specProto.GetMethod(), coinType) {
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
						TenantID:     order.TenantID,
						OrderID:      order.ID.String(),
						FundingTotal: fundingTotal.String(),
						CoinType:     paymentSent.Coin,
					})
				})
			}
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected: Order %s fully funded", order.ID)
		}

	case models.RoleVendor:
		if funded && order.IsPaymentVerified() {
			if err := op.EnsureRatingSignatures(dbtx, order, orderOpen); err != nil {
				logger.LogInfoWithIDf(log, op.nodeID, "Error sending rating signatures: %s", err)
			}

			dbtx.RegisterCommitHook(func() {
				listing := orderOpen.Listings[0].Listing
				var thumb events.Thumbnail
				if len(listing.Item.Images) > 0 {
					thumb = events.Thumbnail{
						Tiny:  listing.Item.Images[0].Tiny,
						Small: listing.Item.Images[0].Small,
					}
				}
				op.bus.Emit(&events.OrderFunded{
					TenantID:    order.TenantID,
					BuyerName:   orderOpen.BuyerID.DisplayName(),
					BuyerAvatar: orderOpen.BuyerID.DisplayAvatar(),
					BuyerID:     orderOpen.BuyerID.PeerID,
					ListingType: listing.Metadata.ContractType.String(),
					OrderID:     order.ID.String(),
					Price: events.ListingPrice{
						Amount:        orderOpen.Amount,
						CurrencyCode:  orderOpen.PricingCoin,
						PriceModifier: listing.Item.CryptoListingPriceModifier,
					},
					Slug:      listing.Slug,
					Thumbnail: thumb,
					Title:     listing.Item.Title,
				})
			})
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected and chain-verified: Order %s fully funded", order.ID)
		}

		if method, ok := payment.ResolvedPaymentMethod(order, paymentSent); ok && payment.MethodIsCancelable(method) && order.IsPaymentVerified() {
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

		if paymentSent.GetSettlementSpec() != nil && paymentSent.GetSettlementSpec().GetMethod() == pb.PaymentSent_RWA_INSTANT && order.IsPaymentVerified() {
			dbtx.RegisterCommitHook(func() {
				op.bus.Emit(&events.RwaInstantBuyCompleted{
					OrderID:       order.ID.String(),
					TransactionID: paymentSent.TransactionID,
					Coin:          paymentSent.Coin,
				})
			})
			logger.LogInfoWithIDf(log, op.nodeID, "RWA instant buy chain-verified, ready for auto-confirm: order %s", order.ID)
		}

		if !order.IsPaymentVerified() {
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
	order.MarkPaymentVerified()
	if order.PaidAt == nil {
		now := time.Now()
		order.PaidAt = &now
	}
	op.advanceToPendingAfterVerification(order)

	// Replay parked messages that were waiting for payment verification.
	// ORDER_CONFIRMATION (and potentially other messages) may have arrived
	// while the order was at AWAITING_PAYMENT_VERIFICATION and been parked.
	// Now that we've advanced to PENDING, they can be processed.
	op.replayParkedMessages(dbtx, order)

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

func (op *OrderProcessor) advanceToPendingAfterVerification(order *models.Order) {
	if op.stateValidator != nil {
		currentState := coreorders.OrderState(order.State)
		if newState, valid := op.stateValidator.ValidateTransition(
			int(currentState), int(coreorders.EventPaymentVerified),
		); valid {
			order.SetFSMState(models.OrderState(newState))
			return
		}
	}

	switch order.State {
	case models.OrderState_AWAITING_PAYMENT,
		models.OrderState_AWAITING_PAYMENT_VERIFICATION,
		models.OrderState_PROCESSING_ERROR:
		order.SetFSMState(models.OrderState_PENDING)
	}
}

// replayParkedMessages processes any messages that were parked while the
// order was in a state that couldn't accept them (e.g., ORDER_CONFIRMATION
// arriving at AWAITING_PAYMENT_VERIFICATION). Follows the same pattern as
// ProcessMessage's post-handler replay loop.
func (op *OrderProcessor) replayParkedMessages(dbtx database.Tx, order *models.Order) {
	parkedMsgs, err := order.GetParkedMessages()
	if err != nil || len(parkedMsgs.Messages) == 0 {
		return
	}

	sort.Slice(parkedMsgs.Messages, func(i, j int) bool {
		return parkedMsgs.Messages[i].MessageType < parkedMsgs.Messages[j].MessageType
	})

	for _, parked := range parkedMsgs.Messages {
		_, replayErr := op.processMessage(dbtx, order, parked)
		if errors.Is(replayErr, ErrMessageParked) {
			continue
		}
		if replayErr != nil {
			logger.LogInfoWithIDf(log, op.nodeID,
				"Error replaying parked message for order %s (type=%s): %s",
				order.ID, parked.MessageType, replayErr)
		} else {
			_ = order.DeleteParkedMessage(parked.MessageType)
		}
	}
}

func isKnownTxConfirmed(tx *iwallet.Transaction) bool {
	if tx == nil {
		return false
	}
	if tx.Height > 0 {
		return true
	}
	return tx.BlockInfo != nil && tx.BlockInfo.Height > 0
}

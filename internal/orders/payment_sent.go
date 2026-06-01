package orders

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
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
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (op *OrderProcessor) processPaymentSentMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	paymentSent := new(pb.PaymentSent)
	if err := message.Message.UnmarshalTo(paymentSent); err != nil {
		return nil, err
	}

	dup, _, err := isDuplicatePaymentSent(paymentSent, order.SerializedPaymentSent)
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

	knownTx := knownTransactionForPaymentSent(txs, paymentSent)
	var verifiedTxForEvents *iwallet.Transaction
	if knownTx != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Received PAYMENT_SENT message for order %s but already know about transaction", order.ID)
		method, methodOK := payment.ResolvedPaymentMethod(order, paymentSent)
		knownVerified := methodOK && isKnownTxVerifiedForRoute(paymentSent, method, coinType, knownTx)
		if knownVerified {
			order.MarkPaymentVerified()
			verifiedTxForEvents = knownTx
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

	op.emitPaymentSentEvents(dbtx, order, orderOpen, paymentSent, verifiedTxForEvents)

	logger.LogInfoWithIDf(log, op.nodeID, "Received PAYMENT_SENT message for order %s", order.ID)

	return &events.PaymentSentReceived{
		OrderID: order.ID.String(),
		Txid:    paymentSent.TransactionID,
	}, nil
}

func isDuplicatePaymentSent(incoming *pb.PaymentSent, serialized []byte) (bool, *pb.PaymentSent, error) {
	if len(serialized) == 0 {
		return false, nil, nil
	}
	dup, err := isDuplicate(incoming, serialized)
	if err != nil || dup {
		return dup, nil, err
	}
	persisted := new(pb.PaymentSent)
	if err := protojson.Unmarshal(serialized, persisted); err != nil {
		return false, nil, err
	}
	if !isCompatiblePaymentSentDuplicate(incoming, persisted) {
		return false, nil, nil
	}
	return true, persisted, nil
}

func isCompatiblePaymentSentDuplicate(incoming, persisted *pb.PaymentSent) bool {
	if incoming == nil || persisted == nil {
		return false
	}
	if incoming.Coin != persisted.Coin ||
		incoming.ToAddress != persisted.ToAddress ||
		incoming.ContractAddress != persisted.ContractAddress ||
		incoming.Script != persisted.Script ||
		incoming.Chaincode != persisted.Chaincode ||
		incoming.Moderator != persisted.Moderator ||
		incoming.ModeratorAddress != persisted.ModeratorAddress ||
		incoming.PaymentTokenAddress != persisted.PaymentTokenAddress ||
		incoming.BuyerReceiveAddress != persisted.BuyerReceiveAddress ||
		incoming.CancelFeeAmount != persisted.CancelFeeAmount ||
		incoming.PlatformAmount != persisted.PlatformAmount ||
		incoming.PlatformAddr != persisted.PlatformAddr ||
		incoming.EscrowReleaseFee != persisted.EscrowReleaseFee ||
		incoming.EscrowTimeoutHours != persisted.EscrowTimeoutHours {
		return false
	}
	if incoming.RefundAddress != "" && persisted.RefundAddress != "" && incoming.RefundAddress != persisted.RefundAddress {
		return false
	}
	if incoming.ConfirmationPolicy != "" && persisted.ConfirmationPolicy != "" &&
		models.NormalizePaymentConfirmationPolicy(incoming.ConfirmationPolicy) != models.NormalizePaymentConfirmationPolicy(persisted.ConfirmationPolicy) {
		return false
	}
	if !proto.Equal(incoming.GetPaymentMethod(), persisted.GetPaymentMethod()) ||
		!proto.Equal(incoming.GetSettlementSpec(), persisted.GetSettlementSpec()) {
		return false
	}
	if incoming.TransactionID != "" && incoming.TransactionID == persisted.TransactionID {
		return incoming.Amount == persisted.Amount || incoming.Amount == "" || persisted.Amount == ""
	}
	if paymentSentContainsFundingFact(persisted, incoming.TransactionID, incoming.Amount) {
		return true
	}
	if paymentSentContainsFundingFact(incoming, persisted.TransactionID, persisted.Amount) {
		return true
	}
	return false
}

func paymentSentContainsFundingFact(ps *pb.PaymentSent, txHash, amount string) bool {
	if ps == nil || txHash == "" {
		return false
	}
	for _, fact := range ps.GetFundingFacts() {
		if fact.GetTxHash() != txHash {
			continue
		}
		return amount == "" || fact.GetAmount() == "" || fact.GetAmount() == amount
	}
	return false
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
			if !markPaymentSettlementSignaled(order) {
				return
			}
			amount := parsePaymentSentEventAmount(paymentSent)
			dbtx.RegisterCommitHook(func() {
				op.bus.Emit(&events.CancelablePaymentReady{
					TenantID:      order.TenantID,
					OrderID:       order.ID.String(),
					TransactionID: paymentSent.TransactionID,
					Coin:          paymentSent.Coin,
					Amount:        amount,
				})
			})
			logger.LogInfoWithIDf(log, op.nodeID, "CANCELABLE payment chain-verified, ready for auto-confirm: order %s (coin=%s)", order.ID, paymentSent.Coin)
		}

		if paymentSent.GetSettlementSpec() != nil && paymentSent.GetSettlementSpec().GetMethod() == pb.PaymentSent_RWA_INSTANT && order.IsPaymentVerified() {
			if !markPaymentSettlementSignaled(order) {
				return
			}
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
	alreadyVerified := order.IsPaymentVerified()
	if err := order.PutTransaction(tx); err != nil {
		if !models.IsDuplicateTransactionError(err) {
			return err
		}
		if err := order.UpdateTransaction(tx); err != nil {
			return err
		}
	}
	if alreadyVerified {
		paymentSent, err := order.PaymentSentMessage()
		if err == nil {
			op.emitVerifiedPaymentSettlementRecovery(dbtx, order, paymentSent)
		} else if !models.IsMessageNotExistError(err) {
			return err
		}
		return dbtx.Save(order)
	}
	order.MarkPaymentVerified()
	if order.PaidAt == nil {
		now := time.Now()
		order.PaidAt = &now
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		normalizeAwaitingPaymentBeforePaymentSent(order)
		if order.State != models.OrderState_AWAITING_PAYMENT {
			order.SetFSMState(order.State)
		}
		return dbtx.Save(order)
	}

	op.advanceToPendingAfterVerification(order)

	// Replay parked messages that were waiting for payment verification.
	// ORDER_CONFIRMATION (and potentially other messages) may have arrived
	// while the order was at AWAITING_PAYMENT_VERIFICATION and been parked.
	// Now that we've advanced to PENDING, they can be processed.
	op.replayParkedMessages(dbtx, order)

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return dbtx.Save(order)
	}

	op.emitPaymentSentEvents(dbtx, order, orderOpen, paymentSent, &tx)
	return dbtx.Save(order)
}

func (op *OrderProcessor) emitVerifiedPaymentSettlementRecovery(
	dbtx database.Tx,
	order *models.Order,
	paymentSent *pb.PaymentSent,
) {
	if order == nil || paymentSent == nil ||
		order.Role() != models.RoleVendor ||
		!order.IsPaymentVerified() ||
		order.State != models.OrderState_PENDING {
		return
	}
	if orderOpen, err := order.OrderOpenMessage(); err == nil {
		if err := op.EnsureRatingSignatures(dbtx, order, orderOpen); err != nil {
			logger.LogInfoWithIDf(log, op.nodeID,
				"Error recovering rating signatures for verified order %s: %s",
				order.ID, err)
		}
	}
	coinType := iwallet.CoinType(paymentSent.Coin)
	method, ok := payment.ResolvedPaymentMethod(order, paymentSent)
	if ok && payment.MethodIsCancelable(method) {
		if !markPaymentSettlementSignaled(order) {
			return
		}
		amount := parsePaymentSentEventAmount(paymentSent)
		dbtx.RegisterCommitHook(func() {
			op.bus.Emit(&events.CancelablePaymentReady{
				TenantID:      order.TenantID,
				OrderID:       order.ID.String(),
				TransactionID: paymentSent.TransactionID,
				Coin:          paymentSent.Coin,
				Amount:        amount,
			})
		})
		logger.LogInfoWithIDf(log, op.nodeID,
			"Recovered CANCELABLE payment ready event for verified order %s (coin=%s)",
			order.ID, paymentSent.Coin)
		return
	}
	if payment.IsFiatPaymentRoute(method, coinType) {
		return
	}
	if paymentSent.GetSettlementSpec() != nil && paymentSent.GetSettlementSpec().GetMethod() == pb.PaymentSent_RWA_INSTANT {
		if !markPaymentSettlementSignaled(order) {
			return
		}
		dbtx.RegisterCommitHook(func() {
			op.bus.Emit(&events.RwaInstantBuyCompleted{
				OrderID:       order.ID.String(),
				TransactionID: paymentSent.TransactionID,
				Coin:          paymentSent.Coin,
			})
		})
		logger.LogInfoWithIDf(log, op.nodeID,
			"Recovered RWA instant completion event for verified order %s",
			order.ID)
	}
}

func markPaymentSettlementSignaled(order *models.Order) bool {
	if order == nil || order.PaymentSettlementSignaledAt != nil ||
		len(order.SerializedOrderConfirmation) > 0 {
		return false
	}
	now := time.Now()
	order.PaymentSettlementSignaledAt = &now
	return true
}

func normalizeAwaitingPaymentBeforePaymentSent(order *models.Order) {
	if order == nil {
		return
	}
	if order.SerializedPaymentSent == nil && order.State == models.OrderState_PENDING {
		order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	}
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

func knownTransactionForPaymentSent(txs []iwallet.Transaction, paymentSent *pb.PaymentSent) *iwallet.Transaction {
	if paymentSent == nil {
		return nil
	}
	for i := range txs {
		if paymentSentContainsTransaction(paymentSent, txs[i].ID.String()) {
			return &txs[i]
		}
	}
	return nil
}

func paymentSentContainsTransaction(paymentSent *pb.PaymentSent, txID string) bool {
	if paymentSent == nil || txID == "" {
		return false
	}
	if paymentSent.TransactionID == txID {
		return true
	}
	return paymentSentContainsFundingFact(paymentSent, txID, "")
}

func isKnownTxVerifiedForRoute(
	paymentSent *pb.PaymentSent,
	method pb.PaymentSent_Method,
	coinType iwallet.CoinType,
	tx *iwallet.Transaction,
) bool {
	if tx == nil || paymentSent == nil {
		return false
	}
	if payment.IsFiatPaymentRoute(method, coinType) {
		return true
	}
	if isKnownTxConfirmed(tx) {
		return true
	}
	return paymentSentAllowsPendingKnownTx(paymentSent, tx.ID.String())
}

func paymentSentAllowsPendingKnownTx(paymentSent *pb.PaymentSent, txID string) bool {
	if paymentSent == nil || txID == "" ||
		models.NormalizePaymentConfirmationPolicy(paymentSent.GetConfirmationPolicy()) != models.PaymentConfirmationPolicyMempoolAccepted {
		return false
	}
	for _, fact := range paymentSent.GetFundingFacts() {
		if fact.GetTxHash() == txID && fact.GetStatus() == models.PaymentObservationStatusPending {
			return true
		}
	}
	return false
}

func parsePaymentSentEventAmount(paymentSent *pb.PaymentSent) uint64 {
	if paymentSent == nil {
		return 0
	}
	amount, err := strconv.ParseUint(paymentSent.Amount, 10, 64)
	if err != nil {
		return 0
	}
	return amount
}

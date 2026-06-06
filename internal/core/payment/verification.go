//go:build !private_distribution

package payment

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ── Payment Verification ────────────────────────────────────────

const (
	paymentVerifySlowInterval = 5 * time.Minute
	paymentVerifySlowAfter    = 2 * time.Hour
	paymentVerifyExpireAfter  = 48 * time.Hour
)

// RunPaymentVerificationOnce executes a single pass of payment verification.
// Called by the shared scheduler's NodeFn.
func (s *PaymentAppService) RunPaymentVerificationOnce() {
	s.VerifyPendingPayments()
}

func (s *PaymentAppService) VerifyPendingPayments() {
	var orders []models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("serialized_payment_sent IS NOT NULL AND payment_verification_status IN ? AND open = ? AND my_role IN ?",
				[]string{"", string(models.PaymentVerificationStatusPending)}, true,
				[]string{string(models.RoleVendor), string(models.RoleBuyer)}).
			Find(&orders).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Payment verification: failed to query unverified orders: %v", err)
		return
	}
	if len(orders) == 0 {
		return
	}
	logger.LogInfoWithIDf(log, s.nodeID, "Payment verification: checking %d unverified orders", len(orders))
	for i := range orders {
		order := &orders[i]
		age := time.Since(order.CreatedAt)

		if !order.LastCheckForPayments.IsZero() &&
			order.LastCheckForPayments.After(order.CreatedAt.Add(paymentVerifyExpireAfter)) {
			continue
		}
		if age > paymentVerifyExpireAfter {
			s.expireUnverifiedPayment(order, "timeout")
			continue
		}
		if age > paymentVerifySlowAfter &&
			time.Since(order.LastCheckForPayments) < paymentVerifySlowInterval {
			continue
		}
		s.verifyOrderPayment(order)
	}
}

// verifyOrderPayment uses PaymentVerificationService for unified fetch+verify,
// then delegates DB recording to OrderProcessor.RecordVerifiedPayment.
func (s *PaymentAppService) verifyOrderPayment(order *models.Order) {
	if s.verificationService == nil {
		return
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Payment verification: failed to get PaymentSent for order %s: %v", order.ID, err)
		return
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Payment verification: failed to get OrderOpen for order %s: %v", order.ID, err)
		return
	}

	vp, err := s.verificationService.FetchAndVerify(
		context.Background(), orderOpen, paymentSent, paymentSent.ToAddress)
	if err != nil {
		if errors.Is(err, ErrPaymentAddressMismatch) {
			logger.LogErrorWithIDf(log, s.nodeID,
				"Payment verification: address mismatch for order %s: %v", order.ID, err)
			s.expireUnverifiedPayment(order, "address_mismatch")
			return
		}
		logger.LogInfoWithIDf(log, s.nodeID,
			"Payment verification: tx %s not yet confirmed for order %s: %v",
			paymentSent.TransactionID, order.ID, err)
		s.updateLastCheckTime(order)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Payment verification: tx %s confirmed for order %s",
		paymentSent.TransactionID, order.ID)

	err = s.db.Update(func(dbtx database.Tx) error {
		var freshOrder models.Order
		if err := dbtx.Read().Where("id = ?", order.ID).First(&freshOrder).Error; err != nil {
			return err
		}
		if freshOrder.IsPaymentVerified() {
			return nil
		}
		if s.paymentRecorder != nil {
			return s.paymentRecorder.RecordVerifiedPayment(dbtx, &freshOrder, vp.Transaction)
		}
		if putErr := freshOrder.PutTransaction(vp.Transaction); putErr != nil {
			if !models.IsDuplicateTransactionError(putErr) {
				return putErr
			}
		}
		freshOrder.MarkPaymentVerified()
		if freshOrder.PaidAt == nil {
			now := time.Now()
			freshOrder.PaidAt = &now
		}
		return dbtx.Save(&freshOrder)
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Payment verification: failed to save verified order %s: %v", order.ID, err)
		return
	}

	if s.paymentRecorder == nil {
		order.MarkPaymentVerified()
		s.emitVerifiedPaymentEvents(order, paymentSent, &vp.Transaction)
	}

	if s.paymentVerifiedHandler != nil && shouldInvokeAsyncPaymentVerifiedHandler(order) {
		go s.paymentVerifiedHandler(order.ID.String(), paymentSent)
	}
}

func shouldInvokeAsyncPaymentVerifiedHandler(order *models.Order) bool {
	return order != nil && (order.Role() == models.RoleBuyer || order.Role() == models.RoleVendor)
}

func (s *PaymentAppService) expireUnverifiedPayment(order *models.Order, reason string) {
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Payment verification expire: cannot read PaymentSent for order %s: %v", order.ID, err)
		return
	}

	logger.LogWarningWithIDf(log, s.nodeID,
		"Payment verification expired for order %s (reason=%s, txid=%s, age=%s)",
		order.ID, reason, paymentSent.TransactionID, time.Since(order.CreatedAt).Round(time.Minute))

	expiredMark := order.CreatedAt.Add(paymentVerifyExpireAfter + time.Hour)
	if err := s.db.Update(func(dbtx database.Tx) error {
		var freshOrder models.Order
		if err := dbtx.Read().Where("id = ?", order.ID).First(&freshOrder).Error; err != nil {
			return err
		}
		freshOrder.MarkPaymentVerificationFailed(reason)
		freshOrder.LastCheckForPayments = expiredMark
		return dbtx.Save(&freshOrder)
	}); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Payment verification expire: failed to persist for order %s: %v", order.ID, err)
		return
	}

	s.eventBus.Emit(&events.PaymentVerificationExpired{
		OrderID:       order.ID.String(),
		TransactionID: paymentSent.TransactionID,
		Coin:          paymentSent.Coin,
		Reason:        reason,
	})
}

func (s *PaymentAppService) emitVerifiedPaymentEvents(order *models.Order, paymentSent *pb.PaymentSent, tx *iwallet.Transaction) {
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Payment verification: cannot read OrderOpen for order %s: %v", order.ID, err)
		return
	}
	if len(orderOpen.Listings) == 0 {
		logger.LogErrorWithIDf(log, s.nodeID, "Payment verification: order %s has no listings", order.ID)
		return
	}

	listing := orderOpen.Listings[0].Listing
	fundedEvent := &events.OrderFunded{
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
		Slug:  listing.Slug,
		Title: listing.Item.Title,
	}
	if len(listing.Item.Images) > 0 {
		fundedEvent.Thumbnail = events.Thumbnail{
			Tiny:  listing.Item.Images[0].Tiny,
			Small: listing.Item.Images[0].Small,
		}
	}
	s.eventBus.Emit(fundedEvent)

	spec := paymentSent.GetSettlementSpec()
	if spec == nil {
		logger.LogWarningWithIDf(log, s.nodeID, "Payment verification: order %s missing settlement spec in PaymentSent", order.ID)
		return
	}

	switch spec.GetMethod() {
	case pb.PaymentSent_CANCELABLE:
		var total *big.Int
		if tx != nil {
			amt := (*big.Int)(&tx.Value)
			total = new(big.Int).Set(amt)
		}
		if ready := payment.CancelablePaymentReadyEvent(order, paymentSent, total); ready != nil {
			s.eventBus.Emit(ready)
			logger.LogInfoWithIDf(log, s.nodeID, "Payment verification: emitted CancelablePaymentReady for order %s", order.ID)
		}

	case pb.PaymentSent_RWA_INSTANT:
		s.eventBus.Emit(&events.RwaInstantBuyCompleted{
			OrderID:       order.ID.String(),
			TransactionID: paymentSent.TransactionID,
			Coin:          paymentSent.Coin,
		})
		logger.LogInfoWithIDf(log, s.nodeID, "Payment verification: emitted RwaInstantBuyCompleted for order %s", order.ID)
	}
}

func (s *PaymentAppService) updateLastCheckTime(order *models.Order) {
	if err := s.db.Update(func(dbtx database.Tx) error {
		return dbtx.Update("last_check_for_payments", time.Now(),
			map[string]interface{}{"id = ?": order.ID}, &models.Order{})
	}); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Payment verification: failed to update LastCheckForPayments for order %s: %v", order.ID, err)
	}
}

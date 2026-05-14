//go:build !private_distribution

package order

import (
	"time"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// orderTimeoutScheduleInterval is the expected scheduler tick interval for
// order timeout jobs. Used to size the deduplication window in reminder checks.
const orderTimeoutScheduleInterval = 1 * time.Minute

// autoCompleteShippedOrders scans SHIPPED orders whose protection period
// has expired (shipped_at + autoCompleteDays < now) and auto-completes them.
// Only operates on buyer-side orders (CanComplete guard inside CompleteOrder).
func (s *OrderAppService) autoCompleteShippedOrders() {
	now := time.Now()
	shortestWindow := 3 * 24 * time.Hour // digital goods minimum

	var orders []models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("state = ? AND open = ? AND shipped_at IS NOT NULL AND shipped_at < ?",
				int32(models.OrderState_SHIPPED), true, now.Add(-shortestWindow)).
			Find(&orders).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Auto-complete: failed to query shipped orders: %v", err)
		return
	}

	for i := range orders {
		order := &orders[i]
		if !order.CanComplete() {
			continue
		}

		policy := models.ResolvePolicyForOrder(order)
		totalDuration := policy.AutoCompleteDuration()
		if order.ProtectionExtendedAt != nil {
			totalDuration += time.Duration(policy.ExtendProtectionDays) * 24 * time.Hour
		}
		deadline := order.ShippedAt.Add(totalDuration)
		if now.Before(deadline) {
			continue
		}

		if s.isClientSignedModerated(order) {
			logger.LogInfoWithIDf(log, s.nodeID,
				"Auto-complete: skipping CLIENT_SIGNED MODERATED order %s (requires contract timeout)", order.ID)
			continue
		}

		s.executeAutoComplete(order)
	}
}

// isClientSignedModerated returns true when the order uses MODERATED payment
// backed by a CLIENT_SIGNED chain (EVM/Solana). These cannot be auto-completed
// server-side because the escrow release requires an on-chain transaction from
// the buyer's wallet. They rely on the contract's built-in timeout mechanism.
func (s *OrderAppService) isClientSignedModerated(order *models.Order) bool {
	if order.PaymentMethod() != pb.PaymentSent_MODERATED {
		return false
	}
	ps, err := order.PaymentSentMessage()
	if err != nil {
		return false
	}
	strategy, err := s.paymentRegistry.ForCoin(iwallet.CoinType(ps.Coin))
	if err != nil {
		return false
	}
	return strategy.Model() == payment.PaymentModelClientSigned
}

func (s *OrderAppService) executeAutoComplete(order *models.Order) {
	meta := extractOrderNotifMeta(order)

	err := s.CompleteOrder(order.ID, "", nil, false, nil)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"Auto-complete: failed to complete order %s: %v", order.ID, err)
		return
	}

	if wErr := s.db.Update(func(tx database.Tx) error {
		return WriteOutboxEvent(tx, &events.OrderAutoCompleted{
			OrderID:      order.ID.String(),
			Reason:       "protection_expired",
			BuyerName:    meta.buyerName,
			BuyerID:      meta.buyerID,
			BuyerAvatar:  meta.buyerAvatar,
			VendorName:   meta.vendorName,
			VendorID:     meta.vendorID,
			VendorAvatar: meta.vendorAvatar,
			Thumbnail:    meta.thumb,
			Title:        meta.title,
		})
	}); wErr != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"Auto-complete: order %s completed but event write failed: %v", order.ID, wErr)
	}

	logger.LogInfoWithIDf(log, s.nodeID,
		"Auto-complete: order %s auto-completed after protection period", order.ID)
}

// autoRefundUnshippedOrders scans AWAITING_SHIPMENT orders whose max
// ship period has expired (paid_at + maxShipDays < now).
// CANCELABLE orders are auto-completed (funds already released to vendor);
// MODERATED and Fiat orders are auto-cancelled with refund.
func (s *OrderAppService) autoRefundUnshippedOrders() {
	now := time.Now()
	shortestWindow := 3 * 24 * time.Hour // digital/service goods minimum

	var orders []models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("state = ? AND open = ? AND paid_at IS NOT NULL AND paid_at < ?",
				int32(models.OrderState_AWAITING_SHIPMENT), true, now.Add(-shortestWindow)).
			Find(&orders).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Auto-refund: failed to query unshipped orders: %v", err)
		return
	}

	for i := range orders {
		order := &orders[i]

		policy := models.ResolvePolicyForOrder(order)
		deadline := order.PaidAt.Add(policy.MaxShipDuration())
		if now.Before(deadline) {
			continue
		}

		if order.PaymentMethod() == pb.PaymentSent_CANCELABLE {
			s.executeAutoCompleteUnshipped(order)
		} else {
			s.executeAutoCancel(order)
		}
	}
}

func (s *OrderAppService) executeAutoCancel(order *models.Order) {
	if !s.tryCancelFiatPayment(order) {
		return
	}

	meta := extractOrderNotifMeta(order)

	err := s.db.Update(func(tx database.Tx) error {
		var fresh models.Order
		if err := tx.Read().Where("id = ?", order.ID).First(&fresh).Error; err != nil {
			return err
		}
		if fresh.State != models.OrderState_AWAITING_SHIPMENT || !fresh.Open {
			return nil
		}
		fresh.SetFSMState(models.OrderState_CANCELED)
		fresh.Open = false
		if err := tx.Save(&fresh); err != nil {
			return err
		}
		return WriteOutboxEvent(tx, &events.OrderAutoCancelled{
			OrderID:      order.ID.String(),
			Reason:       "shipment_overdue",
			BuyerName:    meta.buyerName,
			BuyerID:      meta.buyerID,
			BuyerAvatar:  meta.buyerAvatar,
			VendorName:   meta.vendorName,
			VendorID:     meta.vendorID,
			VendorAvatar: meta.vendorAvatar,
			Thumbnail:    meta.thumb,
			Title:        meta.title,
		})
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"Auto-refund: failed to cancel order %s: %v", order.ID, err)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID,
		"Auto-refund: order %s auto-cancelled (vendor did not ship in time)", order.ID)
}

// executeAutoCompleteUnshipped handles CANCELABLE orders where the vendor
// did not ship in time. Since funds were already released to the vendor at
// ConfirmOrder/AutoConfirm time (1-of-2 escrow is irreversible), canceling
// with a refund is impossible. Instead, transition to COMPLETED so the buyer
// enters the after-sale window and can file an After-Sale Dispute.
func (s *OrderAppService) executeAutoCompleteUnshipped(order *models.Order) {
	meta := extractOrderNotifMeta(order)

	err := s.db.Update(func(tx database.Tx) error {
		var fresh models.Order
		if err := tx.Read().Where("id = ?", order.ID).First(&fresh).Error; err != nil {
			return err
		}
		if fresh.State != models.OrderState_AWAITING_SHIPMENT || !fresh.Open {
			return nil
		}
		now := time.Now()
		fresh.SetFSMState(models.OrderState_COMPLETED)
		fresh.CompletedAt = &now
		fresh.Open = false
		if err := tx.Save(&fresh); err != nil {
			return err
		}
		return WriteOutboxEvent(tx, &events.OrderAutoCompleted{
			OrderID:      order.ID.String(),
			Reason:       "unshipped_cancelable",
			BuyerName:    meta.buyerName,
			BuyerID:      meta.buyerID,
			BuyerAvatar:  meta.buyerAvatar,
			VendorName:   meta.vendorName,
			VendorID:     meta.vendorID,
			VendorAvatar: meta.vendorAvatar,
			Thumbnail:    meta.thumb,
			Title:        meta.title,
		})
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"Auto-refund: failed to auto-complete unshipped CANCELABLE order %s: %v", order.ID, err)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID,
		"Auto-refund: CANCELABLE order %s auto-completed (unshipped, funds already released to vendor)", order.ID)
}

// emitProtectionReminders sends countdown warnings for orders approaching
// auto-complete or auto-cancel deadlines.
func (s *OrderAppService) emitProtectionReminders() {
	now := time.Now()

	s.emitAutoCompleteReminders(now)
	s.emitAutoRefundReminders(now)
}

func (s *OrderAppService) emitAutoCompleteReminders(now time.Time) {
	var orders []models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("state = ? AND open = ? AND shipped_at IS NOT NULL",
				int32(models.OrderState_SHIPPED), true).
			Find(&orders).Error
	})
	if err != nil {
		return
	}

	for i := range orders {
		order := &orders[i]
		if !order.CanComplete() {
			continue
		}
		policy := models.ResolvePolicyForOrder(order)
		totalDuration := policy.AutoCompleteDuration()
		if order.ProtectionExtendedAt != nil {
			totalDuration += time.Duration(policy.ExtendProtectionDays) * 24 * time.Hour
		}
		deadline := order.ShippedAt.Add(totalDuration)

		for _, reminderDay := range policy.ReminderBeforeDays {
			if isInReminderWindow(now, deadline, reminderDay) {
				s.emitProtectionReminder(order, "auto_complete", reminderDay)
				break
			}
		}
	}
}

func (s *OrderAppService) emitAutoRefundReminders(now time.Time) {
	var orders []models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("state = ? AND open = ? AND paid_at IS NOT NULL",
				int32(models.OrderState_AWAITING_SHIPMENT), true).
			Find(&orders).Error
	})
	if err != nil {
		return
	}

	for i := range orders {
		order := &orders[i]
		policy := models.ResolvePolicyForOrder(order)
		deadline := order.PaidAt.Add(policy.MaxShipDuration())

		for _, reminderDay := range policy.ReminderBeforeDays {
			if isInReminderWindow(now, deadline, reminderDay) {
				s.emitProtectionReminder(order, "auto_cancel", reminderDay)
				break
			}
		}
	}
}

// isInReminderWindow returns true only during the first tick window after
// the remaining time to deadline crosses the reminderDay boundary.
// This prevents duplicate reminders: the scheduler ticks every 1 minute,
// but exact-day matching (daysLeft == reminderDay) would fire ~1440 times/day.
func isInReminderWindow(now time.Time, deadline time.Time, reminderDay int) bool {
	reminderAt := deadline.Add(-time.Duration(reminderDay) * 24 * time.Hour)
	return !now.Before(reminderAt) && now.Before(reminderAt.Add(2*orderTimeoutScheduleInterval))
}

func (s *OrderAppService) emitProtectionReminder(order *models.Order, reminderType string, daysLeft int) {
	meta := extractOrderNotifMeta(order)

	err := s.db.Update(func(tx database.Tx) error {
		return WriteOutboxEvent(tx, &events.OrderProtectionReminder{
			OrderID:       order.ID.String(),
			Type:          reminderType,
			DaysRemaining: daysLeft,
			BuyerName:     meta.buyerName,
			BuyerID:       meta.buyerID,
			BuyerAvatar:   meta.buyerAvatar,
			VendorName:    meta.vendorName,
			VendorID:      meta.vendorID,
			VendorAvatar:  meta.vendorAvatar,
			Thumbnail:     meta.thumb,
			Title:         meta.title,
		})
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"Protection reminder: failed for order %s: %v", order.ID, err)
	}
}

//go:build !private_distribution

package order

import (
	"context"
	"fmt"
	"time"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

const (
	pendingWarnThreshold         = 7 * 24 * time.Hour  // 7d
	pendingExpireThreshold       = 14 * 24 * time.Hour // 14d
	awaitingShipmentWarnThresh   = 14 * 24 * time.Hour // 14d
	awaitingShipmentExpireThresh = 30 * 24 * time.Hour // 30d
	disputedExpireThreshold      = 7 * 24 * time.Hour  // 7d
)

// RunOrderTimeoutOnce executes a single pass of all order timeout checks.
// Called by the shared scheduler's NodeFn.
func (s *OrderAppService) RunOrderTimeoutOnce() {
	s.expireTimedOutOrders()
	s.processStaleOrders()
	s.autoCompleteShippedOrders()
	s.autoRefundUnshippedOrders()
	s.emitProtectionReminders()
}

// ── AWAITING_PAYMENT timeout (existing) ─────────────────────────────────

func (s *OrderAppService) expireTimedOutOrders() {
	now := time.Now()
	var orders []models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("state = ? AND open = ? AND expires_at IS NOT NULL AND expires_at < ?",
				int32(models.OrderState_AWAITING_PAYMENT), true, now).
			Find(&orders).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Order timeout: failed to query expired orders: %v", err)
		return
	}
	if len(orders) == 0 {
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Order timeout: found %d expired AWAITING_PAYMENT orders", len(orders))
	for i := range orders {
		s.cancelExpiredOrder(&orders[i])
	}
}

func (s *OrderAppService) cancelExpiredOrder(order *models.Order) {
	if !s.tryCancelFiatPayment(order) {
		return
	}

	meta := extractOrderNotifMeta(order)

	err := s.db.Update(func(tx database.Tx) error {
		var fresh models.Order
		if err := tx.Read().Where("id = ?", order.ID).First(&fresh).Error; err != nil {
			return err
		}
		if fresh.State != models.OrderState_AWAITING_PAYMENT || !fresh.Open {
			return nil
		}
		fresh.SetFSMState(models.OrderState_CANCELED)
		fresh.Open = false
		fresh.SetSystemCancelReason("payment_timeout")
		if err := tx.Save(&fresh); err != nil {
			return err
		}
		return WriteOutboxEvent(tx, &events.OrderExpired{
			OrderID:      order.ID.String(),
			Reason:       "payment_timeout",
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
		logger.LogErrorWithIDf(log, s.nodeID, "Order timeout: failed to cancel expired order %s: %v", order.ID, err)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Order timeout: order %s expired and set to CANCELED", order.ID)
}

// tryCancelFiatPayment attempts to cancel the external payment session (e.g. Stripe
// PaymentIntent) for a fiat order. Returns true if order cancellation should proceed.
//
// Strategy (cancel-then-check, no TOCTOU):
//  1. Try CancelPayment — if succeeds, the PI is dead, safe to cancel order.
//  2. If cancel fails, query GetPayment — if the PI has already succeeded,
//     the buyer paid; skip order cancellation and let reconciliation handle it.
//  3. If both fail (network down, etc.) — proceed with cancel (conservative).
//
// For non-fiat orders (no fiat metadata), always returns true.
func (s *OrderAppService) tryCancelFiatPayment(order *models.Order) bool {
	if s.fiatOps == nil || order.FiatMetadata == nil || len(order.FiatMetadata) <= 2 {
		return true
	}
	meta, err := order.GetFiatMetadata()
	if err != nil {
		return true
	}
	providerID := meta["fiat_provider"]
	sessionID := meta["fiat_session_id"]
	if providerID == "" || sessionID == "" {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.fiatOps.CancelPayment(ctx, providerID, sessionID); err == nil {
		logger.LogInfoWithIDf(log, s.nodeID,
			"Order timeout: canceled fiat session %s for order %s", sessionID, order.ID)
		return true
	}

	status, checkErr := s.fiatOps.GetPaymentStatus(ctx, providerID, sessionID)
	if checkErr == nil && status == "succeeded" {
		logger.LogWarningWithIDf(log, s.nodeID,
			"Order timeout: order %s fiat payment already succeeded, deferring to reconciliation",
			order.ID)
		return false
	}

	logger.LogWarningWithIDf(log, s.nodeID,
		"Order timeout: cancel fiat session %s for order %s failed, proceeding with order cancel",
		sessionID, order.ID)
	return true
}

// ── Extended timeouts: AWAITING_PAYMENT_VERIFICATION, PENDING,
// AWAITING_SHIPMENT, DISPUTED ─────────────────────────────────────────────

func (s *OrderAppService) processStaleOrders() {
	now := time.Now()
	s.processStaleOrdersForState(now, models.OrderState_AWAITING_PAYMENT_VERIFICATION,
		pendingWarnThreshold, pendingExpireThreshold, "payment_verification_timeout")
	s.processStaleOrdersForState(now, models.OrderState_PENDING,
		pendingWarnThreshold, pendingExpireThreshold, "pending_unconfirmed")
	s.processStaleOrdersForState(now, models.OrderState_AWAITING_SHIPMENT,
		awaitingShipmentWarnThresh, awaitingShipmentExpireThresh, "shipment_overdue")
	s.processStaleOrdersForState(now, models.OrderState_DISPUTED,
		disputedExpireThreshold, 0, "dispute_no_response")
}

func (s *OrderAppService) processStaleOrdersForState(
	now time.Time, state models.OrderState,
	warnAfter, expireAfter time.Duration, reason string,
) {
	warnCutoff := now.Add(-warnAfter)

	var orders []models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("state = ? AND open = ? AND last_state_change_at IS NOT NULL AND last_state_change_at < ?",
				int32(state), true, warnCutoff).
			Find(&orders).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Order timeout: failed to query stale %s orders: %v", state, err)
		return
	}

	for i := range orders {
		order := &orders[i]
		stuckDuration := now.Sub(*order.LastStateChangeAt)

		if expireAfter > 0 && stuckDuration >= expireAfter {
			s.handleStaleOrderExpiry(order, state, reason)
		} else if order.TimeoutWarnedAt == nil {
			s.emitStaleWarning(order, state, stuckDuration)
		}
	}
}

func (s *OrderAppService) handleStaleOrderExpiry(order *models.Order, state models.OrderState, reason string) {
	switch state {
	case models.OrderState_AWAITING_PAYMENT_VERIFICATION:
		s.cancelStaleOrder(order, state, reason)
	case models.OrderState_PENDING:
		s.cancelStaleOrder(order, state, reason)
	case models.OrderState_AWAITING_SHIPMENT:
		if order.TimeoutWarnedAt == nil {
			s.emitStaleWarning(order, state, time.Since(*order.LastStateChangeAt))
		}
		logger.LogInfoWithIDf(log, s.nodeID,
			"Order timeout: order %s stuck in AWAITING_SHIPMENT > 30d, buyer may open dispute", order.ID)
	case models.OrderState_DISPUTED:
		if order.TimeoutWarnedAt == nil {
			s.emitStaleWarning(order, state, time.Since(*order.LastStateChangeAt))
		}
		logger.LogInfoWithIDf(log, s.nodeID,
			"Order timeout: order %s disputed > 7d with no seller response", order.ID)
	}
}

func (s *OrderAppService) cancelStaleOrder(order *models.Order, expectedState models.OrderState, reason string) {
	meta := extractOrderNotifMeta(order)

	err := s.db.Update(func(tx database.Tx) error {
		var fresh models.Order
		if err := tx.Read().Where("id = ?", order.ID).First(&fresh).Error; err != nil {
			return err
		}
		if fresh.State != expectedState || !fresh.Open {
			return nil
		}
		fresh.SetFSMState(models.OrderState_CANCELED)
		fresh.Open = false
		fresh.SetSystemCancelReason(reason)
		if err := tx.Save(&fresh); err != nil {
			return err
		}
		return WriteOutboxEvent(tx, &events.OrderExpired{
			OrderID:      order.ID.String(),
			Reason:       reason,
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
		logger.LogErrorWithIDf(log, s.nodeID, "Order timeout: failed to cancel stale order %s: %v", order.ID, err)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID,
		"Order timeout: order %s stale %s > 14d, auto-canceled", order.ID, expectedState)
}

func (s *OrderAppService) emitStaleWarning(order *models.Order, state models.OrderState, stuckFor time.Duration) {
	days := int(stuckFor.Hours() / 24)
	meta := extractOrderNotifMeta(order)

	err := s.db.Update(func(tx database.Tx) error {
		now := time.Now()
		if err := tx.Update("timeout_warned_at", now, map[string]interface{}{
			"id = ?": order.ID,
		}, &models.Order{}); err != nil {
			return err
		}
		return WriteOutboxEvent(tx, &events.OrderStaleWarning{
			OrderID:      order.ID.String(),
			State:        state.String(),
			StuckFor:     fmt.Sprintf("%dd", days),
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
		logger.LogErrorWithIDf(log, s.nodeID, "Order timeout: failed to mark warned for order %s: %v", order.ID, err)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Order timeout: stale warning queued for order %s (state=%s, stuck=%dd)", order.ID, state, days)
}

type orderNotifMeta struct {
	buyerName, buyerID, buyerAvatar    string
	vendorName, vendorID, vendorAvatar string
	thumb                              events.Thumbnail
	title                              string
}

// extractOrderNotifMeta best-effort extracts buyer/vendor identity,
// thumbnail, and listing title from the serialized OrderOpen protobuf.
// Returns zero values on any parse error — callers must tolerate empty fields.
func extractOrderNotifMeta(order *models.Order) orderNotifMeta {
	var m orderNotifMeta
	oo, err := order.OrderOpenMessage()
	if err != nil || oo == nil {
		return m
	}
	m.buyerName = oo.BuyerID.DisplayName()
	m.buyerID = oo.BuyerID.GetPeerID()
	m.buyerAvatar = oo.BuyerID.DisplayAvatar()
	if len(oo.Listings) > 0 && oo.Listings[0].Listing != nil {
		listing := oo.Listings[0].Listing
		m.vendorName = listing.VendorID.DisplayName()
		m.vendorID = listing.VendorID.GetPeerID()
		m.vendorAvatar = listing.VendorID.DisplayAvatar()
		if listing.Item != nil {
			m.title = listing.Item.Title
			if len(listing.Item.Images) > 0 {
				m.thumb = events.Thumbnail{
					Tiny:  listing.Item.Images[0].Tiny,
					Small: listing.Item.Images[0].Small,
				}
			}
		}
	}
	return m
}

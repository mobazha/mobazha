package models

import (
	"testing"
	"time"
)

func TestCanRequestAfterSale_BuyerInWindow(t *testing.T) {
	completedAt := time.Now().Add(-3 * 24 * time.Hour)
	order := Order{State: OrderState_COMPLETED, CompletedAt: &completedAt}
	order.SetRole(RoleBuyer)

	if !order.CanRequestAfterSale(time.Now()) {
		t.Error("buyer should be able to request after-sale within 7-day window")
	}
}

func TestCanRequestAfterSale_PaymentFinalizedInWindow(t *testing.T) {
	completedAt := time.Now().Add(-3 * 24 * time.Hour)
	order := Order{State: OrderState_PAYMENT_FINALIZED, CompletedAt: &completedAt}
	order.SetRole(RoleBuyer)

	if !order.CanRequestAfterSale(time.Now()) {
		t.Error("buyer should be able to request after-sale for PAYMENT_FINALIZED orders")
	}
}

func TestCanRequestAfterSale_WindowExpired(t *testing.T) {
	completedAt := time.Now().Add(-8 * 24 * time.Hour)
	order := Order{State: OrderState_COMPLETED, CompletedAt: &completedAt}
	order.SetRole(RoleBuyer)

	if order.CanRequestAfterSale(time.Now()) {
		t.Error("should reject after-sale request when window expired (>7 days)")
	}
}

func TestCanRequestAfterSale_VendorRejected(t *testing.T) {
	completedAt := time.Now().Add(-1 * 24 * time.Hour)
	order := Order{State: OrderState_COMPLETED, CompletedAt: &completedAt}
	order.SetRole(RoleVendor)

	if order.CanRequestAfterSale(time.Now()) {
		t.Error("vendor should not be allowed to request after-sale")
	}
}

func TestCanRequestAfterSale_WrongState(t *testing.T) {
	completedAt := time.Now().Add(-1 * 24 * time.Hour)
	states := []OrderState{
		OrderState_AWAITING_FULFILLMENT,
		OrderState_FULFILLED,
		OrderState_CANCELED,
		OrderState_DECLINED,
		OrderState_DISPUTED,
	}
	for _, state := range states {
		order := Order{State: state, CompletedAt: &completedAt}
		order.SetRole(RoleBuyer)

		if order.CanRequestAfterSale(time.Now()) {
			t.Errorf("state %v should not allow after-sale request", state)
		}
	}
}

func TestCanRequestAfterSale_NilCompletedAt(t *testing.T) {
	order := Order{State: OrderState_COMPLETED}
	order.SetRole(RoleBuyer)

	if order.CanRequestAfterSale(time.Now()) {
		t.Error("nil CompletedAt should reject after-sale request")
	}
}

func TestCanRequestAfterSale_DisputeOpened(t *testing.T) {
	completedAt := time.Now().Add(-1 * 24 * time.Hour)
	order := Order{
		State:                  OrderState_COMPLETED,
		CompletedAt:            &completedAt,
		SerializedDisputeOpen:  []byte("dispute-data"),
	}
	order.SetRole(RoleBuyer)

	if order.CanRequestAfterSale(time.Now()) {
		t.Error("order with open dispute should reject after-sale request")
	}
}

func TestCanRequestAfterSale_AlreadyHasAfterSale(t *testing.T) {
	completedAt := time.Now().Add(-1 * 24 * time.Hour)
	disputeAt := time.Now()
	order := Order{
		State:              OrderState_COMPLETED,
		CompletedAt:        &completedAt,
		AfterSaleDisputeAt: &disputeAt,
	}
	order.SetRole(RoleBuyer)

	if order.CanRequestAfterSale(time.Now()) {
		t.Error("duplicate after-sale request should be rejected")
	}
}

func TestCanRequestAfterSale_BoundaryExactExpiry(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	completedAt := now.Add(-7 * 24 * time.Hour)
	order := Order{State: OrderState_COMPLETED, CompletedAt: &completedAt}
	order.SetRole(RoleBuyer)

	if order.CanRequestAfterSale(now) {
		t.Error("exactly at 7-day boundary should be rejected (now == deadline)")
	}
}

func TestCanRequestAfterSale_BoundaryOneSecondBefore(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	completedAt := now.Add(-7*24*time.Hour + time.Second)
	order := Order{State: OrderState_COMPLETED, CompletedAt: &completedAt}
	order.SetRole(RoleBuyer)

	if !order.CanRequestAfterSale(now) {
		t.Error("one second before 7-day boundary should be accepted")
	}
}

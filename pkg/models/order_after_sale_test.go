package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCanRequestAfterSale_BuyerInWindow(t *testing.T) {
	completedAt := time.Now().Add(-3 * 24 * time.Hour)
	order := Order{
		State: OrderState_COMPLETED,
		OrderLifecycle: OrderLifecycle{
			CompletedAt: &completedAt,
		},
	}
	order.SetRole(RoleBuyer)

	if !order.CanRequestAfterSale(time.Now()) {
		t.Error("buyer should be able to request after-sale within 7-day window")
	}
}

func TestCanRequestAfterSale_PaymentFinalizedInWindow(t *testing.T) {
	completedAt := time.Now().Add(-3 * 24 * time.Hour)
	order := Order{
		State: OrderState_PAYMENT_FINALIZED,
		OrderLifecycle: OrderLifecycle{
			CompletedAt: &completedAt,
		},
	}
	order.SetRole(RoleBuyer)

	if !order.CanRequestAfterSale(time.Now()) {
		t.Error("buyer should be able to request after-sale for PAYMENT_FINALIZED orders")
	}
}

func TestCanRequestAfterSale_WindowExpired(t *testing.T) {
	completedAt := time.Now().Add(-8 * 24 * time.Hour)
	order := Order{
		State: OrderState_COMPLETED,
		OrderLifecycle: OrderLifecycle{
			CompletedAt: &completedAt,
		},
	}
	order.SetRole(RoleBuyer)

	if order.CanRequestAfterSale(time.Now()) {
		t.Error("should reject after-sale request when window expired (>7 days)")
	}
}

func TestCanRequestAfterSale_VendorRejected(t *testing.T) {
	completedAt := time.Now().Add(-1 * 24 * time.Hour)
	order := Order{
		State: OrderState_COMPLETED,
		OrderLifecycle: OrderLifecycle{
			CompletedAt: &completedAt,
		},
	}
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
		order := Order{
			State: state,
			OrderLifecycle: OrderLifecycle{
				CompletedAt: &completedAt,
			},
		}
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
		State:                 OrderState_COMPLETED,
		OrderLifecycle:        OrderLifecycle{CompletedAt: &completedAt},
		SerializedDisputeOpen: []byte("dispute-data"),
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
		State:          OrderState_COMPLETED,
		OrderLifecycle: OrderLifecycle{CompletedAt: &completedAt},
		AfterSaleDispute: AfterSaleDispute{
			OpenedAt: &disputeAt,
		},
	}
	order.SetRole(RoleBuyer)

	if order.CanRequestAfterSale(time.Now()) {
		t.Error("duplicate after-sale request should be rejected")
	}
}

func TestCanRequestAfterSale_BoundaryExactExpiry(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	completedAt := now.Add(-7 * 24 * time.Hour)
	order := Order{
		State: OrderState_COMPLETED,
		OrderLifecycle: OrderLifecycle{
			CompletedAt: &completedAt,
		},
	}
	order.SetRole(RoleBuyer)

	if order.CanRequestAfterSale(now) {
		t.Error("exactly at 7-day boundary should be rejected (now == deadline)")
	}
}

func TestCanRequestAfterSale_BoundaryOneSecondBefore(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	completedAt := now.Add(-7*24*time.Hour + time.Second)
	order := Order{
		State: OrderState_COMPLETED,
		OrderLifecycle: OrderLifecycle{
			CompletedAt: &completedAt,
		},
	}
	order.SetRole(RoleBuyer)

	if !order.CanRequestAfterSale(now) {
		t.Error("one second before 7-day boundary should be accepted")
	}
}

func TestOrderMarshalJSON_IncludesAfterSaleDisputeFields(t *testing.T) {
	completedAt := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	disputeAt := time.Date(2026, 3, 21, 9, 30, 0, 0, time.UTC)
	order := Order{
		ID:    "order-after-sale-json",
		State: OrderState_COMPLETED,
		OrderLifecycle: OrderLifecycle{
			CompletedAt: &completedAt,
		},
		AfterSaleDispute: AfterSaleDispute{
			Reason:      "QUALITY_ISSUE",
			Description: "broken item",
			OpenedAt:    &disputeAt,
		},
	}
	order.SetRole(RoleBuyer)

	raw, err := json.Marshal(&order)
	if err != nil {
		t.Fatalf("json.Marshal(order): %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal(payload): %v", err)
	}

	nested, ok := payload["afterSaleDispute"].(map[string]interface{})
	if !ok {
		t.Fatalf("afterSaleDispute should be an object, got %#v", payload["afterSaleDispute"])
	}
	if got, _ := nested["reason"].(string); got != "QUALITY_ISSUE" {
		t.Fatalf("afterSaleDispute.reason = %q, want %q", got, "QUALITY_ISSUE")
	}
	if got, _ := nested["description"].(string); got != "broken item" {
		t.Fatalf("afterSaleDispute.description = %q, want %q", got, "broken item")
	}
	if got, _ := nested["openedAt"].(string); got == "" {
		t.Fatal("afterSaleDispute.openedAt should be present in marshaled JSON")
	}
}

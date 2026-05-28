//go:build !private_distribution

package payment

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestPromoteAfterVerificationRecoversPaymentTimeoutCancellation(t *testing.T) {
	order := &models.Order{
		ID:    models.OrderID("order-1"),
		State: models.OrderState_CANCELED,
		Open:  false,
	}
	order.SetSystemCancelReason("payment_timeout")
	order.OrderCancelSignature = "sig"
	order.OrderCancelAcked = true

	now := time.Unix(100, 0)
	promoteAfterVerification(order, now)

	if order.State != models.OrderState_PENDING {
		t.Fatalf("state = %s, want PENDING", order.State)
	}
	if !order.Open {
		t.Fatal("order should be reopened after verified payment timeout recovery")
	}
	if order.SerializedOrderCancel != nil || order.OrderCancelSignature != "" || order.OrderCancelAcked {
		t.Fatal("payment timeout cancel fields should be cleared")
	}
	if order.PaidAt == nil || !order.PaidAt.Equal(now) {
		t.Fatalf("PaidAt = %v, want %v", order.PaidAt, now)
	}
}

func TestPromoteAfterVerificationLeavesNonTimeoutCancellationAlone(t *testing.T) {
	order := &models.Order{
		ID:    models.OrderID("order-1"),
		State: models.OrderState_CANCELED,
		Open:  false,
	}
	order.SetSystemCancelReason("buyer_requested")

	promoteAfterVerification(order, time.Unix(100, 0))

	if order.State != models.OrderState_CANCELED {
		t.Fatalf("state = %s, want CANCELED", order.State)
	}
	if order.SerializedOrderCancel == nil {
		t.Fatal("non-timeout cancel should not be cleared")
	}
}

package models

import (
	"testing"
	"time"
)

func TestIsPaymentReady(t *testing.T) {
	t.Run("nil order", func(t *testing.T) {
		if IsPaymentReady(nil) {
			t.Fatal("expected false for nil order")
		}
	})

	t.Run("payment_ready_at", func(t *testing.T) {
		now := time.Now()
		if !IsPaymentReady(&Order{PaymentReadyAt: &now}) {
			t.Fatal("expected ready when payment_ready_at is set")
		}
	})

	t.Run("legacy order_open_acked", func(t *testing.T) {
		if !IsPaymentReady(&Order{OrderOpenAcked: true}) {
			t.Fatal("expected ready when order_open_acked is set")
		}
	})

	t.Run("not ready", func(t *testing.T) {
		if IsPaymentReady(&Order{}) {
			t.Fatal("expected not ready")
		}
	})
}

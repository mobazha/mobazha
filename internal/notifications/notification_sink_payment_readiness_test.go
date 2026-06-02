package notifications

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/events"
)

func TestWrapForWebSocket_PaymentReadiness(t *testing.T) {
	meta := events.EventMeta{Name: "payment.readiness", Category: "payment"}
	wrapped := wrapForWebSocket(meta, &events.OrderPaymentReady{OrderID: "QmOrder1"})
	if wrapped == nil {
		t.Fatal("expected payment readiness wrapper")
	}

	msg, ok := wrapped.(paymentReadinessPushMessage)
	if !ok {
		t.Fatalf("expected paymentReadinessPushMessage, got %T", wrapped)
	}
	if msg.Type != "paymentReadiness" {
		t.Fatalf("type = %q, want paymentReadiness", msg.Type)
	}
	ready, ok := msg.Data.PaymentReadiness.(*events.OrderPaymentReady)
	if !ok || ready.OrderID != "QmOrder1" {
		t.Fatalf("paymentReadiness payload = %+v", msg.Data.PaymentReadiness)
	}
}

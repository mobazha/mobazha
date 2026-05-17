//go:build !private_distribution

package payment

import (
	"context"
	"strings"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

func TestPaymentSessionServiceImpl_CreateSession_RejectsNonCanonicalPaymentCoin(t *testing.T) {
	svc := NewPaymentSessionService(nil)

	_, err := svc.CreateSession(context.Background(), contracts.CreatePaymentSessionRequest{
		OrderID:     "any-order-id",
		PaymentCoin: "USDC",
	})
	if err == nil {
		t.Fatal("expected error for ambiguous non-canonical coin")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "canonical") && !strings.Contains(msg, "payment coin") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

//go:build !private_distribution

package payment

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	pkpayment "github.com/mobazha/mobazha3.0/pkg/payment"
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

func TestPaymentSessionServiceImpl_CreateSession_RejectsProductDisabledZEC(t *testing.T) {
	svc := NewPaymentSessionService(nil)

	_, err := svc.CreateSession(context.Background(), contracts.CreatePaymentSessionRequest{
		OrderID:     "any-order-id",
		PaymentCoin: "crypto:zcash:mainnet:native",
	})
	if err == nil {
		t.Fatal("expected error for product-disabled ZEC")
	}
	if !errors.Is(err, ErrPaymentCoinDisabled) {
		t.Fatalf("error = %v, want ErrPaymentCoinDisabled", err)
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "not enabled") || !strings.Contains(msg, "zcash") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCanReprovisionForCoinSwitch_AllowsUnfundedFiatCheckoutToCrypto(t *testing.T) {
	view := &pkpayment.PaymentSession{
		PaymentCoin:    "fiat:stripe:USD",
		SettlementMode: pkpayment.SettlementModeProviderCheckout,
		PaymentProgress: pkpayment.PaymentProgressView{
			ObservedAmount: "0.00",
			FundingState:   pkpayment.FundingStateProviderProcessing,
		},
	}

	if !canReprovisionForCoinSwitch(view, "crypto:solana:mainnet:native") {
		t.Fatal("expected unfunded provider checkout to allow crypto reprovision")
	}
}

func TestCanReprovisionForCoinSwitch_RejectsFundedFiatCheckout(t *testing.T) {
	view := &pkpayment.PaymentSession{
		PaymentCoin:    "fiat:stripe:USD",
		SettlementMode: pkpayment.SettlementModeProviderCheckout,
		PaymentProgress: pkpayment.PaymentProgressView{
			ObservedAmount: "29.00",
			FundingState:   pkpayment.FundingStateFullyFunded,
		},
	}

	if canReprovisionForCoinSwitch(view, "crypto:solana:mainnet:native") {
		t.Fatal("funded provider checkout must not allow crypto reprovision")
	}
}

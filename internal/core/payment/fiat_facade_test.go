package payment

import (
	"testing"

	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
)

// ── parseFiatCoin ────────────────────────────────────────────────────────────

func TestParseFiatCoin_Valid(t *testing.T) {
	cases := []struct {
		input    string
		wantProv string
		wantCurr string
	}{
		{"fiat:stripe:USD", "stripe", "USD"},
		{"fiat:paypal:EUR", "paypal", "EUR"},
		{"FIAT:Stripe:usd", "stripe", "USD"},
	}
	for _, tc := range cases {
		prov, curr, err := parseFiatCoin(tc.input)
		if err != nil {
			t.Errorf("parseFiatCoin(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if prov != tc.wantProv {
			t.Errorf("parseFiatCoin(%q) provider = %q, want %q", tc.input, prov, tc.wantProv)
		}
		if curr != tc.wantCurr {
			t.Errorf("parseFiatCoin(%q) currency = %q, want %q", tc.input, curr, tc.wantCurr)
		}
	}
}

func TestParseFiatCoin_Invalid(t *testing.T) {
	invalid := []string{
		"crypto:eth:usdc",
		"fiat:",
		"fiat:stripe",
		"fiat::USD",
		"stripe:USD",
		"",
	}
	for _, s := range invalid {
		if _, _, err := parseFiatCoin(s); err == nil {
			t.Errorf("parseFiatCoin(%q) expected error, got nil", s)
		}
	}
}

// ── deriveFundingState (fiat branch) ─────────────────────────────────────────

func TestDeriveFundingState_Fiat_ProviderProcessing(t *testing.T) {
	cases := []models.PaymentVerificationStatus{
		"",
		models.PaymentVerificationStatusPending,
	}
	for _, vs := range cases {
		got := deriveFundingState("0", "0", vs, true)
		if got != payment.FundingStateProviderProcessing {
			t.Errorf("deriveFundingState(isFiat=true, status=%q) = %v, want ProviderProcessing", vs, got)
		}
	}
}

func TestDeriveFundingState_FiatSessionInactive_FallsBackToAwaitingFunds(t *testing.T) {
	got := deriveFundingState("0", "0", "", false)
	if got != payment.FundingStateAwaitingFunds {
		t.Errorf("deriveFundingState(isFiatSession=false) = %v, want AwaitingFunds", got)
	}
}

func TestDeriveFundingState_Fiat_FullyFunded_WhenVerified(t *testing.T) {
	got := deriveFundingState("0", "0", models.PaymentVerificationStatusVerified, true)
	if got != payment.FundingStateFullyFunded {
		t.Errorf("deriveFundingState(isFiat=true, verified) = %v, want FullyFunded", got)
	}
}

func TestDeriveFundingState_Fiat_ExpiredUnfunded_WhenFailed(t *testing.T) {
	got := deriveFundingState("0", "0", models.PaymentVerificationStatusFailed, true)
	if got != payment.FundingStateExpiredUnfunded {
		t.Errorf("deriveFundingState(isFiat=true, failed) = %v, want ExpiredUnfunded", got)
	}
}

// ── deriveFundingState (crypto branch, regression) ───────────────────────────

func TestDeriveFundingState_Crypto_AwaitingFunds_WhenZeroObserved(t *testing.T) {
	got := deriveFundingState("0", "1000", "", false)
	if got != payment.FundingStateAwaitingFunds {
		t.Errorf("deriveFundingState(crypto, 0/1000) = %v, want AwaitingFunds", got)
	}
}

func TestDeriveFundingState_Crypto_FullyFunded_WhenExactMatch(t *testing.T) {
	got := deriveFundingState("1000", "1000", "", false)
	if got != payment.FundingStateFullyFunded {
		t.Errorf("deriveFundingState(crypto, 1000/1000) = %v, want FullyFunded", got)
	}
}

func TestDeriveFundingState_Crypto_Overfunded(t *testing.T) {
	got := deriveFundingState("1001", "1000", "", false)
	if got != payment.FundingStateOverfunded {
		t.Errorf("deriveFundingState(crypto, 1001/1000) = %v, want Overfunded", got)
	}
}

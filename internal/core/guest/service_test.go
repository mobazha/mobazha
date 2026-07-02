package guest

import (
	"strings"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestNormalizeGuestPaymentCoin_Aliases(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "ethereum", in: "ETH", want: "crypto:eip155:1:native"},
		{name: "lowercase ethereum", in: " eth ", want: "crypto:eip155:1:native"},
		{name: "litecoin", in: "LTC", want: "crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native"},
		{name: "base native", in: "BASE", want: "crypto:eip155:8453:native"},
		{name: "canonical crypto preserved", in: "crypto:eip155:1:native", want: "crypto:eip155:1:native"},
		{name: "canonical fiat preserved", in: "fiat:stripe:USD", want: "fiat:stripe:USD"},
		{name: "test coin uppercased", in: "testcoin", want: "TESTCOIN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeGuestPaymentCoin(tt.in); got != tt.want {
				t.Fatalf("normalizeGuestPaymentCoin(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGenerateOrderToken_FitsPersistedSchema(t *testing.T) {
	token, err := generateOrderToken()
	if err != nil {
		t.Fatalf("generateOrderToken: %v", err)
	}
	if len(token) != 64 {
		t.Fatalf("token length = %d, want 64", len(token))
	}
	if !strings.HasPrefix(token, guestOrderTokenPrefix) {
		t.Fatalf("token prefix = %q, want %q", token[:len(guestOrderTokenPrefix)], guestOrderTokenPrefix)
	}
}

func TestValidateAcceptedCoin_AcceptsAliasAndCanonicalEquivalent(t *testing.T) {
	svc := &GuestOrderAppService{}
	cfg := &models.GuestCheckoutConfig{AcceptedCoins: "ETH,LTC"}

	if err := svc.validateAcceptedCoin(cfg, "ETH"); err != nil {
		t.Fatalf("validateAcceptedCoin alias: %v", err)
	}
	if err := svc.validateAcceptedCoin(cfg, "crypto:eip155:1:native"); err != nil {
		t.Fatalf("validateAcceptedCoin canonical equivalent: %v", err)
	}
	if err := svc.validateAcceptedCoin(cfg, "crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native"); err != nil {
		t.Fatalf("validateAcceptedCoin canonical ltc equivalent: %v", err)
	}
	if err := svc.validateAcceptedCoin(cfg, "crypto:monero:mainnet:native"); err == nil {
		t.Fatal("validateAcceptedCoin accepted an unconfigured coin")
	}
}

func TestFilterAvailableCoins_PreservesConfigCodesForVisibleCoins(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)

	got := svc.filterAvailableCoins("ETH,LTC,TRX,XMR,NOTREAL")
	if got != "LTC" {
		t.Fatalf("filterAvailableCoins() = %q, want LTC", got)
	}
	if strings.Contains(got, "crypto:") {
		t.Fatalf("filterAvailableCoins should preserve configured display codes, got %q", got)
	}
}

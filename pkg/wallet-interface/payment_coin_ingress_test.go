package wallet_interface

import (
	"strings"
	"testing"
)

func TestTryNormalizePaymentCoin_LegacyBTC(t *testing.T) {
	ct, ok := TryNormalizePaymentCoin("BTC")
	if !ok {
		t.Fatal("expected BTC to normalize")
	}
	s := string(ct)
	if !strings.HasPrefix(strings.ToLower(s), "crypto:") || !strings.Contains(strings.ToLower(s), "native") {
		t.Fatalf("unexpected canonical coin %q", s)
	}
}

func TestNormalizePaymentCoinIngress_LegacyEth(t *testing.T) {
	ct, err := NormalizePaymentCoinIngress(" eth ")
	if err != nil {
		t.Fatal(err)
	}
	if err := ct.ValidateCanonicalPaymentCoin(); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizePaymentCoinIngress_FiatCasing(t *testing.T) {
	ct, err := NormalizePaymentCoinIngress("fiat:Stripe:usd")
	if err != nil {
		t.Fatal(err)
	}
	if string(ct) != "fiat:stripe:USD" {
		t.Fatalf("got %q", ct)
	}
}

func TestNormalizePaymentCoinIngress_CryptoUsesAssetIDNormalize(t *testing.T) {
	raw := "CRYPTO:EIP155:1:native"
	ct, err := NormalizePaymentCoinIngress(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := ct.ValidateCanonicalPaymentCoin(); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizePaymentCoinIngress_RejectsIncompleteFiat(t *testing.T) {
	if _, err := NormalizePaymentCoinIngress("fiat:stripe"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizePaymentCoinIngress_RejectsAmbiguousTicker(t *testing.T) {
	if _, err := NormalizePaymentCoinIngress("USDC"); err == nil {
		t.Fatal("expected error")
	}
}

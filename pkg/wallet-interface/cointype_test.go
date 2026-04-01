package wallet_interface

import (
	"testing"
)

func TestCoinType_FiatBaseCurrency(t *testing.T) {
	tests := []struct {
		coin     CoinType
		expected string
	}{
		{"fiat:stripe:USD", "USD"},
		{"fiat:paypal:EUR", "EUR"},
		{"fiat:USD", "USD"},
		{"fiat:EUR", "EUR"},
		{"USD", "USD"},
		{"FIAT:STRIPE:USD", "USD"},
		{"fiat:stripe:usd", "usd"},
	}
	for _, tt := range tests {
		got := tt.coin.FiatBaseCurrency()
		if got != tt.expected {
			t.Errorf("CoinType(%q).FiatBaseCurrency() = %q, want %q", tt.coin, got, tt.expected)
		}
	}
}

func TestCoinType_IsFiatPayment(t *testing.T) {
	tests := []struct {
		coin     CoinType
		expected bool
	}{
		{"fiat:stripe:USD", true},
		{"fiat:USD", true},
		{"FIAT:USD", true},
		{"BTC", false},
		{"ETH", false},
		{"Stripe", false},
	}
	for _, tt := range tests {
		got := tt.coin.IsFiatPayment()
		if got != tt.expected {
			t.Errorf("CoinType(%q).IsFiatPayment() = %v, want %v", tt.coin, got, tt.expected)
		}
	}
}

func TestCoinInfoFromCoinType_LegacyTokenCoinRejected(t *testing.T) {
	if _, err := CoinInfoFromCoinType(CoinType("BSCUSDT")); err == nil {
		t.Fatalf("expected legacy token coin to be rejected")
	}
	if _, err := CoinInfoFromCoinType(CoinType("BSCFAKE")); err == nil {
		t.Fatalf("expected unknown chain+token coin to be rejected")
	}
}

func TestCoinInfoFromCoinType_LegacyNativeCoinRejected(t *testing.T) {
	if _, err := CoinInfoFromCoinType(CoinType("BTC")); err == nil {
		t.Fatalf("expected legacy native coin code to be rejected")
	}
}

func TestNewCoinInfo_CanonicalNativeByChain(t *testing.T) {
	info, err := NewCoinInfo("ETH", "")
	if err != nil {
		t.Fatalf("NewCoinInfo(ETH, \"\"): %v", err)
	}
	if info.Chain != ChainEthereum {
		t.Fatalf("chain = %s, want %s", info.Chain, ChainEthereum)
	}
	if !info.IsNative {
		t.Fatalf("IsNative = false, want true")
	}
}

func TestNewCoinInfo_LegacyTokenRejected(t *testing.T) {
	if _, err := NewCoinInfo("BSC", "USDT"); err == nil {
		t.Fatalf("expected legacy chain+token to be rejected")
	}
}

func TestCoinInfoFromCoinType_FiatCoinInfo(t *testing.T) {
	info, err := CoinInfoFromCoinType(CoinType("fiat:stripe:usd"))
	if err != nil {
		t.Fatalf("CoinInfoFromCoinType(fiat:stripe:usd): %v", err)
	}
	if info.Chain != ChainFiat {
		t.Fatalf("chain = %s, want %s", info.Chain, ChainFiat)
	}
	if info.Symbol != "USD" {
		t.Fatalf("symbol = %s, want USD", info.Symbol)
	}
	if !info.IsNative {
		t.Fatalf("IsNative = false, want true")
	}

	fiatInfo, err := CoinInfoFromCoinType(CtFiat)
	if err != nil {
		t.Fatalf("CoinInfoFromCoinType(Fiat): %v", err)
	}
	if fiatInfo.Symbol != "Fiat" {
		t.Fatalf("symbol = %s, want Fiat", fiatInfo.Symbol)
	}
}

package wallet_interface

import (
	"strings"
	"testing"
)

func TestCoinInfoFromCoinType_CanonicalAssetID(t *testing.T) {
	coin := CoinType("crypto:eip155:56:erc20:0x55d398326f99059ff775485246999027b3197955")

	info, err := CoinInfoFromCoinType(coin)
	if err != nil {
		t.Fatalf("CoinInfoFromCoinType(%s): %v", coin, err)
	}

	if info.Chain != ChainBSC {
		t.Fatalf("chain = %s, want %s", info.Chain, ChainBSC)
	}
	if info.IsNative {
		t.Fatalf("IsNative = true, want false")
	}
	if info.Symbol != "USDT" {
		t.Fatalf("symbol = %s, want USDT", info.Symbol)
	}
	if info.Decimals != 6 {
		t.Fatalf("decimals = %d, want 6", info.Decimals)
	}
	if !strings.EqualFold(info.Contract, "0x55d398326f99059ff775485246999027b3197955") {
		t.Fatalf("contract = %s", info.Contract)
	}
}

func TestCoinInfoFromCoinType_InvalidCanonicalAssetID(t *testing.T) {
	coin := CoinType("crypto:eip155:56:erc20:not-an-address")
	if _, err := CoinInfoFromCoinType(coin); err == nil {
		t.Fatalf("expected error for invalid canonical asset id")
	}
}

func TestCoinType_PricingCurrencyCode(t *testing.T) {
	tests := []struct {
		coin     CoinType
		expected string
	}{
		{"crypto:eip155:1:native", "ETH"},
		{"crypto:eip155:56:erc20:0x55d398326f99059ff775485246999027b3197955", "BSCUSDT"},
		{"fiat:stripe:usd", "USD"},
	}

	for _, tt := range tests {
		got, err := tt.coin.PricingCurrencyCode()
		if err != nil {
			t.Fatalf("PricingCurrencyCode(%s): %v", tt.coin, err)
		}
		if got != tt.expected {
			t.Fatalf("PricingCurrencyCode(%s) = %s, want %s", tt.coin, got, tt.expected)
		}
	}
}

func TestCoinType_IsCanonicalCryptoAssetID(t *testing.T) {
	if !CoinType("crypto:eip155:1:native").IsCanonicalCryptoAssetID() {
		t.Fatal("canonical asset id should be true")
	}
	if CoinType("ETHUSDT").IsCanonicalCryptoAssetID() {
		t.Fatal("legacy coin code should be false")
	}
}

func TestCoinType_IsCanonicalPaymentCoin(t *testing.T) {
	tests := []struct {
		coin     CoinType
		expected bool
	}{
		{"crypto:eip155:1:native", true},
		{"fiat:stripe:USD", true},
		{CtMock, true},
		{CtTestCoin, true},
		{"ETHUSDT", false},
	}
	for _, tt := range tests {
		got := tt.coin.IsCanonicalPaymentCoin()
		if got != tt.expected {
			t.Fatalf("IsCanonicalPaymentCoin(%s)=%v, want %v", tt.coin, got, tt.expected)
		}
	}
}

func TestCoinType_ValidateCanonicalPaymentCoin(t *testing.T) {
	if err := CoinType("crypto:tron:mainnet:trc20:TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t").ValidateCanonicalPaymentCoin(); err != nil {
		t.Fatalf("unexpected error for canonical coin: %v", err)
	}
	if err := CoinType("TRXUSDT").ValidateCanonicalPaymentCoin(); err == nil {
		t.Fatalf("expected error for legacy payment coin")
	}
}

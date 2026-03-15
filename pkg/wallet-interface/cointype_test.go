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

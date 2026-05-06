package types

import (
	"testing"
)

func TestHash_String(t *testing.T) {
	h := Hash{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
		0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f}

	want := "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	if got := h.String(); got != want {
		t.Errorf("Hash.String() = %v, want %v", got, want)
	}
}

func TestHash_IsZero(t *testing.T) {
	tests := []struct {
		name string
		h    Hash
		want bool
	}{
		{"zero hash", Hash{}, true},
		{"non-zero hash", Hash{0x01}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.h.IsZero(); got != tt.want {
				t.Errorf("Hash.IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimestamp_Unix(t *testing.T) {
	ts := Now()

	unix := ts.Unix()
	if unix <= 0 {
		t.Error("Timestamp.Unix() should return positive value")
	}
}

func TestCurrency_IsCrypto(t *testing.T) {
	tests := []struct {
		c    Currency
		want bool
	}{
		{CurrencyBTC, true},
		{CurrencyBCH, true},
		{CurrencyLTC, true},
		{CurrencyZEC, true},
		{CurrencyETH, true},
		{CurrencySOL, true},
		{CurrencyUSD, false},
		{CurrencyEUR, false},
		{CurrencyCNY, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.c), func(t *testing.T) {
			if got := tt.c.IsCrypto(); got != tt.want {
				t.Errorf("Currency.IsCrypto() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCurrency_IsFiat(t *testing.T) {
	tests := []struct {
		c    Currency
		want bool
	}{
		{CurrencyBTC, false},
		{CurrencyUSD, true},
		{CurrencyEUR, true},
		{CurrencyCNY, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.c), func(t *testing.T) {
			if got := tt.c.IsFiat(); got != tt.want {
				t.Errorf("Currency.IsFiat() = %v, want %v", got, tt.want)
			}
		})
	}
}

package payment

import "testing"

func TestFormatSessionAmount(t *testing.T) {
	tests := []struct {
		name        string
		rawAmount   string
		paymentCoin string
		want        string
	}{
		{
			name:        "btc smallest units",
			rawAmount:   "30116",
			paymentCoin: "crypto:bip122:000000000019d6689c085ae165831e93:native",
			want:        "0.00030116",
		},
		{
			name:        "evm testnet native maps to canonical decimals",
			rawAmount:   "11000000000000000",
			paymentCoin: "crypto:eip155:11155111:native",
			want:        "0.011",
		},
		{
			name:        "fiat compound code",
			rawAmount:   "1599",
			paymentCoin: "fiat:stripe:USD",
			want:        "15.99",
		},
		{
			name:        "runtime erc20 standard decimals",
			rawAmount:   "1100000000000000000",
			paymentCoin: "crypto:eip155:1:erc20:0x9fe46736679d2d9a65f0992f2272de9f3c7fa6e0",
			want:        "1.1",
		},
		{
			name:        "unknown coin leaves raw amount unchanged",
			rawAmount:   "12345",
			paymentCoin: "crypto:unknown",
			want:        "12345",
		},
		{
			name:        "invalid amount leaves raw amount unchanged",
			rawAmount:   "12.345",
			paymentCoin: "crypto:bip122:000000000019d6689c085ae165831e93:native",
			want:        "12.345",
		},
		{
			name:        "negative amount keeps sign",
			rawAmount:   "-1500000000000000000",
			paymentCoin: "ETH",
			want:        "-1.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatSessionAmount(tt.rawAmount, tt.paymentCoin); got != tt.want {
				t.Fatalf("FormatSessionAmount(%q, %q) = %q, want %q", tt.rawAmount, tt.paymentCoin, got, tt.want)
			}
		})
	}
}

func TestSessionAmountDecimals(t *testing.T) {
	tests := []struct {
		name        string
		paymentCoin string
		want        int
		wantOK      bool
	}{
		{
			name:        "btc canonical asset",
			paymentCoin: "crypto:bip122:000000000019d6689c085ae165831e93:native",
			want:        8,
			wantOK:      true,
		},
		{
			name:        "safe sepolia native",
			paymentCoin: "crypto:eip155:11155111:native",
			want:        18,
			wantOK:      true,
		},
		{
			name:        "fiat provider code",
			paymentCoin: "fiat:stripe:USD",
			want:        2,
			wantOK:      true,
		},
		{
			name:        "runtime spl standard decimals",
			paymentCoin: "crypto:solana:devnet:spl:EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			want:        9,
			wantOK:      true,
		},
		{
			name:        "unknown",
			paymentCoin: "crypto:unknown",
			want:        0,
			wantOK:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := SessionAmountDecimals(tt.paymentCoin)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("SessionAmountDecimals(%q) = (%d, %v), want (%d, %v)", tt.paymentCoin, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

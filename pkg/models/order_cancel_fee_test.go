package models

import (
	"math/big"
	"testing"
)

// TestOrder_GetCancelFeeAmount covers the Tier 1 / Tier 2 cancel fee semantics
// introduced by Phase managed EVM v0.3.0 (D-Hybrid-29 Path Lite Tiered).
func TestOrder_GetCancelFeeAmount(t *testing.T) {
	tests := []struct {
		name   string
		stored string
		want   *big.Int
		wantOK bool
		hasFee bool
	}{
		{
			name:   "tier 2 absorbed empty string",
			stored: "",
			want:   big.NewInt(0),
			wantOK: true,
			hasFee: false,
		},
		{
			name:   "tier 2 absorbed whitespace only",
			stored: "   ",
			want:   big.NewInt(0),
			wantOK: true,
			hasFee: false,
		},
		{
			name:   "tier 2 absorbed explicit zero",
			stored: "0",
			want:   big.NewInt(0),
			wantOK: true,
			hasFee: false,
		},
		{
			name:   "tier 1 ETH locked fee in wei",
			stored: "1500000000000000", // 0.0015 ETH ≈ Tier 1 cancel buffer
			want:   mustBig("1500000000000000"),
			wantOK: true,
			hasFee: true,
		},
		{
			name:   "tier 1 BSC locked fee in wei",
			stored: "30000000000000000", // 0.03 BNB
			want:   mustBig("30000000000000000"),
			wantOK: true,
			hasFee: true,
		},
		{
			name:   "malformed string rejected",
			stored: "not-a-number",
			want:   big.NewInt(0),
			wantOK: false,
			hasFee: false,
		},
		{
			name:   "negative value rejected",
			stored: "-1",
			want:   big.NewInt(0),
			wantOK: false,
			hasFee: false,
		},
		{
			name:   "scientific notation rejected (only base-10 integers allowed)",
			stored: "1.5e15",
			want:   big.NewInt(0),
			wantOK: false,
			hasFee: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Order{CancelFeeAmount: tt.stored}
			got, ok := o.GetCancelFeeAmount()
			if ok != tt.wantOK {
				t.Fatalf("GetCancelFeeAmount ok=%v, want %v", ok, tt.wantOK)
			}
			if got.Cmp(tt.want) != 0 {
				t.Errorf("GetCancelFeeAmount value = %s, want %s", got.String(), tt.want.String())
			}
			if has := o.HasCancelFee(); has != tt.hasFee {
				t.Errorf("HasCancelFee = %v, want %v", has, tt.hasFee)
			}
		})
	}
}

// TestOrder_RefundAddress_Persistence verifies the new RefundAddress field
// can be set and retrieved (no validation at the model layer; D-Hybrid-27
// validation happens at App Service / handler layer).
func TestOrder_RefundAddress_Persistence(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{"empty allowed (validation upstream)", ""},
		{"DApp wallet EOA", "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"},
		{"CEX-deposit declared address", "0x1234567890AbcdEF1234567890aBcdef12345678"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Order{RefundAddress: tt.address}
			if got := o.RefundAddress; got != tt.address {
				t.Errorf("RefundAddress = %q, want %q", got, tt.address)
			}
		})
	}
}

func mustBig(s string) *big.Int {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("mustBig: invalid integer literal: " + s)
	}
	return v
}

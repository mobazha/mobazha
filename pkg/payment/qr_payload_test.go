package payment

import (
	"strings"
	"testing"
)

func TestBuildFundingQRPayload_UTXO(t *testing.T) {
	tests := []struct {
		name   string
		coin   string
		addr   string
		amount string
		want   string
	}{
		{
			name:   "btc",
			coin:   "crypto:bip122:000000000019d6689c085ae165831e93:native",
			addr:   "bc1qtest",
			amount: "0.001",
			want:   "bitcoin:bc1qtest?amount=0.001",
		},
		{
			name:   "bch",
			coin:   "crypto:bitcoincash:mainnet:native",
			addr:   "ppu9yncdpjgwmq8h5khefmkhrat6pdp08sqsjd0mrc",
			amount: "0.00016522",
			want:   "bitcoincash:ppu9yncdpjgwmq8h5khefmkhrat6pdp08sqsjd0mrc?amount=0.00016522",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildFundingQRPayload(tt.coin, tt.addr, tt.amount); got != tt.want {
				t.Fatalf("BuildFundingQRPayload() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildFundingQRPayload_EVMNative(t *testing.T) {
	const (
		coin = "crypto:eip155:1:native"
		addr = "0x259d0C6C6c53a746Fd8EA025AB5b47dfd842baCB"
	)
	amount := "0.011"
	want := "ethereum:0x259d0C6C6c53a746Fd8EA025AB5b47dfd842baCB@1?value=0.011e18"

	if got := BuildFundingQRPayload(coin, addr, amount); got != want {
		t.Fatalf("BuildFundingQRPayload() = %q, want %q", got, want)
	}
}

func TestBuildFundingQRPayload_EVMNativeWithoutAmount(t *testing.T) {
	got := BuildFundingQRPayload(
		"crypto:eip155:1:native",
		"0x259d0C6C6c53a746Fd8EA025AB5b47dfd842baCB",
		"",
	)
	want := "ethereum:0x259d0C6C6c53a746Fd8EA025AB5b47dfd842baCB@1"
	if got != want {
		t.Fatalf("BuildFundingQRPayload() = %q, want %q", got, want)
	}
}

func TestBuildFundingQRPayload_EVMERC20(t *testing.T) {
	const (
		coin = "crypto:eip155:1:erc20:0xdAC17F958D2ee523a2206206994597C13D831ec7"
		addr = "0x259d0C6C6c53a746Fd8EA025AB5b47dfd842baCB"
	)
	amount := "15.99"
	got := BuildFundingQRPayload(coin, addr, amount)
	wantPrefix := "ethereum:0xdAC17F958D2ee523a2206206994597C13D831ec7@1/transfer?"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("BuildFundingQRPayload() = %q, want prefix %q", got, wantPrefix)
	}
	if !strings.Contains(got, "address=0x259d0C6C6c53a746Fd8EA025AB5b47dfd842baCB") {
		t.Fatalf("BuildFundingQRPayload() = %q, missing recipient", got)
	}
	if !strings.Contains(got, "uint256=15990000") {
		t.Fatalf("BuildFundingQRPayload() = %q, missing uint256 atomic amount", got)
	}
}

func TestBuildFundingQRPayload_SolanaNative(t *testing.T) {
	got := BuildFundingQRPayload(
		"crypto:solana:mainnet:native",
		"7EqQDM5s8MWTD5M9s8MWTD5M9s8MWTD5M9s8MWTD5M9",
		"0.5",
	)
	want := "solana:7EqQDM5s8MWTD5M9s8MWTD5M9s8MWTD5M9s8MWTD5M9?amount=0.5"
	if got != want {
		t.Fatalf("BuildFundingQRPayload() = %q, want %q", got, want)
	}
}

func TestBuildFundingQRPayload_SolanaSPL(t *testing.T) {
	const mint = "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
	got := BuildFundingQRPayload(
		"crypto:solana:mainnet:spl:"+mint,
		"7EqQDM5s8MWTD5M9s8MWTD5M9s8MWTD5M9s8MWTD5M9",
		"100",
	)
	want := "solana:7EqQDM5s8MWTD5M9s8MWTD5M9s8MWTD5M9s8MWTD5M9?amount=100&spl-token=" + mint
	if got != want {
		t.Fatalf("BuildFundingQRPayload() = %q, want %q", got, want)
	}
}

func TestBuildFundingQRPayload_UnknownCoinReturnsEmpty(t *testing.T) {
	if got := BuildFundingQRPayload("crypto:unknown:mainnet:native", "addr", "1"); got != "" {
		t.Fatalf("BuildFundingQRPayload() = %q, want empty", got)
	}
}

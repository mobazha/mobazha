package models

import (
	"math/big"
	"strings"
	"testing"
)

// TestPaymentObservation_AmountBigInt covers the smallest-unit decimal-string
// codec used by PaymentObservation.Amount. The contract is critical: the
// monitor-driven payment aggregator sums these values, and any silent
// truncation at the schema boundary corrupts payment verification.
func TestPaymentObservation_AmountBigInt(t *testing.T) {
	// 256-bit max value (uint256.max). Round-tripping this exactly is the
	// reason we use TEXT/decimal-string instead of int64 / numeric scalars.
	uint256Max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

	tests := []struct {
		name   string
		stored string
		want   *big.Int
		wantOK bool
	}{
		{name: "empty string is missing", stored: "", want: nil, wantOK: false},
		{name: "zero", stored: "0", want: big.NewInt(0), wantOK: true},
		{name: "1 wei", stored: "1", want: big.NewInt(1), wantOK: true},
		{name: "1 ETH in wei", stored: "1000000000000000000", want: mustBig("1000000000000000000"), wantOK: true},
		{name: "uint256 max round-trip", stored: uint256Max.String(), want: uint256Max, wantOK: true},
		{name: "garbage decimal", stored: "1.5", want: nil, wantOK: false},
		{name: "negative not parsed as uint", stored: "abc", want: nil, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := &PaymentObservation{Amount: tt.stored}
			got, ok := obs.AmountBigInt()
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (stored=%q)", ok, tt.wantOK, tt.stored)
			}
			if !ok {
				return
			}
			if got.Cmp(tt.want) != 0 {
				t.Fatalf("got %s, want %s", got.String(), tt.want.String())
			}
		})
	}
}

// TestPaymentObservation_SetAmountBigInt_RoundTrip confirms encoder/decoder
// symmetry for representative chain amounts (BTC sat, ETH wei, XMR atomic
// units) plus the boundary uint256 max, then re-decodes via AmountBigInt
// to guard against asymmetric encoding bugs.
func TestPaymentObservation_SetAmountBigInt_RoundTrip(t *testing.T) {
	uint256Max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

	cases := []*big.Int{
		nil,
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(100_000_000),        // 1 BTC in sat
		mustBig("1000000000000000000"), // 1 ETH in wei
		mustBig("123456789012345678901234567890"), // > int64 range
		uint256Max, // overflow guard
	}

	for _, want := range cases {
		obs := &PaymentObservation{}
		obs.SetAmountBigInt(want)

		if want == nil {
			if obs.Amount != "" {
				t.Fatalf("nil should encode as empty string, got %q", obs.Amount)
			}
			if _, ok := obs.AmountBigInt(); ok {
				t.Fatal("AmountBigInt should report missing for nil round-trip")
			}
			continue
		}

		got, ok := obs.AmountBigInt()
		if !ok {
			t.Fatalf("AmountBigInt failed after SetAmountBigInt(%s) (stored=%q)", want.String(), obs.Amount)
		}
		if got.Cmp(want) != 0 {
			t.Fatalf("round-trip drift: got %s, want %s", got.String(), want.String())
		}
	}
}

// TestPaymentObservation_NilReceiver guards the convenience contract that
// AmountBigInt on a nil receiver does not panic and reports missing.
func TestPaymentObservation_NilReceiver(t *testing.T) {
	var obs *PaymentObservation
	got, ok := obs.AmountBigInt()
	if ok || got != nil {
		t.Fatalf("nil receiver should report missing, got (%v, %v)", got, ok)
	}
}

// TestPaymentObservation_TableName pins the GORM table name. The migration
// path (autoMigrateDatabase / autoMigrateDatabaseSafe in internal/repo/repo.go)
// and any hand-written SQL in MONITOR_DRIVEN_PAYMENT.md (DISTINCT ON / ROW_NUMBER
// aggregation queries) all assume this exact name. Renaming silently would
// drop the existing data in production.
func TestPaymentObservation_TableName(t *testing.T) {
	if got := (PaymentObservation{}).TableName(); got != "payment_observations" {
		t.Fatalf("TableName = %q, want %q (do not rename — see MONITOR_DRIVEN_PAYMENT.md §3.1)", got, "payment_observations")
	}
}

// TestPaymentObservation_KnownConstants pins the public-facing string values
// for source / status / event_type. These flow into JSON APIs and SQL filters
// (e.g. `WHERE source = 'monitor' AND status = 'confirmed'`); any reshuffling
// silently breaks downstream aggregation.
func TestPaymentObservation_KnownConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"source=monitor", PaymentObservationSourceMonitor, "monitor"},
		{"source=buyer_reported", PaymentObservationSourceBuyerReported, "buyer_reported"},
		{"status=pending", PaymentObservationStatusPending, "pending"},
		{"status=confirmed", PaymentObservationStatusConfirmed, "confirmed"},
		{"status=reverted", PaymentObservationStatusReverted, "reverted"},
		{"event=managed_escrow_received", PaymentEventManagedEscrowReceived, "managed_escrow_received"},
		{"event=erc20_transfer", PaymentEventERC20Transfer, "erc20_transfer"},
		{"event=xmr_deposit", PaymentEventXMRDeposit, "xmr_deposit"},
		{"event=utxo_funding", PaymentEventUTXOFunding, "utxo_funding"},
		{"event=solana_transfer", PaymentEventSolanaTransfer, "solana_transfer"},
		{"tx_hash_source=chain_tx", PaymentTxHashSourceChainTx, "chain_tx"},
		{"tx_hash_source=balance_poll", PaymentTxHashSourceBalancePoll, "balance_poll"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("constant drifted: got %q, want %q", tt.got, tt.want)
			}
			// Defense-in-depth: detect accidental whitespace / case noise.
			if tt.got != strings.TrimSpace(tt.got) || strings.ToLower(tt.got) != tt.got {
				t.Fatalf("constant %q must be lowercase, no whitespace", tt.got)
			}
		})
	}
}

func TestPaymentObservation_HasChainTxHash(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   bool
	}{
		{"empty defaults to chain tx", "", true},
		{"chain tx", PaymentTxHashSourceChainTx, true},
		{"balance poll", PaymentTxHashSourceBalancePoll, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PaymentObservation{TxHashSource: tt.source}.HasChainTxHash()
			if got != tt.want {
				t.Fatalf("HasChainTxHash = %v, want %v", got, tt.want)
			}
		})
	}
}

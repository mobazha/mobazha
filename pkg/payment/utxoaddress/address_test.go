package utxoaddress_test

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/payment/utxoaddress"
)

func TestSameUTXOAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b string
		want bool
	}{
		{"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", true},
		{"bitcoincash:qp0rg0n0", "qp0rg0n0", true},
		{"bitcoincash:qp0rg0n0", "bitcoincash:qp0rg0n0", true},
		{"qp0rg0n0", "different", false},
		{"", "qp0rg0n0", false},
	}

	for _, tc := range tests {
		if got := utxoaddress.SameUTXOAddress(tc.a, tc.b); got != tc.want {
			t.Fatalf("SameUTXOAddress(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

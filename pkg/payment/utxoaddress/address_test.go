package utxoaddress_test

import (
	"testing"

	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/mobazha/mobazha/pkg/payment/utxoaddress"
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
		{
			"tb1qtqv2u3dnxmsk2ys2q6cwhxh6xc0l28m5vxxun656nx43qmskhl5qcd035s",
			"bcrt1qtqv2u3dnxmsk2ys2q6cwhxh6xc0l28m5vxxun656nx43qmskhl5q459hp2",
			true,
		},
		{
			"tb1qtqv2u3dnxmsk2ys2q6cwhxh6xc0l28m5vxxun656nx43qmskhl5qcd035s",
			"bcrt1q509lfnvktlenk5cectemfjkmthv3kcflvata8yxxvj5qs2t0xt5snaptmn",
			false,
		},
		{"qp0rg0n0", "different", false},
		{"", "qp0rg0n0", false},
	}

	for _, tc := range tests {
		if got := utxoaddress.SameUTXOAddress(tc.a, tc.b); got != tc.want {
			t.Fatalf("SameUTXOAddress(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}

	const testnetAddress = "tb1qtqv2u3dnxmsk2ys2q6cwhxh6xc0l28m5vxxun656nx43qmskhl5qcd035s"
	_, witnessProgram, version, err := bech32.DecodeGeneric(testnetAddress)
	if err != nil {
		t.Fatalf("decode testnet witness address: %v", err)
	}
	var mainnetAddress string
	switch version {
	case bech32.Version0:
		mainnetAddress, err = bech32.Encode("bc", witnessProgram)
	case bech32.VersionM:
		mainnetAddress, err = bech32.EncodeM("bc", witnessProgram)
	default:
		t.Fatalf("unexpected bech32 version: %v", version)
	}
	if err != nil {
		t.Fatalf("encode equivalent mainnet witness address: %v", err)
	}
	if utxoaddress.SameUTXOAddress(testnetAddress, mainnetAddress) {
		t.Fatalf("mainnet and testnet witness addresses must not compare equal: %s vs %s", mainnetAddress, testnetAddress)
	}
}

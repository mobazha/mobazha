package payment

import "testing"

func TestSameCryptoAddress_UsesRailSemantics(t *testing.T) {
	tests := []struct {
		name        string
		assetID     string
		left, right string
		want        bool
	}{
		{name: "EVM ignores checksum case", assetID: "crypto:eip155:1:native", left: "0xAbCd", right: "0xabcd", want: true},
		{name: "Solana is exact", assetID: "crypto:solana:mainnet:native", left: "AbCd", right: "abcd", want: false},
		{name: "Solana exact match", assetID: "crypto:solana:mainnet:native", left: "AbCd", right: "AbCd", want: true},
		{name: "UTXO rejects mainnet testnet aliases", assetID: "crypto:bip122:000000000019d6689c085ae165831e93:native", left: "bc1qexample", right: "tb1qexample", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SameCryptoAddress(test.assetID, test.left, test.right); got != test.want {
				t.Fatalf("SameCryptoAddress() = %v, want %v", got, test.want)
			}
		})
	}
}

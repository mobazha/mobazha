//go:build !private_distribution

package payment

import (
	"testing"

	"github.com/stretchr/testify/require"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestUtxoObservationChainRef_CanonicalAndLegacy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		coin      iwallet.CoinType
		namespace string
		chainRef  string
	}{
		{
			coin:      iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native"),
			namespace: "bip122",
			chainRef:  "000000000019d6689c085ae165831e93",
		},
		{
			coin:      iwallet.CoinType("btc"),
			namespace: "bip122",
			chainRef:  "000000000019d6689c085ae165831e93",
		},
		{
			coin:      iwallet.CoinType("crypto:bitcoincash:mainnet:native"),
			namespace: "bitcoincash",
			chainRef:  "mainnet",
		},
		{
			coin:      iwallet.CoinType("bch"),
			namespace: "bitcoincash",
			chainRef:  "mainnet",
		},
		{
			coin:      iwallet.CoinType("crypto:zcash:mainnet:native"),
			namespace: "zcash",
			chainRef:  "mainnet",
		},
		{
			coin:      iwallet.CoinType("ltc"),
			namespace: "bip122",
			chainRef:  "12a765e31ffd4059bada1e25190f6e98",
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.coin), func(t *testing.T) {
			ns, ref, ok := utxoObservationChainRef(tc.coin)
			require.True(t, ok)
			require.Equal(t, tc.namespace, ns)
			require.Equal(t, tc.chainRef, ref)
		})
	}
}

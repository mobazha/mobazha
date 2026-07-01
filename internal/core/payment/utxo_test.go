package payment

import (
	"encoding/hex"
	"testing"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

func TestPaymentOutputsForAddressUseOutpointIndexForObservationEvent(t *testing.T) {
	outpoint0, err := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f00000000")
	require.NoError(t, err)
	outpoint2, err := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f02000000")
	require.NoError(t, err)

	tx := &iwallet.Transaction{
		ID: iwallet.TransactionID("txid"),
		To: []iwallet.SpendInfo{
			{
				ID:      outpoint0,
				Address: iwallet.NewAddress("payment-address", iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native")),
				Amount:  iwallet.NewAmount(100),
			},
			{
				ID:      outpoint2,
				Address: iwallet.NewAddress("payment-address", iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native")),
				Amount:  iwallet.NewAmount(200),
			},
		},
	}

	outputs := paymentOutputsForAddress(tx, "payment-address")
	require.Len(t, outputs, 2)
	require.Equal(t, 0, outputs[0].eventIndex)
	require.Equal(t, 2, outputs[1].eventIndex)
	require.Equal(t, uint64(100), outputs[0].spend.Amount.Uint64())
	require.Equal(t, uint64(200), outputs[1].spend.Amount.Uint64())
}

package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPendingEscrowPaymentInfo_RoundTrip(t *testing.T) {
	order := &Order{}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&PendingEscrowPaymentInfo{
		Coin:          "crypto:eip155:1:native",
		EscrowAddress: "0xescrow",
		SettlementSpec: &PendingSettlementSpec{
			Method:     "CANCELABLE",
			PayMode:    "client_signed",
			EscrowType: "evm_contract",
		},
	}))

	got, err := order.GetPendingEscrowPaymentInfo()
	require.NoError(t, err)
	require.Equal(t, "escrow", got.Type)
	require.Equal(t, "0xescrow", got.EscrowAddress)

	// UTXO getter must not mis-read escrow JSON.
	utxo, err := order.GetPendingPaymentInfo()
	require.NoError(t, err)
	require.Nil(t, utxo)
}

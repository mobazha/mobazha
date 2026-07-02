package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPaymentIntentGetPendingManagedEscrowInfo_InvalidJSONErrors(t *testing.T) {
	intent := &SharedPaymentIntent{PendingPaymentInfo: []byte(`{`)}

	info, err := intent.GetPendingManagedEscrowInfo()
	require.Error(t, err)
	require.Nil(t, info)
}

func TestSharedPaymentIntentHydrateOrder_InvalidOrderPendingInfoErrors(t *testing.T) {
	intent := &SharedPaymentIntent{}
	require.NoError(t, intent.SetPendingManagedEscrowInfo(&PendingManagedEscrowInfo{
		Coin:    "crypto:eip155:1:native",
		Address: "0xmanagedescrow",
	}))
	order := &Order{PendingPaymentInfo: []byte(`{`)}

	err := intent.HydrateOrder(order)
	require.Error(t, err)
}

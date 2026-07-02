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

func TestSharedPaymentIntentGetPendingManagedEscrowInfo_LegacyTypedRoute(t *testing.T) {
	intent := &SharedPaymentIntent{PendingPaymentInfo: []byte(`{
		"type":"legacy_typed_route",
		"coin":"crypto:eip155:1:native",
		"address":"0xabc",
		"settlementSpec":{
			"method":"MODERATED",
			"payMode":"address_monitored",
			"escrowType":"managed"
		}
	}`)}

	info, err := intent.GetPendingManagedEscrowInfo()
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, pendingManagedEscrowType, info.Type)
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

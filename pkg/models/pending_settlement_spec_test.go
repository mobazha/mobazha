package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPendingManagedEscrowInfo_SettlementSpecJSONRoundTrip(t *testing.T) {
	order := &Order{}
	require.NoError(t, order.SetPendingManagedEscrowInfo(&PendingManagedEscrowInfo{
		Type:      "managed_escrow",
		Coin:      "crypto:eth:eth",
		Address:   "0xabc",
		Moderated: true,
		SettlementSpec: &PendingSettlementSpec{
			Method:     "MODERATED",
			PayMode:    "address_monitored",
			EscrowType: "managed",
		},
	}))

	got, err := order.GetPendingManagedEscrowInfo()
	require.NoError(t, err)
	require.NotNil(t, got.SettlementSpec)
	require.Equal(t, "MODERATED", got.SettlementSpec.Method)

	raw, err := json.Marshal(got)
	require.NoError(t, err)
	var decoded PendingManagedEscrowInfo
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, got.SettlementSpec, decoded.SettlementSpec)
}

func TestPendingManagedEscrowInfo_LegacyTypedRouteIsRecognizedStructurally(t *testing.T) {
	order := &Order{PendingPaymentInfo: []byte(`{
		"type":"legacy_typed_route",
		"coin":"crypto:eip155:1:native",
		"address":"0xabc",
		"settlementSpec":{
			"method":"CANCELABLE",
			"payMode":"address_monitored",
			"escrowType":"managed"
		}
	}`)}

	got, err := order.GetPendingManagedEscrowInfo()
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, pendingManagedEscrowType, got.Type)
	require.Equal(t, "0xabc", got.Address)
}

func TestPendingManagedEscrowInfo_UnrelatedTypedRouteIsIgnored(t *testing.T) {
	order := &Order{PendingPaymentInfo: []byte(`{
		"type":"another_route",
		"coin":"crypto:solana:mainnet:native",
		"address":"example-address",
		"settlementSpec":{
			"method":"CANCELABLE",
			"payMode":"address_monitored",
			"escrowType":"program_escrow"
		}
	}`)}

	got, err := order.GetPendingManagedEscrowInfo()
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestPendingUTXOPaymentInfo_SettlementSpecJSONRoundTrip(t *testing.T) {
	order := &Order{}
	require.NoError(t, order.SetPendingPaymentInfo(&PendingUTXOPaymentInfo{
		Coin:   "BTC",
		Script: "5221...",
		SettlementSpec: &PendingSettlementSpec{
			Method:     "MODERATED",
			PayMode:    "address_monitored",
			EscrowType: "utxo_script",
		},
	}))

	got, err := order.GetPendingPaymentInfo()
	require.NoError(t, err)
	require.NotNil(t, got.SettlementSpec)
	require.Equal(t, "MODERATED", got.SettlementSpec.Method)

	raw, err := json.Marshal(got)
	require.NoError(t, err)
	var decoded PendingUTXOPaymentInfo
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, got.SettlementSpec, decoded.SettlementSpec)
}

func TestPendingUTXOPaymentInfo_LegacyJSONWithoutSpec(t *testing.T) {
	order := &Order{}
	legacy := `{"coin":"BTC","moderator":"mod-peer","script":"ab12"}`
	order.PendingPaymentInfo = []byte(legacy)

	info, err := order.GetPendingPaymentInfo()
	require.NoError(t, err)
	require.Nil(t, info.SettlementSpec)
	require.Equal(t, "mod-peer", info.Moderator)
}

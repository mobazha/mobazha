package models

import (
	"encoding/json"
	"testing"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

func TestValidateRefundReceivingAddresses_EmptyMap(t *testing.T) {
	require.NoError(t, ValidateRefundReceivingAddresses(nil))
	require.NoError(t, ValidateRefundReceivingAddresses(map[string]string{}))
}

func TestValidateRefundReceivingAddresses_ValidEVM(t *testing.T) {
	err := ValidateRefundReceivingAddresses(map[string]string{
		"crypto:eip155:1:native": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
	})
	require.NoError(t, err)
}

func TestValidateRefundReceivingAddresses_InvalidAddress(t *testing.T) {
	err := ValidateRefundReceivingAddresses(map[string]string{
		"crypto:eip155:1:native": "not-an-address",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrRefundAddressInvalid)
}

func TestValidateRefundReceivingAddresses_InvalidCoin(t *testing.T) {
	err := ValidateRefundReceivingAddresses(map[string]string{
		"bogus-coin": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
	})
	require.Error(t, err)
}

func TestUserPreferences_RefundReceivingAddressesRoundTrip(t *testing.T) {
	var prefs UserPreferences
	require.NoError(t, prefs.SetRefundReceivingAddresses(map[string]string{
		string(iwallet.CoinType("crypto:eip155:1:native")): "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
	}))

	addrs, err := prefs.RefundReceivingAddresses()
	require.NoError(t, err)
	require.Equal(t, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", addrs["crypto:eip155:1:native"])
}

func TestUserPreferences_SetRefundReceivingAddressesCanonicalizesKeys(t *testing.T) {
	var prefs UserPreferences
	require.NoError(t, prefs.SetRefundReceivingAddresses(map[string]string{
		"ETH": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
	}))

	addrs, err := prefs.RefundReceivingAddresses()
	require.NoError(t, err)
	require.Equal(t, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", addrs["crypto:eip155:1:native"])
}

func TestUserPreferences_UnmarshalJSONCanonicalizesRefundReceivingAddresses(t *testing.T) {
	var prefs UserPreferences
	require.NoError(t, json.Unmarshal([]byte(`{
		"refundReceivingAddresses": {
			"ETH": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
		}
	}`), &prefs))

	addrs, err := prefs.RefundReceivingAddresses()
	require.NoError(t, err)
	require.Equal(t, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", addrs["crypto:eip155:1:native"])
	require.NotContains(t, addrs, "ETH")
}

func TestUserPreferences_RefundReceivingAddressesJSONRoundTrip(t *testing.T) {
	var prefs UserPreferences
	require.NoError(t, prefs.SetRefundReceivingAddresses(map[string]string{
		"crypto:bip122:000000000019d6689c085ae165831e93:native": "bc1qnsk5lxk26kqlt8up3l728f0men3ewfe8ds0ev5",
		"crypto:eip155:1:native":                                "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
	}))

	raw, err := json.Marshal(&prefs)
	require.NoError(t, err)

	var decoded UserPreferences
	require.NoError(t, json.Unmarshal(raw, &decoded))

	addrs, err := decoded.RefundReceivingAddresses()
	require.NoError(t, err)
	require.Len(t, addrs, 2)
	require.Equal(t, "bc1qnsk5lxk26kqlt8up3l728f0men3ewfe8ds0ev5", addrs["crypto:bip122:000000000019d6689c085ae165831e93:native"])
}

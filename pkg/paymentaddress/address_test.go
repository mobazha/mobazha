package paymentaddress

import (
	"errors"
	"testing"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

const (
	testETH = iwallet.CoinType("crypto:eip155:1:native")
	testBTC = iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native")
	testSOL = iwallet.CoinType("crypto:solana:mainnet:native")
)

func TestValidate_AddressFamilies(t *testing.T) {
	require.NoError(t, Validate(testETH, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"))
	require.ErrorIs(t, Validate(testETH, "0x0000000000000000000000000000000000000000"), ErrInvalid)
	require.NoError(t, Validate(testBTC, "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4"))
	require.NoError(t, Validate(testSOL, "11111111111111111111111111111111"))
	require.ErrorIs(t, Validate(testSOL, "not-a-pubkey"), ErrInvalid)
	require.NoError(t, Validate("fiat:stripe:USD", ""))
	require.True(t, errors.Is(Validate(testETH, ""), ErrRequired))
}

func TestValidatePaymentCoinAddressMap(t *testing.T) {
	require.NoError(t, ValidatePaymentCoinAddressMap(map[string]string{
		" ETH ": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
		"BTC":   "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		"SOL":   "",
	}))

	require.ErrorIs(t, ValidatePaymentCoinAddressMap(map[string]string{
		"": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
	}), ErrInvalid)
	require.ErrorIs(t, ValidatePaymentCoinAddressMap(map[string]string{
		"bogus-coin": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
	}), ErrInvalid)
	require.ErrorIs(t, ValidatePaymentCoinAddressMap(map[string]string{
		"ETH": "not-an-address",
	}), ErrInvalid)
}

func TestLookupByPaymentCoin(t *testing.T) {
	require.Equal(t, "0xexact", LookupByPaymentCoin(map[string]string{
		string(testETH): " 0xexact ",
	}, testETH))

	require.Equal(t, "0xnormalized", LookupByPaymentCoin(map[string]string{
		string(testETH): "0xnormalized",
	}, "ETH"))

	require.Equal(t, "0xlegacy-key", LookupByPaymentCoin(map[string]string{
		"ETH": "0xlegacy-key",
	}, testETH))

	require.Empty(t, LookupByPaymentCoin(map[string]string{
		string(testETH): "0xnormalized",
	}, "USDC"))
}

func TestCanonicalizePaymentCoinAddressMap(t *testing.T) {
	out, err := CanonicalizePaymentCoinAddressMap(map[string]string{
		" ETH ": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
		"BTC":   "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		"SOL":   "",
	})
	require.NoError(t, err)
	require.Equal(t, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", out[string(testETH)])
	require.Equal(t, "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4", out[string(testBTC)])
	require.NotContains(t, out, "SOL")

	_, err = CanonicalizePaymentCoinAddressMap(map[string]string{
		"": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
	})
	require.ErrorIs(t, err, ErrInvalid)

	_, err = CanonicalizePaymentCoinAddressMap(map[string]string{
		"ETH":                    "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
		"crypto:eip155:1:native": "0x0000000000000000000000000000000000000001",
	})
	require.ErrorIs(t, err, ErrInvalid)
}

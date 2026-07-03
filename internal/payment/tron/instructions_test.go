package tronpayment

import (
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
	evm "github.com/mobazha/mobazha/internal/chains/evm"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tronUSDTAssetID = iwallet.CoinType("crypto:tron:mainnet:trc20:TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")

func TestBuildEscrowReleaseParams_Success(t *testing.T) {
	tos := []iwallet.SpendInfo{
		{
			Address: iwallet.NewAddress("0x1234567890abcdef1234567890abcdef12345678", tronUSDTAssetID),
			Amount:  iwallet.NewAmount(1000000),
		},
	}

	script := buildTestScript(t)

	receivers, amounts, message, err := BuildEscrowReleaseParams(tos, script)
	require.NoError(t, err)
	assert.Len(t, receivers, 1)
	assert.Len(t, amounts, 1)
	assert.Equal(t, uint64(1000000), amounts[0])
	assert.Len(t, message, 32)
}

func TestSignEscrowRelease_Success(t *testing.T) {
	tronKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	tos := []iwallet.SpendInfo{
		{
			Address: iwallet.NewAddress("0x1234567890abcdef1234567890abcdef12345678", tronUSDTAssetID),
			Amount:  iwallet.NewAmount(500000),
		},
	}

	script := buildTestScript(t)

	sigs, err := SignEscrowRelease(tos, script, tronKey)
	require.NoError(t, err)
	assert.Len(t, sigs, 1)
	assert.Equal(t, 0, sigs[0].Index)
	assert.Len(t, sigs[0].Signature, 65) // secp256k1: R(32) + S(32) + V(1)
}

func TestSignEscrowRelease_TwoRecipients(t *testing.T) {
	tronKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	tos := []iwallet.SpendInfo{
		{
			Address: iwallet.NewAddress("0x1111111111111111111111111111111111111111", tronUSDTAssetID),
			Amount:  iwallet.NewAmount(900000),
		},
		{
			Address: iwallet.NewAddress("0x2222222222222222222222222222222222222222", tronUSDTAssetID),
			Amount:  iwallet.NewAmount(100000),
		},
	}

	script := buildTestScript(t)

	sigs, err := SignEscrowRelease(tos, script, tronKey)
	require.NoError(t, err)
	assert.Len(t, sigs, 1)
	assert.Len(t, sigs[0].Signature, 65)
}

func TestSignEscrowRelease_DifferentKeys_DifferentSignatures(t *testing.T) {
	key1, _ := btcec.NewPrivateKey()
	key2, _ := btcec.NewPrivateKey()

	tos := []iwallet.SpendInfo{
		{
			Address: iwallet.NewAddress("0x1234567890abcdef1234567890abcdef12345678", tronUSDTAssetID),
			Amount:  iwallet.NewAmount(500000),
		},
	}
	script := buildTestScript(t)

	sigs1, err := SignEscrowRelease(tos, script, key1)
	require.NoError(t, err)

	sigs2, err := SignEscrowRelease(tos, script, key2)
	require.NoError(t, err)

	assert.NotEqual(t, sigs1[0].Signature, sigs2[0].Signature)
}

// buildTestScript creates a minimal valid EthRedeemScript for testing.
func buildTestScript(t *testing.T) []byte {
	t.Helper()

	script := evm.EthRedeemScript{
		UniqueID:        common.HexToAddress("0x0000000000000000000000000000000000000001"),
		Threshold:       1,
		Timeout:         86400,
		Buyer:           common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		Seller:          common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		Moderator:       common.HexToAddress("0xcccccccccccccccccccccccccccccccccccccccc"),
		ContractAddress: common.HexToAddress("0xdddddddddddddddddddddddddddddddddddddd"),
		TokenAddress:    common.HexToAddress("0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"),
	}

	serialized, err := evm.SerializeEthScript(script)
	require.NoError(t, err)
	return serialized
}

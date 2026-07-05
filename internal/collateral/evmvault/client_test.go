// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package evmvault

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/stretchr/testify/require"
)

func TestExecutionDigestMatchesSolidityGoldenValues(t *testing.T) {
	release := ExecutionCommand{
		Request:       pkgcollateral.RailExecutionRequest{Kind: pkgcollateral.ExecutionRelease},
		CollateralKey: common.HexToHash("0x01"), ActionKey: common.HexToHash("0x02"),
		Destination: testPrincipal, Amount: big.NewInt(100),
	}
	digest, err := executionDigest(release)
	require.NoError(t, err)
	require.Equal(t, "0xe78a582d9486ed50ed95188f3b4fc4d1e02a37132d35c332e33fa0d60a160d04", common.BytesToHash(digest[:]).Hex())

	slash := release
	slash.Request.Kind = pkgcollateral.ExecutionSlash
	slash.Destination = testBuyer
	slash.Amount = big.NewInt(25)
	digest, err = executionDigest(slash)
	require.NoError(t, err)
	require.Equal(t, "0xd74d613cfe972ad9f0c5252a2b045709ff535bfc1397b3b118c0fc1d9354bddd", common.BytesToHash(digest[:]).Hex())
}

func TestFundingCallDataUsesVaultABI(t *testing.T) {
	collateralKey := common.HexToHash("0x01")
	fundingKey := common.HexToHash("0x02")
	client := &BindingClient{}
	callData, err := client.FundingCallData(collateralKey, fundingKey, big.NewInt(125))
	require.NoError(t, err)

	parsed, err := CollateralVaultMetaData.GetAbi()
	require.NoError(t, err)
	require.Equal(t, parsed.Methods["fund"].ID, callData[:4])
	values, err := parsed.Methods["fund"].Inputs.Unpack(callData[4:])
	require.NoError(t, err)
	require.Equal(t, [32]byte(collateralKey), values[0])
	require.Equal(t, [32]byte(fundingKey), values[1])
	require.Zero(t, big.NewInt(125).Cmp(values[2].(*big.Int)))
}

func TestVerifyExecutionObligationEnforcesCoreResidualInvariant(t *testing.T) {
	base := CollateralVaultObligation{
		Principal: testPrincipal, State: obligationFunded,
		RequiredAmount: big.NewInt(100), Balance: big.NewInt(120),
	}
	slash := ExecutionCommand{
		Request:     pkgcollateral.RailExecutionRequest{Kind: pkgcollateral.ExecutionSlash},
		Destination: testBuyer, Amount: big.NewInt(20),
	}
	require.NoError(t, verifyExecutionObligation(base, slash))

	slash.Amount = big.NewInt(21)
	require.ErrorContains(t, verifyExecutionObligation(base, slash), "underfunded residual")
	slash.Amount = big.NewInt(120)
	require.NoError(t, verifyExecutionObligation(base, slash), "a full slash is terminal")

	release := ExecutionCommand{
		Request:     pkgcollateral.RailExecutionRequest{Kind: pkgcollateral.ExecutionRelease},
		Destination: testPrincipal, Amount: big.NewInt(120),
	}
	require.NoError(t, verifyExecutionObligation(base, release))
	release.Destination = testBuyer
	require.ErrorContains(t, verifyExecutionObligation(base, release), "principal residual")
}

func TestVerifyObligationRejectsImmutableBindingChanges(t *testing.T) {
	command := ObligationCommand{
		Principal: testPrincipal, Amount: big.NewInt(100), ExpiresAt: 1_800_000_000,
	}
	obligation := CollateralVaultObligation{
		Principal: testPrincipal, State: obligationOpen,
		RequiredAmount: big.NewInt(100), ExpiresAt: command.ExpiresAt, Balance: big.NewInt(0),
	}
	require.NoError(t, verifyObligation(obligation, command, true))

	obligation.ExpiresAt++
	require.ErrorContains(t, verifyObligation(obligation, command, true), "binding mismatch")
	obligation.ExpiresAt = command.ExpiresAt
	obligation.State = obligationFunded
	require.ErrorContains(t, verifyObligation(obligation, command, true), "not open")
}

func TestReceiptContainsExactCanonicalLog(t *testing.T) {
	expected := types.Log{
		Address: testVault, Topics: []common.Hash{common.HexToHash("0x01"), common.HexToHash("0x02")},
		Data: []byte{0xaa, 0xbb}, TxHash: common.HexToHash("0x03"), Index: 4,
	}
	receipt := &types.Receipt{Logs: []*types.Log{&expected}}
	require.True(t, receiptContainsLog(receipt, expected))

	tampered := expected
	tampered.Data = []byte{0xaa, 0xbc}
	require.False(t, receiptContainsLog(receipt, tampered))
	tampered = expected
	tampered.Topics = append([]common.Hash(nil), expected.Topics...)
	tampered.Topics[1] = common.HexToHash("0xff")
	require.False(t, receiptContainsLog(receipt, tampered))
}

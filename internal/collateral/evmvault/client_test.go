// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package evmvault

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/stretchr/testify/require"
)

type receiptVerificationBackend struct {
	Backend
	receipt *types.Receipt
	header  *types.Header
	head    uint64
}

func (b receiptVerificationBackend) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	return b.receipt, nil
}

func (b receiptVerificationBackend) HeaderByNumber(context.Context, *big.Int) (*types.Header, error) {
	return b.header, nil
}

func (b receiptVerificationBackend) BlockNumber(context.Context) (uint64, error) {
	return b.head, nil
}

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

func TestVerifyEventRejectsReceiptFromNonCanonicalBlock(t *testing.T) {
	canonicalHeader := &types.Header{Number: big.NewInt(10), Time: 1_800_000_000, Extra: []byte("canonical")}
	orphanedHeader := &types.Header{Number: big.NewInt(10), Time: 1_799_999_999, Extra: []byte("orphaned")}
	event := types.Log{
		Address: testVault, TxHash: common.HexToHash("0x01"), BlockHash: orphanedHeader.Hash(),
		BlockNumber: 10, Index: 1,
	}
	receipt := &types.Receipt{
		Status: types.ReceiptStatusSuccessful, BlockHash: event.BlockHash, BlockNumber: big.NewInt(10),
		Logs: []*types.Log{&event},
	}
	client := &BindingClient{
		config:  Config{VaultAddress: testVault, Confirmations: 3},
		backend: receiptVerificationBackend{receipt: receipt, header: canonicalHeader, head: 12},
	}

	confirmed, confirmations, observedAt, err := client.verifyEvent(context.Background(), event)
	require.ErrorContains(t, err, "receipt block is not canonical")
	require.False(t, confirmed)
	require.Zero(t, confirmations)
	require.Equal(t, time.Time{}, observedAt)
}

func TestVerifyEventAcceptsReceiptBoundToCanonicalBlock(t *testing.T) {
	header := &types.Header{Number: big.NewInt(10), Time: 1_800_000_000, Extra: []byte("canonical")}
	event := types.Log{
		Address: testVault, TxHash: common.HexToHash("0x02"), BlockHash: header.Hash(),
		BlockNumber: 10, Index: 2,
	}
	receipt := &types.Receipt{
		Status: types.ReceiptStatusSuccessful, BlockHash: event.BlockHash, BlockNumber: big.NewInt(10),
		Logs: []*types.Log{&event},
	}
	client := &BindingClient{
		config:  Config{VaultAddress: testVault, Confirmations: 3},
		backend: receiptVerificationBackend{receipt: receipt, header: header, head: 12},
	}

	confirmed, confirmations, observedAt, err := client.verifyEvent(context.Background(), event)
	require.NoError(t, err)
	require.True(t, confirmed)
	require.Equal(t, uint64(3), confirmations)
	require.Equal(t, time.Unix(int64(header.Time), 0).UTC(), observedAt)
}

func TestWaitSuccessfulReceiptRejectsNonCanonicalBlock(t *testing.T) {
	canonicalHeader := &types.Header{Number: big.NewInt(10), Extra: []byte("canonical")}
	orphanedHeader := &types.Header{Number: big.NewInt(10), Extra: []byte("orphaned")}
	receipt := &types.Receipt{
		Status: types.ReceiptStatusSuccessful, BlockHash: orphanedHeader.Hash(), BlockNumber: big.NewInt(10),
	}
	client := &BindingClient{
		config:       Config{Confirmations: 1},
		backend:      receiptVerificationBackend{receipt: receipt, header: canonicalHeader, head: 10},
		pollInterval: time.Millisecond,
	}

	_, err := client.waitSuccessfulReceipt(context.Background(), common.HexToHash("0x03"))
	require.ErrorContains(t, err, "transaction block is not canonical")
}

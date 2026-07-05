// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package evmvault

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/stretchr/testify/require"
)

const testAssetID = "crypto:eip155:56:erc20:0x55d398326f99059fF775485246999027B3197955"

var (
	testToken     = common.HexToAddress("0x55d398326f99059fF775485246999027B3197955")
	testVault     = common.HexToAddress("0x1111111111111111111111111111111111111111")
	testOperator  = common.HexToAddress("0x2222222222222222222222222222222222222222")
	testPrincipal = common.HexToAddress("0x3333333333333333333333333333333333333333")
	testBuyer     = common.HexToAddress("0x4444444444444444444444444444444444444444")
)

func TestRailPreparesFullyBoundFundingTarget(t *testing.T) {
	client := &fakeVaultClient{callData: []byte{0xde, 0xad, 0xbe, 0xef}}
	rail := newTestRail(t, client)
	now := time.Unix(1_800_000_000, 0).UTC()
	rail.now = func() time.Time { return now }
	request := testFundingRequest(now)

	target, err := rail.PrepareFunding(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, request.PrincipalDestination, target.PrincipalDestination)
	require.Equal(t, request.IdempotencyKey, target.IdempotencyKey)
	require.Equal(t, testVault.Hex(), target.Destination)
	require.Equal(t, domainKey("collateral", request.TenantID, request.CollateralID), client.obligation.CollateralKey)
	require.Equal(t, domainKey("funding", request.TenantID, request.CollateralID, request.IdempotencyKey), client.fundingKey)
	require.Equal(t, testPrincipal, client.obligation.Principal)
	require.Equal(t, "100", client.obligation.Amount.String())

	var payload fundingPayload
	require.NoError(t, json.Unmarshal(target.Payload, &payload))
	require.Equal(t, "0xdeadbeef", payload.CallData)
	require.Equal(t, testVault.Hex(), payload.ApprovalSpender)
	require.Equal(t, "100", payload.ApprovalAmount)

	encoded, err := json.Marshal(target)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), request.IdempotencyKey)
	require.NotContains(t, string(encoded), "principalDestination")
}

func TestRailFundingStatusVerifiesBindingsAndWorksWhilePaused(t *testing.T) {
	observedAt := time.Unix(1_800_000_010, 0).UTC()
	client := &fakeVaultClient{
		callData: []byte{0xde, 0xad, 0xbe, 0xef},
		status: pkgcollateral.RailFundingStatus{
			State: pkgcollateral.RailActionConfirmed, Reference: "0xfunding",
			Amount: "100", ObservedAt: observedAt,
		},
		readyErr: errors.New("vault is paused"),
	}
	rail := newTestRail(t, client)
	now := time.Unix(1_800_000_000, 0).UTC()
	rail.now = func() time.Time { return now }
	client.readyErr = nil
	target, err := rail.PrepareFunding(context.Background(), testFundingRequest(now))
	require.NoError(t, err)
	client.readyErr = errors.New("vault is paused")
	readyCalls := client.readyCalls

	status, err := rail.FundingStatus(context.Background(), target)
	require.NoError(t, err)
	require.Equal(t, pkgcollateral.RailActionConfirmed, status.State)
	require.Equal(t, testAssetID, status.AssetID)
	require.Equal(t, readyCalls, client.readyCalls, "read-only reconciliation must remain available while paused")
	require.Equal(t, domainKey("collateral", target.TenantID, target.CollateralID), client.query.CollateralKey)
	require.Equal(t, domainKey("funding", target.TenantID, target.CollateralID, target.IdempotencyKey), client.query.FundingKey)

	var payload fundingPayload
	require.NoError(t, json.Unmarshal(target.Payload, &payload))
	payload.CallData = "0x00"
	target.Payload, err = json.Marshal(payload)
	require.NoError(t, err)
	_, err = rail.FundingStatus(context.Background(), target)
	require.ErrorContains(t, err, "calldata binding mismatch")
}

func TestRailExecutionPassesCompleteImmutableRequest(t *testing.T) {
	observedAt := time.Unix(1_800_000_020, 0).UTC()
	client := &fakeVaultClient{}
	rail := newTestRail(t, client)
	request := pkgcollateral.RailExecutionRequest{
		ActionID: "slash-1", TenantID: "tenant-1", CollateralID: "collateral-1", ClaimID: "claim-1",
		Kind: pkgcollateral.ExecutionSlash, AssetID: testAssetID, Amount: "25", Destination: testBuyer.Hex(),
		ExpectedRevision: 4, IdempotencyKey: "slash-idempotency-1",
	}
	client.submitResult = pkgcollateral.RailActionResult{
		ActionID: request.ActionID, State: pkgcollateral.RailActionConfirmed,
		Reference: "0xslash", ObservedAt: observedAt,
	}

	result, err := rail.SubmitExecution(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, request, client.execution.Request)
	require.Equal(t, domainKey("collateral", request.TenantID, request.CollateralID), client.execution.CollateralKey)
	require.Equal(t, executionKey(request), client.execution.ActionKey)
	require.Equal(t, testBuyer, client.execution.Destination)
	require.Equal(t, "25", client.execution.Amount.String())
	require.Equal(t, client.submitResult, result)
	changedClaim := request
	changedClaim.ClaimID = "claim-2"
	require.NotEqual(t, executionKey(request), executionKey(changedClaim))

	client.executionResult = pkgcollateral.RailActionResult{
		ActionID: "different-action", State: pkgcollateral.RailActionConfirmed,
		Reference: "0xslash", ObservedAt: observedAt,
	}
	_, err = rail.ExecutionStatus(context.Background(), request)
	require.ErrorContains(t, err, "action mismatch")
}

func TestNewRailRejectsAssetAndAddressMismatches(t *testing.T) {
	config := testConfig()
	config.ChainID = 1
	_, err := NewRail(config, &fakeVaultClient{})
	require.ErrorContains(t, err, "does not match")

	config = testConfig()
	config.StartBlock = 0
	_, err = NewRail(config, &fakeVaultClient{})
	require.ErrorContains(t, err, "start block")

	config = testConfig()
	rail, err := NewRail(config, &fakeVaultClient{})
	require.NoError(t, err)
	request := testFundingRequest(time.Now().UTC())
	request.PrincipalDestination = strings.ToLower(testToken.Hex())
	_, err = rail.PrepareFunding(context.Background(), request)
	require.ErrorContains(t, err, "checksum")
}

func newTestRail(t *testing.T, client VaultClient) *Rail {
	t.Helper()
	rail, err := NewRail(testConfig(), client)
	require.NoError(t, err)
	return rail
}

func testConfig() Config {
	return Config{
		AssetID: testAssetID, ChainID: 56, VaultAddress: testVault, TokenAddress: testToken,
		OperatorAddress: testOperator, StartBlock: 10, Confirmations: 2,
	}
}

func testFundingRequest(now time.Time) pkgcollateral.FundingTargetRequest {
	return pkgcollateral.FundingTargetRequest{
		TenantID: "tenant-1", CollateralID: "collateral-1", PrincipalID: "seller-1",
		PrincipalDestination: testPrincipal.Hex(), AssetID: testAssetID, Amount: "100",
		IdempotencyKey: "funding-1", ExpiresAt: now.Add(time.Hour),
	}
}

type fakeVaultClient struct {
	readyErr        error
	readyCalls      int
	callData        []byte
	obligation      ObligationCommand
	fundingKey      [32]byte
	query           FundingQuery
	execution       ExecutionCommand
	status          pkgcollateral.RailFundingStatus
	submitResult    pkgcollateral.RailActionResult
	executionResult pkgcollateral.RailActionResult
}

func (c *fakeVaultClient) CheckReady(context.Context) error {
	c.readyCalls++
	return c.readyErr
}

func (c *fakeVaultClient) EnsureObligation(_ context.Context, command ObligationCommand) (string, error) {
	c.obligation = command
	return "0xobligation", nil
}

func (c *fakeVaultClient) FundingCallData(_ [32]byte, fundingKey [32]byte, _ *big.Int) ([]byte, error) {
	c.fundingKey = fundingKey
	return append([]byte(nil), c.callData...), nil
}

func (c *fakeVaultClient) FundingStatus(_ context.Context, query FundingQuery) (pkgcollateral.RailFundingStatus, error) {
	c.query = query
	return c.status, nil
}

func (c *fakeVaultClient) SubmitExecution(_ context.Context, command ExecutionCommand) (pkgcollateral.RailActionResult, error) {
	c.execution = command
	return c.submitResult, nil
}

func (c *fakeVaultClient) ExecutionStatus(_ context.Context, command ExecutionCommand) (pkgcollateral.RailActionResult, error) {
	c.execution = command
	return c.executionResult, nil
}

var _ VaultClient = (*fakeVaultClient)(nil)

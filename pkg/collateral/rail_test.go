// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRailDescriptorRequiresCompleteLifecycleAndSortedAssets(t *testing.T) {
	descriptor := completeRailDescriptor()
	require.NoError(t, descriptor.ValidateForAsset("crypto:solana:devnet:usdc"))

	descriptor.SupportsClaimSlash = false
	require.ErrorContains(t, descriptor.ValidateForAsset("crypto:solana:devnet:usdc"), "complete v1 lifecycle")
	descriptor = completeRailDescriptor()
	descriptor.Assets = []string{"z", "a"}
	require.ErrorContains(t, descriptor.ValidateForAsset("a"), "sorted")
}

func TestRailFundingTargetIsIntentNotFundingProof(t *testing.T) {
	now := time.Now().UTC()
	request := FundingTargetRequest{
		TenantID: "tenant-1", CollateralID: "col-1", PrincipalID: "seller-1",
		PrincipalDestination: "principal:seller-1",
		AssetID:              "crypto:solana:devnet:usdc", Amount: "100", IdempotencyKey: "target-1",
		ExpiresAt: now.Add(time.Hour),
	}
	require.NoError(t, request.Validate(now))
	target := FundingTarget{
		RailID: "io.mobazha.collateral.solana-vault", TenantID: request.TenantID, CollateralID: request.CollateralID,
		PrincipalDestination: request.PrincipalDestination, IdempotencyKey: request.IdempotencyKey,
		AssetID: request.AssetID, Amount: request.Amount, Destination: "vault-address",
		ExpiresAt: request.ExpiresAt,
	}
	require.NoError(t, target.Validate(now))
	target.Destination = ""
	require.ErrorContains(t, target.Validate(now), "destination or payload")
}

func TestRailExecutionRequiresCoreDerivedDestinationAndClaimBinding(t *testing.T) {
	request := RailExecutionRequest{
		ActionID: "action-1", TenantID: "tenant-1", CollateralID: "col-1", ClaimID: "claim-1",
		Kind: ExecutionSlash, AssetID: "crypto:solana:devnet:usdc", Amount: "25",
		Destination: "core-derived-beneficiary", ExpectedRevision: 4, IdempotencyKey: "slash-1",
	}
	require.NoError(t, request.Validate())
	request.Destination = ""
	require.ErrorContains(t, request.Validate(), "destination")
	request.Destination = "core-derived-beneficiary"
	request.ClaimID = ""
	require.ErrorContains(t, request.Validate(), "requires claim")
}

func TestRailConfirmedResultsRequireReceiptReference(t *testing.T) {
	now := time.Now().UTC()
	funding := RailFundingStatus{
		State: RailActionConfirmed, AssetID: "crypto:solana:devnet:usdc", Amount: "100", ObservedAt: now,
	}
	require.ErrorContains(t, funding.Validate(), "reference")
	funding.Reference = "funding-tx-1"
	require.NoError(t, funding.Validate())

	action := RailActionResult{ActionID: "action-1", State: RailActionConfirmed, ObservedAt: now}
	require.ErrorContains(t, action.Validate(), "reference")
	action.Reference = "execution-tx-1"
	require.NoError(t, action.Validate())
}

func completeRailDescriptor() RailDescriptor {
	return RailDescriptor{
		ID: "io.mobazha.collateral.solana-vault", Version: "v1", CustodyModel: "program-vault",
		Assets: []string{"crypto:solana:devnet:usdc"}, SupportsFundingTargets: true,
		SupportsFundingObserve: true, SupportsPrincipalRelease: true, SupportsClaimSlash: true,
		SupportsReconciliation: true, HasReceiptVerification: true,
	}
}

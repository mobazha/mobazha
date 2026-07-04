// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenRequestRequiresCanonicalPositiveBaseUnits(t *testing.T) {
	now := time.Now().UTC()
	request := validOpenRequest(now)
	require.NoError(t, request.Validate(now))

	for _, amount := range []string{"", "0", "-1", "+1", "01", "1.0", " 1"} {
		request.RequiredAmount = amount
		require.ErrorContains(t, request.Validate(now), "required amount", amount)
	}
}

func TestOpenRequestBindsResourcePrincipalPolicyAndExpiry(t *testing.T) {
	now := time.Now().UTC()
	request := validOpenRequest(now)
	request.ResourceID = ""
	require.ErrorContains(t, request.Validate(now), "resource")

	request = validOpenRequest(now)
	request.ExpiresAt = now
	require.ErrorContains(t, request.Validate(now), "expiry")
}

func TestActiveAccountRequiresConfirmedFundingAndSufficientAmount(t *testing.T) {
	now := time.Now().UTC()
	account := validAccount(now)
	require.NoError(t, account.Validate())

	account.FundingReference = ""
	require.ErrorContains(t, account.Validate(), "funding reference")

	account = validAccount(now)
	account.FundedAmount = "99"
	account.AvailableAmount = "99"
	require.ErrorContains(t, account.Validate(), "below the requirement")

	account = validAccount(now)
	account.AvailableAmount = "101"
	require.ErrorContains(t, account.Validate(), "exceeds funded")
}

func TestAllocationRequestBindsExactOrderExtensionAndRevision(t *testing.T) {
	request := AllocationRequest{
		CollateralID: "col-1", TenantID: "tenant-1", ProviderID: "io.mobazha.collectibles",
		ResourceID: "source-1", PrincipalID: "seller-1", OrderID: "order-1", ExtensionID: "ext-1",
		Amount: "25", ExpectedCollateralRevision: 3, IdempotencyKey: "allocate-1",
	}
	require.NoError(t, request.Validate())
	request.ExtensionID = ""
	require.ErrorContains(t, request.Validate(), "extension")
	request.ExtensionID = "ext-1"
	request.ExpectedCollateralRevision = 0
	require.ErrorContains(t, request.Validate(), "expected revision")
}

func TestAllocationReferenceRejectsUnknownStateAndNonCanonicalAmount(t *testing.T) {
	reference := AllocationReference{
		AllocationID: "alloc-1", CollateralID: "col-1", TenantID: "tenant-1", OrderID: "order-1",
		ExtensionID: "ext-1", AssetID: "crypto:solana:mainnet:usdc", Amount: "25",
		CollateralRevision: 3, AllocationRevision: 1, State: AllocationActive,
	}
	require.NoError(t, reference.Validate())
	reference.State = "provider-says-funded"
	require.ErrorContains(t, reference.Validate(), "unsupported")
	reference.State = AllocationActive
	reference.Amount = "025"
	require.ErrorContains(t, reference.Validate(), "amount")
}

func TestClaimAttestationRequiresFreshRevisionBoundEvidence(t *testing.T) {
	now := time.Now().UTC()
	attestation := ClaimAttestation{
		AttestationID: "att-1", IdempotencyKey: "claim-1", Issuer: "io.mobazha.collectibles",
		TenantID: "tenant-1", CollateralID: "col-1", AllocationID: "alloc-1", OrderID: "order-1",
		ExtensionID: "ext-1", ExpectedCollateralRevision: 3, ExpectedAllocationRevision: 1,
		ConditionType: "physical-delivery-default", ConditionVersion: "v1", EvidenceDigest: "sha256:evidence",
		ObservedAt: now, ExpiresAt: now.Add(time.Minute),
	}
	require.NoError(t, attestation.Validate(now))

	attestation.ExpectedAllocationRevision = 0
	require.ErrorContains(t, attestation.Validate(now), "expected revisions")
	attestation.ExpectedAllocationRevision = 1
	attestation.ExpiresAt = now.Add(-time.Second)
	require.ErrorContains(t, attestation.Validate(now), "time window")
}

func validOpenRequest(now time.Time) OpenRequest {
	return OpenRequest{
		TenantID: "tenant-1", ProviderID: "io.mobazha.collectibles", ResourceID: "source-1",
		PrincipalID: "seller-1", AssetID: "crypto:solana:mainnet:usdc", RequiredAmount: "100",
		PolicyID: "collectibles-source-custody", PolicyVersion: "v1", IdempotencyKey: "open-1",
		ExpiresAt: now.Add(24 * time.Hour),
	}
}

func validAccount(now time.Time) Account {
	activatedAt := now.Add(-time.Minute)
	return Account{
		CollateralID: "col-1", TenantID: "tenant-1", ProviderID: "io.mobazha.collectibles",
		ResourceID: "source-1", PrincipalID: "seller-1", AssetID: "crypto:solana:mainnet:usdc",
		RequiredAmount: "100", FundedAmount: "100", AvailableAmount: "100",
		PolicyID: "collectibles-source-custody", PolicyVersion: "v1", FundingReference: "rail-funding-1",
		Revision: 2, State: StateActive, ActivatedAt: &activatedAt, ExpiresAt: now.Add(24 * time.Hour),
	}
}

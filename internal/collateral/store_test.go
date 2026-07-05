// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/repo"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestCollateralOpenFundingAllocateReleaseLifecycle(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open := openRequest(now)

	var account pkgcollateral.Account
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = OpenTx(tx, open, now)
		return err
	}))
	require.Equal(t, pkgcollateral.StatePendingFunding, account.State)
	require.Equal(t, uint64(1), account.Revision)
	require.Equal(t, "0", account.AvailableAmount)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = RecordFundingTx(tx, pkgcollateral.FundingObservation{
			TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
			AssetID: open.AssetID, FundedAmount: "120", FundingReference: "funding-1",
			ExpectedRevision: account.Revision, IdempotencyKey: "fund-1", ObservedAt: now,
		}, now)
		return err
	}))
	require.Equal(t, pkgcollateral.StateActive, account.State)
	require.Equal(t, uint64(2), account.Revision)
	require.Equal(t, "120", account.AvailableAmount)

	var allocation pkgcollateral.AllocationReference
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		allocation, err = AllocateTx(tx, pkgcollateral.AllocationRequest{
			CollateralID: account.CollateralID, TenantID: database.StandaloneTenantID,
			ProviderID: open.ProviderID, ResourceID: open.ResourceID, PrincipalID: open.PrincipalID,
			OrderID: "order-1", ExtensionID: "ext-1", Amount: "25",
			ExpectedCollateralRevision: account.Revision, IdempotencyKey: "allocate-1",
		}, now)
		return err
	}))
	require.Equal(t, pkgcollateral.AllocationActive, allocation.State)
	require.Equal(t, uint64(3), allocation.CollateralRevision)

	require.NoError(t, db.View(func(tx database.Tx) error {
		var err error
		account, err = AccountByIDTx(tx, account.CollateralID)
		return err
	}))
	require.Equal(t, "95", account.AvailableAmount)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		allocation, err = ReleaseAllocationTx(tx, pkgcollateral.AllocationReleaseRequest{
			TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID, AllocationID: allocation.AllocationID,
			ExpectedCollateralRevision: account.Revision, ExpectedAllocationRevision: allocation.AllocationRevision,
			IdempotencyKey: "release-1", Reason: "order-cancelled",
		}, now)
		return err
	}))
	require.Equal(t, pkgcollateral.AllocationReleased, allocation.State)
	require.Equal(t, uint64(2), allocation.AllocationRevision)
	require.NoError(t, db.View(func(tx database.Tx) error {
		var err error
		account, err = AccountByIDTx(tx, account.CollateralID)
		return err
	}))
	require.Equal(t, "120", account.AvailableAmount)
	require.Equal(t, uint64(4), account.Revision)

	var actions int64
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.CollateralActionRecord{}).Where("collateral_id = ?", account.CollateralID).Count(&actions).Error
	}))
	require.Equal(t, int64(4), actions)
}

func TestCollateralTransitionsAreIdempotentAndRevisionBound(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open := openRequest(now)
	var account pkgcollateral.Account
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = OpenTx(tx, open, now)
		return err
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		repeated, err := OpenTx(tx, open, now)
		require.Equal(t, account.CollateralID, repeated.CollateralID)
		return err
	}))

	observation := pkgcollateral.FundingObservation{
		TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID, AssetID: open.AssetID,
		FundedAmount: "100", FundingReference: "funding-1", ExpectedRevision: 1,
		IdempotencyKey: "fund-1", ObservedAt: now,
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = RecordFundingTx(tx, observation, now)
		return err
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		repeated, err := RecordFundingTx(tx, observation, now)
		require.Equal(t, account.Revision, repeated.Revision)
		return err
	}))

	err := db.Update(func(tx database.Tx) error {
		_, err := AllocateTx(tx, pkgcollateral.AllocationRequest{
			CollateralID: account.CollateralID, TenantID: database.StandaloneTenantID,
			ProviderID: open.ProviderID, ResourceID: open.ResourceID, PrincipalID: open.PrincipalID,
			OrderID: "order-stale", ExtensionID: "ext-stale", Amount: "1",
			ExpectedCollateralRevision: 1, IdempotencyKey: "allocate-stale",
		}, now)
		return err
	})
	require.ErrorContains(t, err, "revision conflict")
}

func TestCollateralAllocationFailsClosedForWrongScopeAndInsufficientCoverage(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open := openRequest(now)
	var account pkgcollateral.Account
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = OpenTx(tx, open, now)
		if err != nil {
			return err
		}
		account, err = RecordFundingTx(tx, pkgcollateral.FundingObservation{
			TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID, AssetID: open.AssetID,
			FundedAmount: "100", FundingReference: "funding-1", ExpectedRevision: 1,
			IdempotencyKey: "fund-1", ObservedAt: now,
		}, now)
		return err
	}))

	base := pkgcollateral.AllocationRequest{
		CollateralID: account.CollateralID, TenantID: database.StandaloneTenantID,
		ProviderID: open.ProviderID, ResourceID: open.ResourceID, PrincipalID: open.PrincipalID,
		OrderID: "order-1", ExtensionID: "ext-1", Amount: "101",
		ExpectedCollateralRevision: account.Revision, IdempotencyKey: "allocate-1",
	}
	err := db.Update(func(tx database.Tx) error { _, err := AllocateTx(tx, base, now); return err })
	require.ErrorContains(t, err, "insufficient")
	base.Amount = "1"
	base.PrincipalID = "other-seller"
	err = db.Update(func(tx database.Tx) error { _, err := AllocateTx(tx, base, now); return err })
	require.ErrorContains(t, err, "scope")
}

func TestCollateralFundingReferenceCannotActivateTwoAccounts(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	firstOpen := openRequest(now)
	secondOpen := openRequest(now)
	secondOpen.ResourceID = "source-2"
	secondOpen.IdempotencyKey = "open-2"
	var first, second pkgcollateral.Account
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		first, err = OpenTx(tx, firstOpen, now)
		if err != nil {
			return err
		}
		second, err = OpenTx(tx, secondOpen, now)
		return err
	}))

	fund := func(account pkgcollateral.Account, key string) error {
		return db.Update(func(tx database.Tx) error {
			_, err := RecordFundingTx(tx, pkgcollateral.FundingObservation{
				TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
				AssetID: firstOpen.AssetID, FundedAmount: "100", FundingReference: "same-chain-transfer",
				ExpectedRevision: account.Revision, IdempotencyKey: key, ObservedAt: now,
			}, now)
			return err
		})
	}
	require.NoError(t, fund(first, "fund-first"))
	require.ErrorContains(t, fund(second, "fund-second"), "already claimed")
}

func TestExpiredCollateralCannotBeActivated(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	request := openRequest(now)
	var account pkgcollateral.Account
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = OpenTx(tx, request, now)
		return err
	}))
	err := db.Update(func(tx database.Tx) error {
		_, err := RecordFundingTx(tx, pkgcollateral.FundingObservation{
			TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
			AssetID: request.AssetID, FundedAmount: "100", FundingReference: "late-funding",
			ExpectedRevision: account.Revision, IdempotencyKey: "fund-late", ObservedAt: now.Add(25 * time.Hour),
		}, now.Add(25*time.Hour))
		return err
	})
	require.ErrorContains(t, err, "expired")
}

func TestCollateralOpenRejectsTenantDifferentFromTransactionScope(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	request := openRequest(now)
	request.TenantID = "other-tenant"
	err := db.Update(func(tx database.Tx) error {
		_, err := OpenTx(tx, request, now)
		return err
	})
	require.ErrorContains(t, err, "transaction scope")

	var count int64
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.CollateralAccountRecord{}).Count(&count).Error
	}))
	require.Zero(t, count)
}

func TestCollateralAccountReleaseRequiresNoAllocationsAndConfirmedExecution(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open, account := openAndFundCollateral(t, db, now, "source-release", "open-release", "fund-release", "funding-release", "100")

	request := pkgcollateral.AccountReleaseRequest{
		TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
		ExpectedRevision: account.Revision, IdempotencyKey: "request-release", Reason: "resource-obligation-ended",
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = RequestAccountReleaseTx(tx, request, now)
		return err
	}))
	require.Equal(t, pkgcollateral.StateReleasePending, account.State)
	require.Equal(t, uint64(3), account.Revision)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		repeated, err := RequestAccountReleaseTx(tx, request, now)
		require.Equal(t, account.Revision, repeated.Revision)
		return err
	}))

	observation := pkgcollateral.ExecutionObservation{
		TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
		Kind: pkgcollateral.ExecutionRelease, AssetID: open.AssetID, Amount: "100",
		ExecutionReference: "release-tx-1", ExpectedRevision: account.Revision,
		IdempotencyKey: "confirm-release", ObservedAt: now,
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = RecordExecutionTx(tx, observation, now)
		return err
	}))
	require.Equal(t, pkgcollateral.StateReleased, account.State)
	require.Equal(t, "0", account.FundedAmount)
	require.Equal(t, "0", account.AvailableAmount)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		repeated, err := RecordExecutionTx(tx, observation, now)
		require.Equal(t, account.Revision, repeated.Revision)
		return err
	}))
}

func TestCollateralAccountReleaseFailsWhileAllocationIsActive(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open, account := openAndFundCollateral(t, db, now, "source-busy", "open-busy", "fund-busy", "funding-busy", "100")
	allocation := allocateCollateral(t, db, now, open, account, "order-busy", "ext-busy", "25", "allocate-busy")

	err := db.Update(func(tx database.Tx) error {
		_, err := RequestAccountReleaseTx(tx, pkgcollateral.AccountReleaseRequest{
			TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
			ExpectedRevision: allocation.CollateralRevision, IdempotencyKey: "release-busy", Reason: "too-early",
		}, now)
		return err
	})
	require.ErrorContains(t, err, "allocations")
}

func TestCollateralClaimAndPartialSlashPreserveRemainingCoverage(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open, account := openAndFundCollateral(t, db, now, "source-claim", "open-claim", "fund-claim", "funding-claim", "120")
	allocation := allocateCollateral(t, db, now, open, account, "order-claim", "ext-claim", "25", "allocate-claim")

	decision := claimDecision(now, account.CollateralID, allocation, "20")
	var claim pkgcollateral.Claim
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		claim, err = AcceptClaimTx(tx, decision, now)
		return err
	}))
	require.Equal(t, pkgcollateral.ClaimPendingSlash, claim.State)
	require.Equal(t, uint64(4), claim.CollateralRevision)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		repeated, err := AcceptClaimTx(tx, decision, now)
		require.Equal(t, claim.ClaimID, repeated.ClaimID)
		return err
	}))
	replayed := decision
	replayed.ClaimID = "claim-replayed-under-new-id"
	replayed.IdempotencyKey = "claim-replayed-under-new-id"
	err := db.Update(func(tx database.Tx) error { _, err := AcceptClaimTx(tx, replayed, now); return err })
	require.ErrorContains(t, err, "evidence replay")

	require.NoError(t, db.View(func(tx database.Tx) error {
		var err error
		account, err = AccountByIDTx(tx, account.CollateralID)
		return err
	}))
	require.Equal(t, pkgcollateral.StateSlashPending, account.State)
	require.Equal(t, "95", account.AvailableAmount)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = RecordExecutionTx(tx, pkgcollateral.ExecutionObservation{
			TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID, ClaimID: claim.ClaimID,
			Kind: pkgcollateral.ExecutionSlash, AssetID: open.AssetID, Amount: "20",
			ExecutionReference: "slash-tx-1", ExpectedRevision: account.Revision,
			IdempotencyKey: "confirm-slash", ObservedAt: now,
		}, now)
		return err
	}))
	require.Equal(t, pkgcollateral.StateActive, account.State)
	require.Equal(t, "100", account.FundedAmount)
	require.Equal(t, "100", account.AvailableAmount)
	require.Equal(t, uint64(5), account.Revision)

	require.NoError(t, db.View(func(tx database.Tx) error {
		var err error
		claim, err = ClaimByIDTx(tx, claim.ClaimID)
		return err
	}))
	require.Equal(t, pkgcollateral.ClaimSlashed, claim.State)
	require.Equal(t, "slash-tx-1", claim.ExecutionReference)

	otherOpen, other := openAndFundCollateral(t, db, now, "source-execution-replay", "open-execution-replay", "fund-execution-replay", "funding-execution-replay", "100")
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		other, err = RequestAccountReleaseTx(tx, pkgcollateral.AccountReleaseRequest{
			TenantID: database.StandaloneTenantID, CollateralID: other.CollateralID,
			ExpectedRevision: other.Revision, IdempotencyKey: "request-execution-replay", Reason: "resource-ended",
		}, now)
		return err
	}))
	err = db.Update(func(tx database.Tx) error {
		_, err := RecordExecutionTx(tx, pkgcollateral.ExecutionObservation{
			TenantID: database.StandaloneTenantID, CollateralID: other.CollateralID,
			Kind: pkgcollateral.ExecutionRelease, AssetID: otherOpen.AssetID, Amount: "100",
			ExecutionReference: "slash-tx-1", ExpectedRevision: other.Revision,
			IdempotencyKey: "confirm-execution-replay", ObservedAt: now,
		}, now)
		return err
	})
	require.ErrorContains(t, err, "already claimed")
}

func TestCollateralClaimRejectsWrongIssuerAndExcessAmount(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open, account := openAndFundCollateral(t, db, now, "source-invalid-claim", "open-invalid-claim", "fund-invalid-claim", "funding-invalid-claim", "100")
	allocation := allocateCollateral(t, db, now, open, account, "order-invalid-claim", "ext-invalid-claim", "25", "allocate-invalid-claim")
	decision := claimDecision(now, account.CollateralID, allocation, "26")
	err := db.Update(func(tx database.Tx) error { _, err := AcceptClaimTx(tx, decision, now); return err })
	require.ErrorContains(t, err, "exceeds allocation")

	decision = claimDecision(now, account.CollateralID, allocation, "20")
	decision.ClaimID = "claim-wrong-issuer"
	decision.IdempotencyKey = "claim-wrong-issuer"
	decision.Attestation.AttestationID = "att-wrong-issuer"
	decision.Attestation.Issuer = "io.mobazha.other"
	err = db.Update(func(tx database.Tx) error { _, err := AcceptClaimTx(tx, decision, now); return err })
	require.ErrorContains(t, err, "issuer")
}

func collateralTestDB(t *testing.T) database.Database {
	t.Helper()
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		for _, model := range []interface{}{&models.CollateralAccountRecord{}, &models.CollateralFundingRecord{}, &models.CollateralExecutionRecord{}, &models.CollateralAllocationRecord{}, &models.CollateralClaimRecord{}, &models.CollateralActionRecord{}, &models.CollateralFundingTargetRecord{}, &models.CollateralRailActionRecord{}} {
			if err := tx.Migrate(model); err != nil {
				return err
			}
		}
		return nil
	}))
	return db
}

func openRequest(now time.Time) pkgcollateral.OpenRequest {
	return pkgcollateral.OpenRequest{
		TenantID: database.StandaloneTenantID, ProviderID: "io.mobazha.collectibles", ResourceID: "source-1",
		PrincipalID: "seller-1", AssetID: "crypto:solana:mainnet:usdc", RequiredAmount: "100",
		PolicyID: "collectibles-source-custody", PolicyVersion: "v1", IdempotencyKey: "open-1",
		ExpiresAt: now.Add(24 * time.Hour),
	}
}

func openAndFundCollateral(t *testing.T, db database.Database, now time.Time, resourceID, openKey, fundKey, fundingReference, fundedAmount string) (pkgcollateral.OpenRequest, pkgcollateral.Account) {
	t.Helper()
	request := openRequest(now)
	request.ResourceID = resourceID
	request.IdempotencyKey = openKey
	var account pkgcollateral.Account
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = OpenTx(tx, request, now)
		if err != nil {
			return err
		}
		account, err = RecordFundingTx(tx, pkgcollateral.FundingObservation{
			TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
			AssetID: request.AssetID, FundedAmount: fundedAmount, FundingReference: fundingReference,
			ExpectedRevision: account.Revision, IdempotencyKey: fundKey, ObservedAt: now,
		}, now)
		return err
	}))
	return request, account
}

func allocateCollateral(t *testing.T, db database.Database, now time.Time, open pkgcollateral.OpenRequest, account pkgcollateral.Account, orderID, extensionID, amount, key string) pkgcollateral.AllocationReference {
	t.Helper()
	var allocation pkgcollateral.AllocationReference
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		allocation, err = AllocateTx(tx, pkgcollateral.AllocationRequest{
			CollateralID: account.CollateralID, TenantID: database.StandaloneTenantID,
			ProviderID: open.ProviderID, ResourceID: open.ResourceID, PrincipalID: open.PrincipalID,
			OrderID: orderID, ExtensionID: extensionID, Amount: amount,
			ExpectedCollateralRevision: account.Revision, IdempotencyKey: key,
		}, now)
		return err
	}))
	return allocation
}

func claimDecision(now time.Time, collateralID string, allocation pkgcollateral.AllocationReference, amount string) pkgcollateral.ClaimDecision {
	return pkgcollateral.ClaimDecision{
		ClaimID: "claim-1", Amount: amount, Reason: "accepted-physical-default", IdempotencyKey: "claim-1",
		Attestation: pkgcollateral.ClaimAttestation{
			AttestationID: "att-claim-1", IdempotencyKey: "att-claim-1", Issuer: "io.mobazha.collectibles",
			TenantID: database.StandaloneTenantID, CollateralID: collateralID, AllocationID: allocation.AllocationID,
			OrderID: allocation.OrderID, ExtensionID: allocation.ExtensionID,
			ExpectedCollateralRevision: allocation.CollateralRevision, ExpectedAllocationRevision: allocation.AllocationRevision,
			ExpectedOrderStateVersion: "sha256:order-state",
			ConditionType:             "physical-delivery-default", ConditionVersion: "v1", EvidenceDigest: "sha256:evidence",
			ObservedAt: now, ExpiresAt: now.Add(time.Hour),
		},
	}
}

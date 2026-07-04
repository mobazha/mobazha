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

func collateralTestDB(t *testing.T) database.Database {
	t.Helper()
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		for _, model := range []interface{}{&models.CollateralAccountRecord{}, &models.CollateralFundingRecord{}, &models.CollateralAllocationRecord{}, &models.CollateralActionRecord{}} {
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

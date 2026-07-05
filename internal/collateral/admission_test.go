// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/orderextensions"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestAdmitOrderExtensionV2ReloadsCompleteCoreBinding(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open, account := openAndFundCollateral(t, db, now, "source-v2", "open-v2", "fund-v2", "funding-v2", "120")
	extension, err := extensions.NewOrderExtension("order-v2", open.ProviderID, "source-custody", "v1", open.ResourceID, map[string]string{"mode": "M2"})
	require.NoError(t, err)
	reference := allocateCollateral(t, db, now, open, account, "order-v2", extension.ExtensionID, "25", "allocate-v2")
	request := OrderExtensionV2Admission{
		TenantID: open.TenantID, OrderID: "order-v2", PrincipalID: open.PrincipalID,
		RequiredAssetID: open.AssetID, RequiredAmount: "25",
		Envelope: extensions.OrderExtensionV2{
			ContractVersion: extensions.ContractVersionV2,
			Extension:       extension, CollateralAllocation: &reference,
		},
	}

	var admitted pkgcollateral.AllocationReference
	require.NoError(t, db.View(func(tx database.Tx) error {
		var err error
		admitted, err = AdmitOrderExtensionV2Tx(tx, request, now)
		return err
	}))
	require.Equal(t, reference, admitted)

	wrongTenant := request
	wrongTenant.TenantID = "other-tenant"
	requireAdmissionError(t, db, wrongTenant, now, "requirement binding")
	wrongOrder := request
	wrongOrder.OrderID = "other-order"
	requireAdmissionError(t, db, wrongOrder, now, "not bound")
	wrongPrincipal := request
	wrongPrincipal.PrincipalID = "other-seller"
	requireAdmissionError(t, db, wrongPrincipal, now, "requirement binding")
	wrongAsset := request
	wrongAsset.RequiredAssetID = "crypto:eip155:1:native"
	requireAdmissionError(t, db, wrongAsset, now, "requirement binding")
	wrongAmount := request
	wrongAmount.RequiredAmount = "26"
	requireAdmissionError(t, db, wrongAmount, now, "requirement binding")
}

func TestBindOrderExtensionV2PersistsAndReadmitsExactRevision(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open, account := openAndFundCollateral(t, db, now, "source-v2-bind", "open-v2-bind", "fund-v2-bind", "funding-v2-bind", "120")
	extension, err := extensions.NewOrderExtension("order-v2-bind", open.ProviderID, "source-custody", "v1", open.ResourceID, map[string]string{"mode": "M2"})
	require.NoError(t, err)
	reference := allocateCollateral(t, db, now, open, account, "order-v2-bind", extension.ExtensionID, "25", "allocate-v2-bind")

	var persisted extensions.OrderExtension
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := orderextensions.PersistTx(tx, "order-v2-bind", extension); err != nil {
			return err
		}
		var loadErr error
		persisted, loadErr = orderextensions.LatestByIDTx(tx, "order-v2-bind", extension.ExtensionID)
		return loadErr
	}))
	request := OrderExtensionV2Admission{
		TenantID: open.TenantID, OrderID: "order-v2-bind", PrincipalID: open.PrincipalID,
		RequiredAssetID: open.AssetID, RequiredAmount: "25",
		Envelope: extensions.OrderExtensionV2{
			ContractVersion: extensions.ContractVersionV2, Extension: persisted, CollateralAllocation: &reference,
		},
	}

	require.NoError(t, db.Update(func(tx database.Tx) error {
		first, err := BindOrderExtensionV2Tx(tx, request, now)
		if err != nil {
			return err
		}
		second, err := BindOrderExtensionV2Tx(tx, request, now)
		if err != nil {
			return err
		}
		require.Equal(t, first, second)
		return nil
	}))
	require.NoError(t, db.View(func(tx database.Tx) error {
		envelopes, err := OrderExtensionsV2ByOrderTx(tx, "order-v2-bind")
		if err != nil {
			return err
		}
		require.Len(t, envelopes, 1)
		require.Equal(t, persisted, envelopes[0].Extension)
		require.Equal(t, reference, *envelopes[0].CollateralAllocation)
		return AdmitPersistedOrderExtensionsV2Tx(tx, "order-v2-bind", now)
	}))

	// An unrelated allocation may advance the aggregate revision without
	// invalidating this still-active allocation.
	var current pkgcollateral.Account
	require.NoError(t, db.View(func(tx database.Tx) error {
		var err error
		current, err = AccountByIDTx(tx, account.CollateralID)
		return err
	}))
	allocateCollateral(t, db, now, open, current, "order-v2-other", "ext-v2-other", "10", "allocate-v2-other")
	require.NoError(t, db.View(func(tx database.Tx) error {
		return AdmitPersistedOrderExtensionsV2Tx(tx, "order-v2-bind", now)
	}))
}

func TestPersistedOrderExtensionV2RejectsChangedExtensionRevision(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open, account := openAndFundCollateral(t, db, now, "source-v2-revision", "open-v2-revision", "fund-v2-revision", "funding-v2-revision", "120")
	first, err := extensions.NewOrderExtension("order-v2-revision", open.ProviderID, "source-custody", "v1", open.ResourceID, map[string]string{"version": "one"})
	require.NoError(t, err)
	reference := allocateCollateral(t, db, now, open, account, "order-v2-revision", first.ExtensionID, "25", "allocate-v2-revision")

	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := orderextensions.PersistTx(tx, "order-v2-revision", first); err != nil {
			return err
		}
		persisted, err := orderextensions.LatestByIDTx(tx, "order-v2-revision", first.ExtensionID)
		if err != nil {
			return err
		}
		_, err = BindOrderExtensionV2Tx(tx, OrderExtensionV2Admission{
			TenantID: open.TenantID, OrderID: "order-v2-revision", PrincipalID: open.PrincipalID,
			RequiredAssetID: open.AssetID, RequiredAmount: "25",
			Envelope: extensions.OrderExtensionV2{ContractVersion: extensions.ContractVersionV2, Extension: persisted, CollateralAllocation: &reference},
		}, now)
		return err
	}))

	second, err := extensions.NewOrderExtension("order-v2-revision", open.ProviderID, "source-custody", "v1", open.ResourceID, map[string]string{"version": "two"})
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return orderextensions.PersistTx(tx, "order-v2-revision", second)
	}))
	err = db.View(func(tx database.Tx) error {
		return AdmitPersistedOrderExtensionsV2Tx(tx, "order-v2-revision", now)
	})
	require.ErrorContains(t, err, "binding revision is stale")

	// Re-admission appends a v2 binding for the new extension revision while
	// retaining the previous revision as audit history.
	require.NoError(t, db.Update(func(tx database.Tx) error {
		persisted, err := orderextensions.LatestByIDTx(tx, "order-v2-revision", first.ExtensionID)
		if err != nil {
			return err
		}
		_, err = BindOrderExtensionV2Tx(tx, OrderExtensionV2Admission{
			TenantID: open.TenantID, OrderID: "order-v2-revision", PrincipalID: open.PrincipalID,
			RequiredAssetID: open.AssetID, RequiredAmount: "25",
			Envelope: extensions.OrderExtensionV2{ContractVersion: extensions.ContractVersionV2, Extension: persisted, CollateralAllocation: &reference},
		}, now.Add(time.Second))
		return err
	}))
	require.NoError(t, db.View(func(tx database.Tx) error {
		var count int64
		if err := tx.Read().Model(&models.CollateralOrderExtensionBindingRecord{}).
			Where("order_id = ? AND extension_id = ?", "order-v2-revision", first.ExtensionID).Count(&count).Error; err != nil {
			return err
		}
		require.Equal(t, int64(2), count)
		return AdmitPersistedOrderExtensionsV2Tx(tx, "order-v2-revision", now.Add(time.Second))
	}))
}

func TestAdmitOrderExtensionV2RejectsMissingStaleAndWrongResourceReferences(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open, account := openAndFundCollateral(t, db, now, "source-v2-negative", "open-v2-negative", "fund-v2-negative", "funding-v2-negative", "120")
	extension, err := extensions.NewOrderExtension("order-v2-negative", open.ProviderID, "source-custody", "v1", open.ResourceID, map[string]string{"mode": "M2"})
	require.NoError(t, err)
	reference := allocateCollateral(t, db, now, open, account, "order-v2-negative", extension.ExtensionID, "25", "allocate-v2-negative")
	base := OrderExtensionV2Admission{
		TenantID: open.TenantID, OrderID: "order-v2-negative", PrincipalID: open.PrincipalID, RequiredAssetID: open.AssetID, RequiredAmount: "25",
		Envelope: extensions.OrderExtensionV2{ContractVersion: extensions.ContractVersionV2, Extension: extension, CollateralAllocation: &reference},
	}

	missing := base
	missing.Envelope.CollateralAllocation = nil
	requireAdmissionError(t, db, missing, now, "allocation is required")

	missing = base
	missingReference := *base.Envelope.CollateralAllocation
	missing.Envelope.CollateralAllocation = &missingReference
	missing.Envelope.CollateralAllocation.AllocationID = "alloc_missing"
	requireAdmissionError(t, db, missing, now, "load collateral allocation")

	stale := base
	staleReference := *base.Envelope.CollateralAllocation
	stale.Envelope.CollateralAllocation = &staleReference
	stale.Envelope.CollateralAllocation.AllocationRevision++
	requireAdmissionError(t, db, stale, now, "missing or stale")

	wrongResource := base
	wrongResourceReference := *base.Envelope.CollateralAllocation
	wrongResource.Envelope.CollateralAllocation = &wrongResourceReference
	wrongResource.Envelope.CollateralAllocation.ResourceID = "other-source"
	requireAdmissionError(t, db, wrongResource, now, "binding mismatch")

	wrongProvider := base
	wrongProviderReference := *base.Envelope.CollateralAllocation
	wrongProvider.Envelope.CollateralAllocation = &wrongProviderReference
	wrongProvider.Envelope.CollateralAllocation.ProviderID = "other-provider"
	requireAdmissionError(t, db, wrongProvider, now, "binding mismatch")
}

func requireAdmissionError(t *testing.T, db database.Database, request OrderExtensionV2Admission, now time.Time, contains string) {
	t.Helper()
	err := db.View(func(tx database.Tx) error {
		_, err := AdmitOrderExtensionV2Tx(tx, request, now)
		return err
	})
	require.ErrorContains(t, err, contains)
}

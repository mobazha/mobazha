// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"testing"
	"time"

	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
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

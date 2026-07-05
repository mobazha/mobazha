// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"testing"
	"time"

	corecollateral "github.com/mobazha/mobazha/internal/collateral"
	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/internal/repo"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestCollateralAllocationServiceAllocatesAndBindsInOneCoreTransaction(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		for _, model := range []any{
			&models.CollateralAccountRecord{}, &models.CollateralFundingRecord{}, &models.CollateralAllocationRecord{},
			&models.CollateralActionRecord{}, &models.OrderExtensionRecord{}, &models.CollateralOrderExtensionBindingRecord{},
		} {
			if err := tx.Migrate(model); err != nil {
				return err
			}
		}
		return nil
	}))
	now := time.Now().UTC().Truncate(time.Second)
	assetID := "crypto:eip155:56:erc20:0x55d398326f99059fF775485246999027B3197955"
	open := pkgcollateral.OpenRequest{
		TenantID: database.StandaloneTenantID, ProviderID: "io.mobazha.collectibles", ResourceID: "srcdep-1",
		PrincipalID: "seller-1", AssetID: assetID, RequiredAmount: "100",
		PolicyID: "io.mobazha.collectibles.source-custody", PolicyVersion: "1",
		IdempotencyKey: "open-seller-authority", ExpiresAt: now.Add(24 * time.Hour),
	}
	var account pkgcollateral.Account
	require.NoError(t, db.Update(func(tx database.Tx) error {
		account, err = corecollateral.OpenTx(tx, open, now)
		if err != nil {
			return err
		}
		account, err = corecollateral.RecordFundingTx(tx, pkgcollateral.FundingObservation{
			TenantID: open.TenantID, CollateralID: account.CollateralID, AssetID: assetID,
			FundedAmount: "100", FundingReference: "funding-seller-authority",
			ExpectedRevision: account.Revision, IdempotencyKey: "fund-seller-authority", ObservedAt: now,
		}, now)
		return err
	}))
	extension, err := extensions.NewOrderExtension(
		"order-seller-authority", open.ProviderID, "source-custody", "v1", open.ResourceID, map[string]string{"mode": "M2"},
	)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return orderextensions.PersistTx(tx, "order-seller-authority", extension)
	}))
	request := contracts.AllocateOrderExtensionCollateralRequest{
		TenantID: open.TenantID, CollateralID: account.CollateralID, OrderID: "order-seller-authority",
		ExpectedCollateralRevision: account.Revision, IdempotencyKey: "allocate-seller-authority",
		Extension: extension,
		Requirement: extensions.CollateralRequirement{
			ProviderID: open.ProviderID, ResourceID: open.ResourceID, PrincipalID: open.PrincipalID,
			AssetID: assetID, Amount: "25", PolicyID: open.PolicyID, PolicyVersion: open.PolicyVersion,
		},
	}
	service := &collateralAllocationService{db: db, now: func() time.Time { return now }}
	first, err := service.AllocateOrderExtensionCollateral(context.Background(), request)
	require.NoError(t, err)
	wrongPolicy := request
	wrongPolicy.IdempotencyKey = "allocate-wrong-policy"
	wrongPolicy.ExpectedCollateralRevision = first.CollateralAllocation.CollateralRevision
	wrongPolicy.Requirement.PolicyVersion = "2"
	_, err = service.AllocateOrderExtensionCollateral(context.Background(), wrongPolicy)
	require.ErrorContains(t, err, "does not match the declared requirement")
	second, err := service.AllocateOrderExtensionCollateral(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.NotNil(t, first.CollateralAllocation)
	require.Equal(t, "25", first.CollateralAllocation.Amount)

	require.NoError(t, db.View(func(tx database.Tx) error {
		stored, err := corecollateral.AccountByIDTx(tx, account.CollateralID)
		if err != nil {
			return err
		}
		require.Equal(t, "75", stored.AvailableAmount)
		return corecollateral.AdmitPersistedOrderExtensionsV2Tx(tx, request.OrderID, now)
	}))
}

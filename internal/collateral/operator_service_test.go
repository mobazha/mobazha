package collateral

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestOperatorServiceBindsTenantPrincipalAndFailsClosedWithoutRail(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	serviceAPI, err := NewOperatorService(db, database.StandaloneTenantID, "seller-peer", nil)
	require.NoError(t, err)
	service := serviceAPI.(*operatorService)
	service.now = func() time.Time { return now }

	account, err := service.Open(context.Background(), pkgcollateral.OperatorOpenRequest{
		ProviderID: "io.mobazha.collectibles", ResourceID: "source-operator-1",
		AssetID: "crypto:solana:mainnet:usdc", RequiredAmount: "100",
		PolicyID: "collectibles-source-custody", PolicyVersion: "v1",
		IdempotencyKey: "operator-open-1", ExpiresAt: now.Add(time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, database.StandaloneTenantID, account.TenantID)
	require.Equal(t, "seller-peer", account.PrincipalID)
	require.Equal(t, pkgcollateral.StatePendingFunding, account.State)

	replayed, err := service.Open(context.Background(), pkgcollateral.OperatorOpenRequest{
		ProviderID: "io.mobazha.collectibles", ResourceID: "source-operator-1",
		AssetID: "crypto:solana:mainnet:usdc", RequiredAmount: "100",
		PolicyID: "collectibles-source-custody", PolicyVersion: "v1",
		IdempotencyKey: "operator-open-1", ExpiresAt: now.Add(time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, account, replayed)

	_, err = service.PrepareFunding(context.Background(), pkgcollateral.OperatorPrepareFundingRequest{
		CollateralID: account.CollateralID, PrincipalDestination: "wallet:seller", IdempotencyKey: "operator-fund-1",
	})
	require.ErrorIs(t, err, pkgcollateral.ErrOperatorUnavailable)

	otherPrincipalAPI, err := NewOperatorService(db, database.StandaloneTenantID, "other-seller", nil)
	require.NoError(t, err)
	_, err = otherPrincipalAPI.Status(context.Background(), account.CollateralID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestOperatorServicePreparesAndReconcilesReceiptVerifiedFunding(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	rail := completeFakeRail()
	serviceAPI, err := NewOperatorService(db, database.StandaloneTenantID, "seller-peer", rail)
	require.NoError(t, err)
	service := serviceAPI.(*operatorService)
	service.now = func() time.Time { return now }
	service.executor.now = func() time.Time { return now }

	account, err := service.Open(context.Background(), pkgcollateral.OperatorOpenRequest{
		ProviderID: "io.mobazha.collectibles", ResourceID: "source-operator-2",
		AssetID: "crypto:solana:mainnet:usdc", RequiredAmount: "100",
		PolicyID: "collectibles-source-custody", PolicyVersion: "v1",
		IdempotencyKey: "operator-open-2", ExpiresAt: now.Add(time.Hour),
	})
	require.NoError(t, err)

	rail.prepareTarget = pkgcollateral.FundingTarget{
		RailID: rail.descriptor.ID, TenantID: database.StandaloneTenantID,
		CollateralID: account.CollateralID, PrincipalDestination: "wallet:seller",
		IdempotencyKey: "operator-fund-2", AssetID: account.AssetID,
		Amount: account.RequiredAmount, Destination: "vault:deposit-2", ExpiresAt: account.ExpiresAt,
	}
	target, err := service.PrepareFunding(context.Background(), pkgcollateral.OperatorPrepareFundingRequest{
		CollateralID: account.CollateralID, PrincipalDestination: "wallet:seller", IdempotencyKey: "operator-fund-2",
	})
	require.NoError(t, err)
	require.Equal(t, "vault:deposit-2", target.Destination)
	require.Equal(t, 1, rail.prepareCalls)

	status, err := service.Status(context.Background(), account.CollateralID)
	require.NoError(t, err)
	require.NotNil(t, status.Funding)
	require.Equal(t, pkgcollateral.RailActionPending, status.Funding.State)
	require.Equal(t, "vault:deposit-2", status.Funding.Destination)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(
			map[string]interface{}{"last_error": "private rpc endpoint rejected request"},
			map[string]interface{}{"tenant_id = ?": database.StandaloneTenantID, "collateral_id = ?": account.CollateralID},
			&models.CollateralFundingTargetRecord{},
		)
		return err
	}))
	status, err = service.Status(context.Background(), account.CollateralID)
	require.NoError(t, err)
	require.Equal(t, "rail_operation_failed", status.Funding.LastErrorCode)
	require.NotContains(t, status.Funding.LastErrorCode, "rpc endpoint")

	rail.fundingStatus = pkgcollateral.RailFundingStatus{
		State: pkgcollateral.RailActionConfirmed, Reference: "funding-receipt-2",
		AssetID: account.AssetID, Amount: account.RequiredAmount, ObservedAt: now.Add(time.Minute),
	}
	service.executor.now = func() time.Time { return now.Add(time.Minute) }
	status, err = service.ReconcileFunding(context.Background(), account.CollateralID)
	require.NoError(t, err)
	require.Equal(t, pkgcollateral.StateActive, status.Account.State)
	require.Equal(t, account.RequiredAmount, status.Account.FundedAmount)
	require.Equal(t, pkgcollateral.RailActionConfirmed, status.Funding.State)
	require.Equal(t, "funding-receipt-2", status.Funding.FundingReference)
	require.Equal(t, 1, rail.fundingStatusCalls)
}

func TestOperatorServiceRejectsChangedOpenIdempotencyIntent(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	serviceAPI, err := NewOperatorService(db, database.StandaloneTenantID, "seller-peer", nil)
	require.NoError(t, err)
	service := serviceAPI.(*operatorService)
	service.now = func() time.Time { return now }

	input := pkgcollateral.OperatorOpenRequest{
		ProviderID: "io.mobazha.collectibles", ResourceID: "source-operator-3",
		AssetID: "crypto:solana:mainnet:usdc", RequiredAmount: "100",
		PolicyID: "collectibles-source-custody", PolicyVersion: "v1",
		IdempotencyKey: "operator-open-3", ExpiresAt: now.Add(time.Hour),
	}
	_, err = service.Open(context.Background(), input)
	require.NoError(t, err)
	input.RequiredAmount = "101"
	_, err = service.Open(context.Background(), input)
	require.Error(t, err)
	require.True(t, errors.Is(err, pkgcollateral.ErrOperatorConflict), err)
}

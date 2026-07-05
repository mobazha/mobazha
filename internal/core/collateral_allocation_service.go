// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"fmt"
	"time"

	corecollateral "github.com/mobazha/mobazha/internal/collateral"
	"github.com/mobazha/mobazha/internal/orderextensions"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
)

type collateralAllocationService struct {
	db  database.Database
	now func() time.Time
}

func newCollateralAllocationService(db database.Database) contracts.CollateralAllocationService {
	if db == nil {
		return nil
	}
	return &collateralAllocationService{db: db, now: func() time.Time { return time.Now().UTC() }}
}

func (s *collateralAllocationService) AllocateOrderExtensionCollateral(
	ctx context.Context,
	request contracts.AllocateOrderExtensionCollateralRequest,
) (extensions.OrderExtensionV2, error) {
	if s == nil || s.db == nil {
		return extensions.OrderExtensionV2{}, fmt.Errorf("collateral allocation service is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return extensions.OrderExtensionV2{}, err
	}
	if err := request.Validate(); err != nil {
		return extensions.OrderExtensionV2{}, err
	}
	now := s.now()
	var envelope extensions.OrderExtensionV2
	err := s.db.Update(func(tx database.Tx) error {
		account, err := corecollateral.AccountByIDTx(tx, request.CollateralID)
		if err != nil {
			return fmt.Errorf("load collateral account: %w", err)
		}
		if account.TenantID != request.TenantID || account.ProviderID != request.Requirement.ProviderID ||
			account.ResourceID != request.Requirement.ResourceID || account.PrincipalID != request.Requirement.PrincipalID ||
			account.AssetID != request.Requirement.AssetID || account.PolicyID != request.Requirement.PolicyID ||
			account.PolicyVersion != request.Requirement.PolicyVersion {
			return fmt.Errorf("collateral account does not match the declared requirement")
		}
		persisted, err := orderextensions.LatestByIDTx(tx, request.OrderID, request.Extension.ExtensionID)
		if err != nil {
			return fmt.Errorf("load collateral order extension: %w", err)
		}
		if persisted.ExtensionID != request.Extension.ExtensionID || persisted.Revision != request.Extension.Revision ||
			persisted.PayloadHash != request.Extension.PayloadHash {
			return fmt.Errorf("collateral order extension revision is stale")
		}
		reference, err := corecollateral.AllocateTx(tx, pkgcollateral.AllocationRequest{
			CollateralID: request.CollateralID, TenantID: request.TenantID,
			ProviderID: request.Requirement.ProviderID, ResourceID: request.Requirement.ResourceID,
			PrincipalID: request.Requirement.PrincipalID, OrderID: request.OrderID,
			ExtensionID: persisted.ExtensionID, Amount: request.Requirement.Amount,
			ExpectedCollateralRevision: request.ExpectedCollateralRevision, IdempotencyKey: request.IdempotencyKey,
		}, now)
		if err != nil {
			return err
		}
		envelope, err = corecollateral.BindOrderExtensionV2Tx(tx, corecollateral.OrderExtensionV2Admission{
			TenantID: request.TenantID, OrderID: request.OrderID, PrincipalID: request.Requirement.PrincipalID,
			RequiredAssetID: request.Requirement.AssetID, RequiredAmount: request.Requirement.Amount,
			Envelope: extensions.OrderExtensionV2{
				ContractVersion: extensions.ContractVersionV2, Extension: persisted, CollateralAllocation: &reference,
			},
		}, now)
		return err
	})
	return envelope, err
}

var _ contracts.CollateralAllocationService = (*collateralAllocationService)(nil)

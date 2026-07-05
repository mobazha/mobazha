// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import (
	"context"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/extensions"
)

// AllocateOrderExtensionCollateralRequest asks the owning Core tenant to bind
// active collateral to one exact persisted Order Extension revision.
type AllocateOrderExtensionCollateralRequest struct {
	TenantID                   string
	CollateralID               string
	OrderID                    string
	ExpectedCollateralRevision uint64
	IdempotencyKey             string
	Extension                  extensions.OrderExtension
	Requirement                extensions.CollateralRequirement
}

func (r AllocateOrderExtensionCollateralRequest) Validate() error {
	if strings.TrimSpace(r.TenantID) == "" || strings.TrimSpace(r.CollateralID) == "" ||
		strings.TrimSpace(r.OrderID) == "" || strings.TrimSpace(r.IdempotencyKey) == "" {
		return fmt.Errorf("collateral allocation tenant, account, order, and idempotency key are required")
	}
	if r.ExpectedCollateralRevision == 0 {
		return fmt.Errorf("collateral allocation expected revision is required")
	}
	if err := r.Extension.ValidateForOrder(r.OrderID); err != nil {
		return fmt.Errorf("collateral allocation extension: %w", err)
	}
	if err := r.Requirement.ValidateForExtension(r.Extension); err != nil {
		return fmt.Errorf("collateral allocation requirement: %w", err)
	}
	return nil
}

// CollateralAllocationService is the seller-Core authority boundary. It can
// allocate only already-funded Core collateral and returns a Core-persisted v2
// envelope; it cannot accept provider assertions of funding.
type CollateralAllocationService interface {
	AllocateOrderExtensionCollateral(context.Context, AllocateOrderExtensionCollateralRequest) (extensions.OrderExtensionV2, error)
}

// CollateralAllocationServiceProvider is optional so distributions can expose
// the authority without expanding the universal NodeService locator contract.
type CollateralAllocationServiceProvider interface {
	CollateralAllocation() CollateralAllocationService
}

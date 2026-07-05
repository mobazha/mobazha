// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import (
	"context"
	"fmt"
	"strings"
	"time"

	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
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
	RequestOrderExtensionCollateralCredential(context.Context, RequestOrderExtensionCollateralCredentialRequest) error
	IssueOrderExtensionCollateralCredential(context.Context, IssueOrderExtensionCollateralCredentialRequest) (pkgcollateral.AllocationCredential, error)
	ImportOrderExtensionCollateralCredential(context.Context, ImportOrderExtensionCollateralCredentialRequest) error
}

// CollateralAllocationServiceProvider is optional so distributions can expose
// the authority without expanding the universal NodeService locator contract.
type CollateralAllocationServiceProvider interface {
	CollateralAllocation() CollateralAllocationService
}

type IssueOrderExtensionCollateralCredentialRequest struct {
	TenantID       string
	OrderID        string
	Extension      extensions.OrderExtension
	Requirement    extensions.CollateralRequirement
	AudiencePeerID string
	ExpiresAt      time.Time
}

// RequestOrderExtensionCollateralCredentialRequest starts a durable buyer-to-
// seller request over Core messaging. The seller identity is derived from the
// buyer's persisted order and is never accepted from the caller.
type RequestOrderExtensionCollateralCredentialRequest struct {
	OrderID     string
	Extension   extensions.OrderExtension
	Requirement extensions.CollateralRequirement
	ExpiresAt   time.Time
}

func (r RequestOrderExtensionCollateralCredentialRequest) Validate(now time.Time) error {
	if strings.TrimSpace(r.OrderID) == "" {
		return fmt.Errorf("collateral credential order is required")
	}
	if err := r.Extension.ValidateForOrder(r.OrderID); err != nil {
		return err
	}
	if err := r.Requirement.ValidateForExtension(r.Extension); err != nil {
		return err
	}
	if !r.ExpiresAt.After(now) || r.ExpiresAt.Sub(now) > pkgcollateral.MaxAllocationCredentialTTL {
		return fmt.Errorf("collateral credential expiry must be future and within the maximum TTL")
	}
	return nil
}

func (r IssueOrderExtensionCollateralCredentialRequest) Validate(now time.Time) error {
	if strings.TrimSpace(r.TenantID) == "" || strings.TrimSpace(r.OrderID) == "" || strings.TrimSpace(r.AudiencePeerID) == "" {
		return fmt.Errorf("collateral credential tenant, order, and audience are required")
	}
	if err := r.Extension.ValidateForOrder(r.OrderID); err != nil {
		return err
	}
	if err := r.Requirement.ValidateForExtension(r.Extension); err != nil {
		return err
	}
	if !r.ExpiresAt.After(now) || r.ExpiresAt.Sub(now) > pkgcollateral.MaxAllocationCredentialTTL {
		return fmt.Errorf("collateral credential expiry must be future and within the maximum TTL")
	}
	return nil
}

type ImportOrderExtensionCollateralCredentialRequest struct {
	OrderID     string
	Extension   extensions.OrderExtension
	Requirement extensions.CollateralRequirement
	Credential  pkgcollateral.AllocationCredential
}

func (r ImportOrderExtensionCollateralCredentialRequest) Validate() error {
	if strings.TrimSpace(r.OrderID) == "" {
		return fmt.Errorf("collateral credential order is required")
	}
	if err := r.Extension.ValidateForOrder(r.OrderID); err != nil {
		return err
	}
	return r.Requirement.ValidateForExtension(r.Extension)
}

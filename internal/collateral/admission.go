// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
)

// OrderExtensionV2Admission contains the trusted Core/order expectations used
// to validate an untrusted v2 envelope. It is read-only and cannot allocate,
// release, slash, or otherwise mutate collateral state.
type OrderExtensionV2Admission struct {
	TenantID        string
	OrderID         string
	PrincipalID     string
	RequiredAssetID string
	RequiredAmount  string
	Envelope        extensions.OrderExtensionV2
}

// AdmitOrderExtensionV2Tx reloads the Core-issued allocation and account and
// fails closed on a missing, stale, cross-tenant, or cross-resource binding.
func AdmitOrderExtensionV2Tx(tx database.Tx, request OrderExtensionV2Admission, now time.Time) (pkgcollateral.AllocationReference, error) {
	if tx == nil {
		return pkgcollateral.AllocationReference{}, fmt.Errorf("collateral admission transaction is required")
	}
	if strings.TrimSpace(request.TenantID) == "" || strings.TrimSpace(request.OrderID) == "" || strings.TrimSpace(request.PrincipalID) == "" ||
		strings.TrimSpace(request.RequiredAssetID) == "" {
		return pkgcollateral.AllocationReference{}, fmt.Errorf("collateral admission tenant, order, principal, and asset are required")
	}
	amount, ok := new(big.Int).SetString(request.RequiredAmount, 10)
	if !ok || amount.Sign() <= 0 || amount.String() != request.RequiredAmount {
		return pkgcollateral.AllocationReference{}, fmt.Errorf("collateral admission amount must be canonical positive base units")
	}
	if now.IsZero() {
		return pkgcollateral.AllocationReference{}, fmt.Errorf("collateral admission time is required")
	}
	if request.Envelope.CollateralAllocation == nil {
		return pkgcollateral.AllocationReference{}, fmt.Errorf("order extension v2 collateral allocation is required")
	}
	reference := *request.Envelope.CollateralAllocation
	if err := request.Envelope.ValidateForOrder(request.OrderID); err != nil {
		return pkgcollateral.AllocationReference{}, err
	}
	if reference.TenantID != request.TenantID || reference.OrderID != request.OrderID || reference.PrincipalID != request.PrincipalID ||
		reference.AssetID != request.RequiredAssetID || reference.Amount != request.RequiredAmount {
		return pkgcollateral.AllocationReference{}, fmt.Errorf("collateral admission requirement binding mismatch")
	}

	stored, err := AllocationByIDTx(tx, reference.AllocationID)
	if err != nil {
		return pkgcollateral.AllocationReference{}, fmt.Errorf("load collateral allocation for admission: %w", err)
	}
	if stored != reference {
		return pkgcollateral.AllocationReference{}, fmt.Errorf("collateral admission reference is missing or stale")
	}
	account, err := AccountByIDTx(tx, reference.CollateralID)
	if err != nil {
		return pkgcollateral.AllocationReference{}, fmt.Errorf("load collateral account for admission: %w", err)
	}
	if account.TenantID != reference.TenantID || account.ProviderID != reference.ProviderID ||
		account.ResourceID != reference.ResourceID || account.PrincipalID != reference.PrincipalID ||
		account.AssetID != reference.AssetID {
		return pkgcollateral.AllocationReference{}, fmt.Errorf("collateral admission account scope mismatch")
	}
	if account.State != pkgcollateral.StateActive || !account.ExpiresAt.After(now) ||
		account.Revision != reference.CollateralRevision {
		return pkgcollateral.AllocationReference{}, fmt.Errorf("collateral admission account is inactive, expired, or stale")
	}
	return stored, nil
}

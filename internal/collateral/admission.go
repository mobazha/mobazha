// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/mobazha/mobazha/internal/orderextensions"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
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
		account.Revision < reference.CollateralRevision {
		return pkgcollateral.AllocationReference{}, fmt.Errorf("collateral admission account is inactive, expired, or precedes the allocation")
	}
	return stored, nil
}

// BindOrderExtensionV2Tx persists an already-admitted Core reference against
// the exact latest v1 extension revision. Repeating the same binding is a
// no-op; a provider cannot replace it with a different allocation.
func BindOrderExtensionV2Tx(tx database.Tx, request OrderExtensionV2Admission, now time.Time) (extensions.OrderExtensionV2, error) {
	reference, err := AdmitOrderExtensionV2Tx(tx, request, now)
	if err != nil {
		return extensions.OrderExtensionV2{}, err
	}
	latest, err := orderextensions.LatestByIDTx(tx, request.OrderID, request.Envelope.Extension.ExtensionID)
	if err != nil {
		return extensions.OrderExtensionV2{}, fmt.Errorf("load order extension for collateral binding: %w", err)
	}
	if !sameExtensionRevision(latest, request.Envelope.Extension) {
		return extensions.OrderExtensionV2{}, fmt.Errorf("collateral order extension revision is stale")
	}

	var existing models.CollateralOrderExtensionBindingRecord
	err = tx.Read().Where("order_id = ? AND extension_id = ? AND extension_revision = ?", request.OrderID, latest.ExtensionID, latest.Revision).First(&existing).Error
	if err == nil {
		if !sameOrderExtensionBinding(existing, latest, reference) {
			return extensions.OrderExtensionV2{}, fmt.Errorf("collateral order extension binding is immutable")
		}
		return orderExtensionV2FromBinding(latest, existing)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return extensions.OrderExtensionV2{}, err
	}
	record := models.CollateralOrderExtensionBindingRecord{
		OrderID: request.OrderID, ExtensionID: latest.ExtensionID,
		ContractVersion: extensions.ContractVersionV2, ExtensionRevision: latest.Revision,
		AllocationID: reference.AllocationID, CollateralID: reference.CollateralID,
		ProviderID: reference.ProviderID, ResourceID: reference.ResourceID, PrincipalID: reference.PrincipalID,
		AssetID: reference.AssetID, Amount: reference.Amount,
		CollateralRevision: reference.CollateralRevision, AllocationRevision: reference.AllocationRevision,
		AllocationState: string(reference.State), CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Create(&record); err != nil {
		return extensions.OrderExtensionV2{}, err
	}
	return orderExtensionV2FromBinding(latest, record)
}

// OrderExtensionsV2ByOrderTx returns every Core-owned collateral binding for
// an order, joined to the exact persisted extension revision.
func OrderExtensionsV2ByOrderTx(tx database.Tx, orderID string) ([]extensions.OrderExtensionV2, error) {
	if tx == nil {
		return nil, fmt.Errorf("collateral order extension transaction is required")
	}
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, fmt.Errorf("collateral order extension order ID is required")
	}
	if !tx.Read().Migrator().HasTable(&models.CollateralOrderExtensionBindingRecord{}) {
		return nil, nil
	}
	var records []models.CollateralOrderExtensionBindingRecord
	if err := tx.Read().Where("order_id = ?", orderID).Order("extension_id ASC, extension_revision DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]extensions.OrderExtensionV2, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if _, exists := seen[record.ExtensionID]; exists {
			continue
		}
		seen[record.ExtensionID] = struct{}{}
		extension, err := orderextensions.LatestByIDTx(tx, orderID, record.ExtensionID)
		if err != nil {
			return nil, fmt.Errorf("load collateral-bound order extension: %w", err)
		}
		if extension.Revision != record.ExtensionRevision {
			return nil, fmt.Errorf("collateral order extension binding revision is stale")
		}
		envelope, err := orderExtensionV2FromBinding(extension, record)
		if err != nil {
			return nil, err
		}
		result = append(result, envelope)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Extension.ExtensionID < result[j].Extension.ExtensionID })
	return result, nil
}

// AdmitPersistedOrderExtensionsV2Tx revalidates all v2 bindings immediately
// before payment provisioning. An empty set is a no-op until a provider's C4
// contract requires collateral for that order.
func AdmitPersistedOrderExtensionsV2Tx(tx database.Tx, orderID string, now time.Time) error {
	envelopes, err := OrderExtensionsV2ByOrderTx(tx, orderID)
	if err != nil {
		return err
	}
	for _, envelope := range envelopes {
		reference := envelope.CollateralAllocation
		if reference == nil {
			return fmt.Errorf("persisted collateral order extension has no allocation")
		}
		_, err := AdmitOrderExtensionV2Tx(tx, OrderExtensionV2Admission{
			TenantID: reference.TenantID, OrderID: orderID, PrincipalID: reference.PrincipalID,
			RequiredAssetID: reference.AssetID, RequiredAmount: reference.Amount, Envelope: envelope,
		}, now)
		if err != nil {
			return fmt.Errorf("admit collateral order extension %s: %w", envelope.Extension.ExtensionID, err)
		}
	}
	return nil
}

func orderExtensionV2FromBinding(extension extensions.OrderExtension, record models.CollateralOrderExtensionBindingRecord) (extensions.OrderExtensionV2, error) {
	reference := pkgcollateral.AllocationReference{
		AllocationID: record.AllocationID, CollateralID: record.CollateralID, TenantID: record.TenantID,
		ProviderID: record.ProviderID, ResourceID: record.ResourceID, PrincipalID: record.PrincipalID,
		OrderID: record.OrderID, ExtensionID: record.ExtensionID, AssetID: record.AssetID, Amount: record.Amount,
		CollateralRevision: record.CollateralRevision, AllocationRevision: record.AllocationRevision,
		State: pkgcollateral.AllocationState(record.AllocationState),
	}
	envelope := extensions.OrderExtensionV2{
		ContractVersion: record.ContractVersion, Extension: extension, CollateralAllocation: &reference,
	}
	return envelope, envelope.ValidateForOrder(record.OrderID)
}

func sameExtensionRevision(left, right extensions.OrderExtension) bool {
	return left.ExtensionID == right.ExtensionID && left.ProviderID == right.ProviderID && left.Type == right.Type &&
		left.SchemaVersion == right.SchemaVersion && left.Revision == right.Revision && left.ResourceID == right.ResourceID &&
		left.ReservationRequired == right.ReservationRequired && left.SettlementPolicy == right.SettlementPolicy &&
		left.PayloadHash == right.PayloadHash && left.CreatedAt.Equal(right.CreatedAt) &&
		string(left.Payload) == string(right.Payload) && strings.Join(left.LifecycleEvents, "\x00") == strings.Join(right.LifecycleEvents, "\x00")
}

func sameOrderExtensionBinding(record models.CollateralOrderExtensionBindingRecord, extension extensions.OrderExtension, reference pkgcollateral.AllocationReference) bool {
	return record.ContractVersion == extensions.ContractVersionV2 && record.ExtensionRevision == extension.Revision &&
		record.AllocationID == reference.AllocationID && record.CollateralID == reference.CollateralID &&
		record.ProviderID == reference.ProviderID && record.ResourceID == reference.ResourceID && record.PrincipalID == reference.PrincipalID &&
		record.OrderID == reference.OrderID && record.ExtensionID == reference.ExtensionID && record.AssetID == reference.AssetID &&
		record.Amount == reference.Amount && record.CollateralRevision == reference.CollateralRevision &&
		record.AllocationRevision == reference.AllocationRevision && record.AllocationState == string(reference.State)
}

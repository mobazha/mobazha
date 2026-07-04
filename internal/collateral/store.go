// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Package collateral persists Core-owned collateral state transitions. It has
// no payment execution authority; C2 adapters will submit observations and
// execute separately audited rail actions.
package collateral

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
)

const (
	actionOpen              = "open"
	actionFundingConfirmed  = "funding-confirmed"
	actionAllocate          = "allocate"
	actionAllocationRelease = "allocation-release"
)

func OpenTx(tx database.Tx, request collateral.OpenRequest, now time.Time) (collateral.Account, error) {
	if tx == nil {
		return collateral.Account{}, fmt.Errorf("collateral transaction is required")
	}
	if err := request.Validate(now); err != nil {
		return collateral.Account{}, err
	}
	id := stableID("col", request.TenantID, request.ProviderID, request.ResourceID, request.PrincipalID, request.AssetID, request.PolicyID, request.PolicyVersion)
	var existing models.CollateralAccountRecord
	err := tx.Read().Where("collateral_id = ? OR open_idempotency_key = ?", id, request.IdempotencyKey).First(&existing).Error
	if err == nil {
		if !sameOpenRequest(existing, request, id) {
			return collateral.Account{}, fmt.Errorf("collateral open idempotency or resource binding conflict")
		}
		return accountFromRecord(existing)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return collateral.Account{}, err
	}
	record := models.CollateralAccountRecord{
		CollateralID: id, ProviderID: request.ProviderID, ResourceID: request.ResourceID,
		PrincipalID: request.PrincipalID, AssetID: request.AssetID, RequiredAmount: request.RequiredAmount,
		FundedAmount: "0", AvailableAmount: "0", PolicyID: request.PolicyID, PolicyVersion: request.PolicyVersion,
		OpenIdempotencyKey: request.IdempotencyKey, Revision: 1, State: string(collateral.StatePendingFunding),
		ExpiresAt: request.ExpiresAt, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Create(&record); err != nil {
		return collateral.Account{}, err
	}
	if record.TenantID != request.TenantID {
		return collateral.Account{}, fmt.Errorf("collateral request tenant does not match transaction scope")
	}
	if err := recordAction(tx, models.CollateralActionRecord{
		ActionID: stableID("cola", request.TenantID, request.IdempotencyKey), CollateralID: id,
		Kind: actionOpen, IdempotencyKey: request.IdempotencyKey, ResultCollateralRevision: 1,
		Amount: request.RequiredAmount, AssetID: request.AssetID, CreatedAt: now,
	}); err != nil {
		return collateral.Account{}, err
	}
	return accountFromRecord(record)
}

func RecordFundingTx(tx database.Tx, observation collateral.FundingObservation, now time.Time) (collateral.Account, error) {
	if tx == nil {
		return collateral.Account{}, fmt.Errorf("collateral transaction is required")
	}
	if err := observation.Validate(now); err != nil {
		return collateral.Account{}, err
	}
	if action, err := actionByIdempotency(tx, observation.IdempotencyKey); err == nil {
		if action.Kind != actionFundingConfirmed || action.CollateralID != observation.CollateralID || action.Amount != observation.FundedAmount || action.AssetID != observation.AssetID || action.Reference != observation.FundingReference {
			return collateral.Account{}, fmt.Errorf("collateral funding idempotency conflict")
		}
		return AccountByIDTx(tx, observation.CollateralID)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return collateral.Account{}, err
	}
	record, err := accountRecordByID(tx, observation.CollateralID)
	if err != nil {
		return collateral.Account{}, err
	}
	if record.TenantID != observation.TenantID {
		return collateral.Account{}, fmt.Errorf("collateral funding tenant does not match account")
	}
	if record.Revision != observation.ExpectedRevision {
		return collateral.Account{}, fmt.Errorf("collateral revision conflict: got %d want %d", record.Revision, observation.ExpectedRevision)
	}
	if record.State != string(collateral.StatePendingFunding) {
		return collateral.Account{}, fmt.Errorf("collateral funding requires pending-funding state")
	}
	if !record.ExpiresAt.After(now) {
		return collateral.Account{}, fmt.Errorf("expired collateral cannot be activated")
	}
	if record.AssetID != observation.AssetID {
		return collateral.Account{}, fmt.Errorf("collateral funding asset does not match account")
	}
	if compareAmount(observation.FundedAmount, record.RequiredAmount) < 0 {
		return collateral.Account{}, fmt.Errorf("collateral funding is below the required amount")
	}
	var claimed models.CollateralFundingRecord
	err = tx.Read().Where("asset_id = ? AND funding_reference = ?", observation.AssetID, observation.FundingReference).First(&claimed).Error
	if err == nil {
		return collateral.Account{}, fmt.Errorf("collateral funding reference is already claimed by account %s", claimed.CollateralID)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return collateral.Account{}, err
	}
	if err := tx.Create(&models.CollateralFundingRecord{
		FundingID:    stableID("colf", observation.TenantID, observation.AssetID, observation.FundingReference),
		CollateralID: record.CollateralID, AssetID: observation.AssetID, Amount: observation.FundedAmount,
		FundingReference: observation.FundingReference, ObservedAt: observation.ObservedAt, CreatedAt: now,
	}); err != nil {
		return collateral.Account{}, fmt.Errorf("claim collateral funding reference: %w", err)
	}
	nextRevision := record.Revision + 1
	activatedAt := observation.ObservedAt.UTC()
	rows, err := tx.UpdateColumns(map[string]interface{}{
		"funded_amount": observation.FundedAmount, "available_amount": observation.FundedAmount,
		"funding_reference": observation.FundingReference, "state": string(collateral.StateActive),
		"activated_at": activatedAt, "revision": nextRevision, "updated_at": now,
	}, map[string]interface{}{"collateral_id = ?": record.CollateralID, "revision = ?": record.Revision}, &models.CollateralAccountRecord{})
	if err != nil {
		return collateral.Account{}, err
	}
	if rows != 1 {
		return collateral.Account{}, fmt.Errorf("collateral revision conflict")
	}
	if err := recordAction(tx, models.CollateralActionRecord{
		ActionID: stableID("cola", observation.TenantID, observation.IdempotencyKey), CollateralID: record.CollateralID,
		Kind: actionFundingConfirmed, IdempotencyKey: observation.IdempotencyKey,
		ExpectedCollateralRevision: record.Revision, ResultCollateralRevision: nextRevision,
		Amount: observation.FundedAmount, AssetID: observation.AssetID, Reference: observation.FundingReference, CreatedAt: now,
	}); err != nil {
		return collateral.Account{}, err
	}
	return AccountByIDTx(tx, record.CollateralID)
}

func AllocateTx(tx database.Tx, request collateral.AllocationRequest, now time.Time) (collateral.AllocationReference, error) {
	if tx == nil {
		return collateral.AllocationReference{}, fmt.Errorf("collateral transaction is required")
	}
	if err := request.Validate(); err != nil {
		return collateral.AllocationReference{}, err
	}
	allocationID := stableID("alloc", request.TenantID, request.CollateralID, request.OrderID, request.ExtensionID)
	var existing models.CollateralAllocationRecord
	err := tx.Read().Where("allocation_id = ? OR idempotency_key = ?", allocationID, request.IdempotencyKey).First(&existing).Error
	if err == nil {
		if !sameAllocationRequest(existing, request, allocationID) {
			return collateral.AllocationReference{}, fmt.Errorf("collateral allocation idempotency or binding conflict")
		}
		return allocationFromRecord(existing)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return collateral.AllocationReference{}, err
	}
	account, err := accountRecordByID(tx, request.CollateralID)
	if err != nil {
		return collateral.AllocationReference{}, err
	}
	if account.TenantID != request.TenantID {
		return collateral.AllocationReference{}, fmt.Errorf("collateral allocation tenant does not match account")
	}
	if account.Revision != request.ExpectedCollateralRevision {
		return collateral.AllocationReference{}, fmt.Errorf("collateral revision conflict: got %d want %d", account.Revision, request.ExpectedCollateralRevision)
	}
	if account.State != string(collateral.StateActive) || !account.ExpiresAt.After(now) {
		return collateral.AllocationReference{}, fmt.Errorf("collateral account is not active and unexpired")
	}
	if account.ProviderID != request.ProviderID || account.ResourceID != request.ResourceID || account.PrincipalID != request.PrincipalID {
		return collateral.AllocationReference{}, fmt.Errorf("collateral allocation scope does not match account")
	}
	if compareAmount(account.AvailableAmount, request.Amount) < 0 {
		return collateral.AllocationReference{}, fmt.Errorf("insufficient available collateral")
	}
	nextAvailable := subtractAmount(account.AvailableAmount, request.Amount)
	nextRevision := account.Revision + 1
	rows, err := tx.UpdateColumns(map[string]interface{}{
		"available_amount": nextAvailable, "revision": nextRevision, "updated_at": now,
	}, map[string]interface{}{"collateral_id = ?": account.CollateralID, "revision = ?": account.Revision}, &models.CollateralAccountRecord{})
	if err != nil {
		return collateral.AllocationReference{}, err
	}
	if rows != 1 {
		return collateral.AllocationReference{}, fmt.Errorf("collateral revision conflict")
	}
	record := models.CollateralAllocationRecord{
		AllocationID: allocationID, CollateralID: account.CollateralID, ProviderID: account.ProviderID,
		ResourceID: account.ResourceID, PrincipalID: account.PrincipalID, OrderID: request.OrderID,
		ExtensionID: request.ExtensionID, AssetID: account.AssetID, Amount: request.Amount,
		CollateralRevision: nextRevision, AllocationRevision: 1, State: string(collateral.AllocationActive),
		IdempotencyKey: request.IdempotencyKey, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Create(&record); err != nil {
		return collateral.AllocationReference{}, err
	}
	if err := recordAction(tx, models.CollateralActionRecord{
		ActionID: stableID("cola", request.TenantID, request.IdempotencyKey), CollateralID: account.CollateralID,
		AllocationID: allocationID, Kind: actionAllocate, IdempotencyKey: request.IdempotencyKey,
		ExpectedCollateralRevision: account.Revision, ResultCollateralRevision: nextRevision,
		ResultAllocationRevision: 1, Amount: request.Amount, AssetID: account.AssetID, CreatedAt: now,
	}); err != nil {
		return collateral.AllocationReference{}, err
	}
	return allocationFromRecord(record)
}

func ReleaseAllocationTx(tx database.Tx, request collateral.AllocationReleaseRequest, now time.Time) (collateral.AllocationReference, error) {
	if tx == nil {
		return collateral.AllocationReference{}, fmt.Errorf("collateral transaction is required")
	}
	if err := request.Validate(); err != nil {
		return collateral.AllocationReference{}, err
	}
	if action, err := actionByIdempotency(tx, request.IdempotencyKey); err == nil {
		if action.Kind != actionAllocationRelease || action.CollateralID != request.CollateralID || action.AllocationID != request.AllocationID || action.Reason != request.Reason {
			return collateral.AllocationReference{}, fmt.Errorf("collateral allocation release idempotency conflict")
		}
		return AllocationByIDTx(tx, request.AllocationID)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return collateral.AllocationReference{}, err
	}
	account, err := accountRecordByID(tx, request.CollateralID)
	if err != nil {
		return collateral.AllocationReference{}, err
	}
	if account.TenantID != request.TenantID {
		return collateral.AllocationReference{}, fmt.Errorf("collateral allocation release tenant does not match account")
	}
	var allocation models.CollateralAllocationRecord
	if err := tx.Read().Where("allocation_id = ? AND collateral_id = ?", request.AllocationID, request.CollateralID).First(&allocation).Error; err != nil {
		return collateral.AllocationReference{}, err
	}
	if account.Revision != request.ExpectedCollateralRevision || allocation.AllocationRevision != request.ExpectedAllocationRevision {
		return collateral.AllocationReference{}, fmt.Errorf("collateral allocation release revision conflict")
	}
	if account.State != string(collateral.StateActive) || allocation.State != string(collateral.AllocationActive) {
		return collateral.AllocationReference{}, fmt.Errorf("collateral allocation release requires active account and allocation")
	}
	nextCollateralRevision := account.Revision + 1
	nextAllocationRevision := allocation.AllocationRevision + 1
	nextAvailable := addAmount(account.AvailableAmount, allocation.Amount)
	if compareAmount(nextAvailable, account.FundedAmount) > 0 {
		return collateral.AllocationReference{}, fmt.Errorf("collateral allocation release would exceed funded amount")
	}
	rows, err := tx.UpdateColumns(map[string]interface{}{
		"available_amount": nextAvailable, "revision": nextCollateralRevision, "updated_at": now,
	}, map[string]interface{}{"collateral_id = ?": account.CollateralID, "revision = ?": account.Revision}, &models.CollateralAccountRecord{})
	if err != nil || rows != 1 {
		if err != nil {
			return collateral.AllocationReference{}, err
		}
		return collateral.AllocationReference{}, fmt.Errorf("collateral revision conflict")
	}
	rows, err = tx.UpdateColumns(map[string]interface{}{
		"state": string(collateral.AllocationReleased), "allocation_revision": nextAllocationRevision,
		"collateral_revision": nextCollateralRevision, "updated_at": now,
	}, map[string]interface{}{"allocation_id = ?": allocation.AllocationID, "allocation_revision = ?": allocation.AllocationRevision}, &models.CollateralAllocationRecord{})
	if err != nil || rows != 1 {
		if err != nil {
			return collateral.AllocationReference{}, err
		}
		return collateral.AllocationReference{}, fmt.Errorf("collateral allocation revision conflict")
	}
	if err := recordAction(tx, models.CollateralActionRecord{
		ActionID: stableID("cola", request.TenantID, request.IdempotencyKey), CollateralID: account.CollateralID,
		AllocationID: allocation.AllocationID, Kind: actionAllocationRelease, IdempotencyKey: request.IdempotencyKey,
		ExpectedCollateralRevision: account.Revision, ResultCollateralRevision: nextCollateralRevision,
		ExpectedAllocationRevision: allocation.AllocationRevision, ResultAllocationRevision: nextAllocationRevision,
		Amount: allocation.Amount, AssetID: allocation.AssetID, Reason: request.Reason, CreatedAt: now,
	}); err != nil {
		return collateral.AllocationReference{}, err
	}
	return AllocationByIDTx(tx, allocation.AllocationID)
}

func AccountByIDTx(tx database.Tx, collateralID string) (collateral.Account, error) {
	record, err := accountRecordByID(tx, collateralID)
	if err != nil {
		return collateral.Account{}, err
	}
	return accountFromRecord(record)
}

func AllocationByIDTx(tx database.Tx, allocationID string) (collateral.AllocationReference, error) {
	var record models.CollateralAllocationRecord
	if err := tx.Read().Where("allocation_id = ?", strings.TrimSpace(allocationID)).First(&record).Error; err != nil {
		return collateral.AllocationReference{}, err
	}
	return allocationFromRecord(record)
}

func accountRecordByID(tx database.Tx, collateralID string) (models.CollateralAccountRecord, error) {
	var record models.CollateralAccountRecord
	err := tx.Read().Where("collateral_id = ?", strings.TrimSpace(collateralID)).First(&record).Error
	return record, err
}

func actionByIdempotency(tx database.Tx, key string) (models.CollateralActionRecord, error) {
	var record models.CollateralActionRecord
	err := tx.Read().Where("idempotency_key = ?", strings.TrimSpace(key)).First(&record).Error
	return record, err
}

func recordAction(tx database.Tx, record models.CollateralActionRecord) error {
	return tx.Create(&record)
}

func accountFromRecord(record models.CollateralAccountRecord) (collateral.Account, error) {
	account := collateral.Account{
		CollateralID: record.CollateralID, TenantID: record.TenantID, ProviderID: record.ProviderID,
		ResourceID: record.ResourceID, PrincipalID: record.PrincipalID, AssetID: record.AssetID,
		RequiredAmount: record.RequiredAmount, FundedAmount: record.FundedAmount, AvailableAmount: record.AvailableAmount,
		PolicyID: record.PolicyID, PolicyVersion: record.PolicyVersion, FundingReference: record.FundingReference,
		Revision: record.Revision, State: collateral.State(record.State), ActivatedAt: record.ActivatedAt, ExpiresAt: record.ExpiresAt,
	}
	return account, account.Validate()
}

func allocationFromRecord(record models.CollateralAllocationRecord) (collateral.AllocationReference, error) {
	reference := collateral.AllocationReference{
		AllocationID: record.AllocationID, CollateralID: record.CollateralID, TenantID: record.TenantID,
		OrderID: record.OrderID, ExtensionID: record.ExtensionID, AssetID: record.AssetID, Amount: record.Amount,
		CollateralRevision: record.CollateralRevision, AllocationRevision: record.AllocationRevision,
		State: collateral.AllocationState(record.State),
	}
	return reference, reference.Validate()
}

func sameOpenRequest(record models.CollateralAccountRecord, request collateral.OpenRequest, id string) bool {
	return record.TenantID == request.TenantID && record.CollateralID == id && record.ProviderID == request.ProviderID && record.ResourceID == request.ResourceID &&
		record.PrincipalID == request.PrincipalID && record.AssetID == request.AssetID && record.RequiredAmount == request.RequiredAmount &&
		record.PolicyID == request.PolicyID && record.PolicyVersion == request.PolicyVersion && record.OpenIdempotencyKey == request.IdempotencyKey &&
		record.ExpiresAt.Equal(request.ExpiresAt)
}

func sameAllocationRequest(record models.CollateralAllocationRecord, request collateral.AllocationRequest, id string) bool {
	return record.AllocationID == id && record.CollateralID == request.CollateralID && record.ProviderID == request.ProviderID &&
		record.ResourceID == request.ResourceID && record.PrincipalID == request.PrincipalID && record.OrderID == request.OrderID &&
		record.ExtensionID == request.ExtensionID && record.Amount == request.Amount && record.IdempotencyKey == request.IdempotencyKey
}

func stableID(prefix string, parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return prefix + "_" + hex.EncodeToString(digest[:16])
}

func compareAmount(left, right string) int { return amount(left).Cmp(amount(right)) }
func addAmount(left, right string) string {
	return new(big.Int).Add(amount(left), amount(right)).String()
}
func subtractAmount(left, right string) string {
	return new(big.Int).Sub(amount(left), amount(right)).String()
}
func amount(value string) *big.Int {
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok {
		panic("validated collateral amount is not an integer")
	}
	return parsed
}

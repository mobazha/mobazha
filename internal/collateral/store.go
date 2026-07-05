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
	"strconv"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
)

const (
	actionOpen              = "open"
	actionFundingConfirmed  = "funding-confirmed"
	actionAllocate          = "allocate"
	actionAllocationRelease = "allocation-release"
	actionReleaseRequested  = "release-requested"
	actionClaimAccepted     = "claim-accepted"
	actionReleaseConfirmed  = "release-confirmed"
	actionSlashConfirmed    = "slash-confirmed"
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
	if compareAmount(account.FundedAmount, account.RequiredAmount) < 0 {
		return collateral.AllocationReference{}, fmt.Errorf("collateral account is below its funding requirement")
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
	if (account.State != string(collateral.StateActive) && account.State != string(collateral.StateSlashPending)) || allocation.State != string(collateral.AllocationActive) {
		return collateral.AllocationReference{}, fmt.Errorf("collateral allocation release requires serviceable account and active allocation")
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

// RequestAccountReleaseTx transitions an unallocated active account to
// release-pending. It records intent only and never chooses a wallet address.
func RequestAccountReleaseTx(tx database.Tx, request collateral.AccountReleaseRequest, now time.Time) (collateral.Account, error) {
	if tx == nil {
		return collateral.Account{}, fmt.Errorf("collateral transaction is required")
	}
	if err := request.Validate(); err != nil {
		return collateral.Account{}, err
	}
	if action, err := actionByIdempotency(tx, request.IdempotencyKey); err == nil {
		if action.Kind != actionReleaseRequested || action.CollateralID != request.CollateralID || action.Reason != request.Reason {
			return collateral.Account{}, fmt.Errorf("collateral release request idempotency conflict")
		}
		return AccountByIDTx(tx, request.CollateralID)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return collateral.Account{}, err
	}
	account, err := accountRecordByID(tx, request.CollateralID)
	if err != nil {
		return collateral.Account{}, err
	}
	if account.TenantID != request.TenantID {
		return collateral.Account{}, fmt.Errorf("collateral release tenant does not match account")
	}
	if account.Revision != request.ExpectedRevision {
		return collateral.Account{}, fmt.Errorf("collateral release revision conflict")
	}
	if account.State != string(collateral.StateActive) {
		return collateral.Account{}, fmt.Errorf("collateral release requires active account")
	}
	if compareAmount(account.AvailableAmount, account.FundedAmount) != 0 {
		return collateral.Account{}, fmt.Errorf("collateral release requires all allocations to be released")
	}
	var activeAllocations int64
	if err := tx.Read().Model(&models.CollateralAllocationRecord{}).
		Where("collateral_id = ? AND state = ?", account.CollateralID, string(collateral.AllocationActive)).
		Count(&activeAllocations).Error; err != nil {
		return collateral.Account{}, err
	}
	if activeAllocations != 0 {
		return collateral.Account{}, fmt.Errorf("collateral release requires all allocations to be terminal")
	}
	nextRevision := account.Revision + 1
	rows, err := tx.UpdateColumns(map[string]interface{}{
		"state": string(collateral.StateReleasePending), "revision": nextRevision, "updated_at": now,
	}, map[string]interface{}{"collateral_id = ?": account.CollateralID, "revision = ?": account.Revision}, &models.CollateralAccountRecord{})
	if err != nil || rows != 1 {
		if err != nil {
			return collateral.Account{}, err
		}
		return collateral.Account{}, fmt.Errorf("collateral release revision conflict")
	}
	if err := recordAction(tx, models.CollateralActionRecord{
		ActionID: stableID("cola", request.TenantID, request.IdempotencyKey), CollateralID: account.CollateralID,
		Kind: actionReleaseRequested, IdempotencyKey: request.IdempotencyKey,
		ExpectedCollateralRevision: account.Revision, ResultCollateralRevision: nextRevision,
		Amount: account.FundedAmount, AssetID: account.AssetID, Reason: request.Reason, CreatedAt: now,
	}); err != nil {
		return collateral.Account{}, err
	}
	return AccountByIDTx(tx, account.CollateralID)
}

// AcceptClaimTx validates provider evidence against the exact active
// allocation and moves the account to slash-pending. No funds move here.
func AcceptClaimTx(tx database.Tx, decision collateral.ClaimDecision, now time.Time) (collateral.Claim, error) {
	if tx == nil {
		return collateral.Claim{}, fmt.Errorf("collateral transaction is required")
	}
	if err := decision.Validate(now); err != nil {
		return collateral.Claim{}, err
	}
	if existing, err := claimByIdempotency(tx, decision.IdempotencyKey); err == nil {
		if !sameClaimDecision(existing, decision) {
			return collateral.Claim{}, fmt.Errorf("collateral claim idempotency conflict")
		}
		return claimFromRecord(existing)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return collateral.Claim{}, err
	}
	a := decision.Attestation
	replayFingerprint := claimReplayFingerprint(a)
	var replayed models.CollateralClaimRecord
	err := tx.Read().Where("attestation_id = ? OR attestation_idempotency_key = ? OR replay_fingerprint = ?", a.AttestationID, a.IdempotencyKey, replayFingerprint).First(&replayed).Error
	if err == nil {
		return collateral.Claim{}, fmt.Errorf("collateral claim evidence replay")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return collateral.Claim{}, err
	}
	account, err := accountRecordByID(tx, a.CollateralID)
	if err != nil {
		return collateral.Claim{}, err
	}
	if account.TenantID != a.TenantID || account.ProviderID != a.Issuer {
		return collateral.Claim{}, fmt.Errorf("collateral claim tenant or issuer does not match account")
	}
	var allocation models.CollateralAllocationRecord
	if err := tx.Read().Where("allocation_id = ? AND collateral_id = ?", a.AllocationID, a.CollateralID).First(&allocation).Error; err != nil {
		return collateral.Claim{}, err
	}
	if allocation.OrderID != a.OrderID || allocation.ExtensionID != a.ExtensionID {
		return collateral.Claim{}, fmt.Errorf("collateral claim order or extension does not match allocation")
	}
	if account.Revision != a.ExpectedCollateralRevision || allocation.AllocationRevision != a.ExpectedAllocationRevision {
		return collateral.Claim{}, fmt.Errorf("collateral claim revision conflict")
	}
	if account.State != string(collateral.StateActive) || allocation.State != string(collateral.AllocationActive) {
		return collateral.Claim{}, fmt.Errorf("collateral claim requires active account and allocation")
	}
	if compareAmount(decision.Amount, allocation.Amount) > 0 {
		return collateral.Claim{}, fmt.Errorf("collateral claim exceeds allocation")
	}
	nextCollateralRevision := account.Revision + 1
	nextAllocationRevision := allocation.AllocationRevision + 1
	rows, err := tx.UpdateColumns(map[string]interface{}{
		"state": string(collateral.StateSlashPending), "revision": nextCollateralRevision, "updated_at": now,
	}, map[string]interface{}{"collateral_id = ?": account.CollateralID, "revision = ?": account.Revision}, &models.CollateralAccountRecord{})
	if err != nil || rows != 1 {
		if err != nil {
			return collateral.Claim{}, err
		}
		return collateral.Claim{}, fmt.Errorf("collateral claim revision conflict")
	}
	rows, err = tx.UpdateColumns(map[string]interface{}{
		"state": string(collateral.AllocationClaimed), "allocation_revision": nextAllocationRevision,
		"collateral_revision": nextCollateralRevision, "updated_at": now,
	}, map[string]interface{}{"allocation_id = ?": allocation.AllocationID, "allocation_revision = ?": allocation.AllocationRevision}, &models.CollateralAllocationRecord{})
	if err != nil || rows != 1 {
		if err != nil {
			return collateral.Claim{}, err
		}
		return collateral.Claim{}, fmt.Errorf("collateral allocation claim revision conflict")
	}
	record := models.CollateralClaimRecord{
		ClaimID: decision.ClaimID, CollateralID: account.CollateralID, AllocationID: allocation.AllocationID,
		OrderID: allocation.OrderID, ExtensionID: allocation.ExtensionID, AttestationID: a.AttestationID,
		AttestationIdempotencyKey: a.IdempotencyKey, ReplayFingerprint: replayFingerprint,
		IdempotencyKey: decision.IdempotencyKey, Issuer: a.Issuer, Amount: decision.Amount, Reason: decision.Reason,
		ConditionType: a.ConditionType, ConditionVersion: a.ConditionVersion, EvidenceDigest: a.EvidenceDigest,
		ExpectedCollateralRevision: a.ExpectedCollateralRevision, ExpectedAllocationRevision: a.ExpectedAllocationRevision,
		ExpectedOrderStateVersion: a.ExpectedOrderStateVersion,
		CollateralRevision:        nextCollateralRevision, AllocationRevision: nextAllocationRevision,
		State: string(collateral.ClaimPendingSlash), ObservedAt: a.ObservedAt, ExpiresAt: a.ExpiresAt,
		AcceptedAt: now, UpdatedAt: now,
	}
	if err := tx.Create(&record); err != nil {
		return collateral.Claim{}, err
	}
	if err := recordAction(tx, models.CollateralActionRecord{
		ActionID: stableID("cola", a.TenantID, decision.IdempotencyKey), CollateralID: account.CollateralID,
		AllocationID: allocation.AllocationID, Kind: actionClaimAccepted, IdempotencyKey: decision.IdempotencyKey,
		ExpectedCollateralRevision: account.Revision, ResultCollateralRevision: nextCollateralRevision,
		ExpectedAllocationRevision: allocation.AllocationRevision, ResultAllocationRevision: nextAllocationRevision,
		Amount: decision.Amount, AssetID: account.AssetID, Reason: decision.Reason, Reference: decision.ClaimID, CreatedAt: now,
	}); err != nil {
		return collateral.Claim{}, err
	}
	return claimFromRecord(record)
}

// RecordExecutionTx accepts a confirmed payment-adapter observation and
// completes either an account release or a claim slash. Ambiguous results must
// remain pending and are not passed to this function.
func RecordExecutionTx(tx database.Tx, observation collateral.ExecutionObservation, now time.Time) (collateral.Account, error) {
	if tx == nil {
		return collateral.Account{}, fmt.Errorf("collateral transaction is required")
	}
	if err := observation.Validate(now); err != nil {
		return collateral.Account{}, err
	}
	actionKind := actionReleaseConfirmed
	if observation.Kind == collateral.ExecutionSlash {
		actionKind = actionSlashConfirmed
	}
	if action, err := actionByIdempotency(tx, observation.IdempotencyKey); err == nil {
		if action.Kind != actionKind || action.CollateralID != observation.CollateralID || action.Amount != observation.Amount || action.AssetID != observation.AssetID || action.Reference != observation.ExecutionReference {
			return collateral.Account{}, fmt.Errorf("collateral execution idempotency conflict")
		}
		return AccountByIDTx(tx, observation.CollateralID)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return collateral.Account{}, err
	}
	account, err := accountRecordByID(tx, observation.CollateralID)
	if err != nil {
		return collateral.Account{}, err
	}
	if account.TenantID != observation.TenantID || account.AssetID != observation.AssetID {
		return collateral.Account{}, fmt.Errorf("collateral execution tenant or asset does not match account")
	}
	if account.Revision != observation.ExpectedRevision {
		return collateral.Account{}, fmt.Errorf("collateral execution revision conflict")
	}
	var claimed models.CollateralExecutionRecord
	err = tx.Read().Where("asset_id = ? AND execution_reference = ?", observation.AssetID, observation.ExecutionReference).First(&claimed).Error
	if err == nil {
		return collateral.Account{}, fmt.Errorf("collateral execution reference is already claimed by account %s", claimed.CollateralID)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return collateral.Account{}, err
	}
	nextRevision := account.Revision + 1
	values := map[string]interface{}{"revision": nextRevision, "updated_at": now}
	action := models.CollateralActionRecord{
		ActionID: stableID("cola", observation.TenantID, observation.IdempotencyKey), CollateralID: account.CollateralID,
		Kind: actionKind, IdempotencyKey: observation.IdempotencyKey,
		ExpectedCollateralRevision: account.Revision, ResultCollateralRevision: nextRevision,
		Amount: observation.Amount, AssetID: observation.AssetID, Reference: observation.ExecutionReference, CreatedAt: now,
	}
	if observation.Kind == collateral.ExecutionRelease {
		if account.State != string(collateral.StateReleasePending) || observation.Amount != account.FundedAmount || account.AvailableAmount != account.FundedAmount {
			return collateral.Account{}, fmt.Errorf("collateral release execution does not match pending account balance")
		}
		values["funded_amount"] = "0"
		values["available_amount"] = "0"
		values["state"] = string(collateral.StateReleased)
	} else {
		if account.State != string(collateral.StateSlashPending) {
			return collateral.Account{}, fmt.Errorf("collateral slash execution requires slash-pending account")
		}
		claim, err := claimRecordByID(tx, observation.ClaimID)
		if err != nil {
			return collateral.Account{}, err
		}
		if claim.CollateralID != account.CollateralID || claim.State != string(collateral.ClaimPendingSlash) || claim.Amount != observation.Amount {
			return collateral.Account{}, fmt.Errorf("collateral slash execution does not match pending claim")
		}
		var allocation models.CollateralAllocationRecord
		if err := tx.Read().Where("allocation_id = ?", claim.AllocationID).First(&allocation).Error; err != nil {
			return collateral.Account{}, err
		}
		if compareAmount(observation.Amount, account.FundedAmount) > 0 || compareAmount(observation.Amount, allocation.Amount) > 0 {
			return collateral.Account{}, fmt.Errorf("collateral slash amount exceeds funded or allocated amount")
		}
		newFunded := subtractAmount(account.FundedAmount, observation.Amount)
		unusedAllocation := subtractAmount(allocation.Amount, observation.Amount)
		newAvailable := addAmount(account.AvailableAmount, unusedAllocation)
		if compareAmount(newAvailable, newFunded) > 0 {
			return collateral.Account{}, fmt.Errorf("collateral slash result exceeds remaining funded balance")
		}
		values["funded_amount"] = newFunded
		values["available_amount"] = newAvailable
		if newFunded == "0" {
			values["state"] = string(collateral.StateSlashed)
		} else {
			values["state"] = string(collateral.StateActive)
		}
		claimRevision := claim.AllocationRevision
		rows, err := tx.UpdateColumns(map[string]interface{}{
			"state": string(collateral.ClaimSlashed), "execution_reference": observation.ExecutionReference,
			"collateral_revision": nextRevision, "updated_at": now,
		}, map[string]interface{}{"claim_id = ?": claim.ClaimID, "state = ?": string(collateral.ClaimPendingSlash)}, &models.CollateralClaimRecord{})
		if err != nil || rows != 1 {
			if err != nil {
				return collateral.Account{}, err
			}
			return collateral.Account{}, fmt.Errorf("collateral claim execution conflict")
		}
		action.AllocationID = claim.AllocationID
		action.ExpectedAllocationRevision = claimRevision
		action.ResultAllocationRevision = claimRevision
	}
	if err := tx.Create(&models.CollateralExecutionRecord{
		ExecutionID:  stableID("cole", observation.TenantID, string(observation.Kind), observation.AssetID, observation.ExecutionReference),
		CollateralID: account.CollateralID, ClaimID: observation.ClaimID, Kind: string(observation.Kind),
		AssetID: observation.AssetID, Amount: observation.Amount, ExecutionReference: observation.ExecutionReference,
		ObservedAt: observation.ObservedAt, CreatedAt: now,
	}); err != nil {
		return collateral.Account{}, fmt.Errorf("claim collateral execution reference: %w", err)
	}
	rows, err := tx.UpdateColumns(values,
		map[string]interface{}{"collateral_id = ?": account.CollateralID, "revision = ?": account.Revision},
		&models.CollateralAccountRecord{})
	if err != nil || rows != 1 {
		if err != nil {
			return collateral.Account{}, err
		}
		return collateral.Account{}, fmt.Errorf("collateral execution revision conflict")
	}
	if err := recordAction(tx, action); err != nil {
		return collateral.Account{}, err
	}
	return AccountByIDTx(tx, account.CollateralID)
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

// ActiveAccountForRequirementTx resolves seller-owned coverage without
// accepting an account identifier from the remote buyer. Selection is stable
// and only returns an active, unexpired account with sufficient availability.
func ActiveAccountForRequirementTx(
	tx database.Tx,
	tenantID string,
	requirement extensions.CollateralRequirement,
	now time.Time,
) (collateral.Account, error) {
	if tx == nil || strings.TrimSpace(tenantID) == "" || now.IsZero() {
		return collateral.Account{}, fmt.Errorf("collateral account lookup is incomplete")
	}
	var records []models.CollateralAccountRecord
	if err := tx.Read().Where(
		"tenant_id = ? AND provider_id = ? AND resource_id = ? AND principal_id = ? AND asset_id = ? AND policy_id = ? AND policy_version = ? AND state = ? AND expires_at > ?",
		tenantID, requirement.ProviderID, requirement.ResourceID, requirement.PrincipalID,
		requirement.AssetID, requirement.PolicyID, requirement.PolicyVersion, collateral.StateActive, now,
	).Order("revision DESC, collateral_id ASC").Find(&records).Error; err != nil {
		return collateral.Account{}, err
	}
	for _, record := range records {
		account, err := accountFromRecord(record)
		if err != nil {
			return collateral.Account{}, err
		}
		if compareAmount(account.AvailableAmount, requirement.Amount) >= 0 {
			return account, nil
		}
	}
	return collateral.Account{}, gorm.ErrRecordNotFound
}

func ClaimByIDTx(tx database.Tx, claimID string) (collateral.Claim, error) {
	record, err := claimRecordByID(tx, claimID)
	if err != nil {
		return collateral.Claim{}, err
	}
	return claimFromRecord(record)
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

func claimByIdempotency(tx database.Tx, key string) (models.CollateralClaimRecord, error) {
	var record models.CollateralClaimRecord
	err := tx.Read().Where("idempotency_key = ?", strings.TrimSpace(key)).First(&record).Error
	return record, err
}

func claimRecordByID(tx database.Tx, claimID string) (models.CollateralClaimRecord, error) {
	var record models.CollateralClaimRecord
	err := tx.Read().Where("claim_id = ?", strings.TrimSpace(claimID)).First(&record).Error
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
		ProviderID: record.ProviderID, ResourceID: record.ResourceID, PrincipalID: record.PrincipalID,
		OrderID: record.OrderID, ExtensionID: record.ExtensionID, AssetID: record.AssetID, Amount: record.Amount,
		CollateralRevision: record.CollateralRevision, AllocationRevision: record.AllocationRevision,
		State: collateral.AllocationState(record.State),
	}
	return reference, reference.Validate()
}

func claimFromRecord(record models.CollateralClaimRecord) (collateral.Claim, error) {
	claim := collateral.Claim{
		ClaimID: record.ClaimID, TenantID: record.TenantID, CollateralID: record.CollateralID,
		AllocationID: record.AllocationID, OrderID: record.OrderID, ExtensionID: record.ExtensionID,
		Issuer: record.Issuer, Amount: record.Amount, ConditionType: record.ConditionType,
		ConditionVersion: record.ConditionVersion, EvidenceDigest: record.EvidenceDigest,
		ExpectedOrderStateVersion: record.ExpectedOrderStateVersion,
		CollateralRevision:        record.CollateralRevision, AllocationRevision: record.AllocationRevision,
		State: collateral.ClaimState(record.State), ExecutionReference: record.ExecutionReference,
	}
	return claim, claim.Validate()
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

func sameClaimDecision(record models.CollateralClaimRecord, decision collateral.ClaimDecision) bool {
	a := decision.Attestation
	return record.ClaimID == decision.ClaimID && record.CollateralID == a.CollateralID && record.AllocationID == a.AllocationID &&
		record.OrderID == a.OrderID && record.ExtensionID == a.ExtensionID && record.AttestationID == a.AttestationID &&
		record.AttestationIdempotencyKey == a.IdempotencyKey && record.ReplayFingerprint == claimReplayFingerprint(a) &&
		record.IdempotencyKey == decision.IdempotencyKey && record.Issuer == a.Issuer && record.Amount == decision.Amount &&
		record.Reason == decision.Reason && record.ConditionType == a.ConditionType && record.ConditionVersion == a.ConditionVersion &&
		record.EvidenceDigest == a.EvidenceDigest && record.ExpectedCollateralRevision == a.ExpectedCollateralRevision &&
		record.ExpectedAllocationRevision == a.ExpectedAllocationRevision && record.ExpectedOrderStateVersion == a.ExpectedOrderStateVersion &&
		record.ObservedAt.Equal(a.ObservedAt) && record.ExpiresAt.Equal(a.ExpiresAt)
}

func claimReplayFingerprint(a collateral.ClaimAttestation) string {
	return stableID("colr", a.TenantID, a.Issuer, a.CollateralID, a.AllocationID, a.OrderID, a.ExtensionID,
		a.ExpectedOrderStateVersion, a.ConditionType, a.ConditionVersion, a.EvidenceDigest,
		strconv.FormatUint(a.ExpectedCollateralRevision, 10), strconv.FormatUint(a.ExpectedAllocationRevision, 10))
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

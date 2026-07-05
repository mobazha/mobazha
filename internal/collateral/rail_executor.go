// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
)

// RailExecutor owns the persistence-first boundary between Core collateral
// state and one reviewed collateral rail. It deliberately has no scheduler or
// API surface; distributions decide when to submit and reconcile pending work.
type RailExecutor struct {
	db         database.Database
	rail       pkgcollateral.Rail
	descriptor pkgcollateral.RailDescriptor
	now        func() time.Time
}

func NewRailExecutor(db database.Database, rail pkgcollateral.Rail) (*RailExecutor, error) {
	if db == nil || rail == nil {
		return nil, fmt.Errorf("collateral rail executor requires database and rail")
	}
	descriptor := rail.Descriptor()
	if strings.TrimSpace(descriptor.ID) == "" || strings.TrimSpace(descriptor.Version) == "" || strings.TrimSpace(descriptor.CustodyModel) == "" {
		return nil, fmt.Errorf("collateral rail descriptor identity, version, and custody model are required")
	}
	return &RailExecutor{db: db, rail: rail, descriptor: descriptor, now: time.Now}, nil
}

// PrepareFunding commits the immutable target request before invoking the
// external rail. A retry repeats the same create-or-retrieve request.
func (e *RailExecutor) PrepareFunding(ctx context.Context, request pkgcollateral.FundingTargetRequest) (pkgcollateral.FundingTarget, error) {
	if err := ctx.Err(); err != nil {
		return pkgcollateral.FundingTarget{}, err
	}
	now := e.now().UTC()
	if err := request.Validate(now); err != nil {
		return pkgcollateral.FundingTarget{}, err
	}
	if err := e.descriptor.ValidateForAsset(request.AssetID); err != nil {
		return pkgcollateral.FundingTarget{}, err
	}

	var existing models.CollateralFundingTargetRecord
	var prepared bool
	err := e.db.Update(func(tx database.Tx) error {
		lookupErr := tx.Read().WithContext(ctx).
			Where("collateral_id = ? OR idempotency_key = ?", request.CollateralID, request.IdempotencyKey).
			First(&existing).Error
		if lookupErr == nil {
			if !sameFundingTargetRequest(existing, request, e.descriptor) {
				return fmt.Errorf("collateral funding target idempotency conflict")
			}
			if existing.Destination != "" || len(existing.Payload) != 0 {
				prepared = true
				return nil
			}
		} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}

		account, err := AccountByIDTx(tx, request.CollateralID)
		if err != nil {
			return err
		}
		if account.TenantID != request.TenantID || account.PrincipalID != request.PrincipalID || account.AssetID != request.AssetID {
			return fmt.Errorf("collateral funding target scope does not match account")
		}
		if account.State != pkgcollateral.StatePendingFunding || account.Revision == 0 {
			return fmt.Errorf("collateral funding target requires pending-funding account")
		}
		if request.Amount != account.RequiredAmount || request.ExpiresAt.After(account.ExpiresAt) {
			return fmt.Errorf("collateral funding target amount or expiry does not match account")
		}

		if lookupErr == nil {
			rows, updateErr := tx.UpdateColumns(map[string]interface{}{
				"attempts": existing.Attempts + 1, "last_error": "", "updated_at": now,
			}, map[string]interface{}{
				"collateral_id = ?": existing.CollateralID, "attempts = ?": existing.Attempts,
			}, &models.CollateralFundingTargetRecord{})
			if updateErr != nil {
				return updateErr
			}
			if rows != 1 {
				return fmt.Errorf("collateral funding target attempt conflict")
			}
			existing.Attempts++
			return nil
		}
		existing = models.CollateralFundingTargetRecord{
			CollateralID: request.CollateralID, RailID: e.descriptor.ID, RailVersion: e.descriptor.Version,
			PrincipalID: request.PrincipalID, AssetID: request.AssetID, Amount: request.Amount,
			IdempotencyKey: request.IdempotencyKey, ExpectedRevision: account.Revision,
			State: string(pkgcollateral.RailActionPending), Attempts: 1, ExpiresAt: request.ExpiresAt,
			CreatedAt: now, UpdatedAt: now,
		}
		return tx.Create(&existing)
	})
	if err != nil {
		return pkgcollateral.FundingTarget{}, err
	}
	if prepared {
		return fundingTargetFromRecord(existing, now)
	}

	target, err := e.rail.PrepareFunding(ctx, request)
	if err != nil {
		return pkgcollateral.FundingTarget{}, e.recordFundingError(existing.CollateralID, err, now)
	}
	if err := target.Validate(now); err != nil {
		return pkgcollateral.FundingTarget{}, e.recordFundingError(existing.CollateralID, fmt.Errorf("invalid collateral funding target: %w", err), now)
	}
	if target.RailID != e.descriptor.ID || target.CollateralID != request.CollateralID || target.AssetID != request.AssetID || target.Amount != request.Amount || target.ExpiresAt.After(request.ExpiresAt) {
		return pkgcollateral.FundingTarget{}, e.recordFundingError(existing.CollateralID, fmt.Errorf("collateral funding target result binding mismatch"), now)
	}
	err = e.db.Update(func(tx database.Tx) error {
		rows, updateErr := tx.UpdateColumns(map[string]interface{}{
			"destination": target.Destination, "payload": []byte(target.Payload), "expires_at": target.ExpiresAt,
			"last_error": "", "updated_at": now,
		}, map[string]interface{}{
			"collateral_id = ?": existing.CollateralID, "idempotency_key = ?": existing.IdempotencyKey,
			"state = ?": string(pkgcollateral.RailActionPending),
		}, &models.CollateralFundingTargetRecord{})
		if updateErr != nil {
			return updateErr
		}
		if rows != 1 {
			return fmt.Errorf("collateral funding target persistence conflict")
		}
		return nil
	})
	if err != nil {
		return pkgcollateral.FundingTarget{}, err
	}
	return target, nil
}

// ReconcileFunding accepts only a confirmed, receipt-verified rail status and
// applies it atomically with the target projection.
func (e *RailExecutor) ReconcileFunding(ctx context.Context, collateralID string) (pkgcollateral.RailFundingStatus, error) {
	if err := ctx.Err(); err != nil {
		return pkgcollateral.RailFundingStatus{}, err
	}
	var record models.CollateralFundingTargetRecord
	if err := e.db.View(func(tx database.Tx) error {
		return tx.Read().WithContext(ctx).Where("collateral_id = ?", strings.TrimSpace(collateralID)).First(&record).Error
	}); err != nil {
		return pkgcollateral.RailFundingStatus{}, err
	}
	if record.State == string(pkgcollateral.RailActionConfirmed) || record.State == string(pkgcollateral.RailActionFailed) {
		return fundingStatusFromRecord(record), nil
	}
	target, err := fundingTargetFromRecord(record, e.now().UTC())
	if err != nil {
		return pkgcollateral.RailFundingStatus{}, err
	}
	status, err := e.rail.FundingStatus(ctx, target)
	if err != nil {
		return pkgcollateral.RailFundingStatus{}, e.recordFundingError(record.CollateralID, err, e.now().UTC())
	}
	if err := status.Validate(); err != nil {
		return pkgcollateral.RailFundingStatus{}, e.recordFundingError(record.CollateralID, err, e.now().UTC())
	}
	if status.AssetID != record.AssetID || compareAmount(status.Amount, record.Amount) < 0 {
		return pkgcollateral.RailFundingStatus{}, e.recordFundingError(record.CollateralID, fmt.Errorf("collateral funding status binding mismatch"), e.now().UTC())
	}
	if err := e.applyFundingStatus(record, status, e.now().UTC()); err != nil {
		return pkgcollateral.RailFundingStatus{}, err
	}
	return status, nil
}

// SubmitExecution persists and validates the immutable release or slash intent
// before external I/O. Ambiguous errors retain pending state for reconciliation.
func (e *RailExecutor) SubmitExecution(ctx context.Context, request pkgcollateral.RailExecutionRequest) (pkgcollateral.RailActionResult, error) {
	if err := ctx.Err(); err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	if err := request.Validate(); err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	if err := e.descriptor.ValidateForAsset(request.AssetID); err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	now := e.now().UTC()
	var record models.CollateralRailActionRecord
	err := e.db.Update(func(tx database.Tx) error {
		lookupErr := tx.Read().WithContext(ctx).
			Where("action_id = ? OR idempotency_key = ?", request.ActionID, request.IdempotencyKey).
			First(&record).Error
		if lookupErr == nil {
			if !sameRailExecutionRequest(record, request, e.descriptor) {
				return fmt.Errorf("collateral rail execution idempotency conflict")
			}
			if record.State == string(pkgcollateral.RailActionConfirmed) || record.State == string(pkgcollateral.RailActionFailed) {
				return nil
			}
		} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}

		account, err := AccountByIDTx(tx, request.CollateralID)
		if err != nil {
			return err
		}
		if account.TenantID != request.TenantID || account.AssetID != request.AssetID || account.Revision != request.ExpectedRevision {
			return fmt.Errorf("collateral rail execution scope or revision does not match account")
		}
		if request.Kind == pkgcollateral.ExecutionRelease {
			if account.State != pkgcollateral.StateReleasePending || request.Amount != account.FundedAmount || account.AvailableAmount != account.FundedAmount {
				return fmt.Errorf("collateral rail release does not match pending account")
			}
		} else {
			if account.State != pkgcollateral.StateSlashPending {
				return fmt.Errorf("collateral rail slash requires slash-pending account")
			}
			claim, claimErr := ClaimByIDTx(tx, request.ClaimID)
			if claimErr != nil {
				return claimErr
			}
			if claim.CollateralID != account.CollateralID || claim.State != pkgcollateral.ClaimPendingSlash || claim.Amount != request.Amount {
				return fmt.Errorf("collateral rail slash does not match pending claim")
			}
		}

		if lookupErr == nil {
			rows, updateErr := tx.UpdateColumns(map[string]interface{}{
				"attempts": record.Attempts + 1, "last_error": "", "updated_at": now,
			}, map[string]interface{}{"action_id = ?": record.ActionID, "attempts = ?": record.Attempts}, &models.CollateralRailActionRecord{})
			if updateErr != nil {
				return updateErr
			}
			if rows != 1 {
				return fmt.Errorf("collateral rail execution attempt conflict")
			}
			record.Attempts++
			return nil
		}
		record = models.CollateralRailActionRecord{
			ActionID: request.ActionID, CollateralID: request.CollateralID, ClaimID: request.ClaimID,
			RailID: e.descriptor.ID, RailVersion: e.descriptor.Version, Kind: string(request.Kind),
			AssetID: request.AssetID, Amount: request.Amount, Destination: request.Destination,
			ExpectedRevision: request.ExpectedRevision, IdempotencyKey: request.IdempotencyKey,
			State: string(pkgcollateral.RailActionPending), Attempts: 1, CreatedAt: now, UpdatedAt: now,
		}
		return tx.Create(&record)
	})
	if err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	if record.State == string(pkgcollateral.RailActionConfirmed) || record.State == string(pkgcollateral.RailActionFailed) {
		return railActionResultFromRecord(record), nil
	}

	result, err := e.rail.SubmitExecution(ctx, request)
	if err != nil {
		return pkgcollateral.RailActionResult{}, e.recordExecutionError(record.ActionID, err, now)
	}
	if err := e.validateExecutionResult(record, result); err != nil {
		return pkgcollateral.RailActionResult{}, e.recordExecutionError(record.ActionID, err, now)
	}
	if err := e.applyExecutionResult(record, result, now); err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	return result, nil
}

func (e *RailExecutor) ReconcileExecution(ctx context.Context, actionID string) (pkgcollateral.RailActionResult, error) {
	if err := ctx.Err(); err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	var record models.CollateralRailActionRecord
	if err := e.db.View(func(tx database.Tx) error {
		return tx.Read().WithContext(ctx).Where("action_id = ?", strings.TrimSpace(actionID)).First(&record).Error
	}); err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	if record.State == string(pkgcollateral.RailActionConfirmed) || record.State == string(pkgcollateral.RailActionFailed) {
		return railActionResultFromRecord(record), nil
	}
	result, err := e.rail.ExecutionStatus(ctx, record.ActionID)
	if err != nil {
		return pkgcollateral.RailActionResult{}, e.recordExecutionError(record.ActionID, err, e.now().UTC())
	}
	if err := e.validateExecutionResult(record, result); err != nil {
		return pkgcollateral.RailActionResult{}, e.recordExecutionError(record.ActionID, err, e.now().UTC())
	}
	if err := e.applyExecutionResult(record, result, e.now().UTC()); err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	return result, nil
}

func (e *RailExecutor) applyFundingStatus(record models.CollateralFundingTargetRecord, status pkgcollateral.RailFundingStatus, now time.Time) error {
	return e.db.Update(func(tx database.Tx) error {
		if status.State == pkgcollateral.RailActionConfirmed {
			if _, err := RecordFundingTx(tx, pkgcollateral.FundingObservation{
				TenantID: record.TenantID, CollateralID: record.CollateralID, AssetID: record.AssetID,
				FundedAmount: status.Amount, FundingReference: status.Reference, ExpectedRevision: record.ExpectedRevision,
				IdempotencyKey: stableID("colrf", record.TenantID, record.IdempotencyKey, "confirmed"), ObservedAt: status.ObservedAt,
			}, now); err != nil {
				return err
			}
		}
		values := map[string]interface{}{
			"state": string(status.State), "funding_reference": status.Reference,
			"observed_amount": status.Amount, "last_error": boundedRailError(status.LastError), "updated_at": now,
		}
		if !status.ObservedAt.IsZero() {
			observedAt := status.ObservedAt.UTC()
			values["observed_at"] = &observedAt
		}
		rows, err := tx.UpdateColumns(values,
			map[string]interface{}{"collateral_id = ?": record.CollateralID, "state = ?": string(pkgcollateral.RailActionPending)},
			&models.CollateralFundingTargetRecord{})
		if err != nil {
			return err
		}
		if rows != 1 {
			var current models.CollateralFundingTargetRecord
			if loadErr := tx.Read().Where("collateral_id = ?", record.CollateralID).First(&current).Error; loadErr == nil &&
				current.State == string(status.State) && current.FundingReference == status.Reference &&
				current.ObservedAmount == status.Amount && current.LastError == boundedRailError(status.LastError) {
				return nil
			}
			return fmt.Errorf("collateral funding reconciliation conflict")
		}
		return nil
	})
}

func (e *RailExecutor) applyExecutionResult(record models.CollateralRailActionRecord, result pkgcollateral.RailActionResult, now time.Time) error {
	return e.db.Update(func(tx database.Tx) error {
		if result.State == pkgcollateral.RailActionConfirmed {
			if _, err := RecordExecutionTx(tx, pkgcollateral.ExecutionObservation{
				TenantID: record.TenantID, CollateralID: record.CollateralID, ClaimID: record.ClaimID,
				Kind: pkgcollateral.ExecutionKind(record.Kind), AssetID: record.AssetID, Amount: record.Amount,
				ExecutionReference: result.Reference, ExpectedRevision: record.ExpectedRevision,
				IdempotencyKey: stableID("colre", record.TenantID, record.ActionID, "confirmed"), ObservedAt: result.ObservedAt,
			}, now); err != nil {
				return err
			}
		}
		values := map[string]interface{}{
			"state": string(result.State), "reference": result.Reference,
			"last_error": boundedRailError(result.LastError), "updated_at": now,
		}
		if !result.ObservedAt.IsZero() {
			observedAt := result.ObservedAt.UTC()
			values["observed_at"] = &observedAt
		}
		rows, err := tx.UpdateColumns(values,
			map[string]interface{}{"action_id = ?": record.ActionID, "state = ?": string(pkgcollateral.RailActionPending)},
			&models.CollateralRailActionRecord{})
		if err != nil {
			return err
		}
		if rows != 1 {
			var current models.CollateralRailActionRecord
			if loadErr := tx.Read().Where("action_id = ?", record.ActionID).First(&current).Error; loadErr == nil &&
				current.State == string(result.State) && current.Reference == result.Reference &&
				current.LastError == boundedRailError(result.LastError) {
				return nil
			}
			return fmt.Errorf("collateral rail execution reconciliation conflict")
		}
		return nil
	})
}

func (e *RailExecutor) validateExecutionResult(record models.CollateralRailActionRecord, result pkgcollateral.RailActionResult) error {
	if err := result.Validate(); err != nil {
		return err
	}
	if result.ActionID != record.ActionID {
		return fmt.Errorf("collateral rail execution result binding mismatch")
	}
	return nil
}

func (e *RailExecutor) recordFundingError(collateralID string, cause error, now time.Time) error {
	persistErr := e.db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(map[string]interface{}{
			"last_error": boundedRailError(cause.Error()), "updated_at": now,
		}, map[string]interface{}{"collateral_id = ?": collateralID, "state = ?": string(pkgcollateral.RailActionPending)}, &models.CollateralFundingTargetRecord{})
		return err
	})
	if persistErr != nil {
		return fmt.Errorf("%v; persist collateral rail error: %w", cause, persistErr)
	}
	return cause
}

func (e *RailExecutor) recordExecutionError(actionID string, cause error, now time.Time) error {
	persistErr := e.db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(map[string]interface{}{
			"last_error": boundedRailError(cause.Error()), "updated_at": now,
		}, map[string]interface{}{"action_id = ?": actionID, "state = ?": string(pkgcollateral.RailActionPending)}, &models.CollateralRailActionRecord{})
		return err
	})
	if persistErr != nil {
		return fmt.Errorf("%v; persist collateral rail error: %w", cause, persistErr)
	}
	return cause
}

func fundingTargetFromRecord(record models.CollateralFundingTargetRecord, now time.Time) (pkgcollateral.FundingTarget, error) {
	target := pkgcollateral.FundingTarget{
		RailID: record.RailID, CollateralID: record.CollateralID, AssetID: record.AssetID,
		Amount: record.Amount, Destination: record.Destination, Payload: json.RawMessage(append([]byte(nil), record.Payload...)),
		ExpiresAt: record.ExpiresAt,
	}
	return target, target.Validate(now)
}

func fundingStatusFromRecord(record models.CollateralFundingTargetRecord) pkgcollateral.RailFundingStatus {
	amount := record.ObservedAmount
	if amount == "" {
		amount = record.Amount
	}
	status := pkgcollateral.RailFundingStatus{
		State: pkgcollateral.RailActionState(record.State), Reference: record.FundingReference,
		AssetID: record.AssetID, Amount: amount, LastError: record.LastError,
	}
	if record.ObservedAt != nil {
		status.ObservedAt = record.ObservedAt.UTC()
	}
	return status
}

func railActionResultFromRecord(record models.CollateralRailActionRecord) pkgcollateral.RailActionResult {
	result := pkgcollateral.RailActionResult{
		ActionID: record.ActionID, State: pkgcollateral.RailActionState(record.State),
		Reference: record.Reference, LastError: record.LastError,
	}
	if record.ObservedAt != nil {
		result.ObservedAt = record.ObservedAt.UTC()
	}
	return result
}

func sameFundingTargetRequest(record models.CollateralFundingTargetRecord, request pkgcollateral.FundingTargetRequest, descriptor pkgcollateral.RailDescriptor) bool {
	return record.TenantID == request.TenantID && record.CollateralID == request.CollateralID &&
		record.PrincipalID == request.PrincipalID && record.AssetID == request.AssetID && record.Amount == request.Amount &&
		record.IdempotencyKey == request.IdempotencyKey && record.RailID == descriptor.ID && record.RailVersion == descriptor.Version &&
		record.ExpiresAt.Equal(request.ExpiresAt)
}

func sameRailExecutionRequest(record models.CollateralRailActionRecord, request pkgcollateral.RailExecutionRequest, descriptor pkgcollateral.RailDescriptor) bool {
	return record.TenantID == request.TenantID && record.ActionID == request.ActionID && record.CollateralID == request.CollateralID &&
		record.ClaimID == request.ClaimID && record.Kind == string(request.Kind) && record.AssetID == request.AssetID &&
		record.Amount == request.Amount && record.Destination == request.Destination && record.ExpectedRevision == request.ExpectedRevision &&
		record.IdempotencyKey == request.IdempotencyKey && record.RailID == descriptor.ID && record.RailVersion == descriptor.Version
}

func boundedRailError(value string) string {
	const maxBytes = 2048
	value = strings.TrimSpace(value)
	if len(value) > maxBytes {
		return value[:maxBytes]
	}
	return value
}

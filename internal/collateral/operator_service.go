// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
)

type operatorService struct {
	db          database.Database
	tenantID    string
	principalID string
	executor    *RailExecutor
	now         func() time.Time
}

// NewOperatorService binds collateral onboarding to one tenant and local
// principal. A nil rail is valid and keeps funding operations fail-closed.
func NewOperatorService(
	db database.Database,
	tenantID string,
	principalID string,
	rail pkgcollateral.Rail,
) (pkgcollateral.OperatorService, error) {
	if db == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(principalID) == "" {
		return nil, fmt.Errorf("collateral operator requires database, tenant, and principal")
	}
	service := &operatorService{
		db: db, tenantID: strings.TrimSpace(tenantID), principalID: strings.TrimSpace(principalID),
		now: func() time.Time { return time.Now().UTC() },
	}
	if rail != nil {
		executor, err := NewRailExecutor(db, rail)
		if err != nil {
			return nil, err
		}
		service.executor = executor
	}
	return service, nil
}

func (s *operatorService) Open(ctx context.Context, input pkgcollateral.OperatorOpenRequest) (pkgcollateral.Account, error) {
	if err := ctx.Err(); err != nil {
		return pkgcollateral.Account{}, err
	}
	now := s.now().UTC()
	request := pkgcollateral.OpenRequest{
		TenantID: s.tenantID, ProviderID: input.ProviderID, ResourceID: input.ResourceID,
		PrincipalID: s.principalID, AssetID: input.AssetID, RequiredAmount: input.RequiredAmount,
		PolicyID: input.PolicyID, PolicyVersion: input.PolicyVersion,
		IdempotencyKey: input.IdempotencyKey, ExpiresAt: input.ExpiresAt,
	}
	if err := request.Validate(now); err != nil {
		return pkgcollateral.Account{}, fmt.Errorf("%w: %v", pkgcollateral.ErrOperatorInvalid, err)
	}
	var account pkgcollateral.Account
	err := s.db.Update(func(tx database.Tx) error {
		var err error
		account, err = OpenTx(tx, request, now)
		return err
	})
	if err != nil {
		return pkgcollateral.Account{}, operatorMutationError(err)
	}
	return account, nil
}

func (s *operatorService) Status(ctx context.Context, collateralID string) (pkgcollateral.OperatorAccountStatus, error) {
	if err := ctx.Err(); err != nil {
		return pkgcollateral.OperatorAccountStatus{}, err
	}
	collateralID = strings.TrimSpace(collateralID)
	if collateralID == "" {
		return pkgcollateral.OperatorAccountStatus{}, fmt.Errorf("%w: collateral account id is required", pkgcollateral.ErrOperatorInvalid)
	}
	var status pkgcollateral.OperatorAccountStatus
	err := s.db.View(func(tx database.Tx) error {
		var accountRecord models.CollateralAccountRecord
		if err := tx.Read().WithContext(ctx).
			Where("tenant_id = ? AND collateral_id = ?", s.tenantID, collateralID).
			First(&accountRecord).Error; err != nil {
			return err
		}
		account, err := accountFromRecord(accountRecord)
		if err != nil {
			return err
		}
		if account.PrincipalID != s.principalID {
			return gorm.ErrRecordNotFound
		}
		status.Account = account

		var fundingRecord models.CollateralFundingTargetRecord
		err = tx.Read().WithContext(ctx).
			Where("tenant_id = ? AND collateral_id = ?", s.tenantID, collateralID).
			First(&fundingRecord).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		funding := operatorFundingStatusFromRecord(fundingRecord)
		status.Funding = &funding
		return nil
	})
	return status, err
}

func (s *operatorService) PrepareFunding(
	ctx context.Context,
	input pkgcollateral.OperatorPrepareFundingRequest,
) (pkgcollateral.FundingTarget, error) {
	if s.executor == nil {
		return pkgcollateral.FundingTarget{}, pkgcollateral.ErrOperatorUnavailable
	}
	if strings.TrimSpace(input.PrincipalDestination) == "" || strings.TrimSpace(input.IdempotencyKey) == "" {
		return pkgcollateral.FundingTarget{}, fmt.Errorf("%w: principal destination and idempotency key are required", pkgcollateral.ErrOperatorInvalid)
	}
	status, err := s.Status(ctx, input.CollateralID)
	if err != nil {
		return pkgcollateral.FundingTarget{}, err
	}
	account := status.Account
	if account.State != pkgcollateral.StatePendingFunding {
		return pkgcollateral.FundingTarget{}, fmt.Errorf("%w: funding target requires pending-funding account", pkgcollateral.ErrOperatorConflict)
	}
	target, err := s.executor.PrepareFunding(ctx, pkgcollateral.FundingTargetRequest{
		TenantID: s.tenantID, CollateralID: account.CollateralID, PrincipalID: s.principalID,
		PrincipalDestination: strings.TrimSpace(input.PrincipalDestination), AssetID: account.AssetID,
		Amount: account.RequiredAmount, IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
		ExpiresAt: account.ExpiresAt,
	})
	if err != nil {
		return pkgcollateral.FundingTarget{}, operatorMutationError(err)
	}
	return target, nil
}

func (s *operatorService) ReconcileFunding(ctx context.Context, collateralID string) (pkgcollateral.OperatorAccountStatus, error) {
	if s.executor == nil {
		return pkgcollateral.OperatorAccountStatus{}, pkgcollateral.ErrOperatorUnavailable
	}
	if _, err := s.Status(ctx, collateralID); err != nil {
		return pkgcollateral.OperatorAccountStatus{}, err
	}
	if _, err := s.executor.ReconcileFunding(ctx, strings.TrimSpace(collateralID)); err != nil {
		return pkgcollateral.OperatorAccountStatus{}, operatorMutationError(err)
	}
	return s.Status(ctx, collateralID)
}

func operatorFundingStatusFromRecord(record models.CollateralFundingTargetRecord) pkgcollateral.OperatorFundingStatus {
	amount := record.ObservedAmount
	if amount == "" {
		amount = record.Amount
	}
	status := pkgcollateral.OperatorFundingStatus{
		RailID: record.RailID, RailVersion: record.RailVersion,
		State: pkgcollateral.RailActionState(record.State), AssetID: record.AssetID, Amount: amount,
		Destination: record.Destination, FundingReference: record.FundingReference,
		Attempts:  record.Attempts,
		ExpiresAt: record.ExpiresAt.UTC(), UpdatedAt: record.UpdatedAt.UTC(),
	}
	if record.LastError != "" {
		status.LastErrorCode = "rail_operation_failed"
	}
	if record.ObservedAt != nil {
		observedAt := record.ObservedAt.UTC()
		status.ObservedAt = &observedAt
	}
	return status
}

func operatorMutationError(err error) error {
	if err == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	message := strings.ToLower(err.Error())
	if (strings.Contains(message, "idempotency") && strings.Contains(message, "conflict")) || strings.Contains(message, "revision conflict") ||
		strings.Contains(message, "requires pending-funding") {
		return fmt.Errorf("%w: %v", pkgcollateral.ErrOperatorConflict, err)
	}
	return err
}

var _ pkgcollateral.OperatorService = (*operatorService)(nil)

// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

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
)

func TestRailExecutorPersistsFundingIntentBeforeIOAndReconciles(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open := openRequest(now)
	var account pkgcollateral.Account
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = OpenTx(tx, open, now)
		return err
	}))

	request := pkgcollateral.FundingTargetRequest{
		TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
		PrincipalID: account.PrincipalID, PrincipalDestination: "principal:seller-1",
		AssetID: account.AssetID, Amount: account.RequiredAmount,
		IdempotencyKey: "rail-funding-1", ExpiresAt: now.Add(time.Hour),
	}
	rail := completeFakeRail()
	rail.prepareTarget = pkgcollateral.FundingTarget{
		RailID: rail.descriptor.ID, TenantID: request.TenantID, CollateralID: account.CollateralID, AssetID: account.AssetID,
		PrincipalDestination: request.PrincipalDestination, IdempotencyKey: request.IdempotencyKey,
		Amount: account.RequiredAmount, Destination: "vault:deposit-1", ExpiresAt: request.ExpiresAt,
	}
	rail.onPrepare = func() {
		var record models.CollateralFundingTargetRecord
		require.NoError(t, db.View(func(tx database.Tx) error {
			return tx.Read().Where("collateral_id = ?", account.CollateralID).First(&record).Error
		}))
		require.Equal(t, string(pkgcollateral.RailActionPending), record.State)
		require.Equal(t, uint64(1), record.Attempts)
		require.Empty(t, record.Destination)
	}
	executor, err := NewRailExecutor(db, rail)
	require.NoError(t, err)
	executor.now = func() time.Time { return now }

	target, err := executor.PrepareFunding(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "vault:deposit-1", target.Destination)
	require.Equal(t, 1, rail.prepareCalls)

	// The create-or-retrieve path returns the durable target without another
	// external call.
	repeated, err := executor.PrepareFunding(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, target, repeated)
	require.Equal(t, 1, rail.prepareCalls)

	rail.fundingStatus = pkgcollateral.RailFundingStatus{
		State: pkgcollateral.RailActionConfirmed, Reference: "funding-receipt-1",
		AssetID: account.AssetID, Amount: "120", ObservedAt: now,
	}
	status, err := executor.ReconcileFunding(context.Background(), account.CollateralID)
	require.NoError(t, err)
	require.Equal(t, "120", status.Amount)
	require.NoError(t, db.View(func(tx database.Tx) error {
		var err error
		account, err = AccountByIDTx(tx, account.CollateralID)
		return err
	}))
	require.Equal(t, pkgcollateral.StateActive, account.State)
	require.Equal(t, "120", account.FundedAmount)

	// A confirmed projection is terminal and does not poll the rail again.
	status, err = executor.ReconcileFunding(context.Background(), account.CollateralID)
	require.NoError(t, err)
	require.Equal(t, "120", status.Amount)
	require.Equal(t, 1, rail.fundingStatusCalls)
}

func TestRailExecutorKeepsAmbiguousReleasePendingUntilReconciled(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	_, account := openAndFundCollateral(t, db, now, "source-release", "open-release", "fund-release", "funding-release", "100")
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = RequestAccountReleaseTx(tx, pkgcollateral.AccountReleaseRequest{
			TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
			ExpectedRevision: account.Revision, IdempotencyKey: "request-release", Reason: "resource-retired",
		}, now)
		return err
	}))

	request := pkgcollateral.RailExecutionRequest{
		ActionID: "release-action-1", TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
		Kind: pkgcollateral.ExecutionRelease, AssetID: account.AssetID, Amount: account.FundedAmount,
		Destination: "principal:seller-1", ExpectedRevision: account.Revision, IdempotencyKey: "release-submit-1",
	}
	rail := completeFakeRail()
	rail.submitErr = errors.New("submit result unknown")
	rail.onSubmit = func() {
		var record models.CollateralRailActionRecord
		require.NoError(t, db.View(func(tx database.Tx) error {
			return tx.Read().Where("action_id = ?", request.ActionID).First(&record).Error
		}))
		require.Equal(t, string(pkgcollateral.RailActionPending), record.State)
		require.Equal(t, uint64(1), record.Attempts)
	}
	executor, err := NewRailExecutor(db, rail)
	require.NoError(t, err)
	executor.now = func() time.Time { return now }

	_, err = executor.SubmitExecution(context.Background(), request)
	require.ErrorContains(t, err, "unknown")
	var record models.CollateralRailActionRecord
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", request.ActionID).First(&record).Error
	}))
	require.Equal(t, string(pkgcollateral.RailActionPending), record.State)
	require.Contains(t, record.LastError, "unknown")
	require.NoError(t, db.View(func(tx database.Tx) error {
		var err error
		account, err = AccountByIDTx(tx, account.CollateralID)
		return err
	}))
	require.Equal(t, pkgcollateral.StateReleasePending, account.State)

	rail.executionStatus = pkgcollateral.RailActionResult{
		ActionID: request.ActionID, State: pkgcollateral.RailActionConfirmed,
		Reference: "release-receipt-1", ObservedAt: now,
	}
	result, err := executor.ReconcileExecution(context.Background(), request.ActionID)
	require.NoError(t, err)
	require.Equal(t, pkgcollateral.RailActionConfirmed, result.State)
	require.Equal(t, request, rail.executionStatusRequest)
	require.NoError(t, db.View(func(tx database.Tx) error {
		var err error
		account, err = AccountByIDTx(tx, account.CollateralID)
		return err
	}))
	require.Equal(t, pkgcollateral.StateReleased, account.State)

	// Repeating the original command returns its durable terminal projection.
	result, err = executor.SubmitExecution(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "release-receipt-1", result.Reference)
	require.Equal(t, 1, rail.submitCalls)
	require.Equal(t, 1, rail.executionStatusCalls)
}

func TestRailExecutorReconcilesFundingAfterTargetExpiry(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open := openRequest(now)
	var account pkgcollateral.Account
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = OpenTx(tx, open, now)
		return err
	}))
	request := pkgcollateral.FundingTargetRequest{
		TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
		PrincipalID: account.PrincipalID, PrincipalDestination: "principal:seller-1",
		AssetID: account.AssetID, Amount: account.RequiredAmount,
		IdempotencyKey: "funding-expiry-1", ExpiresAt: now.Add(time.Hour),
	}
	rail := completeFakeRail()
	rail.prepareTarget = pkgcollateral.FundingTarget{
		RailID: rail.descriptor.ID, TenantID: request.TenantID, CollateralID: account.CollateralID,
		PrincipalDestination: request.PrincipalDestination, IdempotencyKey: request.IdempotencyKey,
		AssetID: account.AssetID, Amount: account.RequiredAmount,
		Destination: "vault:expiry", ExpiresAt: request.ExpiresAt,
	}
	executor, err := NewRailExecutor(db, rail)
	require.NoError(t, err)
	executor.now = func() time.Time { return now }
	_, err = executor.PrepareFunding(context.Background(), request)
	require.NoError(t, err)

	rail.fundingStatus = pkgcollateral.RailFundingStatus{
		State: pkgcollateral.RailActionFailed, AssetID: account.AssetID,
		Amount: account.RequiredAmount, LastError: "collateral funding window expired",
	}
	executor.now = func() time.Time { return now.Add(2 * time.Hour) }
	status, err := executor.ReconcileFunding(context.Background(), account.CollateralID)
	require.NoError(t, err)
	require.Equal(t, pkgcollateral.RailActionFailed, status.State)
	require.Equal(t, 1, rail.fundingStatusCalls)
}

func TestRailExecutorAppliesConfirmedSlash(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open, account := openAndFundCollateral(t, db, now, "source-slash", "open-slash", "fund-slash", "funding-slash", "120")
	allocation := allocateCollateral(t, db, now, open, account, "order-slash", "ext-slash", "25", "allocate-slash")
	decision := claimDecision(now, account.CollateralID, allocation, "20")
	var claim pkgcollateral.Claim
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		claim, err = AcceptClaimTx(tx, decision, now)
		return err
	}))

	request := pkgcollateral.RailExecutionRequest{
		ActionID: "slash-action-1", TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
		ClaimID: claim.ClaimID, Kind: pkgcollateral.ExecutionSlash, AssetID: open.AssetID, Amount: claim.Amount,
		Destination: "beneficiary:buyer-1", ExpectedRevision: claim.CollateralRevision, IdempotencyKey: "slash-submit-1",
	}
	rail := completeFakeRail()
	rail.submitResult = pkgcollateral.RailActionResult{
		ActionID: request.ActionID, State: pkgcollateral.RailActionConfirmed,
		Reference: "slash-receipt-1", ObservedAt: now,
	}
	executor, err := NewRailExecutor(db, rail)
	require.NoError(t, err)
	executor.now = func() time.Time { return now }

	result, err := executor.SubmitExecution(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, pkgcollateral.RailActionConfirmed, result.State)
	require.NoError(t, db.View(func(tx database.Tx) error {
		var err error
		account, err = AccountByIDTx(tx, account.CollateralID)
		if err != nil {
			return err
		}
		claim, err = ClaimByIDTx(tx, claim.ClaimID)
		return err
	}))
	require.Equal(t, pkgcollateral.StateActive, account.State)
	require.Equal(t, "100", account.FundedAmount)
	require.Equal(t, "100", account.AvailableAmount)
	require.Equal(t, pkgcollateral.ClaimSlashed, claim.State)
	require.Equal(t, "slash-receipt-1", claim.ExecutionReference)
}

func TestRailExecutorPersistsConfirmedSlashBelowRequirement(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open, account := openAndFundCollateral(t, db, now, "source-slash-below-required", "open-slash-below-required", "fund-slash-below-required", "funding-slash-below-required", "100")
	allocation := allocateCollateral(t, db, now, open, account, "order-slash-below-required", "ext-slash-below-required", "25", "allocate-slash-below-required")
	decision := claimDecision(now, account.CollateralID, allocation, "20")
	var claim pkgcollateral.Claim
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		claim, err = AcceptClaimTx(tx, decision, now)
		return err
	}))

	request := pkgcollateral.RailExecutionRequest{
		ActionID: "slash-action-below-required", TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
		ClaimID: claim.ClaimID, Kind: pkgcollateral.ExecutionSlash, AssetID: open.AssetID, Amount: claim.Amount,
		Destination: "beneficiary:buyer-below-required", ExpectedRevision: claim.CollateralRevision,
		IdempotencyKey: "slash-submit-below-required",
	}
	rail := completeFakeRail()
	rail.submitResult = pkgcollateral.RailActionResult{
		ActionID: request.ActionID, State: pkgcollateral.RailActionConfirmed,
		Reference: "slash-receipt-below-required", ObservedAt: now,
	}
	executor, err := NewRailExecutor(db, rail)
	require.NoError(t, err)
	executor.now = func() time.Time { return now }

	result, err := executor.SubmitExecution(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, pkgcollateral.RailActionConfirmed, result.State)
	require.NoError(t, db.View(func(tx database.Tx) error {
		var action models.CollateralRailActionRecord
		if err := tx.Read().Where("action_id = ?", request.ActionID).First(&action).Error; err != nil {
			return err
		}
		require.Equal(t, string(pkgcollateral.RailActionConfirmed), action.State)
		var err error
		account, err = AccountByIDTx(tx, account.CollateralID)
		if err != nil {
			return err
		}
		claim, err = ClaimByIDTx(tx, claim.ClaimID)
		return err
	}))
	require.Equal(t, pkgcollateral.StateSlashed, account.State)
	require.Equal(t, "80", account.FundedAmount)
	require.Equal(t, pkgcollateral.ClaimSlashed, claim.State)
	require.Equal(t, "slash-receipt-below-required", claim.ExecutionReference)
}

func TestRailExecutorRecoversPendingExecutionAfterRestart(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	_, account := openAndFundCollateral(t, db, now, "source-restart", "open-restart", "fund-restart", "funding-restart", "100")
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = RequestAccountReleaseTx(tx, pkgcollateral.AccountReleaseRequest{
			TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
			ExpectedRevision: account.Revision, IdempotencyKey: "request-restart", Reason: "resource-retired",
		}, now)
		return err
	}))
	request := pkgcollateral.RailExecutionRequest{
		ActionID: "release-restart-1", TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
		Kind: pkgcollateral.ExecutionRelease, AssetID: account.AssetID, Amount: account.FundedAmount,
		Destination: "principal:seller-1", ExpectedRevision: account.Revision, IdempotencyKey: "submit-restart-1",
	}

	uncertainRail := completeFakeRail()
	uncertainRail.submitErr = errors.New("connection reset after submit")
	first, err := NewRailExecutor(db, uncertainRail)
	require.NoError(t, err)
	first.now = func() time.Time { return now }
	_, err = first.SubmitExecution(context.Background(), request)
	require.ErrorContains(t, err, "connection reset")

	// A fresh executor reconstructs the immutable request and safely repeats
	// the rail's create-or-retrieve operation.
	recoveredRail := completeFakeRail()
	recoveredRail.submitResult = pkgcollateral.RailActionResult{
		ActionID: request.ActionID, State: pkgcollateral.RailActionConfirmed,
		Reference: "release-restart-receipt", ObservedAt: now,
	}
	restarted, err := NewRailExecutor(db, recoveredRail)
	require.NoError(t, err)
	restarted.now = func() time.Time { return now }
	require.NoError(t, restarted.RecoverPending(context.Background(), 10))
	require.Equal(t, 1, recoveredRail.submitCalls)
	require.NoError(t, db.View(func(tx database.Tx) error {
		var err error
		account, err = AccountByIDTx(tx, account.CollateralID)
		return err
	}))
	require.Equal(t, pkgcollateral.StateReleased, account.State)
}

func TestRailExecutorRecoversFundingTargetCreationAfterRestart(t *testing.T) {
	db := collateralTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	open := openRequest(now)
	var account pkgcollateral.Account
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		account, err = OpenTx(tx, open, now)
		return err
	}))
	request := pkgcollateral.FundingTargetRequest{
		TenantID: database.StandaloneTenantID, CollateralID: account.CollateralID,
		PrincipalID: account.PrincipalID, PrincipalDestination: "principal:seller-1",
		AssetID: account.AssetID, Amount: account.RequiredAmount,
		IdempotencyKey: "funding-restart-1", ExpiresAt: now.Add(time.Hour),
	}

	uncertainRail := completeFakeRail()
	uncertainRail.prepareErr = errors.New("connection reset during target creation")
	first, err := NewRailExecutor(db, uncertainRail)
	require.NoError(t, err)
	first.now = func() time.Time { return now }
	_, err = first.PrepareFunding(context.Background(), request)
	require.ErrorContains(t, err, "connection reset")

	recoveredRail := completeFakeRail()
	recoveredRail.prepareTarget = pkgcollateral.FundingTarget{
		RailID: recoveredRail.descriptor.ID, TenantID: request.TenantID, CollateralID: account.CollateralID, AssetID: account.AssetID,
		PrincipalDestination: request.PrincipalDestination, IdempotencyKey: request.IdempotencyKey,
		Amount: account.RequiredAmount, Destination: "vault:recovered", ExpiresAt: request.ExpiresAt,
	}
	restarted, err := NewRailExecutor(db, recoveredRail)
	require.NoError(t, err)
	restarted.now = func() time.Time { return now }
	require.NoError(t, restarted.RecoverPending(context.Background(), 10))
	require.Equal(t, 1, recoveredRail.prepareCalls)
	var record models.CollateralFundingTargetRecord
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("collateral_id = ?", account.CollateralID).First(&record).Error
	}))
	require.Equal(t, "vault:recovered", record.Destination)
	require.Equal(t, uint64(2), record.Attempts)
}

type fakeCollateralRail struct {
	descriptor             pkgcollateral.RailDescriptor
	prepareTarget          pkgcollateral.FundingTarget
	prepareErr             error
	fundingStatus          pkgcollateral.RailFundingStatus
	fundingStatusErr       error
	submitResult           pkgcollateral.RailActionResult
	submitErr              error
	executionStatus        pkgcollateral.RailActionResult
	executionStatusErr     error
	executionStatusRequest pkgcollateral.RailExecutionRequest
	prepareCalls           int
	fundingStatusCalls     int
	submitCalls            int
	executionStatusCalls   int
	onPrepare              func()
	onSubmit               func()
}

func completeFakeRail() *fakeCollateralRail {
	return &fakeCollateralRail{descriptor: pkgcollateral.RailDescriptor{
		ID: "test-vault", Version: "v1", CustodyModel: "dedicated-vault",
		Assets: []string{"crypto:solana:mainnet:usdc"}, SupportsFundingTargets: true,
		SupportsFundingObserve: true, SupportsPrincipalRelease: true, SupportsClaimSlash: true,
		SupportsReconciliation: true, HasReceiptVerification: true,
	}}
}

func (r *fakeCollateralRail) Descriptor() pkgcollateral.RailDescriptor { return r.descriptor }

func (r *fakeCollateralRail) PrepareFunding(context.Context, pkgcollateral.FundingTargetRequest) (pkgcollateral.FundingTarget, error) {
	r.prepareCalls++
	if r.onPrepare != nil {
		r.onPrepare()
	}
	return r.prepareTarget, r.prepareErr
}

func (r *fakeCollateralRail) FundingStatus(context.Context, pkgcollateral.FundingTarget) (pkgcollateral.RailFundingStatus, error) {
	r.fundingStatusCalls++
	return r.fundingStatus, r.fundingStatusErr
}

func (r *fakeCollateralRail) SubmitExecution(context.Context, pkgcollateral.RailExecutionRequest) (pkgcollateral.RailActionResult, error) {
	r.submitCalls++
	if r.onSubmit != nil {
		r.onSubmit()
	}
	return r.submitResult, r.submitErr
}

func (r *fakeCollateralRail) ExecutionStatus(_ context.Context, request pkgcollateral.RailExecutionRequest) (pkgcollateral.RailActionResult, error) {
	r.executionStatusCalls++
	r.executionStatusRequest = request
	return r.executionStatus, r.executionStatusErr
}

var _ pkgcollateral.Rail = (*fakeCollateralRail)(nil)

// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package payment

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pkpayment "github.com/mobazha/mobazha/pkg/payment"
	"github.com/stretchr/testify/require"
)

func TestDeriveFundsStatus_ProjectsChainAndProviderLifecycles(t *testing.T) {
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		orderState models.OrderState
		funding    pkpayment.FundingState
		settlement *models.SettlementAction
		provider   *models.PaymentProviderAction
		want       pkpayment.FundsState
		retryable  bool
	}{
		{name: "unfunded", funding: pkpayment.FundingStateAwaitingFunds, want: pkpayment.FundsStateUnfunded},
		{name: "partially funded", funding: pkpayment.FundingStatePartiallyFunded, want: pkpayment.FundsStatePartiallyFunded},
		{name: "funded", funding: pkpayment.FundingStateFullyFunded, want: pkpayment.FundsStateFunded},
		{name: "disputed", orderState: models.OrderState_DISPUTED, funding: pkpayment.FundingStateFullyFunded, want: pkpayment.FundsStateDisputed},
		{
			name: "chain release pending", funding: pkpayment.FundingStateFullyFunded,
			settlement: &models.SettlementAction{ActionID: "sa-release", ActionKind: "complete", State: "submitted", UpdatedAt: now},
			want:       pkpayment.FundsStateReleasePending,
		},
		{
			name: "chain refund confirmed", funding: pkpayment.FundingStateFullyFunded,
			settlement: &models.SettlementAction{ActionID: "sa-refund", ActionKind: pkpayment.SettlementActionCancel, State: "confirmed", UpdatedAt: now},
			want:       pkpayment.FundsStateRefunded,
		},
		{
			name: "chain action failed recoverably", funding: pkpayment.FundingStateFullyFunded,
			settlement: &models.SettlementAction{ActionID: "sa-failed", ActionKind: "complete", State: "failed", LastError: "rpc unavailable", UpdatedAt: now},
			want:       pkpayment.FundsStateFailedRecoverable, retryable: true,
		},
		{
			name: "provider refund pending", funding: pkpayment.FundingStateFullyFunded,
			provider: &models.PaymentProviderAction{ActionID: "fpa-pending", ActionKind: models.PaymentProviderActionRefund, State: models.PaymentProviderActionPendingExternal, UpdatedAt: now},
			want:     pkpayment.FundsStateRefundPending, retryable: true,
		},
		{
			name: "provider refund needs reconcile", funding: pkpayment.FundingStateFullyFunded,
			provider: &models.PaymentProviderAction{ActionID: "fpa-reconcile", ActionKind: models.PaymentProviderActionRefund, State: models.PaymentProviderActionReconcileRequired, UpdatedAt: now},
			want:     pkpayment.FundsStateFailedRecoverable, retryable: true,
		},
		{
			name: "provider refund failed terminally", funding: pkpayment.FundingStateFullyFunded,
			provider: &models.PaymentProviderAction{ActionID: "fpa-failed", ActionKind: models.PaymentProviderActionRefund, State: models.PaymentProviderActionFailed, UpdatedAt: now},
			want:     pkpayment.FundsStateFailedTerminal,
		},
		{
			name: "provider refund completed", funding: pkpayment.FundingStateFullyFunded,
			provider: &models.PaymentProviderAction{ActionID: "fpa-complete", ActionKind: models.PaymentProviderActionRefund, State: models.PaymentProviderActionCompleted, ResultPayload: []byte(`{"refund":{"status":"succeeded"}}`), UpdatedAt: now},
			want:     pkpayment.FundsStateRefunded,
		},
		{
			name: "provider accepted pending refund", funding: pkpayment.FundingStateFullyFunded,
			provider: &models.PaymentProviderAction{ActionID: "fpa-accepted", ActionKind: models.PaymentProviderActionRefund, State: models.PaymentProviderActionCompleted, ResultPayload: []byte(`{"refund":{"status":"pending"}}`), UpdatedAt: now},
			want:     pkpayment.FundsStateRefundPending,
		},
		{
			name: "provider disbursement pending", funding: pkpayment.FundingStateFullyFunded,
			provider: &models.PaymentProviderAction{ActionID: "fpa-release-pending", ActionKind: models.PaymentProviderActionDisburse, State: models.PaymentProviderActionPendingExternal, UpdatedAt: now},
			want:     pkpayment.FundsStateReleasePending, retryable: true,
		},
		{
			name: "provider disbursement completed", funding: pkpayment.FundingStateFullyFunded,
			provider: &models.PaymentProviderAction{ActionID: "fpa-release-complete", ActionKind: models.PaymentProviderActionDisburse, State: models.PaymentProviderActionCompleted, ResultPayload: []byte(`{"disburse":{"status":"success"}}`), UpdatedAt: now},
			want:     pkpayment.FundsStateReleased,
		},
		{
			name: "provider accepted pending disbursement", funding: pkpayment.FundingStateFullyFunded,
			provider: &models.PaymentProviderAction{ActionID: "fpa-release-accepted", ActionKind: models.PaymentProviderActionDisburse, State: models.PaymentProviderActionCompleted, ResultPayload: []byte(`{"disburse":{"status":"pending"}}`), UpdatedAt: now},
			want:     pkpayment.FundsStateReleasePending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &models.Order{State: tt.orderState}
			got := deriveFundsStatus(order, tt.funding, tt.settlement, tt.provider)
			require.Equal(t, tt.want, got.State)
			require.Equal(t, tt.retryable, got.Retryable)
		})
	}
}

func TestFundsStatusFromRefundedAttempt_ProjectsDurablePartialRefund(t *testing.T) {
	now := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	got := fundsStatusFromRefundedAttempt(&models.PaymentAttempt{
		AttemptID: "attempt-refunded", State: models.PaymentAttemptRefunded,
		ExternalReference: "refund-tx", UpdatedAt: now,
	})
	require.Equal(t, pkpayment.FundsStateRefunded, got.State)
	require.Equal(t, pkpayment.FundsActionPartialRefund, got.Action)
	require.Equal(t, "attempt-refunded", got.ActionID)
	require.Equal(t, "refund-tx", got.TxHash)
	require.Equal(t, now, *got.UpdatedAt)
}

func TestLoadCryptoAttemptProjection_ReturnsLatestRefundedAttemptWithoutTarget(t *testing.T) {
	db := newVerifierTestDB(t)
	require.NoError(t, db.gormDB.AutoMigrate(&models.PaymentAttempt{}))
	now := time.Now().UTC()
	require.NoError(t, db.gormDB.Create(&models.PaymentAttempt{
		TenantID: database.StandaloneTenantID, AttemptID: "attempt-refund", Kind: models.PaymentAttemptKindCryptoFundingTarget,
		PaymentSessionID: "ps-refund", OrderID: "order-refund", RouteBindingID: "route-refund",
		IdempotencyKey: "attempt-key-refund", State: models.PaymentAttemptRefunded,
		Currency: "crypto:bip122:000000000019d6689c085ae165831e93:native", AmountValue: "500",
		ExternalReference: "refund-tx", UpdatedAt: now,
	}).Error)

	attempt, target, err := NewPaymentSessionProjector(db).loadCryptoAttemptProjection("", "order-refund")
	require.NoError(t, err)
	require.NotNil(t, attempt)
	require.Equal(t, models.PaymentAttemptRefunded, attempt.State)
	require.Equal(t, "refund-tx", attempt.ExternalReference)
	require.Nil(t, target)
}

func TestLoadLatestFundsActions_IsTenantAndOrderScoped(t *testing.T) {
	db := newVerifierTestDB(t)
	require.NoError(t, db.gormDB.AutoMigrate(
		&models.PaymentAttempt{}, &models.PaymentProviderAction{}, &models.SettlementAction{},
	))
	now := time.Now().UTC()
	attempts := []models.PaymentAttempt{
		{TenantID: "tenant-a", AttemptID: "attempt-a", PaymentSessionID: "ps-a", OrderID: "order-shared", Kind: models.PaymentAttemptKindProviderSession, RouteBindingID: "route-a", IdempotencyKey: "attempt-key-a", State: models.PaymentAttemptExternalCreated, CreatedAt: now},
		{TenantID: "tenant-b", AttemptID: "attempt-b", PaymentSessionID: "ps-b", OrderID: "order-shared", Kind: models.PaymentAttemptKindProviderSession, RouteBindingID: "route-b", IdempotencyKey: "attempt-key-b", State: models.PaymentAttemptExternalCreated, CreatedAt: now},
	}
	require.NoError(t, db.gormDB.Create(&attempts).Error)
	require.NoError(t, db.gormDB.Create(&models.PaymentProviderAction{
		TenantID: "tenant-a", ActionID: "refund-a", ActionKind: models.PaymentProviderActionRefund,
		AttemptID: "attempt-a", RouteBindingID: "route-a", ProviderBindingID: "binding-a",
		ProviderID: "stripe", ExternalReference: "pi-a", IdempotencyKey: "key-a", IntentFingerprint: "fp-a",
		IntentPayload: []byte(`{}`), State: models.PaymentProviderActionPendingExternal, UpdatedAt: now,
	}).Error)
	require.NoError(t, db.gormDB.Create(&models.PaymentProviderAction{
		TenantID: "tenant-b", ActionID: "refund-b", ActionKind: models.PaymentProviderActionRefund,
		AttemptID: "attempt-b", RouteBindingID: "route-b", ProviderBindingID: "binding-b",
		ProviderID: "paypal", ExternalReference: "cap-b", IdempotencyKey: "key-b", IntentFingerprint: "fp-b",
		IntentPayload: []byte(`{}`), State: models.PaymentProviderActionCompleted, UpdatedAt: now.Add(time.Minute),
	}).Error)
	require.NoError(t, db.gormDB.Create(&models.SettlementAction{
		TenantMixin: models.TenantMixin{TenantID: "tenant-a"}, ActionID: "settle-a", OrderID: "order-shared",
		ActionKind: "complete", State: "submitted", UpdatedAt: now,
	}).Error)

	settlement, provider, err := NewPaymentSessionProjector(db).loadLatestFundsActions("tenant-a", "order-shared")
	require.NoError(t, err)
	require.NotNil(t, settlement)
	require.Equal(t, "settle-a", settlement.ActionID)
	require.NotNil(t, provider)
	require.Equal(t, "refund-a", provider.ActionID)
}

func TestLoadLatestFundsActions_UsesFreshTenantScopedQueryPerModel(t *testing.T) {
	shared := newVerifierTestDB(t)
	require.NoError(t, shared.gormDB.AutoMigrate(
		&models.PaymentAttempt{}, &models.PaymentProviderAction{}, &models.SettlementAction{},
	))
	tenantDB, err := dbstore.NewTenantDBWithPublicData(shared.gormDB, "tenant-a", database.PublicData(nil))
	require.NoError(t, err)
	now := time.Now().UTC()
	require.NoError(t, tenantDB.Update(func(tx database.Tx) error {
		if err := tx.Create(&models.PaymentAttempt{
			AttemptID: "attempt-a", PaymentSessionID: "ps-a", OrderID: "order-a",
			Kind: models.PaymentAttemptKindProviderSession, RouteBindingID: "route-a",
			IdempotencyKey: "attempt-key-a", State: models.PaymentAttemptExternalCreated,
			CreatedAt: now,
		}); err != nil {
			return err
		}
		if err := tx.Create(&models.PaymentProviderAction{
			ActionID: "refund-a", ActionKind: models.PaymentProviderActionRefund,
			AttemptID: "attempt-a", RouteBindingID: "route-a", ProviderBindingID: "binding-a",
			ProviderID: "paypal", ExternalReference: "capture-a", IdempotencyKey: "provider-key-a",
			IntentFingerprint: "fingerprint-a", IntentPayload: []byte(`{}`),
			State: models.PaymentProviderActionCompleted, UpdatedAt: now,
		}); err != nil {
			return err
		}
		return tx.Create(&models.SettlementAction{
			ActionID: "complete-a", OrderID: "order-a", ActionKind: "complete",
			State: "confirmed", UpdatedAt: now,
		})
	}))

	settlement, provider, err := NewPaymentSessionProjector(tenantDB).loadLatestFundsActions("tenant-a", "order-a")
	require.NoError(t, err)
	require.NotNil(t, settlement)
	require.Equal(t, "complete-a", settlement.ActionID)
	require.NotNil(t, provider)
	require.Equal(t, "refund-a", provider.ActionID)
}

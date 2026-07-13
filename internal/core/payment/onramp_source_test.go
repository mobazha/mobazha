// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package payment

import (
	"testing"

	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

// TestLoadOnrampFundingSource exercises the durable 1:N history against a real
// in-memory SQLite migration of the side table: terminal records are retained
// but never selected, the active record drives the projection, and flipping
// the active record to a terminal status falls back to the delivered-to-wallet
// forwarding record.
func TestLoadOnrampFundingSource(t *testing.T) {
	db := newVerifierTestDB(t)
	require.NoError(t, db.gormDB.AutoMigrate(&models.PaymentAttemptOnrampFundingSource{}))
	p := NewPaymentSessionProjector(db)

	attempt := &models.PaymentAttempt{TenantID: "t1", AttemptID: "a1"}

	mk := func(orderID, status string, toWallet bool) *models.PaymentAttemptOnrampFundingSource {
		row := &models.PaymentAttemptOnrampFundingSource{
			TenantID:             "t1",
			AttemptID:            "a1",
			OnrampOrderID:        orderID,
			OrderID:              "order-1",
			ProviderID:           "mock-onramp",
			IdempotencyKey:       "idem-" + orderID,
			DeliverToBuyerWallet: toWallet,
		}
		row.SetStatus(status)
		return row
	}

	// History: one failed purchase, one delivered-to-wallet, one active.
	require.NoError(t, db.gormDB.Create(mk("o1", models.OnrampSourceStatusFailed, false)).Error)
	require.NoError(t, db.gormDB.Create(mk("o2", models.OnrampSourceStatusDelivered, true)).Error)
	active := mk("o3", models.OnrampSourceStatusProcessing, false)
	require.NoError(t, db.gormDB.Create(active).Error)

	got := p.loadOnrampFundingSource("t1", attempt)
	require.NotNil(t, got)
	require.Equal(t, "o3", got.OnrampOrderID, "active purchase must drive the projection")
	require.True(t, got.Active())

	// The active purchase fails: the delivered-to-wallet record (pending
	// forwarding) takes over; the failed history never surfaces.
	active.SetStatus(models.OnrampSourceStatusFailed)
	require.False(t, active.Active, "SetStatus must clear the active flag")
	require.NoError(t, db.gormDB.Save(active).Error)

	got = p.loadOnrampFundingSource("t1", attempt)
	require.NotNil(t, got)
	require.Equal(t, "o2", got.OnrampOrderID, "delivered-to-wallet must drive forwarding")

	// Unknown attempt, nil attempt, other tenant: all nil, no error.
	require.Nil(t, p.loadOnrampFundingSource("t1", &models.PaymentAttempt{TenantID: "t1", AttemptID: "zzz"}))
	require.Nil(t, p.loadOnrampFundingSource("t1", nil))
	require.Nil(t, p.loadOnrampFundingSource("other", attempt))
}

// TestLoadOnrampFundingSourceNoTable proves the HasTable guard: a database
// that never migrated the side table yields nil, not an error.
func TestLoadOnrampFundingSourceNoTable(t *testing.T) {
	db := newVerifierTestDB(t)
	p := NewPaymentSessionProjector(db)
	require.Nil(t, p.loadOnrampFundingSource("t1", &models.PaymentAttempt{TenantID: "t1", AttemptID: "a1"}))
}

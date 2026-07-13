// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package payment

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pkpayment "github.com/mobazha/mobazha/pkg/payment"
	"github.com/stretchr/testify/require"
)

// TestOnrampFundingReachesSessionProjection is the backend end-to-end proof for
// ADR-019: a durable onramp funding-source row, read by the REAL payment
// session projector against a REAL order + frozen attempt, surfaces on the
// session as onrampFunding and refines the fine-grained FundingState — while
// the top-level SessionStatus stays awaiting_funds (onramp never advances the
// session; only the on-chain observation does).
func TestOnrampFundingReachesSessionProjection(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	defer db.Close()

	const orderID = "QmOnrampProjectionE2E"
	readyAt := time.Now()
	attempt := frozenPaymentAttemptForProjectionTest(t, orderID)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		for _, m := range []interface{}{
			&models.Order{}, &models.PaymentAttempt{}, &models.PaymentAttemptOnrampFundingSource{},
		} {
			if err := tx.Migrate(m); err != nil {
				return err
			}
		}
		if err := tx.Save(&models.Order{
			ID: models.OrderID(orderID), MyRole: string(models.RoleBuyer), Open: true, PaymentReadyAt: &readyAt,
		}); err != nil {
			return err
		}
		return tx.Create(&attempt)
	}))

	projector := NewPaymentSessionProjector(db)

	// Baseline: no onramp source yet — plain awaiting_funds, no onrampFunding.
	session := mustProject(t, projector, orderID)
	require.Nil(t, session.OnrampFunding, "no source yet")
	require.Equal(t, pkpayment.FundingStateAwaitingFunds, session.PaymentProgress.FundingState)
	require.Equal(t, pkpayment.SessionStatusAwaitingFunds, session.Status)

	// Insert an active onramp purchase (fiat processing) for the attempt.
	source := &models.PaymentAttemptOnrampFundingSource{
		TenantID: attempt.TenantID, AttemptID: attempt.AttemptID, OnrampOrderID: "mock-onramp-1",
		OrderID: orderID, ProviderID: "mock-onramp", IdempotencyKey: "primary",
		DeliveryTarget: "0x1111111111111111111111111111111111111111",
	}
	source.SetStatus(models.OnrampSourceStatusProcessing)
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Create(source) }))

	// The session now carries the onramp funding leg, refined to onramp_processing,
	// but the top-level status is unchanged.
	session = mustProject(t, projector, orderID)
	require.NotNil(t, session.OnrampFunding)
	require.Equal(t, "mock-onramp-1", session.OnrampFunding.OnrampOrderID)
	require.Equal(t, "mock-onramp", session.OnrampFunding.ProviderID)
	require.Equal(t, pkpayment.FundingStateOnrampProcessing, session.PaymentProgress.FundingState)
	require.Equal(t, pkpayment.SessionStatusAwaitingFunds, session.Status,
		"onramp progress must never advance the top-level session status")

	// Deliver to the buyer wallet: the forwarding phase surfaces.
	source.DeliverToBuyerWallet = true
	source.SetStatus(models.OnrampSourceStatusDelivered)
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Save(source) }))

	session = mustProject(t, projector, orderID)
	require.NotNil(t, session.OnrampFunding)
	require.Equal(t, pkpayment.FundingStateOnrampForwarding, session.PaymentProgress.FundingState)
	require.Equal(t, pkpayment.SessionStatusAwaitingFunds, session.Status)
}

func mustProject(t *testing.T, projector *PaymentSessionProjector, orderID string) *pkpayment.PaymentSession {
	t.Helper()
	input, err := projector.fetchProjectInput(orderID)
	require.NoError(t, err)
	session, err := projector.Project(input)
	require.NoError(t, err)
	return session
}

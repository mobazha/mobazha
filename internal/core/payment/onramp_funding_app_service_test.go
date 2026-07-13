// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package payment

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha/internal/payment/onramp"
	onrampmock "github.com/mobazha/mobazha/internal/payment/onramp/mock"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	"github.com/stretchr/testify/require"
)

// newOnrampServiceFixture wires an in-memory DB with a frozen attempt and a
// mock onramp provider whose rail matches the attempt's currency.
func newOnrampServiceFixture(t *testing.T) (*OnrampFundingAppService, *onrampmock.Provider, models.PaymentAttempt) {
	t.Helper()
	db := newVerifierTestDB(t)
	require.NoError(t, db.gormDB.AutoMigrate(&models.PaymentAttempt{}, &models.PaymentAttemptOnrampFundingSource{}))

	attempt := frozenPaymentAttemptForProjectionTest(t, "QmOnrampOrder")
	require.Equal(t, models.PaymentAttemptFundingTargetReady, attempt.State)
	require.NoError(t, db.gormDB.Create(&attempt).Error)

	provider := onrampmock.New(onrampmock.WithRailCapabilities(onrampmock.OpenRail(attempt.Currency, "USD")))
	registry := onramp.NewRegistry()
	registry.Register(provider)
	return NewOnrampFundingAppService(db, registry), provider, attempt
}

func initiateReq(attempt models.PaymentAttempt) InitiateOnrampFundingRequest {
	return InitiateOnrampFundingRequest{
		OrderID:      attempt.OrderID,
		AttemptID:    attempt.AttemptID,
		Buyer:        contracts.BuyerRef{Subject: "buyer@example.com"},
		ProviderID:   onrampmock.ProviderID,
		FiatCurrency: "USD",
	}
}

func TestOnrampFundingInitiateAndResume(t *testing.T) {
	svc, _, attempt := newOnrampServiceFixture(t)
	ctx := context.Background()

	first, err := svc.InitiateOrResume(ctx, initiateReq(attempt))
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, "awaiting_payment", first.Status)
	require.True(t, first.Active())

	// Leave and return: same (default) idempotency key resumes the purchase.
	again, err := svc.InitiateOrResume(ctx, initiateReq(attempt))
	require.NoError(t, err)
	require.Equal(t, first.OnrampOrderID, again.OnrampOrderID, "resume must not create a second onramp order")
}

func TestOnrampFundingRefreshDrivesProjectionStates(t *testing.T) {
	svc, provider, attempt := newOnrampServiceFixture(t)
	ctx := context.Background()

	req := initiateReq(attempt)
	req.DeliverToBuyerWallet = true
	req.BuyerWalletAddress = "0xbuyerwallet"
	view, err := svc.InitiateOrResume(ctx, req)
	require.NoError(t, err)

	// awaiting_payment refines the pre-observation funding state.
	state := payment.RefineFundingStateForOnramp(payment.FundingStateAwaitingFunds, "0", view)
	require.Equal(t, payment.FundingStateOnrampAwaitingPayment, state)

	// Provider progresses to processing; refresh persists the transition.
	require.NoError(t, provider.SetStatus(view.OnrampOrderID, contracts.OnrampStatusProcessing))
	view, err = svc.RefreshStatus(ctx, "", attempt.AttemptID)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, "processing", view.Status)
	state = payment.RefineFundingStateForOnramp(payment.FundingStateAwaitingFunds, "0", view)
	require.Equal(t, payment.FundingStateOnrampProcessing, state)

	// Delivered to the buyer wallet: selection flips to the forwarding phase.
	require.NoError(t, provider.SetStatus(view.OnrampOrderID, contracts.OnrampStatusDelivered))
	view, err = svc.RefreshStatus(ctx, "", attempt.AttemptID)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, "delivered", view.Status)
	state = payment.RefineFundingStateForOnramp(payment.FundingStateAwaitingFunds, "0", view)
	require.Equal(t, payment.FundingStateOnrampForwarding, state)

	// Once funds are observed on chain, the observation-driven state wins.
	state = payment.RefineFundingStateForOnramp(payment.FundingStateFullyFunded, "1000", view)
	require.Equal(t, payment.FundingStateFullyFunded, state)
}

func TestOnrampFundingGates(t *testing.T) {
	svc, _, attempt := newOnrampServiceFixture(t)
	ctx := context.Background()

	// Unknown attempt.
	req := initiateReq(attempt)
	req.AttemptID = "nope"
	_, err := svc.InitiateOrResume(ctx, req)
	require.ErrorIs(t, err, ErrOnrampAttemptNotFound)

	// Unknown provider.
	req = initiateReq(attempt)
	req.ProviderID = "ghost"
	_, err = svc.InitiateOrResume(ctx, req)
	require.ErrorIs(t, err, contracts.ErrOnrampProviderNotFound)
}

func TestOnrampFundingRefusesUnfrozenAttempt(t *testing.T) {
	dbw := newVerifierTestDB(t)
	require.NoError(t, dbw.gormDB.AutoMigrate(&models.PaymentAttempt{}, &models.PaymentAttemptOnrampFundingSource{}))
	draft := models.PaymentAttempt{
		AttemptID: "attempt-draft", OrderID: "QmDraft", Kind: models.PaymentAttemptKindCryptoFundingTarget,
		PaymentSessionID: "ps_QmDraft", Currency: "crypto:eip155:1:native", RouteBindingID: "route-1",
		IdempotencyKey: "attempt-draft", State: models.PaymentAttemptAuthorizationDraft,
	}
	require.NoError(t, dbw.gormDB.Create(&draft).Error)

	registry := onramp.NewRegistry()
	registry.Register(onrampmock.New(onrampmock.WithRailCapabilities(onrampmock.OpenRail(draft.Currency, "USD"))))
	svc := NewOnrampFundingAppService(dbw, registry)

	req := initiateReq(draft)
	_, err := svc.InitiateOrResume(context.Background(), req)
	require.ErrorIs(t, err, ErrOnrampAttemptNotReady)
}

func TestOnrampFundingCapabilityGate(t *testing.T) {
	dbw := newVerifierTestDB(t)
	require.NoError(t, dbw.gormDB.AutoMigrate(&models.PaymentAttempt{}, &models.PaymentAttemptOnrampFundingSource{}))
	attempt := frozenPaymentAttemptForProjectionTest(t, "QmOnrampGate")
	require.NoError(t, dbw.gormDB.Create(&attempt).Error)

	// Provider registered but rail not opened: fail-closed.
	registry := onramp.NewRegistry()
	registry.Register(onrampmock.New())
	svc := NewOnrampFundingAppService(dbw, registry)

	_, err := svc.InitiateOrResume(context.Background(), initiateReq(attempt))
	require.ErrorIs(t, err, contracts.ErrOnrampCapabilityClosed)
}

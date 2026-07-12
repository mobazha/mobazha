package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	"github.com/stretchr/testify/require"
)

type attemptFundingProjectorStub struct {
	request payment.AttemptSettlementFundingRequest
	target  models.PaymentAttemptFundingTarget
}

func (s *attemptFundingProjectorStub) ProjectAttemptSettlementFundingTarget(
	_ context.Context,
	request payment.AttemptSettlementFundingRequest,
) (models.PaymentAttemptFundingTarget, error) {
	s.request = request
	return s.target, nil
}

func TestStandardOrderStrategyFundingTargetProjector_ForwardsFrozenScope(t *testing.T) {
	stub := &attemptFundingProjectorStub{target: models.PaymentAttemptFundingTarget{
		Version: models.PaymentAttemptFundingTargetVersion, AttemptID: "attempt-safe",
		Type: models.PaymentAttemptFundingTargetAddress, AssetID: "crypto:eip155:1:native",
		AmountAtomic: "1000", Address: "0x1111111111111111111111111111111111111111",
	}}
	attempt := models.PaymentAttempt{AttemptID: "attempt-safe", OrderID: "order-safe"}
	route := models.PaymentRouteBinding{RouteBindingID: "route-safe"}
	offers := []models.SettlementKeyOffer{{ParticipantRole: models.SettlementParticipantBuyer}}

	target, err := (standardOrderStrategyFundingTargetProjector{projector: stub}).
		ProjectStandardOrderFundingTarget(t.Context(), attempt, route, offers)
	require.NoError(t, err)
	require.Equal(t, stub.target, target)
	require.Equal(t, attempt.AttemptID, stub.request.Attempt.AttemptID)
	require.Equal(t, route.RouteBindingID, stub.request.Route.RouteBindingID)
	require.Equal(t, offers, stub.request.Offers)

	offers[0].ParticipantRole = models.SettlementParticipantSeller
	require.Equal(t, models.SettlementParticipantBuyer, stub.request.Offers[0].ParticipantRole)
}

package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

type attemptFundingProjectorStub struct {
	request payment.AttemptSettlementFundingRequest
	target  models.PaymentAttemptFundingTarget
}

func TestStandardOrderSettlementPayoutRail_UsesNativeRailForTokenAsset(t *testing.T) {
	tests := []struct {
		name  string
		asset string
		want  iwallet.CoinType
	}{
		{
			name: "native unchanged", asset: "crypto:eip155:1:native",
			want: "crypto:eip155:1:native",
		},
		{
			name: "erc20 uses chain native", asset: "crypto:eip155:1:erc20:0x1111111111111111111111111111111111111111",
			want: "crypto:eip155:1:native",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := standardOrderSettlementPayoutRail(tt.asset)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}

	_, err := standardOrderSettlementPayoutRail("crypto:eip155:999999:erc20:0x1111111111111111111111111111111111111111")
	require.Error(t, err)
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

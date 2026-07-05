package distribution

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecidePaymentRoute_WorkModes_FailClosed(t *testing.T) {
	tests := []struct {
		name    string
		request PaymentRouteDecisionRequest
		allowed bool
		code    PaymentRouteDecisionCode
	}{
		{name: "admit ready active", request: PaymentRouteDecisionRequest{WorkMode: PaymentRouteAdmitNew, ContributionID: "c", ProviderBindingID: "b", BindingState: "active", ContributionAvailability: PaymentRouteReady, HistoricalImplementationAvailable: true}, allowed: true, code: PaymentRouteAllowedCode},
		{name: "admit existing only denied", request: PaymentRouteDecisionRequest{WorkMode: PaymentRouteAdmitNew, ContributionID: "c", ProviderBindingID: "b", BindingState: "active", ContributionAvailability: PaymentRouteExistingOnly, HistoricalImplementationAvailable: true}, code: PaymentRouteContributionUnavailableCode},
		{name: "reconcile retired allowed", request: PaymentRouteDecisionRequest{WorkMode: PaymentRouteReconcile, ContributionID: "c", ProviderBindingID: "b", BindingState: "retired", ContributionAvailability: PaymentRouteExistingOnly, HistoricalImplementationAvailable: true}, allowed: true, code: PaymentRouteAllowedCode},
		{name: "historical implementation denied", request: PaymentRouteDecisionRequest{WorkMode: PaymentRouteReconcile, ContributionID: "c", ProviderBindingID: "b", BindingState: "retired", ContributionAvailability: PaymentRouteExistingOnly}, code: PaymentRouteHistoricalImplementationCode},
		{name: "unknown binding state denied", request: PaymentRouteDecisionRequest{WorkMode: PaymentRouteServiceExisting, ContributionID: "c", ProviderBindingID: "b", BindingState: "missing", ContributionAvailability: PaymentRouteReady, HistoricalImplementationAvailable: true}, code: PaymentRouteBindingUnavailableCode},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := DecidePaymentRoute(test.request)
			assert.Equal(t, test.allowed, decision.Allowed)
			assert.Equal(t, test.code, decision.Code)
		})
	}
}

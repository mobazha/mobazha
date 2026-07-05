package distribution

import "fmt"

type PaymentRouteWorkMode string

const (
	PaymentRouteAdmitNew        PaymentRouteWorkMode = "admit_new"
	PaymentRouteServiceExisting PaymentRouteWorkMode = "service_existing"
	PaymentRouteReconcile       PaymentRouteWorkMode = "reconcile"
)

type PaymentRouteAvailability string

const (
	PaymentRouteReady        PaymentRouteAvailability = "ready"
	PaymentRouteExistingOnly PaymentRouteAvailability = "existing_only"
	PaymentRouteUnavailable  PaymentRouteAvailability = "unavailable"
)

type PaymentRouteDecisionCode string

const (
	PaymentRouteAllowedCode                  PaymentRouteDecisionCode = "allowed"
	PaymentRouteInvalidRequestCode           PaymentRouteDecisionCode = "invalid_request"
	PaymentRouteContributionUnavailableCode  PaymentRouteDecisionCode = "contribution_unavailable"
	PaymentRouteBindingUnavailableCode       PaymentRouteDecisionCode = "binding_unavailable"
	PaymentRouteHistoricalImplementationCode PaymentRouteDecisionCode = "historical_implementation_unavailable"
)

type PaymentRouteDecisionRequest struct {
	WorkMode                          PaymentRouteWorkMode
	ContributionID                    string
	ProviderBindingID                 string
	ContributionAvailability          PaymentRouteAvailability
	BindingState                      string
	HistoricalImplementationAvailable bool
}

type PaymentRouteDecision struct {
	Allowed bool
	Code    PaymentRouteDecisionCode
	Reason  string
}

func DecidePaymentRoute(request PaymentRouteDecisionRequest) PaymentRouteDecision {
	if request.ContributionID == "" {
		return deniedPaymentRoute(PaymentRouteInvalidRequestCode, "payment contribution is required")
	}
	switch request.WorkMode {
	case PaymentRouteAdmitNew, PaymentRouteServiceExisting, PaymentRouteReconcile:
	default:
		return deniedPaymentRoute(PaymentRouteInvalidRequestCode, fmt.Sprintf("unknown payment route work mode %q", request.WorkMode))
	}
	if !request.HistoricalImplementationAvailable {
		return deniedPaymentRoute(PaymentRouteHistoricalImplementationCode, "historical payment implementation is unavailable")
	}
	if request.ProviderBindingID != "" && request.BindingState != "active" &&
		(request.WorkMode == PaymentRouteAdmitNew || request.BindingState != "retired") {
		return deniedPaymentRoute(PaymentRouteBindingUnavailableCode, fmt.Sprintf("provider binding is %q", request.BindingState))
	}
	if request.WorkMode == PaymentRouteAdmitNew {
		if request.ContributionAvailability != PaymentRouteReady {
			return deniedPaymentRoute(PaymentRouteContributionUnavailableCode, fmt.Sprintf("payment contribution is %q", request.ContributionAvailability))
		}
	} else if request.ContributionAvailability != PaymentRouteReady && request.ContributionAvailability != PaymentRouteExistingOnly {
		return deniedPaymentRoute(PaymentRouteContributionUnavailableCode, fmt.Sprintf("payment contribution is %q", request.ContributionAvailability))
	}
	return PaymentRouteDecision{Allowed: true, Code: PaymentRouteAllowedCode}
}

func deniedPaymentRoute(code PaymentRouteDecisionCode, reason string) PaymentRouteDecision {
	return PaymentRouteDecision{Code: code, Reason: reason}
}

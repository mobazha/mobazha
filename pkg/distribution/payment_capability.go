// SPDX-License-Identifier: MPL-2.0

package distribution

import (
	"context"
	"strings"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// PaymentCapabilityRequest identifies one concrete new-work admission check.
// Provider discovery may use PaymentAssetAny because it is provider-scoped;
// all payment-session requests must carry their concrete canonical asset.
type PaymentCapabilityRequest struct {
	Rail      PaymentRailKind
	Network   iwallet.ChainType
	Asset     iwallet.CoinType
	Operation PaymentRailOperation
}

// PaymentCapabilityDecisionCode is a stable, non-sensitive result suitable
// for policy decisions and metrics. Operator diagnostics remain in Health().
type PaymentCapabilityDecisionCode string

const (
	PaymentCapabilityAllowed                   PaymentCapabilityDecisionCode = "allowed"
	PaymentCapabilityInvalidRequest            PaymentCapabilityDecisionCode = "invalid_request"
	PaymentCapabilityNotComposed               PaymentCapabilityDecisionCode = "not_composed"
	PaymentCapabilityAmbiguous                 PaymentCapabilityDecisionCode = "ambiguous_contribution"
	PaymentCapabilityImplementationUnavailable PaymentCapabilityDecisionCode = "implementation_unavailable"
	PaymentCapabilityModuleNotReady            PaymentCapabilityDecisionCode = "module_not_ready"
	PaymentCapabilityTenantGateUnavailable     PaymentCapabilityDecisionCode = "tenant_gate_unavailable"
	PaymentCapabilityTenantGateError           PaymentCapabilityDecisionCode = "tenant_gate_error"
	PaymentCapabilityNotAuthorized             PaymentCapabilityDecisionCode = "tenant_not_authorized"
	PaymentCapabilityNotConfigured             PaymentCapabilityDecisionCode = "tenant_not_configured"
)

// PaymentCapabilityDecision is derived by Core. Code is the single source of
// truth: callers cannot independently assert a contradictory allowed flag.
type PaymentCapabilityDecision struct {
	Code           PaymentCapabilityDecisionCode
	ModuleID       string
	ContributionID string
}

// Allowed reports whether every admission gate passed.
func (decision PaymentCapabilityDecision) Allowed() bool {
	return decision.Code == PaymentCapabilityAllowed
}

// PaymentTenantCapability is the tenant-owned half of new-work admission.
// Its zero value fails closed.
type PaymentTenantCapability struct {
	Authorized bool
	Configured bool
}

// PaymentTenantCapabilityResolver evaluates the exact requested asset and
// tenant configuration without exposing product databases to the manager.
type PaymentTenantCapabilityResolver interface {
	ResolvePaymentTenantCapability(
		ctx context.Context,
		tenantID string,
		request PaymentCapabilityRequest,
		descriptor PaymentModuleDescriptor,
		contribution PaymentRailContribution,
	) (PaymentTenantCapability, error)
}

// PaymentCapabilityDecisionProvider is the optional node-side policy port
// consumed by discovery and session admission.
type PaymentCapabilityDecisionProvider interface {
	DecidePaymentCapability(context.Context, PaymentCapabilityRequest) PaymentCapabilityDecision
}

// DecidePaymentCapability evaluates one exact request against composed module
// ownership, runtime readiness, and tenant configuration. It is fail-closed and
// intentionally does not expose a mutable or internally contradictory snapshot.
func (m *TrustedPaymentModuleManager) DecidePaymentCapability(
	ctx context.Context,
	tenantID string,
	request PaymentCapabilityRequest,
	resolver PaymentTenantCapabilityResolver,
) PaymentCapabilityDecision {
	if strings.TrimSpace(tenantID) == "" || !validPaymentCapabilityRequest(request) {
		return deniedPaymentCapability(PaymentCapabilityInvalidRequest, PaymentModuleHealth{}, PaymentRailContribution{})
	}

	health, contribution, matched, ambiguous := selectPaymentCapabilityContribution(m.Health(), request)
	if ambiguous {
		return deniedPaymentCapability(PaymentCapabilityAmbiguous, PaymentModuleHealth{}, PaymentRailContribution{})
	}
	if !matched {
		return deniedPaymentCapability(PaymentCapabilityNotComposed, PaymentModuleHealth{}, PaymentRailContribution{})
	}
	if !health.Active {
		return deniedPaymentCapability(PaymentCapabilityImplementationUnavailable, health, contribution)
	}
	if health.State != PaymentModuleReady {
		if health.Descriptor.Activation == PaymentModuleSetupGated && health.State == PaymentModuleNeedsSetup {
			return deniedPaymentCapability(PaymentCapabilityNotConfigured, health, contribution)
		}
		return deniedPaymentCapability(PaymentCapabilityModuleNotReady, health, contribution)
	}
	if resolver == nil {
		return deniedPaymentCapability(PaymentCapabilityTenantGateUnavailable, health, contribution)
	}

	tenant, err := resolver.ResolvePaymentTenantCapability(
		ctx, tenantID, request, clonePaymentModuleDescriptor(health.Descriptor), clonePaymentRailContribution(contribution),
	)
	if err != nil {
		return deniedPaymentCapability(PaymentCapabilityTenantGateError, health, contribution)
	}
	if !tenant.Authorized {
		return deniedPaymentCapability(PaymentCapabilityNotAuthorized, health, contribution)
	}
	if !tenant.Configured {
		return deniedPaymentCapability(PaymentCapabilityNotConfigured, health, contribution)
	}
	return PaymentCapabilityDecision{
		Code:     PaymentCapabilityAllowed,
		ModuleID: contribution.ModuleID, ContributionID: contribution.ContributionID,
	}
}

func validPaymentCapabilityRequest(request PaymentCapabilityRequest) bool {
	if request.Rail == "" || request.Network == "" || request.Asset == "" || request.Operation == "" {
		return false
	}
	return request.Asset != PaymentAssetAny || request.Rail == PaymentRailProviderSession
}

func selectPaymentCapabilityContribution(
	health []PaymentModuleHealth,
	request PaymentCapabilityRequest,
) (PaymentModuleHealth, PaymentRailContribution, bool, bool) {
	type candidate struct {
		health       PaymentModuleHealth
		contribution PaymentRailContribution
		exact        bool
	}
	candidates := make([]candidate, 0, 1)
	for _, module := range health {
		for _, contribution := range module.Contributions {
			if contribution.Rail != request.Rail || contribution.Network != request.Network ||
				!paymentContributionSupportsOperation(contribution, request.Operation) {
				continue
			}
			exact := contribution.Asset == request.Asset
			if !exact && contribution.Asset != PaymentAssetAny {
				continue
			}
			candidates = append(candidates, candidate{health: module, contribution: contribution, exact: exact})
		}
	}

	exactCount := 0
	for _, candidate := range candidates {
		if candidate.exact {
			exactCount++
		}
	}
	if exactCount > 1 || (exactCount == 0 && len(candidates) > 1) {
		return PaymentModuleHealth{}, PaymentRailContribution{}, true, true
	}
	for _, candidate := range candidates {
		if exactCount == 0 || candidate.exact {
			return candidate.health, candidate.contribution, true, false
		}
	}
	return PaymentModuleHealth{}, PaymentRailContribution{}, false, false
}

func paymentContributionSupportsOperation(contribution PaymentRailContribution, operation PaymentRailOperation) bool {
	for _, candidate := range contribution.Operations {
		if candidate == operation {
			return true
		}
	}
	return false
}

func deniedPaymentCapability(
	code PaymentCapabilityDecisionCode,
	health PaymentModuleHealth,
	contribution PaymentRailContribution,
) PaymentCapabilityDecision {
	moduleID := contribution.ModuleID
	if moduleID == "" {
		moduleID = health.Descriptor.ID
	}
	return PaymentCapabilityDecision{
		Code: code, ModuleID: moduleID, ContributionID: contribution.ContributionID,
	}
}

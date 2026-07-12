// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/payment"
	"gorm.io/gorm"
)

// DecidePaymentCapability returns Core's single tenant-scoped new-work
// admission decision. Historical work continues on its captured route.
func (n *MobazhaNode) DecidePaymentCapability(
	ctx context.Context,
	request distribution.PaymentCapabilityRequest,
) distribution.PaymentCapabilityDecision {
	if n == nil || n.paymentModuleManager == nil {
		return distribution.PaymentCapabilityDecision{Code: distribution.PaymentCapabilityNotComposed}
	}
	return n.paymentModuleManager.DecidePaymentCapability(
		ctx, n.nodeID, request, nodePaymentTenantCapabilityResolver{node: n},
	)
}

// ResolveNewPaymentRouteIdentity admits one exact new-work request and returns
// the immutable module route that must be persisted with the attempt.
func (n *MobazhaNode) ResolveNewPaymentRouteIdentity(
	ctx context.Context,
	request distribution.PaymentCapabilityRequest,
) (payment.RouteIdentity, error) {
	if n == nil || n.paymentModuleManager == nil {
		return payment.RouteIdentity{}, fmt.Errorf("%w: payment route manager is unavailable", contracts.ErrCoinUnavailable)
	}
	decision := n.DecidePaymentCapability(ctx, request)
	if !decision.Allowed() {
		return payment.RouteIdentity{}, fmt.Errorf("%w: payment capability denied (%s)", contracts.ErrCoinUnavailable, decision.Code)
	}
	route, err := n.paymentModuleManager.ResolveAllowedPaymentRouteIdentity(request, decision)
	if err != nil {
		return payment.RouteIdentity{}, fmt.Errorf("resolve payment route identity: %w", err)
	}
	return route, nil
}

type nodePaymentTenantCapabilityResolver struct {
	node *MobazhaNode
}

func (r nodePaymentTenantCapabilityResolver) ResolvePaymentTenantCapability(
	ctx context.Context,
	tenantID string,
	_ distribution.PaymentCapabilityRequest,
	_ distribution.PaymentModuleDescriptor,
	contribution distribution.PaymentRailContribution,
) (distribution.PaymentTenantCapability, error) {
	if r.node == nil || strings.TrimSpace(tenantID) == "" || tenantID != r.node.nodeID {
		return distribution.PaymentTenantCapability{}, nil
	}
	switch contribution.Rail {
	case distribution.PaymentRailEscrow:
		// Escrow sessions are provisioned by the buyer from the seller's signed
		// order terms. Requiring a matching receiving-account row on the buyer
		// node would make cross-store checkout impossible. Seller-side payment
		// discovery already filters assets through its configured accounts; the
		// session gate therefore owns composition/readiness and tenant scope only.
		return distribution.PaymentTenantCapability{Authorized: true, Configured: true}, nil
	case distribution.PaymentRailDirectObserved:
		// The module is composed per node and owns setup-aware health. Ready is
		// evaluated independently by the manager; no generic database row exists.
		return distribution.PaymentTenantCapability{Authorized: true, Configured: true}, nil
	case distribution.PaymentRailProviderSession:
		return r.providerSessionCapability(ctx, contribution)
	default:
		return distribution.PaymentTenantCapability{}, fmt.Errorf("unsupported payment rail %q", contribution.Rail)
	}
}
func (r nodePaymentTenantCapabilityResolver) providerSessionCapability(
	_ context.Context,
	contribution distribution.PaymentRailContribution,
) (distribution.PaymentTenantCapability, error) {
	if r.node.fiatPaymentService == nil {
		return distribution.PaymentTenantCapability{}, nil
	}
	providerID := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(string(contribution.Network))), "fiat:")
	if providerID == "" || providerID == strings.ToLower(strings.TrimSpace(string(contribution.Network))) {
		return distribution.PaymentTenantCapability{}, fmt.Errorf("provider-session network %q is invalid", contribution.Network)
	}
	if r.node.fiatRegistry == nil {
		return distribution.PaymentTenantCapability{}, nil
	}
	if _, err := r.node.fiatRegistry.ForProvider(providerID); err != nil {
		return distribution.PaymentTenantCapability{}, nil
	}
	account, err := r.node.fiatPaymentService.getActiveAccount(providerID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return distribution.PaymentTenantCapability{Authorized: true}, nil
	}
	if err != nil {
		return distribution.PaymentTenantCapability{}, fmt.Errorf("resolve provider tenant binding: %w", err)
	}
	return distribution.PaymentTenantCapability{
		Authorized: true,
		Configured: account != nil && strings.TrimSpace(account.Address) != "",
	}, nil
}

var _ distribution.PaymentCapabilityDecisionProvider = (*MobazhaNode)(nil)
var _ distribution.PaymentTenantCapabilityResolver = nodePaymentTenantCapabilityResolver{}

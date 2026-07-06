// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
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

type nodePaymentTenantCapabilityResolver struct {
	node *MobazhaNode
}

func (r nodePaymentTenantCapabilityResolver) ResolvePaymentTenantCapability(
	ctx context.Context,
	tenantID string,
	request distribution.PaymentCapabilityRequest,
	_ distribution.PaymentModuleDescriptor,
	contribution distribution.PaymentRailContribution,
) (distribution.PaymentTenantCapability, error) {
	if r.node == nil || strings.TrimSpace(tenantID) == "" || tenantID != r.node.nodeID {
		return distribution.PaymentTenantCapability{}, nil
	}
	switch contribution.Rail {
	case distribution.PaymentRailEscrow:
		configured, err := r.hasConfiguredReceivingAccount(request, contribution)
		if err != nil {
			return distribution.PaymentTenantCapability{}, err
		}
		return distribution.PaymentTenantCapability{Authorized: true, Configured: configured}, nil
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

func (r nodePaymentTenantCapabilityResolver) hasConfiguredReceivingAccount(
	request distribution.PaymentCapabilityRequest,
	contribution distribution.PaymentRailContribution,
) (bool, error) {
	if r.node.receivingAccountService == nil {
		return false, nil
	}
	accounts, err := r.node.receivingAccountService.List()
	if err != nil {
		return false, fmt.Errorf("list receiving accounts: %w", err)
	}
	for _, account := range accounts {
		if !account.IsActive || account.ChainType != contribution.Network || account.ChainType != request.Network {
			continue
		}
		if receivingAccountAcceptsAsset(account, request.Asset) {
			return true, nil
		}
	}
	return false, nil
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

func receivingAccountAcceptsAsset(account models.ReceivingAccount, asset iwallet.CoinType) bool {
	candidates := map[string]struct{}{}
	if value := strings.TrimSpace(string(asset)); value != "" {
		candidates[strings.ToLower(value)] = struct{}{}
	}
	if value, err := asset.PricingCurrencyCode(); err == nil {
		if value = strings.TrimSpace(value); value != "" {
			candidates[strings.ToLower(value)] = struct{}{}
		}
	}
	if info, err := iwallet.CoinInfoFromCoinType(asset); err == nil && info.IsNative && info.Chain == account.ChainType {
		candidates[strings.ToLower(string(account.ChainType))] = struct{}{}
	}
	if len(candidates) == 0 {
		return false
	}
	for _, accepted := range account.AcceptedCurrencies() {
		if _, ok := candidates[strings.ToLower(strings.TrimSpace(accepted))]; ok {
			return true
		}
	}
	return false
}

var _ distribution.PaymentCapabilityDecisionProvider = (*MobazhaNode)(nil)
var _ distribution.PaymentTenantCapabilityResolver = nodePaymentTenantCapabilityResolver{}

// SPDX-License-Identifier: MPL-2.0

package distribution

import (
	"context"
	"errors"
	"testing"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type paymentTenantCapabilityResolverFunc func(
	context.Context,
	string,
	PaymentCapabilityRequest,
	PaymentModuleDescriptor,
	PaymentRailContribution,
) (PaymentTenantCapability, error)

func (resolve paymentTenantCapabilityResolverFunc) ResolvePaymentTenantCapability(
	ctx context.Context,
	tenantID string,
	request PaymentCapabilityRequest,
	descriptor PaymentModuleDescriptor,
	contribution PaymentRailContribution,
) (PaymentTenantCapability, error) {
	return resolve(ctx, tenantID, request, descriptor, contribution)
}

func readyPaymentCapabilityManager(t *testing.T) *TrustedPaymentModuleManager {
	t.Helper()
	module := &metadataPaymentModule{
		id: "module.direct", network: iwallet.ChainType("private"), asset: iwallet.CoinType("private:native"),
	}
	manager, err := registerTestPaymentModules(context.Background(), newTestPaymentRegistry(), module)
	require.NoError(t, err)
	require.NoError(t, manager.Start(context.Background(), nil))
	t.Cleanup(func() { require.NoError(t, manager.Stop(context.Background())) })
	return manager
}

func directCapabilityRequest() PaymentCapabilityRequest {
	return PaymentCapabilityRequest{
		Rail: PaymentRailDirectObserved, Network: "private", Asset: "private:native", Operation: PaymentOperationObserve,
	}
}

func allowTenantCapabilityResolver() PaymentTenantCapabilityResolver {
	return paymentTenantCapabilityResolverFunc(
		func(context.Context, string, PaymentCapabilityRequest, PaymentModuleDescriptor, PaymentRailContribution) (PaymentTenantCapability, error) {
			return PaymentTenantCapability{Authorized: true, Configured: true}, nil
		},
	)
}

func TestDecidePaymentCapability_AllowsOnlyDerivedIntersection(t *testing.T) {
	decision := readyPaymentCapabilityManager(t).DecidePaymentCapability(
		context.Background(), "tenant-a", directCapabilityRequest(), allowTenantCapabilityResolver(),
	)

	assert.True(t, decision.Allowed())
	assert.Equal(t, PaymentCapabilityAllowed, decision.Code)
	assert.Equal(t, "module.direct", decision.ModuleID)
	assert.NotEmpty(t, decision.ContributionID)
}

func TestResolveAllowedPaymentRouteIdentity_BindsCapabilityContribution(t *testing.T) {
	manager := readyPaymentCapabilityManager(t)
	request := directCapabilityRequest()
	decision := manager.DecidePaymentCapability(
		context.Background(), "tenant-a", request, allowTenantCapabilityResolver(),
	)
	require.True(t, decision.Allowed())

	route, err := manager.ResolveAllowedPaymentRouteIdentity(request, decision)
	require.NoError(t, err)
	assert.Equal(t, decision.ModuleID, route.ModuleID)
	assert.Equal(t, decision.ContributionID, route.ContributionID)
	assert.Equal(t, string(request.Network), route.NetworkID)
	assert.Equal(t, string(request.Asset), route.AssetID)
	require.NoError(t, route.Validate())

	fabricated := decision
	fabricated.ContributionID = "different"
	_, err = manager.ResolveAllowedPaymentRouteIdentity(request, fabricated)
	require.ErrorContains(t, err, "does not match capability decision")
}

func TestDecidePaymentCapability_FailsClosedWithoutTenantGate(t *testing.T) {
	decision := readyPaymentCapabilityManager(t).DecidePaymentCapability(
		context.Background(), "tenant-a", directCapabilityRequest(), nil,
	)

	assert.False(t, decision.Allowed())
	assert.Equal(t, PaymentCapabilityTenantGateUnavailable, decision.Code)
}

func TestDecidePaymentCapability_FailsClosedOnTenantGateError(t *testing.T) {
	resolver := paymentTenantCapabilityResolverFunc(
		func(context.Context, string, PaymentCapabilityRequest, PaymentModuleDescriptor, PaymentRailContribution) (PaymentTenantCapability, error) {
			return PaymentTenantCapability{}, errors.New("configuration unavailable")
		},
	)
	decision := readyPaymentCapabilityManager(t).DecidePaymentCapability(
		context.Background(), "tenant-a", directCapabilityRequest(), resolver,
	)

	assert.False(t, decision.Allowed())
	assert.Equal(t, PaymentCapabilityTenantGateError, decision.Code)
}

func TestDecidePaymentCapability_NotReadyIsDenied(t *testing.T) {
	module := &metadataPaymentModule{id: "module.direct", network: "private", asset: "private:native"}
	manager, err := registerTestPaymentModules(context.Background(), newTestPaymentRegistry(), module)
	require.NoError(t, err)

	decision := manager.DecidePaymentCapability(
		context.Background(), "tenant-a", directCapabilityRequest(), allowTenantCapabilityResolver(),
	)
	assert.False(t, decision.Allowed())
	assert.Equal(t, PaymentCapabilityModuleNotReady, decision.Code)
}

func TestDecidePaymentCapability_SetupGatedNeedsSetupIsNotConfigured(t *testing.T) {
	module := &statusTestPaymentModule{updates: make(chan paymentModuleStatusUpdate, 1)}
	module.updates <- paymentModuleStatusUpdate{state: PaymentModuleNeedsSetup, err: errors.New("setup required")}
	manager, err := NewTrustedPaymentModuleManager(
		NewPaymentRuntimeAuthority(ManagedEVMRuntime{}, ManagedSolanaRuntime{}, ManagedEscrowGuestRuntimePorts{}, DirectObservedRuntimePorts{}),
		newTestPaymentRegistry(), module,
	)
	require.NoError(t, err)
	require.NoError(t, manager.Register(context.Background()))
	require.NoError(t, manager.Start(context.Background(), nil))
	t.Cleanup(func() { require.NoError(t, manager.Stop(context.Background())) })

	decision := manager.DecidePaymentCapability(context.Background(), "tenant-a", PaymentCapabilityRequest{
		Rail: PaymentRailDirectObserved, Network: iwallet.ChainMonero,
		Asset: "crypto:monero:mainnet:native", Operation: PaymentOperationSetup,
	}, allowTenantCapabilityResolver())
	assert.False(t, decision.Allowed())
	assert.Equal(t, PaymentCapabilityNotConfigured, decision.Code)
}

func TestSelectPaymentCapabilityContribution_PrefersExactAssetOverWildcard(t *testing.T) {
	request := PaymentCapabilityRequest{
		Rail: PaymentRailEscrow, Network: iwallet.ChainBSC, Asset: "crypto:bsc:token", Operation: PaymentOperationSetup,
	}
	health := []PaymentModuleHealth{
		{Contributions: []PaymentRailContribution{{ModuleID: "wildcard", ContributionID: "wildcard", Rail: request.Rail, Network: request.Network, Asset: PaymentAssetAny, Operations: []PaymentRailOperation{request.Operation}}}},
		{Contributions: []PaymentRailContribution{{ModuleID: "exact", ContributionID: "exact", Rail: request.Rail, Network: request.Network, Asset: request.Asset, Operations: []PaymentRailOperation{request.Operation}}}},
	}

	_, contribution, matched, ambiguous := selectPaymentCapabilityContribution(health, request)
	assert.True(t, matched)
	assert.False(t, ambiguous)
	assert.Equal(t, "exact", contribution.ModuleID)
}

func TestSelectPaymentCapabilityContribution_RejectsAmbiguousOwners(t *testing.T) {
	request := PaymentCapabilityRequest{
		Rail: PaymentRailEscrow, Network: iwallet.ChainBSC, Asset: "crypto:bsc:token", Operation: PaymentOperationSetup,
	}
	contribution := PaymentRailContribution{
		Rail: request.Rail, Network: request.Network, Asset: PaymentAssetAny, Operations: []PaymentRailOperation{request.Operation},
	}
	health := []PaymentModuleHealth{
		{Contributions: []PaymentRailContribution{contribution}},
		{Contributions: []PaymentRailContribution{contribution}},
	}

	_, _, matched, ambiguous := selectPaymentCapabilityContribution(health, request)
	assert.True(t, matched)
	assert.True(t, ambiguous)
}

func TestDecidePaymentCapability_RejectsWildcardAssetOutsideProviderDiscovery(t *testing.T) {
	request := directCapabilityRequest()
	request.Asset = PaymentAssetAny
	decision := readyPaymentCapabilityManager(t).DecidePaymentCapability(
		context.Background(), "tenant-a", request, allowTenantCapabilityResolver(),
	)
	assert.False(t, decision.Allowed())
	assert.Equal(t, PaymentCapabilityInvalidRequest, decision.Code)
}

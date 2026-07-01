package core

import (
	"context"
	"testing"

	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/stretchr/testify/require"
)

// TestInitFeatureResolver_FallsBackToAllowAllDefaults verifies that a node
// constructed without any feature-flag providers (i.e. no db, no config
// reader, no platform adapter) still ends up with a functional resolver
// rather than a nil reference. This protects handlers that query
// `n.Features()` from having to nil-check on every call.
func TestInitFeatureResolver_FallsBackToAllowAllDefaults(t *testing.T) {
	n := &MobazhaNode{}
	n.initFeatureResolver()

	require.NotNil(t, n.Features(), "resolver must be composed even without injected providers")
	require.NotNil(t, n.platformFeatureProvider, "platform stub must be installed")
	require.NotNil(t, n.tenantFeatureStore, "tenant stub must be installed")
	require.NotNil(t, n.nodeFeatureProvider, "node stub must be installed")

	// Wiring smoke: the resolver must return a value for a registered
	// feature without panicking or erroring. The specific boolean is
	// covered by pkg/config/resolver_test.go — here we only assert that
	// the full chain (feature registry → resolver → three providers)
	// is reachable from the MobazhaNode accessor.
	ctx := pkgconfig.ContextWithTenantID(context.Background(), "t-default")
	ctx = pkgconfig.ContextWithActor(ctx, "test", "anonymous")
	require.NotPanics(t, func() {
		_ = n.Features().IsEnabled(ctx, pkgconfig.FeatureGuestCheckoutEnabled.Key)
	}, "resolver must answer without panic for registered feature")
}

// TestInitFeatureResolver_IsIdempotent ensures double-initialisation (which
// can happen if applyOptions is triggered more than once in tests or the
// SaaS host attaches options after NewNode) doesn't rebuild the resolver
// or swap providers under the caller's feet.
func TestInitFeatureResolver_IsIdempotent(t *testing.T) {
	n := &MobazhaNode{}
	n.initFeatureResolver()
	first := n.Features()

	n.initFeatureResolver()
	second := n.Features()

	require.Same(t, first, second, "resolver must be stable across repeated init calls")
}

// TestInitFeatureResolver_HonoursInjectedProviders verifies that WithNode/
// WithTenant/WithPlatform options take precedence over defaults and that
// all three layers flow into the composed resolver.
func TestInitFeatureResolver_HonoursInjectedProviders(t *testing.T) {
	n := &MobazhaNode{}

	platform := pkgconfig.AllowAllPlatformProvider{}
	tenant := pkgconfig.NoopTenantStore{}
	node := pkgconfig.AllowAllNodeProvider{}

	applyNodeOptions(n, []NodeOption{
		WithPlatformFeatureProvider(platform),
		WithTenantFeatureStore(tenant),
		WithNodeFeatureProvider(node),
	})

	n.initFeatureResolver()

	require.Equal(t, pkgconfig.PlatformGlobalProvider(platform), n.platformFeatureProvider)
	require.Equal(t, pkgconfig.TenantFeatureStore(tenant), n.tenantFeatureStore)
	require.Equal(t, pkgconfig.NodeFeatureProvider(node), n.nodeFeatureProvider)
	require.NotNil(t, n.Features())
}

// TestMobazhaNode_Features_NilReceiverManagedEscrow documents the contract that
// handlers can safely call (*MobazhaNode)(nil).Features() without panicking.
// This is the escape hatch for code paths that run before applyOptions
// (mostly unit-test mocks constructed via &MobazhaNode{...}).
func TestMobazhaNode_Features_NilReceiverManagedEscrow(t *testing.T) {
	var n *MobazhaNode
	require.NotPanics(t, func() { _ = n.Features() })
	require.Nil(t, n.Features())
}

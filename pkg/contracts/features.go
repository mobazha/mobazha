package contracts

import "github.com/mobazha/mobazha3.0/pkg/config"

// FeaturesProvider is the optional accessor for the feature-flag resolver.
// MobazhaNode implements this interface; handlers that need to query
// feature flags should type-assert the NodeService they hold:
//
//	fp, ok := node.(contracts.FeaturesProvider)
//	if ok && fp.Features() != nil {
//	    enabled, _ := fp.Features().IsEnabled(ctx, config.FeatureGuestCheckout.Key)
//	}
//
// The indirection keeps pkg/config as a leaf package (handlers use
// config.ResolverInterface, not *internal/core concrete types) and lets
// alternate implementations (tests, SaaS tenant adapters) substitute the
// resolver without modifying the NodeService surface.
//
// Never embed this interface inside NodeService — feature-flag access is
// cross-cutting concern, not a domain service, and forcing every
// NodeService implementor to expose it would break the Open/Closed
// principle when new cross-cutting concerns emerge.
type FeaturesProvider interface {
	Features() config.ResolverInterface
}

// FeatureAdminProvider exposes the tenant-layer feature store for
// administrative mutations (PUT /v1/settings/features/{key}). The
// read side already flows through FeaturesProvider; admin writes need
// direct access to the store since the Resolver is read-only.
//
// Only nodes that can accept admin writes should implement this
// (e.g. MobazhaNode); SaaS proxy shims without a tenant DB may omit it
// and handlers will surface 501.
type FeatureAdminProvider interface {
	TenantFeatureStore() config.TenantFeatureStore
}

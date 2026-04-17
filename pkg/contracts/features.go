package contracts

import (
	"context"

	"github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

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

// FeatureAuditLogger persists feature-flag change events to the
// feature_flag_audit_logs table. Both the tenant-scope Set path and
// the platform-scope PATCH path go through this interface, which lets
// unit tests substitute an in-memory recorder.
//
// AppendAudit must not block long-running requests: when persistence
// fails the caller logs the error and continues (see
// feature_handlers.go); audit gaps are surfaced by ops alerting rather
// than user-visible failures.
type FeatureAuditLogger interface {
	AppendAudit(ctx context.Context, entry *models.FeatureFlagAuditLog) error
}

// FeatureAuditProvider exposes the optional feature-flag audit logger
// implemented by MobazhaNode. Handlers type-assert:
//
//	ap, ok := node.(contracts.FeatureAuditProvider)
//	if ok && ap.FeatureAuditLogger() != nil {
//	    _ = ap.FeatureAuditLogger().AppendAudit(ctx, entry)
//	}
//
// SaaS proxy shims without a tenant DB may return nil from
// FeatureAuditLogger(); handlers should treat audit writes as
// best-effort (log-and-continue) so missing infrastructure does not
// block feature-flag mutations.
type FeatureAuditProvider interface {
	FeatureAuditLogger() FeatureAuditLogger
}

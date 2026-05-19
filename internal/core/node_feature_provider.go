package core

import (
	"context"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/config"
)

// ConfigNodeFeatureProvider implements config.NodeFeatureProvider by
// reading node-runtime values from repo.Config (populated from CLI
// flags or the on-disk config file).
//
// Binding strategy:
//   - A feature with an explicit reader → the reader's return value is
//     the node layer value.
//   - A feature without a reader → node layer returns true (pass-through;
//     the tenant or default layer decides).
//
// Only features whose AllowedScopes include ScopeNodeRuntime are bound;
// the resolver short-circuits other layers when node scope is absent.
type ConfigNodeFeatureProvider struct {
	cfg     *repo.Config
	readers map[string]func(*repo.Config) bool
}

var _ config.NodeFeatureProvider = (*ConfigNodeFeatureProvider)(nil)

// NewConfigNodeFeatureProvider creates the adapter, registering default
// bindings for all features whose node-runtime state is exposed via
// repo.Config. Consumers can extend the map via WithReader before use.
func NewConfigNodeFeatureProvider(cfg *repo.Config) *ConfigNodeFeatureProvider {
	p := &ConfigNodeFeatureProvider{
		cfg:     cfg,
		readers: make(map[string]func(*repo.Config) bool),
	}
	p.registerDefaults()
	return p
}

// NewNodeFeatureProviderForConfig returns the node-runtime provider for a node
// config. SaaS tenant nodes do not have a per-process CLI flag surface; their
// guest checkout availability is governed by platform policy plus seller
// settings, so the node layer must pass through instead of reading the
// standalone-node GuestCheckout flag default.
func NewNodeFeatureProviderForConfig(cfg *repo.Config) config.NodeFeatureProvider {
	if cfg != nil && cfg.SaaSMode {
		return config.AllowAllNodeProvider{}
	}
	return NewConfigNodeFeatureProvider(cfg)
}

// WithReader registers/overrides a reader for the given feature key. It
// is safe to call before the provider is handed to the resolver.
func (p *ConfigNodeFeatureProvider) WithReader(key string, fn func(*repo.Config) bool) *ConfigNodeFeatureProvider {
	if p == nil || fn == nil || key == "" {
		return p
	}
	p.readers[key] = fn
	return p
}

// IsEnabled returns the node-layer value. Missing bindings return true
// (no node-layer constraint) to keep the resolver AND-merge consistent.
func (p *ConfigNodeFeatureProvider) IsEnabled(ctx context.Context, key string) bool {
	if p == nil {
		return true
	}
	fn, ok := p.readers[key]
	if !ok {
		return true
	}
	return fn(p.cfg)
}

func (p *ConfigNodeFeatureProvider) registerDefaults() {
	p.readers[config.FeatureGuestCheckoutEnabled.Key] = func(c *repo.Config) bool {
		if c == nil {
			return false
		}
		return c.GuestCheckout
	}
}

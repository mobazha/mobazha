package fulfillment

import (
	"context"
	"fmt"
	"sync"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// ProviderFactory creates a FulfillmentProvider from encrypted credentials.
// Injected by the AppService so the registry can rebuild from DB without
// depending on internal packages.
type ProviderFactory func(providerID string, credentials []byte) (contracts.FulfillmentProvider, error)

// registry implements contracts.FulfillmentProviderRegistry with thread-safe access.
type registry struct {
	mu        sync.RWMutex
	providers map[string]contracts.FulfillmentProvider

	// rebuildFn is called by RebuildFromDB to restore providers from persistent storage.
	// Set via SetRebuildFunc; nil means RebuildFromDB is a no-op.
	rebuildFn func(ctx context.Context) error
}

// NewRegistry creates an empty FulfillmentProviderRegistry.
func NewRegistry() contracts.FulfillmentProviderRegistry {
	return &registry{
		providers: make(map[string]contracts.FulfillmentProvider),
	}
}

// SetRebuildFunc injects the DB-aware rebuild function.
// Called by SupplyChainAppService during initialization.
func SetRebuildFunc(r contracts.FulfillmentProviderRegistry, fn func(ctx context.Context) error) {
	if reg, ok := r.(*registry); ok {
		reg.rebuildFn = fn
	}
}

func (r *registry) Register(provider contracts.FulfillmentProvider) error {
	if provider == nil {
		return fmt.Errorf("fulfillment: cannot register nil provider")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.ProviderID()] = provider
	return nil
}

func (r *registry) ForProvider(providerID string) (contracts.FulfillmentProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[providerID]
	if !ok {
		return nil, contracts.ErrFulfillmentProviderNotFound
	}
	return p, nil
}

func (r *registry) ListProviders() []contracts.FulfillmentProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	providers := make([]contracts.FulfillmentProvider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	return providers
}

func (r *registry) Unregister(providerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, providerID)
}

func (r *registry) RebuildFromDB(ctx context.Context) error {
	if r.rebuildFn == nil {
		return nil
	}
	return r.rebuildFn(ctx)
}

package fiat

import (
	"fmt"
	"sync"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// registry implements contracts.FiatProviderRegistry with thread-safe access.
type registry struct {
	mu        sync.RWMutex
	providers map[string]contracts.FiatPaymentProvider
}

// NewRegistry creates an empty FiatProviderRegistry.
func NewRegistry() contracts.FiatProviderRegistry {
	return &registry{
		providers: make(map[string]contracts.FiatPaymentProvider),
	}
}

func (r *registry) Register(provider contracts.FiatPaymentProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.ProviderID()] = provider
}

func (r *registry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, id)
}

func (r *registry) ForProvider(id string) (contracts.FiatPaymentProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	if !ok {
		return nil, fmt.Errorf("fiat provider %q not registered", id)
	}
	return p, nil
}

func (r *registry) Registered() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}

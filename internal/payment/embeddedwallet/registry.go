// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Package embeddedwallet holds the reviewed embedded-wallet provider modules
// (RFC-0012) and a registry that composes them. Concrete providers live in
// child packages (mock, privy, cdp); the registry is the composition seam a
// distribution profile wires (RFC-0006 trusted-module composition).
package embeddedwallet

import (
	"sync"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// registry implements contracts.EmbeddedWalletProviderRegistry with
// thread-safe access.
type registry struct {
	mu        sync.RWMutex
	providers map[string]contracts.EmbeddedWalletProvider
}

// NewRegistry creates an empty EmbeddedWalletProviderRegistry.
func NewRegistry() contracts.EmbeddedWalletProviderRegistry {
	return &registry{
		providers: make(map[string]contracts.EmbeddedWalletProvider),
	}
}

func (r *registry) Register(provider contracts.EmbeddedWalletProvider) {
	if provider == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.ProviderID()] = provider
}

func (r *registry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, id)
}

func (r *registry) ForProvider(id string) (contracts.EmbeddedWalletProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	if !ok {
		return nil, contracts.ErrEmbeddedWalletProviderNotFound
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

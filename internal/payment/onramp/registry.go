// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Package onramp holds the reviewed onramp provider modules (RFC-0012
// Proposal 5) and a registry that composes them. Concrete providers require a
// vendor KYB relationship, so only the mock lives here; a distribution profile
// wires the rest through the registry (RFC-0006 trusted-module composition).
package onramp

import (
	"sync"

	"github.com/mobazha/mobazha/pkg/contracts"
)

type registry struct {
	mu        sync.RWMutex
	providers map[string]contracts.OnrampProvider
}

// NewRegistry creates an empty OnrampProviderRegistry.
func NewRegistry() contracts.OnrampProviderRegistry {
	return &registry{providers: make(map[string]contracts.OnrampProvider)}
}

func (r *registry) Register(provider contracts.OnrampProvider) {
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

func (r *registry) ForProvider(id string) (contracts.OnrampProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	if !ok {
		return nil, contracts.ErrOnrampProviderNotFound
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

// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package embeddedwallet

import (
	"errors"
	"testing"

	"github.com/mobazha/mobazha/internal/payment/embeddedwallet/mock"
	"github.com/mobazha/mobazha/pkg/contracts"
)

func TestRegistryComposition(t *testing.T) {
	reg := NewRegistry()

	if _, err := reg.ForProvider(mock.ProviderID); !errors.Is(err, contracts.ErrEmbeddedWalletProviderNotFound) {
		t.Fatalf("expected not-found before registration, got %v", err)
	}

	reg.Register(mock.New())
	if got := reg.Registered(); len(got) != 1 || got[0] != mock.ProviderID {
		t.Fatalf("unexpected registered set: %v", got)
	}

	p, err := reg.ForProvider(mock.ProviderID)
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if p.ProviderID() != mock.ProviderID {
		t.Fatalf("wrong provider: %s", p.ProviderID())
	}

	reg.Unregister(mock.ProviderID)
	if _, err := reg.ForProvider(mock.ProviderID); !errors.Is(err, contracts.ErrEmbeddedWalletProviderNotFound) {
		t.Fatalf("expected not-found after unregister, got %v", err)
	}

	// Register(nil) must be a no-op, not a panic or a nil entry.
	reg.Register(nil)
	if len(reg.Registered()) != 0 {
		t.Fatalf("nil provider must not be registered")
	}
}

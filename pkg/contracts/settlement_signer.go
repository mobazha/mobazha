// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import (
	"context"
	"fmt"
	"strings"
)

// SettlementKeyRef is an opaque reference to a settlement-domain key. TenantID
// is retained for authorization, routing, tenant-specific root selection, and
// audit; it is not part of the final KDF or signature digest. The reference
// deliberately contains no derivation path, public parent key, or private key
// material.
type SettlementKeyRef struct {
	TenantID    string
	RailID      string
	Purpose     string
	ReferenceID string
}

// Validate rejects references that are not tenant, rail, purpose, and
// order/Safe scoped.
func (r SettlementKeyRef) Validate() error {
	if strings.TrimSpace(r.TenantID) == "" || strings.TrimSpace(r.RailID) == "" ||
		strings.TrimSpace(r.Purpose) == "" || strings.TrimSpace(r.ReferenceID) == "" {
		return fmt.Errorf("settlement key reference requires tenant, rail, purpose, and reference")
	}
	return nil
}

// SettlementSignRequest binds a canonical transaction plan or payload to an
// explicit anti-replay domain. Adapters decide the chain-specific signature
// encoding without returning key material to Core.
type SettlementSignRequest struct {
	KeyRef  SettlementKeyRef
	Domain  string
	Payload []byte
}

// Validate rejects ambiguous or empty signing requests.
func (r SettlementSignRequest) Validate() error {
	if err := r.KeyRef.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.Domain) == "" || len(r.Payload) == 0 {
		return fmt.Errorf("settlement signing requires domain and payload")
	}
	return nil
}

// SettlementSigner exposes settlement-domain public keys and signing through
// opaque references. Implementations may use a local encrypted keystore,
// tenant-scoped Vault, or HSM.
type SettlementSigner interface {
	PublicKey(ctx context.Context, keyRef SettlementKeyRef) ([]byte, error)
	Sign(ctx context.Context, request SettlementSignRequest) ([]byte, error)
}

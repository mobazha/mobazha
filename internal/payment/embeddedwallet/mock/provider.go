// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Package mock is an in-process EmbeddedWalletProvider for tests and local
// end-to-end flows. It produces real EIP-712 signatures from a deterministic
// per-buyer key so downstream code (payment session, escrow verification) can
// be exercised without contacting a third-party provider. It is NOT an
// admitted production module: keys live in-process and there is no independent
// buyer custody, authentication, or recovery path.
package mock

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/mobazha/mobazha/pkg/contracts"
)

// ProviderID is the mock module identifier.
const ProviderID = "mock-embedded"

// Provider is an in-process EmbeddedWalletProvider. The zero value is not
// usable; construct with New.
type Provider struct {
	// caps is the fail-closed capability surface keyed by rail id. A rail
	// absent from the map returns a zero-value (all-closed) declaration.
	caps map[string]contracts.EmbeddedWalletCapabilities
}

// Option configures a Provider.
type Option func(*Provider)

// WithRailCapabilities declares the mock's capability surface for a rail. Use
// it in tests to open exactly the rails/actions under test.
func WithRailCapabilities(caps contracts.EmbeddedWalletCapabilities) Option {
	return func(p *Provider) {
		p.caps[caps.RailID] = caps
	}
}

// New returns a mock provider. By default every rail is fail-closed; open rails
// with WithRailCapabilities.
func New(opts ...Option) *Provider {
	p := &Provider{caps: make(map[string]contracts.EmbeddedWalletCapabilities)}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// FullyOpenRail is a convenience capability declaration that opens every
// settlement action plus export/recovery and onramp for a rail. It exists for
// tests; a real module earns each flag by proving it end to end.
func FullyOpenRail(railID string) contracts.EmbeddedWalletCapabilities {
	actions := make(map[contracts.SettlementAction]bool)
	for _, a := range contracts.AllSettlementActions() {
		actions[a] = true
	}
	return contracts.EmbeddedWalletCapabilities{
		RailID:         railID,
		Actions:        actions,
		ExportRecovery: true,
		OnrampFunding:  true,
	}
}

// ProviderID implements contracts.EmbeddedWalletProvider.
func (p *Provider) ProviderID() string { return ProviderID }

// Capabilities returns the fail-closed capability surface for a rail.
func (p *Provider) Capabilities(_ context.Context, railID string) (contracts.EmbeddedWalletCapabilities, error) {
	if caps, ok := p.caps[railID]; ok {
		return caps, nil
	}
	return contracts.EmbeddedWalletCapabilities{RailID: railID}, nil
}

// EnsureWallet returns a deterministic wallet for the buyer+rail. The address
// is derived from a deterministic key so repeated calls are stable and the
// same buyer signs consistently across a test.
func (p *Provider) EnsureWallet(_ context.Context, req contracts.EnsureWalletRequest) (contracts.EmbeddedWallet, error) {
	if err := req.Validate(); err != nil {
		return contracts.EmbeddedWallet{}, err
	}
	key := deriveKey(req.Buyer.Subject, req.RailID)
	addr := crypto.PubkeyToAddress(key.PublicKey)
	return contracts.EmbeddedWallet{
		ProviderID: ProviderID,
		// The wallet id carries the subject so SignTypedData can re-derive the
		// same key; this is a mock-only shortcut, not a custody model.
		WalletID:    encodeWalletID(req.RailID, req.Buyer.Subject),
		Address:     addr.Hex(),
		RailID:      req.RailID,
		ChainFamily: contracts.ChainFamilyEVM,
	}, nil
}

// SignTypedData validates the request (structured payload + buyer
// authorization) and returns a real EIP-712 signature from the buyer's
// deterministic key. Contract-level guards (non-structured payload, missing
// authorization) are enforced by req.Validate before any signing.
func (p *Provider) SignTypedData(_ context.Context, req contracts.EmbeddedWalletSignRequest) (contracts.EmbeddedWalletSignature, error) {
	if err := req.Validate(); err != nil {
		return contracts.EmbeddedWalletSignature{}, err
	}
	if req.Wallet.ChainFamily != contracts.ChainFamilyEVM {
		return contracts.EmbeddedWalletSignature{}, fmt.Errorf("mock: unsupported chain family %q", req.Wallet.ChainFamily)
	}

	var typedData apitypes.TypedData
	if err := json.Unmarshal(req.Payload.Document, &typedData); err != nil {
		return contracts.EmbeddedWalletSignature{}, fmt.Errorf("%w: %v", contracts.ErrEmbeddedWalletUnsupportedSigning, err)
	}
	digest, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return contracts.EmbeddedWalletSignature{}, fmt.Errorf("mock: eip-712 hashing failed: %w", err)
	}

	// The wallet's subject/rail are encoded in its id; re-derive the same key.
	rail, subject, ok := decodeWalletID(req.Wallet.WalletID)
	if !ok || rail != req.Wallet.RailID {
		return contracts.EmbeddedWalletSignature{}, fmt.Errorf("mock: wallet id %q is not a mock wallet for rail %q", req.Wallet.WalletID, req.Wallet.RailID)
	}
	key := deriveKey(subject, req.Wallet.RailID)
	sig, err := crypto.Sign(digest, key)
	if err != nil {
		return contracts.EmbeddedWalletSignature{}, fmt.Errorf("mock: signing failed: %w", err)
	}
	// go-ethereum returns V in {0,1}; EOA ecrecover (and Safe checkSignatures)
	// expects the {27,28} convention.
	if len(sig) == 65 {
		sig[64] += 27
	}
	return contracts.EmbeddedWalletSignature{
		Signer:    req.Wallet.Address,
		Signature: sig,
	}, nil
}

// encodeWalletID / decodeWalletID carry the rail and buyer subject inside the
// opaque wallet id. Mock-only: a real provider's id is a provider-side handle,
// not a reversible encoding of the buyer identity.
func encodeWalletID(railID, subject string) string {
	return "mock|" + railID + "|" + subject
}

func decodeWalletID(id string) (railID, subject string, ok bool) {
	parts := strings.SplitN(id, "|", 3)
	if len(parts) != 3 || parts[0] != "mock" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// deriveKey deterministically derives a secp256k1 key from a buyer subject and
// rail. Test-only: it makes the mock's addresses and signatures reproducible.
func deriveKey(subject, railID string) *ecdsa.PrivateKey {
	seed := crypto.Keccak256([]byte("mobazha-mock-embedded-wallet|" + subject + "|" + railID))
	// Keccak output is 32 bytes and, with overwhelming probability, a valid
	// secp256k1 scalar; ToECDSA validates the range.
	key, err := crypto.ToECDSA(seed)
	if err != nil {
		// Astronomically unlikely; perturb deterministically and retry once.
		seed = crypto.Keccak256(seed)
		key, err = crypto.ToECDSA(seed)
		if err != nil {
			panic("mock embedded wallet: unable to derive key: " + err.Error())
		}
	}
	return key
}

var _ contracts.EmbeddedWalletProvider = (*Provider)(nil)

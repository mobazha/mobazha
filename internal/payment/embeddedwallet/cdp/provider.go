// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Package cdp is the Coinbase CDP embedded-wallet provider module (RFC-0012),
// present as a skeleton. CDP is the first-priority cost-model candidate
// (per-operation billing, no MAU tiers) in the CDP/Privy dual track; this
// package fixes its place behind the contract so the composition seam and the
// dual-track posture exist before the live integration lands.
//
// It deliberately implements every method fail-closed: Capabilities advertises
// nothing, and EnsureWallet/SignTypedData return ErrNotImplemented. Wiring the
// live client (CDP Wallet API v2: create wallet, sign EIP-712) is a follow-up
// that must satisfy the same RFC-0012 custody boundary as the Privy adapter —
// no standing platform signer for real funds, buyer export/recovery path.
package cdp

import (
	"context"
	"errors"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// ProviderID is the Coinbase CDP module identifier.
const ProviderID = "coinbase-cdp"

// ErrNotImplemented marks the not-yet-wired CDP live integration.
var ErrNotImplemented = errors.New("cdp: embedded-wallet integration is not implemented yet")

// Config configures the CDP provider. Fields are placeholders for the live
// client (API key id/secret, base URL) plus the fail-closed capability surface.
type Config struct {
	APIKeyID     string
	APIKeySecret string
	BaseURL      string

	// Capabilities is the fail-closed capability surface keyed by rail id.
	// Empty until CDP's capability gate (RFC-0012 Proposal 6) closes for a rail.
	Capabilities map[string]contracts.EmbeddedWalletCapabilities
}

// Provider is the CDP EmbeddedWalletProvider skeleton.
type Provider struct {
	caps map[string]contracts.EmbeddedWalletCapabilities
}

// New builds the CDP provider skeleton. It never errors; a real integration
// will validate credentials here.
func New(cfg Config) *Provider {
	caps := make(map[string]contracts.EmbeddedWalletCapabilities, len(cfg.Capabilities))
	for rail, c := range cfg.Capabilities {
		caps[rail] = c
	}
	return &Provider{caps: caps}
}

// ProviderID implements contracts.EmbeddedWalletProvider.
func (p *Provider) ProviderID() string { return ProviderID }

// Capabilities returns the fail-closed capability surface for a rail.
func (p *Provider) Capabilities(_ context.Context, railID string) (contracts.EmbeddedWalletCapabilities, error) {
	if c, ok := p.caps[railID]; ok {
		return c, nil
	}
	return contracts.EmbeddedWalletCapabilities{RailID: railID}, nil
}

// EnsureWallet is not implemented in the skeleton.
func (p *Provider) EnsureWallet(_ context.Context, req contracts.EnsureWalletRequest) (contracts.EmbeddedWallet, error) {
	if err := req.Validate(); err != nil {
		return contracts.EmbeddedWallet{}, err
	}
	return contracts.EmbeddedWallet{}, ErrNotImplemented
}

// SignTypedData enforces the contract guards, then reports not-implemented. The
// guards run first so the skeleton still refuses a non-structured payload or a
// missing buyer authorization exactly like a live module.
func (p *Provider) SignTypedData(_ context.Context, req contracts.EmbeddedWalletSignRequest) (contracts.EmbeddedWalletSignature, error) {
	if err := req.Validate(); err != nil {
		return contracts.EmbeddedWalletSignature{}, err
	}
	return contracts.EmbeddedWalletSignature{}, ErrNotImplemented
}

var _ contracts.EmbeddedWalletProvider = (*Provider)(nil)

// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Package mock is an in-process OnrampProvider for tests and local end-to-end
// flows. It records purchases idempotently (so leave-and-resume can be
// exercised) and lets a test drive the status progression deterministically. It
// is NOT an admitted production module.
package mock

import (
	"context"
	"fmt"
	"sync"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// ProviderID is the mock onramp module identifier.
const ProviderID = "mock-onramp"

// Provider is an in-process OnrampProvider.
type Provider struct {
	mu    sync.Mutex
	caps  map[string]contracts.OnrampCapabilities
	byKey map[string]string                   // (attempt|idempotencyKey) -> onrampOrderID
	byID  map[string]*contracts.OnrampPurchase // onrampOrderID -> purchase
	seq   int
}

// Option configures the mock.
type Option func(*Provider)

// WithRailCapabilities opens a rail for onramp in tests.
func WithRailCapabilities(caps contracts.OnrampCapabilities) Option {
	return func(p *Provider) { p.caps[caps.RailID] = caps }
}

// New returns a mock onramp provider. All rails are fail-closed by default.
func New(opts ...Option) *Provider {
	p := &Provider{
		caps:  make(map[string]contracts.OnrampCapabilities),
		byKey: make(map[string]string),
		byID:  make(map[string]*contracts.OnrampPurchase),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// OpenRail is a convenience capability declaration that offers a rail with
// direct-to-target delivery for the given fiat currencies.
func OpenRail(railID string, fiat ...string) contracts.OnrampCapabilities {
	return contracts.OnrampCapabilities{
		RailID:          railID,
		Offerable:       true,
		DeliverToTarget: true,
		FiatCurrencies:  fiat,
	}
}

// ProviderID implements contracts.OnrampProvider.
func (p *Provider) ProviderID() string { return ProviderID }

// Capabilities returns the fail-closed capability surface for a rail.
func (p *Provider) Capabilities(_ context.Context, railID string) (contracts.OnrampCapabilities, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.caps[railID]; ok {
		return c, nil
	}
	return contracts.OnrampCapabilities{RailID: railID}, nil
}

// Quote returns a deterministic fiat cost: settlement amount plus a flat 4%
// provider fee, echoed as the disclosure line.
func (p *Provider) Quote(_ context.Context, req contracts.OnrampQuoteRequest) (contracts.OnrampQuote, error) {
	if err := req.Validate(); err != nil {
		return contracts.OnrampQuote{}, err
	}
	// A deterministic, illustrative fee; a real provider returns its own quote.
	fee := "4.00"
	return contracts.OnrampQuote{
		ProviderID:       ProviderID,
		FiatCurrency:     req.FiatCurrency,
		FiatAmount:       req.SettlementAmount, // 1:1 illustrative; not a real rate
		ProviderFee:      fee,
		SettlementAsset:  req.SettlementAsset,
		SettlementAmount: req.SettlementAmount,
		Disclosure:       "You are buying crypto from mock-onramp; its fees, KYC, and reversals are between you and the provider.",
	}, nil
}

// InitiatePurchase is idempotent on (AttemptID, IdempotencyKey): a repeated call
// returns the existing purchase instead of a second onramp order.
func (p *Provider) InitiatePurchase(_ context.Context, req contracts.OnrampPurchaseRequest) (contracts.OnrampPurchase, error) {
	if err := req.Validate(); err != nil {
		return contracts.OnrampPurchase{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	key := req.AttemptID + "|" + req.IdempotencyKey
	if id, ok := p.byKey[key]; ok {
		return *p.byID[id], nil
	}

	p.seq++
	orderID := fmt.Sprintf("mock-onramp-%d", p.seq)
	purchase := &contracts.OnrampPurchase{
		ProviderID:           ProviderID,
		OnrampOrderID:        orderID,
		Status:               contracts.OnrampStatusAwaitingPayment,
		BuyerActionURL:       "https://mock-onramp.example/checkout/" + orderID,
		DeliveryTarget:       req.DeliveryTarget,
		DeliverToBuyerWallet: req.DeliverToBuyerWallet,
		BuyerWalletAddress:   req.BuyerWalletAddress,
		Disclosure:           "You are buying crypto from mock-onramp; its fees, KYC, and reversals are between you and the provider.",
	}
	p.byKey[key] = orderID
	p.byID[orderID] = purchase
	return *purchase, nil
}

// PurchaseStatus returns the current purchase.
func (p *Provider) PurchaseStatus(_ context.Context, onrampOrderID string) (contracts.OnrampPurchase, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	purchase, ok := p.byID[onrampOrderID]
	if !ok {
		return contracts.OnrampPurchase{}, fmt.Errorf("onramp: unknown order %q", onrampOrderID)
	}
	return *purchase, nil
}

// SetStatus is a test hook to drive the purchase lifecycle deterministically.
func (p *Provider) SetStatus(onrampOrderID string, status contracts.OnrampStatus) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	purchase, ok := p.byID[onrampOrderID]
	if !ok {
		return fmt.Errorf("onramp: unknown order %q", onrampOrderID)
	}
	purchase.Status = status
	return nil
}

var _ contracts.OnrampProvider = (*Provider)(nil)

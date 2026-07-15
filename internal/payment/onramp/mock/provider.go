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
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// ProviderID is the mock onramp module identifier.
const ProviderID = "mock-onramp"

// autoAdvanceSequence is the happy-path lifecycle the mock walks when auto-
// advance is enabled: one step per configured interval. It deliberately stops
// at delivered — the mock never asserts anything about the chain, and the
// session only reaches funded on a real on-chain observation.
var autoAdvanceSequence = []contracts.OnrampStatus{
	contracts.OnrampStatusAwaitingPayment,
	contracts.OnrampStatusProcessing,
	contracts.OnrampStatusDelivering,
	contracts.OnrampStatusDelivered,
}

// purchase is a mock purchase plus the bookkeeping auto-advance needs.
type purchase struct {
	rec       contracts.OnrampPurchase
	createdAt time.Time
	// pinned is set once SetStatus drives the purchase off the happy path (or
	// a test drives it explicitly); auto-advance never overrides a pinned
	// purchase, so terminal states like failed/reversed stick.
	pinned bool
}

// Provider is an in-process OnrampProvider.
type Provider struct {
	mu    sync.Mutex
	caps  map[string]contracts.OnrampCapabilities
	byKey map[string]string    // (attempt|idempotencyKey) -> onrampOrderID
	byID  map[string]*purchase // onrampOrderID -> purchase
	seq   int

	// autoAdvance is the per-step interval for the happy-path lifecycle. Zero
	// (the default) disables it entirely, leaving SetStatus as the only driver
	// — that is the behavior every existing test relies on.
	autoAdvance time.Duration
	now         func() time.Time
}

// Option configures the mock.
type Option func(*Provider)

// WithRailCapabilities opens a rail for onramp in tests.
func WithRailCapabilities(caps contracts.OnrampCapabilities) Option {
	return func(p *Provider) { p.caps[caps.RailID] = caps }
}

// WithAutoAdvance makes a purchase walk awaiting_payment -> processing ->
// delivering -> delivered on its own, one step per interval, so a local
// end-to-end demo can be driven from a browser instead of from an in-process
// Go harness. A non-positive interval disables it.
func WithAutoAdvance(step time.Duration) Option {
	return func(p *Provider) {
		if step > 0 {
			p.autoAdvance = step
		}
	}
}

// withClock injects a deterministic clock for tests.
func withClock(now func() time.Time) Option {
	return func(p *Provider) {
		if now != nil {
			p.now = now
		}
	}
}

// New returns a mock onramp provider. All rails are fail-closed by default and
// auto-advance is off, so New() alone behaves exactly as a hand-driven mock.
func New(opts ...Option) *Provider {
	p := &Provider{
		caps:  make(map[string]contracts.OnrampCapabilities),
		byKey: make(map[string]string),
		byID:  make(map[string]*purchase),
		now:   time.Now,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// advanced returns the purchase record with the auto-advance schedule applied.
// Callers must hold p.mu. The computed status is persisted so progression is
// monotonic and observable through SetStatus.
func (p *Provider) advanced(item *purchase) contracts.OnrampPurchase {
	if p.autoAdvance <= 0 || item.pinned {
		return item.rec
	}
	steps := int(p.now().Sub(item.createdAt) / p.autoAdvance)
	if steps < 0 {
		steps = 0
	}
	if steps >= len(autoAdvanceSequence) {
		steps = len(autoAdvanceSequence) - 1
	}
	item.rec.Status = autoAdvanceSequence[steps]
	return item.rec
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
		return p.advanced(p.byID[id]), nil
	}

	p.seq++
	orderID := fmt.Sprintf("mock-onramp-%d", p.seq)
	item := &purchase{
		rec: contracts.OnrampPurchase{
			ProviderID:           ProviderID,
			OnrampOrderID:        orderID,
			Status:               contracts.OnrampStatusAwaitingPayment,
			BuyerActionURL:       "https://mock-onramp.example/checkout/" + orderID,
			DeliveryTarget:       req.DeliveryTarget,
			DeliverToBuyerWallet: req.DeliverToBuyerWallet,
			BuyerWalletAddress:   req.BuyerWalletAddress,
			Disclosure:           "You are buying crypto from mock-onramp; its fees, KYC, and reversals are between you and the provider.",
		},
		createdAt: p.now(),
	}
	p.byKey[key] = orderID
	p.byID[orderID] = item
	return item.rec, nil
}

// PurchaseStatus returns the current purchase, applying the auto-advance
// schedule when one is configured.
func (p *Provider) PurchaseStatus(_ context.Context, onrampOrderID string) (contracts.OnrampPurchase, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	item, ok := p.byID[onrampOrderID]
	if !ok {
		return contracts.OnrampPurchase{}, fmt.Errorf("onramp: unknown order %q", onrampOrderID)
	}
	return p.advanced(item), nil
}

// SetStatus drives the purchase lifecycle explicitly. It is the only driver
// when auto-advance is off, and it pins the purchase (disabling auto-advance
// for it) when on, so a caller can still force failed/reversed.
func (p *Provider) SetStatus(onrampOrderID string, status contracts.OnrampStatus) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	item, ok := p.byID[onrampOrderID]
	if !ok {
		return fmt.Errorf("onramp: unknown order %q", onrampOrderID)
	}
	item.rec.Status = status
	item.pinned = true
	return nil
}

var _ contracts.OnrampProvider = (*Provider)(nil)

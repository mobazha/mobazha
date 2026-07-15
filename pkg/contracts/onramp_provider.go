// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/payment"
)

// OnrampProvider (RFC-0012 Proposal 5) funds a payment attempt by letting the
// buyer acquire the already-frozen settlement asset through a fiat purchase
// inside checkout, rather than holding the asset beforehand. It is the fourth
// payment provider abstraction alongside FiatPaymentProvider (PSP fiat
// settlement), EmbeddedWalletProvider (buyer signing key), and SwapProvider
// (on-chain asset conversion).
//
// Architectural invariants this contract encodes:
//   - Onramp is a funding SOURCE, not a settlement mode. Settlement stays on
//     chain against the frozen FundingTarget; an onramp purchase is an ordinary
//     funding observation once the asset arrives (RFC-0012 Proposal 5).
//   - The settlement asset, network, and amount are fixed by the frozen terms
//     (RFC-0009) before onramp is offered; an onramp quote prices the fiat cost
//     of acquiring that fixed amount and does NOT create a second, competing
//     settlement quote.
//   - The onramp leg is a distinct commercial relationship between the buyer
//     and the provider (KYC, fee, reversal); it is disclosed as such and is not
//     represented as part of Mobazha's or the seller's payment terms.

// Sentinel errors for onramp integrations.
var (
	ErrOnrampProviderNotFound  = errors.New("onramp: provider not registered")
	ErrOnrampCapabilityClosed  = errors.New("onramp: capability gate is closed for this rail")
	ErrOnrampTermsNotFrozen    = errors.New("onramp: settlement asset/network/amount must be frozen before quoting")
	ErrOnrampDeliveryUnbound   = errors.New("onramp: a purchase must bind a delivery target or the buyer wallet")
	ErrOnrampMissingIdemponent = errors.New("onramp: initiate requires an idempotency key for leave-and-resume safety")
)

// OnrampStatus is the provider-neutral lifecycle of one onramp purchase. It is
// entirely pre-settlement: the terminal on-chain truth remains the funding
// observation at the frozen target, never an onramp status.
type OnrampStatus string

const (
	// OnrampStatusCreated: purchase intent registered, buyer not yet paying.
	OnrampStatusCreated OnrampStatus = "created"
	// OnrampStatusAwaitingPayment: buyer must complete the fiat payment step.
	OnrampStatusAwaitingPayment OnrampStatus = "awaiting_payment"
	// OnrampStatusProcessing: fiat captured; provider converting to the asset.
	OnrampStatusProcessing OnrampStatus = "processing"
	// OnrampStatusDelivering: asset purchased; delivery to the target (or to the
	// buyer wallet, to be forwarded) is in flight.
	OnrampStatusDelivering OnrampStatus = "delivering"
	// OnrampStatusDelivered: provider reports delivery done. This is provider
	// self-report only; funded/verified still require the on-chain observation.
	OnrampStatusDelivered OnrampStatus = "delivered"
	// OnrampStatusFailed: purchase failed before delivery.
	OnrampStatusFailed OnrampStatus = "failed"
	// OnrampStatusReversed: a completed purchase was later reversed by the
	// provider (chargeback/refund on the fiat leg).
	OnrampStatusReversed OnrampStatus = "reversed"
)

// Active reports whether the purchase is still progressing toward delivery.
func (s OnrampStatus) Active() bool {
	switch s {
	case OnrampStatusCreated, OnrampStatusAwaitingPayment, OnrampStatusProcessing, OnrampStatusDelivering:
		return true
	default:
		return false
	}
}

// OnrampCapabilities declares, for one rail, whether buyer-visible onramp
// funding is proven end to end (delivery observed, confirmed, restart-
// recoverable per RFC-0012 Proposal 6.3). Zero value is fail-closed.
type OnrampCapabilities struct {
	RailID string
	// Offerable reports the rail may be shown to buyers as onramp-fundable.
	Offerable bool
	// DeliverToTarget reports the provider can deliver directly to an arbitrary
	// funding-target address. When false, delivery must go to the buyer wallet
	// first and be forwarded.
	DeliverToTarget bool
	// FiatCurrencies the provider accepts for this rail (advisory).
	FiatCurrencies []string
}

// OnrampQuoteRequest prices the fiat cost of acquiring the frozen settlement
// amount. The settlement side is fixed by RFC-0009 terms and must not be
// re-negotiated here.
type OnrampQuoteRequest struct {
	Buyer            BuyerRef
	RailID           string
	SettlementAsset  string // canonical asset id of the frozen settlement asset
	SettlementAmount string // frozen amount, human-readable decimal
	FiatCurrency     string
}

// Validate rejects a quote request whose settlement side is not frozen.
func (r OnrampQuoteRequest) Validate() error {
	if strings.TrimSpace(r.RailID) == "" || strings.TrimSpace(r.SettlementAsset) == "" ||
		strings.TrimSpace(r.SettlementAmount) == "" {
		return ErrOnrampTermsNotFrozen
	}
	if strings.TrimSpace(r.FiatCurrency) == "" {
		return fmt.Errorf("onramp: quote requires a fiat currency")
	}
	return nil
}

// OnrampQuote is the fiat cost of the fixed settlement amount plus disclosure.
type OnrampQuote struct {
	ProviderID       string
	FiatCurrency     string
	FiatAmount       string // total fiat the buyer pays, human-readable decimal
	ProviderFee      string // provider fee portion, human-readable decimal
	SettlementAsset  string
	SettlementAmount string
	ExpiresAt        int64  // unix seconds; advisory quote validity
	Disclosure       string // buyer-facing disclosure of the buyer<->provider relationship
}

// OnrampPurchaseRequest initiates a purchase. Exactly one delivery mode must be
// chosen: direct to the frozen funding target, or to the buyer's embedded
// wallet for later forwarding. IdempotencyKey makes initiate safe to retry so a
// buyer who leaves and returns does not create a second onramp order.
type OnrampPurchaseRequest struct {
	Buyer            BuyerRef
	OrderID          string
	AttemptID        string
	RailID           string
	SettlementAsset  string
	SettlementAmount string
	FiatCurrency     string

	// DeliveryTarget is the frozen on-chain funding target. Required unless
	// DeliverToBuyerWallet is true.
	DeliveryTarget string
	// DeliverToBuyerWallet routes delivery to the buyer's embedded wallet first,
	// to be forwarded to the target (RFC-0012 Proposal 5). BuyerWalletAddress
	// must be set when true.
	DeliverToBuyerWallet bool
	BuyerWalletAddress   string

	IdempotencyKey string
}

// Validate enforces frozen terms, a bound delivery mode, and an idempotency key.
func (r OnrampPurchaseRequest) Validate() error {
	if strings.TrimSpace(r.OrderID) == "" || strings.TrimSpace(r.AttemptID) == "" ||
		strings.TrimSpace(r.RailID) == "" || strings.TrimSpace(r.SettlementAsset) == "" ||
		strings.TrimSpace(r.SettlementAmount) == "" {
		return ErrOnrampTermsNotFrozen
	}
	if strings.TrimSpace(r.IdempotencyKey) == "" {
		return ErrOnrampMissingIdemponent
	}
	if r.DeliverToBuyerWallet {
		if strings.TrimSpace(r.BuyerWalletAddress) == "" {
			return ErrOnrampDeliveryUnbound
		}
	} else if strings.TrimSpace(r.DeliveryTarget) == "" {
		return ErrOnrampDeliveryUnbound
	}
	return nil
}

// OnrampPurchase is the durable handle to one onramp order. OnrampOrderID is the
// provider-side identifier used for status polling and idempotent resume.
type OnrampPurchase struct {
	ProviderID           string
	OnrampOrderID        string
	Status               OnrampStatus
	BuyerActionURL       string // where the buyer completes the fiat step, if any
	DeliveryTarget       string
	DeliverToBuyerWallet bool
	BuyerWalletAddress   string
	Disclosure           string
}

// OnrampProvider is the Core-facing contract a reviewed onramp module
// implements. Capabilities are fail-closed per rail; a chain becomes
// buyer-visible only when its gate closes (RFC-0012 Proposal 6).
type OnrampProvider interface {
	ProviderID() string
	Capabilities(ctx context.Context, railID string) (OnrampCapabilities, error)
	Quote(ctx context.Context, req OnrampQuoteRequest) (OnrampQuote, error)
	// InitiatePurchase is idempotent on (AttemptID, IdempotencyKey): a repeated
	// call for the same attempt returns the existing purchase, never a second
	// onramp order.
	InitiatePurchase(ctx context.Context, req OnrampPurchaseRequest) (OnrampPurchase, error)
	PurchaseStatus(ctx context.Context, onrampOrderID string) (OnrampPurchase, error)
}

// OnrampProviderRegistry composes reviewed onramp modules. A distribution may
// register zero, one, or more; no chain or client may assume a specific vendor.
type OnrampProviderRegistry interface {
	Register(provider OnrampProvider)
	Unregister(id string)
	ForProvider(id string) (OnrampProvider, error)
	Registered() []string
}

// OnrampFundingInitiation is the client-facing request to fund an order's
// current payable attempt through an onramp purchase. The buyer subject comes
// from the authenticated request context, never from the client body.
type OnrampFundingInitiation struct {
	Buyer        BuyerRef
	ProviderID   string
	FiatCurrency string
	// IdempotencyKey defaults to "primary": leave-and-resume returns the same
	// purchase; a retry after a terminal failure supplies a new key.
	IdempotencyKey string
	// DeliverToBuyerWallet routes delivery to the buyer's embedded wallet for a
	// later forwarding step instead of directly to the frozen funding target.
	DeliverToBuyerWallet bool
	BuyerWalletAddress   string
}

// OnrampFundingService is the node-facing orchestration surface for
// onramp-funded attempts (ADR-019). Both methods are order-oriented: the
// implementation resolves the order's current payable attempt and enforces the
// frozen-terms gate. Returns are the same projection view the payment session
// exposes as onrampFunding; a nil view from Refresh means nothing is in flight.
type OnrampFundingService interface {
	InitiateOrResumeForOrder(ctx context.Context, orderID string, req OnrampFundingInitiation) (*payment.OnrampFundingSourceView, error)
	RefreshForOrder(ctx context.Context, orderID string) (*payment.OnrampFundingSourceView, error)
	// ListProvidersForOrder enumerates the registered providers whose
	// capability gate is open for the order's frozen settlement rail
	// (RFC-0012 Proposal 6). Empty means the affordance must not render.
	ListProvidersForOrder(ctx context.Context, orderID string) ([]payment.OnrampProviderOption, error)
}

// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package payment

import (
	"math/big"
	"strings"
	"time"
)

// Onramp-funded attempts (RFC-0012 Proposal 5) surface as fine-grained
// FundingState progress, NOT as new top-level SessionStatus values. An onramp
// purchase is a funding SOURCE feeding the existing frozen on-chain funding
// target; settlement stays on chain and the only truth for funded/verified
// remains the on-chain funding observation. These states therefore refine the
// pre-observation awaiting_funds window and never claim funded or verified.
//
// deriveSessionStatus maps any unrecognized FundingState to
// SessionStatusAwaitingFunds via its default case, so every value below is,
// by construction, a pre-observation refinement — no SessionStatus change is
// needed or wanted.
const (
	// FundingStateOnrampAwaitingPayment: the buyer opened an onramp purchase and
	// must complete the fiat payment step.
	FundingStateOnrampAwaitingPayment FundingState = "onramp_awaiting_payment"
	// FundingStateOnrampProcessing: fiat captured; the provider is converting to
	// the settlement asset.
	FundingStateOnrampProcessing FundingState = "onramp_processing"
	// FundingStateOnrampDelivering: asset purchased; delivery to the frozen
	// funding target is in flight (direct-to-target mode).
	FundingStateOnrampDelivering FundingState = "onramp_delivering"
	// FundingStateOnrampForwarding: asset delivered to the buyer's embedded
	// wallet; forwarding into the frozen target (e.g. EIP-3009 authorization via
	// the platform relayer) is in flight. This is the "escrow-submitting" phase.
	FundingStateOnrampForwarding FundingState = "onramp_forwarding"
)

// Onramp status string mirror. pkg/payment cannot import pkg/contracts (that
// package imports this one), so these mirror contracts.OnrampStatus by value.
// Keep in sync with pkg/contracts/onramp_provider.go.
const (
	onrampStatusCreated         = "created"
	onrampStatusAwaitingPayment = "awaiting_payment"
	onrampStatusProcessing      = "processing"
	onrampStatusDelivering      = "delivering"
	onrampStatusDelivered       = "delivered"
)

// OnrampFundingSourceView is the durable, resumable record of an onramp
// purchase attached to a payment attempt. It is what makes a session
// leave-and-resume safe: re-entry reads the existing source instead of
// creating a second onramp order, and the projector reads it to surface the
// funding leg. It is a funding SOURCE descriptor, never a settlement mode.
type OnrampFundingSourceView struct {
	ProviderID           string     `json:"providerID"`
	OnrampOrderID        string     `json:"onrampOrderID"`
	Status               string     `json:"status"` // mirrors contracts.OnrampStatus
	DeliverToBuyerWallet bool       `json:"deliverToBuyerWallet"`
	BuyerWalletAddress   string     `json:"buyerWalletAddress,omitempty"`
	// BuyerActionURL is where the buyer completes the fiat payment step while
	// the purchase awaits payment (provider-hosted checkout).
	BuyerActionURL string     `json:"buyerActionURL,omitempty"`
	Disclosure     string     `json:"disclosure,omitempty"`
	UpdatedAt      *time.Time `json:"updatedAt,omitempty"`
}

// Active reports whether the onramp source is still progressing toward
// delivery (mirrors contracts.OnrampStatus.Active).
func (s *OnrampFundingSourceView) Active() bool {
	if s == nil {
		return false
	}
	switch s.Status {
	case onrampStatusCreated, onrampStatusAwaitingPayment, onrampStatusProcessing, onrampStatusDelivering:
		return true
	default:
		return false
	}
}

// RefineFundingStateForOnramp refines a base FundingState with onramp progress.
//
// It is the exact pure function the session projector calls after
// deriveFundingState. The invariant it enforces:
//
//   - It overrides ONLY when the base is the pre-observation AwaitingFunds
//     state AND no on-chain funds have been observed at the frozen target AND
//     an onramp source is present. In every other case the observation-driven
//     base state wins unchanged — so onramp status can never advance a session
//     to funded or verified. A nil source is always a no-op.
func RefineFundingStateForOnramp(base FundingState, observedAmount string, source *OnrampFundingSourceView) FundingState {
	if source == nil {
		return base
	}
	// Observation-driven states always win: once any funds are observed at the
	// frozen target, or the base has already advanced past awaiting_funds, the
	// chain is the source of truth.
	if base != FundingStateAwaitingFunds || hasObservedFunds(observedAmount) {
		return base
	}

	switch source.Status {
	case onrampStatusCreated, onrampStatusAwaitingPayment:
		return FundingStateOnrampAwaitingPayment
	case onrampStatusProcessing:
		return FundingStateOnrampProcessing
	case onrampStatusDelivering:
		// Asset en route to its delivery destination (target directly, or the
		// buyer wallet to be forwarded); both read as "delivering" pre-observation.
		return FundingStateOnrampDelivering
	case onrampStatusDelivered:
		if source.DeliverToBuyerWallet {
			// Asset is in the buyer wallet; forwarding into the frozen target is
			// the remaining pre-observation step.
			return FundingStateOnrampForwarding
		}
		// Direct-to-target delivery reported: fall through to awaiting the
		// authoritative on-chain observation, which the base already expresses.
		return base
	default:
		// failed / reversed / unknown: leave the base state; terminal handling
		// belongs to the observation and attempt layers, not this refinement.
		return base
	}
}

// SelectOnrampFundingSource picks, from an attempt's onramp purchase history
// (1:N — failed/reversed records are retained for reconciliation), the single
// record the session projection surfaces:
//
//  1. the most recently updated ACTIVE purchase, if any (at most one exists by
//     the storage constraint, but selection stays order-defined regardless);
//  2. otherwise the most recent delivered purchase whose delivery went to the
//     buyer wallet — its forwarding step is still pending pre-observation;
//  3. otherwise nil: terminal failed/reversed history never drives funding
//     state, and delivered-to-target resolves through the chain observation.
func SelectOnrampFundingSource(sources []OnrampFundingSourceView) *OnrampFundingSourceView {
	var active, forwarding *OnrampFundingSourceView
	for i := range sources {
		s := &sources[i]
		switch {
		case s.Active():
			if active == nil || laterThan(s.UpdatedAt, active.UpdatedAt) {
				active = s
			}
		case s.Status == onrampStatusDelivered && s.DeliverToBuyerWallet:
			if forwarding == nil || laterThan(s.UpdatedAt, forwarding.UpdatedAt) {
				forwarding = s
			}
		}
	}
	if active != nil {
		return active
	}
	return forwarding
}

// laterThan compares two optional timestamps; a set timestamp beats nil.
func laterThan(a, b *time.Time) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	return a.After(*b)
}

// hasObservedFunds reports whether a positive amount has been observed at the
// frozen target. Amount strings use the same unit as FundingTargetView.Amount;
// an empty, zero, or unparseable value counts as no funds.
func hasObservedFunds(observedAmount string) bool {
	trimmed := strings.TrimSpace(observedAmount)
	if trimmed == "" {
		return false
	}
	// Accept both integer raw amounts and decimal human amounts: any non-zero,
	// non-negative magnitude counts as observed.
	if v, ok := new(big.Int).SetString(trimmed, 10); ok {
		return v.Sign() > 0
	}
	if f, ok := new(big.Float).SetString(trimmed); ok {
		return f.Sign() > 0
	}
	return false
}

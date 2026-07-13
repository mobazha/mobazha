// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Embedded-wallet provisioning (RFC-0012) admits a buyer-owned, buyer-
// authenticated signing key into a moderated-escrow attempt as a genuine
// co-owner (for example, one signature of a 2-of-3 Safe), alongside RFC-0011's
// Settlement-root-derived keys and RFC-0010's seller/platform-held guest
// custody. The key is generated and held under a reviewed third-party
// provider's custody model; it is never derived from a Mobazha Settlement
// root and is never held or unilaterally exercisable by Mobazha or the seller.
//
// The types below are the Core-facing contract. Concrete providers (Privy,
// Coinbase CDP, ...) live under internal/payment/embeddedwallet/{provider}/
// and are admitted as RFC-0006 trusted modules before they may hold real
// buyer funds.

// Sentinel errors for embedded-wallet integrations.
var (
	// ErrEmbeddedWalletProviderNotFound is returned by a registry lookup for an
	// unregistered provider id.
	ErrEmbeddedWalletProviderNotFound = errors.New("embedded-wallet: provider not registered")

	// ErrEmbeddedWalletUnsupportedSigning marks a signing surface that is not
	// EIP-712 (or the chain family's equivalent structured-signing standard).
	// RFC-0012 Proposal 3: an arbitrary raw hash, personal_sign, or a
	// look-alike domain is not an equivalent authorization and must be refused.
	ErrEmbeddedWalletUnsupportedSigning = errors.New("embedded-wallet: only structured typed-data signing is accepted")

	// ErrEmbeddedWalletNoBuyerAuthorization marks a signing request that lacks
	// fresh buyer authorization. RFC-0012 Proposal 2: there is no server-side
	// signer and no standing delegated-signing grant to Mobazha or the seller,
	// so a signature can never be produced on platform authority alone.
	ErrEmbeddedWalletNoBuyerAuthorization = errors.New("embedded-wallet: signing requires buyer authorization")

	// ErrEmbeddedWalletCapabilityClosed marks a rail/action for which the
	// provider has not proven its structured-signature format is accepted by
	// the owning contract end to end (RFC-0012 Proposal 6). Capabilities are
	// fail-closed: the zero value is unavailable.
	ErrEmbeddedWalletCapabilityClosed = errors.New("embedded-wallet: capability gate is closed for this rail/action")
)

// SettlementAction names a moderated-escrow settlement action whose structured
// signature the provider must be able to authorize. RFC-0012 Proposal 6
// requires acceptance for every action the escrow must perform before the path
// becomes buyer-visible on a chain.
type SettlementAction string

const (
	SettlementActionConfirm       SettlementAction = "confirm"
	SettlementActionCancel        SettlementAction = "cancel"
	SettlementActionSellerDecline SettlementAction = "seller-decline"
	SettlementActionDisputeRelease SettlementAction = "dispute-release"
	SettlementActionRefund        SettlementAction = "refund"
)

// AllSettlementActions is the full set an escrow must authorize for a chain to
// pass RFC-0012 Proposal 6's capability gate.
func AllSettlementActions() []SettlementAction {
	return []SettlementAction{
		SettlementActionConfirm,
		SettlementActionCancel,
		SettlementActionSellerDecline,
		SettlementActionDisputeRelease,
		SettlementActionRefund,
	}
}

// EmbeddedWalletCapabilities declares, for one rail, which settlement actions
// this provider's structured-signature format is proven to satisfy and whether
// the buyer has a provider-native export/recovery path. Its zero value is
// intentionally unavailable/fail-closed (RFC-0012 Proposal 4/6).
type EmbeddedWalletCapabilities struct {
	// RailID is the network-qualified rail this declaration applies to.
	RailID string

	// Actions maps each settlement action to whether the provider's signature
	// is accepted on-chain for it. A missing or false entry is fail-closed.
	Actions map[SettlementAction]bool

	// ExportRecovery reports that the buyer can independently authenticate to,
	// export, or recover this key through the provider's own account-recovery
	// path without Mobazha or seller involvement. RFC-0012 Proposal 2 requires
	// this before a provider may hold real funds.
	ExportRecovery bool

	// OnrampFunding reports that fiat-to-asset onramp delivery to the frozen
	// funding target (or to the buyer wallet, forwarded) is observed,
	// confirmed, and restart-recoverable for this rail (RFC-0012 Proposal 6.3).
	OnrampFunding bool
}

// Allows reports whether every listed action is accepted and, when
// requireExport is true, the buyer export/recovery path is present. It is the
// single fail-closed predicate callers should use before offering the path.
func (c EmbeddedWalletCapabilities) Allows(requireExport bool, actions ...SettlementAction) bool {
	if requireExport && !c.ExportRecovery {
		return false
	}
	for _, a := range actions {
		if !c.Actions[a] {
			return false
		}
	}
	return true
}

// BuyerRef binds an embedded wallet to exactly one buyer-controlled provider
// account. Subject is the stable buyer identity Core authenticated (for
// example, the Casdoor `sub` resolved to a Mobazha user); the provider
// performs its own independent buyer authentication on top of this. BuyerRef
// deliberately carries no key material and no derivation path.
type BuyerRef struct {
	// Subject is the Core-authenticated buyer identity (opaque to the provider
	// beyond binding).
	Subject string

	// TenantID scopes the buyer to a hosted tenant for routing and audit; it is
	// not part of any key derivation (there is none — the key is provider-held).
	TenantID string
}

// Validate rejects an unbound buyer reference.
func (r BuyerRef) Validate() error {
	if strings.TrimSpace(r.Subject) == "" {
		return fmt.Errorf("embedded-wallet: buyer reference requires a subject")
	}
	return nil
}

// EnsureWalletRequest asks the provider to return the buyer's wallet for a
// rail, creating it at the provider if it does not exist yet.
type EnsureWalletRequest struct {
	Buyer  BuyerRef
	RailID string
}

// Validate rejects incomplete provisioning requests.
func (r EnsureWalletRequest) Validate() error {
	if err := r.Buyer.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.RailID) == "" {
		return fmt.Errorf("embedded-wallet: ensure-wallet requires a rail")
	}
	return nil
}

// EmbeddedWallet is an opaque handle to a buyer-owned provider wallet. It
// exposes only what Core needs to admit the wallet as a settlement
// participant: the provider, an opaque wallet id, and the on-chain address
// (participant public identity). It never carries private key material.
type EmbeddedWallet struct {
	ProviderID  string
	WalletID    string
	Address     string
	RailID      string
	ChainFamily ChainFamily
}

// ChainFamily discriminates the structured-signing standard a payload uses.
// RFC-0012 Proposal 3 admits EIP-712 for EVM and "the equivalent
// structured-signing standard of a non-EVM chain family".
type ChainFamily string

const (
	ChainFamilyEVM    ChainFamily = "evm"
	ChainFamilySolana ChainFamily = "solana"
)

// StructuredSignPayload carries the full structured-signing document the owning
// contract expects, exactly as it must be presented to the provider's signing
// method (for EVM, the eth_signTypedData_v4 typed-data JSON). It MUST be
// complete structured data — never a pre-hashed digest or an arbitrary
// message. RFC-0012 Proposal 3 forbids admitting a raw hash or a look-alike
// domain as equivalent authorization.
type StructuredSignPayload struct {
	ChainFamily ChainFamily
	// Document is the canonical structured-signing JSON. For EVM this is the
	// EIP-712 typed-data object with `domain`, `types`, `primaryType`, and
	// `message` members.
	Document json.RawMessage
}

// requiredEVMTypedDataMembers are the members a genuine EIP-712 typed-data
// document must contain. Their presence is what distinguishes structured typed
// data from a raw hash or an ad-hoc message.
var requiredEVMTypedDataMembers = []string{"domain", "types", "primaryType", "message"}

// Validate confirms the payload is structured signing data for its chain
// family, not a raw digest. It does not validate the signing-domain contents
// (order, attempt, action, terms hash) — that binding is asserted by the
// caller through EmbeddedWalletSignRequest and enforced by the owning contract.
func (p StructuredSignPayload) Validate() error {
	if len(p.Document) == 0 {
		return ErrEmbeddedWalletUnsupportedSigning
	}
	switch p.ChainFamily {
	case ChainFamilyEVM:
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(p.Document, &doc); err != nil {
			return fmt.Errorf("%w: payload is not a structured typed-data object: %v", ErrEmbeddedWalletUnsupportedSigning, err)
		}
		for _, member := range requiredEVMTypedDataMembers {
			if _, ok := doc[member]; !ok {
				return fmt.Errorf("%w: EIP-712 payload missing %q", ErrEmbeddedWalletUnsupportedSigning, member)
			}
		}
		return nil
	case ChainFamilySolana:
		if !json.Valid(p.Document) {
			return fmt.Errorf("%w: solana structured payload is not valid JSON", ErrEmbeddedWalletUnsupportedSigning)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown chain family %q", ErrEmbeddedWalletUnsupportedSigning, p.ChainFamily)
	}
}

// BuyerAuthorization carries fresh, buyer-supplied consent for one signing
// request. It is opaque to Core and interpreted by the provider adapter (for
// Privy, a buyer auth/session token or an authorization signature the buyer's
// client obtained). An empty authorization is rejected: RFC-0012 Proposal 2
// forbids any platform-authority-only signing path.
type BuyerAuthorization struct {
	// Scheme names how the token should be interpreted (adapter-specific).
	Scheme string
	// Token is the buyer-supplied consent material.
	Token string
}

// IsZero reports the absence of buyer authorization.
func (a BuyerAuthorization) IsZero() bool {
	return strings.TrimSpace(a.Scheme) == "" && strings.TrimSpace(a.Token) == ""
}

// EmbeddedWalletSignRequest asks the provider to produce one structured
// signature over Payload, authorized by the buyer, and binds the request to
// the RFC-0011 signing domain (order, attempt, action, frozen-terms hash) for
// validation and audit. The domain itself lives inside Payload; these fields
// let Core reject an ambiguous request before it reaches the provider and are
// available to provider-side policy and audit.
type EmbeddedWalletSignRequest struct {
	Wallet        EmbeddedWallet
	Payload       StructuredSignPayload
	Authorization BuyerAuthorization

	OrderID   string
	AttemptID string
	Action    SettlementAction
	TermsHash string
}

// Validate rejects requests that are unbound, lack buyer authorization, or
// carry a non-structured payload.
func (r EmbeddedWalletSignRequest) Validate() error {
	if strings.TrimSpace(r.Wallet.WalletID) == "" || strings.TrimSpace(r.Wallet.Address) == "" {
		return fmt.Errorf("embedded-wallet: sign request requires a resolved wallet")
	}
	if r.Authorization.IsZero() {
		return ErrEmbeddedWalletNoBuyerAuthorization
	}
	if strings.TrimSpace(r.OrderID) == "" || strings.TrimSpace(r.AttemptID) == "" ||
		strings.TrimSpace(string(r.Action)) == "" || !validSettlementTermsHash(r.TermsHash) {
		return fmt.Errorf("embedded-wallet: sign request requires order, attempt, action, and frozen-terms hash")
	}
	return r.Payload.Validate()
}

// EmbeddedWalletSignature is the produced structured signature and the address
// that produced it. Signer must equal the participant wallet address; the
// owning contract verifies the signature against it. No key material is
// returned.
type EmbeddedWalletSignature struct {
	Signer    string
	Signature []byte
}

// EmbeddedWalletProvider is the Core-facing contract a reviewed embedded-wallet
// module implements (RFC-0012). Implementations MUST:
//   - refuse any signing surface other than structured typed-data
//     (ErrEmbeddedWalletUnsupportedSigning);
//   - refuse to sign without fresh buyer authorization
//     (ErrEmbeddedWalletNoBuyerAuthorization);
//   - report capabilities fail-closed per rail (zero value = unavailable);
//   - never return, cache, or reconstruct buyer private key material.
type EmbeddedWalletProvider interface {
	// ProviderID returns the stable module identifier ("privy", "coinbase-cdp").
	ProviderID() string

	// Capabilities declares this provider's proven capability surface for a
	// rail. It is fail-closed: an unproven rail returns a zero-value
	// EmbeddedWalletCapabilities (all actions false), not an error.
	Capabilities(ctx context.Context, railID string) (EmbeddedWalletCapabilities, error)

	// EnsureWallet returns the buyer's wallet for the rail, creating it at the
	// provider if absent. The wallet is bound to exactly one buyer-controlled
	// provider account.
	EnsureWallet(ctx context.Context, req EnsureWalletRequest) (EmbeddedWallet, error)

	// SignTypedData produces one structured signature authorized by the buyer.
	// It MUST return ErrEmbeddedWalletUnsupportedSigning for a non-structured
	// payload and ErrEmbeddedWalletNoBuyerAuthorization for a missing
	// authorization, before contacting the provider.
	SignTypedData(ctx context.Context, req EmbeddedWalletSignRequest) (EmbeddedWalletSignature, error)
}

// EmbeddedWalletProviderRegistry is a thread-safe set of admitted providers.
// A distribution may compose zero, one, or more modules (RFC-0006 / RFC-0012
// Proposal 4); no chain or client may assume a specific vendor is present.
type EmbeddedWalletProviderRegistry interface {
	Register(provider EmbeddedWalletProvider)
	Unregister(id string)
	ForProvider(id string) (EmbeddedWalletProvider, error)
	Registered() []string
}

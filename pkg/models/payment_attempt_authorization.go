// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

const (
	SettlementAuthorizationVersion  = 1
	settlementKeyOfferSigningDomain = "mobazha/settlement-key-offer/v1\x00"
)

// SettlementParticipantRole identifies one attempt-scoped settlement
// participant. It is independent of a chain's threshold implementation.
type SettlementParticipantRole string

const (
	SettlementParticipantBuyer     SettlementParticipantRole = "buyer"
	SettlementParticipantSeller    SettlementParticipantRole = "seller"
	SettlementParticipantModerator SettlementParticipantRole = "moderator"
)

// Valid reports whether the role is supported by the first authorization
// bundle protocol.
func (r SettlementParticipantRole) Valid() bool {
	switch r {
	case SettlementParticipantBuyer, SettlementParticipantSeller, SettlementParticipantModerator:
		return true
	default:
		return false
	}
}

// NewSettlementAuthorizationContextID returns the non-secret, random
// 32-byte context that locates one payment attempt's settlement keys.
func NewSettlementAuthorizationContextID() (string, error) {
	value := make([]byte, sha256.Size)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate settlement authorization context: %w", err)
	}
	return hex.EncodeToString(value), nil
}

// SettlementKeyOffer is an Identity-signed binding of one participant's
// attempt-scoped Settlement public key. Moderated offers also bind the selected
// moderator, funding amount, timeout, and moderator payout. Availability leases,
// clocks, and separately assigned offer identifiers remain outside the protocol.
type SettlementKeyOffer struct {
	Version                 uint32                    `json:"version"`
	AuthorizationContextID  string                    `json:"authorizationContextID"`
	OrderID                 string                    `json:"orderID"`
	AttemptID               string                    `json:"attemptID"`
	ParticipantPeerID       string                    `json:"participantPeerID"`
	ParticipantRole         SettlementParticipantRole `json:"participantRole"`
	RailID                  string                    `json:"railID"`
	Purpose                 string                    `json:"purpose"`
	PublicKey               []byte                    `json:"publicKey"`
	ExpectedModeratorPeerID string                    `json:"expectedModeratorPeerID,omitempty"`
	AmountAtomic            string                    `json:"amountAtomic,omitempty"`
	ModeratorPayoutAddress  string                    `json:"moderatorPayoutAddress,omitempty"`
	ModeratorFeeAmount      string                    `json:"moderatorFeeAmount,omitempty"`
	EscrowTimeoutHours      uint32                    `json:"escrowTimeoutHours,omitempty"`
	Signature               []byte                    `json:"signature"`
}

type settlementKeyOfferPayload struct {
	Version                 uint32                    `json:"version"`
	AuthorizationContextID  string                    `json:"authorizationContextID"`
	OrderID                 string                    `json:"orderID"`
	AttemptID               string                    `json:"attemptID"`
	ParticipantPeerID       string                    `json:"participantPeerID"`
	ParticipantRole         SettlementParticipantRole `json:"participantRole"`
	RailID                  string                    `json:"railID"`
	Purpose                 string                    `json:"purpose"`
	PublicKey               []byte                    `json:"publicKey"`
	ExpectedModeratorPeerID string                    `json:"expectedModeratorPeerID,omitempty"`
	AmountAtomic            string                    `json:"amountAtomic,omitempty"`
	ModeratorPayoutAddress  string                    `json:"moderatorPayoutAddress,omitempty"`
	ModeratorFeeAmount      string                    `json:"moderatorFeeAmount,omitempty"`
	EscrowTimeoutHours      uint32                    `json:"escrowTimeoutHours,omitempty"`
}

// SigningPayload validates an unsigned offer and returns its domain-separated
// canonical Identity-signing payload.
func (o SettlementKeyOffer) SigningPayload() ([]byte, error) {
	if err := o.validate(false); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(settlementKeyOfferPayload{
		Version: o.Version, AuthorizationContextID: o.AuthorizationContextID,
		OrderID: o.OrderID, AttemptID: o.AttemptID, ParticipantPeerID: o.ParticipantPeerID,
		ParticipantRole: o.ParticipantRole, RailID: o.RailID, Purpose: o.Purpose,
		PublicKey: o.PublicKey, ExpectedModeratorPeerID: o.ExpectedModeratorPeerID,
		AmountAtomic: o.AmountAtomic, ModeratorPayoutAddress: o.ModeratorPayoutAddress,
		ModeratorFeeAmount: o.ModeratorFeeAmount,
		EscrowTimeoutHours: o.EscrowTimeoutHours,
	})
	if err != nil {
		return nil, fmt.Errorf("encode settlement key offer: %w", err)
	}
	return append([]byte(settlementKeyOfferSigningDomain), payload...), nil
}

// Verify validates the offer scope and its Identity signature.
func (o SettlementKeyOffer) Verify() error {
	if err := o.validate(true); err != nil {
		return err
	}
	payload, err := o.SigningPayload()
	if err != nil {
		return err
	}
	pid, err := peer.Decode(strings.TrimSpace(o.ParticipantPeerID))
	if err != nil {
		return fmt.Errorf("decode settlement key offer peer ID: %w", err)
	}
	identityKey, err := pid.ExtractPublicKey()
	if err != nil {
		return fmt.Errorf("extract settlement key offer identity key: %w", err)
	}
	valid, err := identityKey.Verify(payload, o.Signature)
	if err != nil {
		return fmt.Errorf("verify settlement key offer signature: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid settlement key offer signature")
	}
	return nil
}

// CanonicalBytesAndHash verifies and canonically encodes one signed offer for
// durable idempotency checks before the complete authorization bundle exists.
func (o SettlementKeyOffer) CanonicalBytesAndHash() ([]byte, string, error) {
	if err := o.Verify(); err != nil {
		return nil, "", err
	}
	canonical, err := json.Marshal(o)
	if err != nil {
		return nil, "", fmt.Errorf("encode settlement key offer: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(digest[:]), nil
}

func (o SettlementKeyOffer) validate(requireSignature bool) error {
	if o.Version != SettlementAuthorizationVersion ||
		!validSettlementAuthorizationContextID(o.AuthorizationContextID) ||
		strings.TrimSpace(o.OrderID) == "" || strings.TrimSpace(o.AttemptID) == "" ||
		strings.TrimSpace(o.RailID) == "" || strings.TrimSpace(o.Purpose) == "" ||
		!o.ParticipantRole.Valid() || len(o.PublicKey) == 0 {
		return fmt.Errorf("invalid settlement key offer scope")
	}
	if !strings.HasSuffix(o.Purpose, ":"+string(o.ParticipantRole)) {
		return fmt.Errorf("settlement key offer purpose must bind participant role")
	}
	if !validCanonicalNativeRail(o.RailID) {
		return fmt.Errorf("settlement key offer rail must be a canonical native rail")
	}
	if _, err := peer.Decode(strings.TrimSpace(o.ParticipantPeerID)); err != nil {
		return fmt.Errorf("invalid settlement key offer participant")
	}
	moderatorPeerID := strings.TrimSpace(o.ExpectedModeratorPeerID)
	amountAtomic := strings.TrimSpace(o.AmountAtomic)
	payoutAddress := strings.TrimSpace(o.ModeratorPayoutAddress)
	feeAmount := strings.TrimSpace(o.ModeratorFeeAmount)
	if moderatorPeerID == "" {
		if amountAtomic != "" || payoutAddress != "" || feeAmount != "" {
			return fmt.Errorf("settlement key offer moderator scope is incomplete")
		}
	} else {
		if _, err := peer.Decode(moderatorPeerID); err != nil || amountAtomic == "" {
			return fmt.Errorf("invalid settlement key offer moderator scope")
		}
		if o.EscrowTimeoutHours == 0 {
			return fmt.Errorf("moderated settlement key offer requires escrow timeout")
		}
		if _, err := settlementAtomicAmount(amountAtomic, true); err != nil {
			return fmt.Errorf("invalid settlement key offer amount")
		}
		if o.ParticipantRole == SettlementParticipantModerator {
			if strings.TrimSpace(o.ParticipantPeerID) != moderatorPeerID || payoutAddress == "" || feeAmount == "" {
				return fmt.Errorf("settlement moderator offer does not match selected moderator")
			}
			fee, err := settlementAtomicAmount(feeAmount, false)
			if err != nil {
				return fmt.Errorf("invalid settlement moderator fee")
			}
			amount, _ := settlementAtomicAmount(amountAtomic, true)
			if fee.Cmp(amount) >= 0 {
				return fmt.Errorf("settlement moderator fee must be less than funding amount")
			}
		} else if payoutAddress != "" || feeAmount != "" {
			return fmt.Errorf("non-moderator settlement offer cannot bind moderator payout")
		}
	}
	if requireSignature && len(o.Signature) == 0 {
		return fmt.Errorf("settlement key offer signature is required")
	}
	return nil
}

// PaymentAttemptAuthorizationBundle freezes the complete public authorization
// material that makes a crypto payment attempt actionable.
type PaymentAttemptAuthorizationBundle struct {
	Version                uint32                      `json:"version"`
	AuthorizationContextID string                      `json:"authorizationContextID"`
	OrderID                string                      `json:"orderID"`
	AttemptID              string                      `json:"attemptID"`
	RailID                 string                      `json:"railID"`
	SettlementTermsHash    string                      `json:"settlementTermsHash"`
	FundingTargetHash      string                      `json:"fundingTargetHash"`
	RequiredRoles          []SettlementParticipantRole `json:"requiredRoles"`
	Offers                 []SettlementKeyOffer        `json:"offers"`
	SellerTermsSigner      string                      `json:"sellerTermsSigner"`
	SellerTermsSignature   []byte                      `json:"sellerTermsSignature"`
}

// CanonicalBytesAndHash validates, verifies, and canonically encodes a frozen
// authorization bundle. Offer order is canonicalized by participant role.
func (b PaymentAttemptAuthorizationBundle) CanonicalBytesAndHash() ([]byte, string, error) {
	if err := b.Validate(); err != nil {
		return nil, "", err
	}
	b.RequiredRoles = append([]SettlementParticipantRole(nil), b.RequiredRoles...)
	sort.Slice(b.RequiredRoles, func(i, j int) bool {
		return b.RequiredRoles[i] < b.RequiredRoles[j]
	})
	b.Offers = append([]SettlementKeyOffer(nil), b.Offers...)
	sort.Slice(b.Offers, func(i, j int) bool {
		return b.Offers[i].ParticipantRole < b.Offers[j].ParticipantRole
	})
	canonical, err := json.Marshal(b)
	if err != nil {
		return nil, "", fmt.Errorf("encode payment attempt authorization bundle: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(digest[:]), nil
}

// Validate checks all offer proofs and bundle-wide scope bindings.
func (b PaymentAttemptAuthorizationBundle) Validate() error {
	if b.Version != SettlementAuthorizationVersion ||
		!validSettlementAuthorizationContextID(b.AuthorizationContextID) ||
		strings.TrimSpace(b.OrderID) == "" || strings.TrimSpace(b.AttemptID) == "" ||
		strings.TrimSpace(b.RailID) == "" || !validSHA256Hex(b.SettlementTermsHash) ||
		!validSHA256Hex(b.FundingTargetHash) || strings.TrimSpace(b.SellerTermsSigner) == "" ||
		len(b.SellerTermsSignature) == 0 || len(b.RequiredRoles) == 0 ||
		len(b.RequiredRoles) != len(b.Offers) {
		return fmt.Errorf("invalid payment attempt authorization bundle identity")
	}
	if _, err := peer.Decode(strings.TrimSpace(b.SellerTermsSigner)); err != nil {
		return fmt.Errorf("invalid authorization bundle seller signer")
	}
	required := make(map[SettlementParticipantRole]struct{}, len(b.RequiredRoles))
	for _, role := range b.RequiredRoles {
		if !role.Valid() {
			return fmt.Errorf("invalid authorization bundle required role")
		}
		if _, exists := required[role]; exists {
			return fmt.Errorf("duplicate authorization bundle required role")
		}
		required[role] = struct{}{}
	}
	if _, hasBuyer := required[SettlementParticipantBuyer]; !hasBuyer {
		return fmt.Errorf("authorization bundle requires a buyer offer")
	}
	if _, hasSeller := required[SettlementParticipantSeller]; !hasSeller {
		return fmt.Errorf("authorization bundle requires a seller offer")
	}
	seen := make(map[SettlementParticipantRole]struct{}, len(b.Offers))
	publicKeys := make(map[string]SettlementParticipantRole, len(b.Offers))
	sellerOfferPeerID := ""
	expectedModeratorPeerID := ""
	amountAtomic := ""
	escrowTimeoutHours := uint32(0)
	for _, offer := range b.Offers {
		if err := offer.Verify(); err != nil {
			return err
		}
		if offer.AuthorizationContextID != b.AuthorizationContextID || offer.OrderID != b.OrderID ||
			offer.AttemptID != b.AttemptID || offer.RailID != b.RailID {
			return fmt.Errorf("settlement key offer does not belong to authorization bundle")
		}
		if len(seen) == 0 {
			expectedModeratorPeerID = strings.TrimSpace(offer.ExpectedModeratorPeerID)
			amountAtomic = strings.TrimSpace(offer.AmountAtomic)
			escrowTimeoutHours = offer.EscrowTimeoutHours
		} else if strings.TrimSpace(offer.ExpectedModeratorPeerID) != expectedModeratorPeerID ||
			strings.TrimSpace(offer.AmountAtomic) != amountAtomic || offer.EscrowTimeoutHours != escrowTimeoutHours {
			return fmt.Errorf("settlement key offers disagree on moderator scope")
		}
		if _, requiredRole := required[offer.ParticipantRole]; !requiredRole {
			return fmt.Errorf("unexpected settlement key offer role")
		}
		if _, exists := seen[offer.ParticipantRole]; exists {
			return fmt.Errorf("duplicate settlement key offer role")
		}
		if otherRole, exists := publicKeys[string(offer.PublicKey)]; exists {
			return fmt.Errorf("settlement key offer public key is reused by %q and %q", otherRole, offer.ParticipantRole)
		}
		seen[offer.ParticipantRole] = struct{}{}
		publicKeys[string(offer.PublicKey)] = offer.ParticipantRole
		if offer.ParticipantRole == SettlementParticipantSeller {
			sellerOfferPeerID = strings.TrimSpace(offer.ParticipantPeerID)
		}
	}
	if len(seen) != len(required) {
		return fmt.Errorf("incomplete settlement key offers")
	}
	_, requiresModerator := required[SettlementParticipantModerator]
	if requiresModerator != (expectedModeratorPeerID != "") {
		return fmt.Errorf("authorization bundle moderator scope does not match required roles")
	}
	if sellerOfferPeerID == "" || strings.TrimSpace(b.SellerTermsSigner) != sellerOfferPeerID {
		return fmt.Errorf("authorization bundle seller signer does not match seller offer")
	}
	return nil
}

func validSettlementAuthorizationContextID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func validSHA256Hex(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func validCanonicalNativeRail(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "crypto:") && (strings.HasSuffix(value, ":native") || strings.HasSuffix(value, "/native")) &&
		strings.ToLower(value) == value && !strings.ContainsAny(value, " \t\n\r")
}

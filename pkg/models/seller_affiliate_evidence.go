// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	SellerAffiliateReferralEvidenceDomain    = "mobazha:seller-affiliate-referral:v1"
	SellerAffiliateReferralEvidenceVersionV1 = "1"
	SellerAffiliateNetworkMainnet            = "mainnet"
	SellerAffiliateNetworkTestnet            = "testnet"
)

// SellerAffiliateReferralEvidence is a seller-Peer-signed, expiry-bound
// transport projection of one referral session's frozen facts. It can be
// verified without trusting Hosting, but order acceptance still owns replay
// prevention and the atomic attribution commit.
type SellerAffiliateReferralEvidence struct {
	Domain                     string               `json:"domain" enum:"mobazha:seller-affiliate-referral:v1"`
	Version                    string               `json:"version" enum:"1"`
	Network                    string               `json:"network" enum:"mainnet,testnet"`
	ReferralSessionID          string               `json:"referralSessionID"`
	ProgramID                  string               `json:"programID"`
	PolicyVersion              string               `json:"policyVersion"`
	AffiliateLinkID            string               `json:"affiliateLinkID"`
	SellerPeerID               string               `json:"sellerPeerID"`
	PromoterPeerID             string               `json:"promoterPeerID"`
	CommissionRateBPS          uint32               `json:"commissionRateBPS" minimum:"1" maximum:"10000"`
	AttributionWindowSeconds   uint64               `json:"attributionWindowSeconds" minimum:"1"`
	PromoterPayoutDestinations PayoutDestinationSet `json:"promoterPayoutDestinations"`
	IssuedAtUnix               int64                `json:"issuedAtUnix"`
	ExpiresAtUnix              int64                `json:"expiresAtUnix"`
	Signature                  []byte               `json:"signature"`
	IssuerPublicKey            []byte               `json:"issuerPublicKey"`
}

type sellerAffiliateReferralEvidenceBody struct {
	Domain                     string               `json:"domain"`
	Version                    string               `json:"version"`
	Network                    string               `json:"network"`
	ReferralSessionID          string               `json:"referralSessionID"`
	ProgramID                  string               `json:"programID"`
	PolicyVersion              string               `json:"policyVersion"`
	AffiliateLinkID            string               `json:"affiliateLinkID"`
	SellerPeerID               string               `json:"sellerPeerID"`
	PromoterPeerID             string               `json:"promoterPeerID"`
	CommissionRateBPS          uint32               `json:"commissionRateBPS"`
	AttributionWindowSeconds   uint64               `json:"attributionWindowSeconds"`
	PromoterPayoutDestinations PayoutDestinationSet `json:"promoterPayoutDestinations"`
	IssuedAtUnix               int64                `json:"issuedAtUnix"`
	ExpiresAtUnix              int64                `json:"expiresAtUnix"`
	IssuerPublicKey            []byte               `json:"issuerPublicKey"`
}

// NewSellerAffiliateReferralEvidence builds the unsigned canonical projection
// for a newly persisted referral session.
func NewSellerAffiliateReferralEvidence(session *AffiliateReferralSession, network string) (SellerAffiliateReferralEvidence, error) {
	if session == nil || session.Validate() != nil {
		return SellerAffiliateReferralEvidence{}, ErrInvalidSellerAffiliate
	}
	window := session.ExpiresAt.Sub(session.IssuedAt)
	if window <= 0 || window%time.Second != 0 {
		return SellerAffiliateReferralEvidence{}, ErrInvalidSellerAffiliate
	}
	network = strings.TrimSpace(network)
	if network != SellerAffiliateNetworkMainnet && network != SellerAffiliateNetworkTestnet {
		return SellerAffiliateReferralEvidence{}, ErrInvalidSellerAffiliate
	}
	policyVersion := sellerAffiliateReferralPolicyVersion(
		session.ProgramID,
		session.SellerPeerID,
		session.CommissionRateBPSSnapshot,
		uint64(window/time.Second),
	)
	sellerPeerID, err := peer.Decode(strings.TrimSpace(session.SellerPeerID))
	if err != nil {
		return SellerAffiliateReferralEvidence{}, ErrInvalidSellerAffiliate
	}
	publicKey, err := sellerPeerID.ExtractPublicKey()
	if err != nil {
		return SellerAffiliateReferralEvidence{}, ErrInvalidSellerAffiliate
	}
	rawPublicKey, err := publicKey.Raw()
	if err != nil || len(rawPublicKey) != ed25519.PublicKeySize {
		return SellerAffiliateReferralEvidence{}, ErrInvalidSellerAffiliate
	}
	return SellerAffiliateReferralEvidence{
		Domain: SellerAffiliateReferralEvidenceDomain, Version: SellerAffiliateReferralEvidenceVersionV1,
		Network: network, ReferralSessionID: session.ID, ProgramID: session.ProgramID,
		PolicyVersion: policyVersion, AffiliateLinkID: session.AffiliateLinkID,
		SellerPeerID: session.SellerPeerID, PromoterPeerID: session.PromoterPeerID,
		CommissionRateBPS:          session.CommissionRateBPSSnapshot,
		AttributionWindowSeconds:   uint64(window / time.Second),
		PromoterPayoutDestinations: session.PromoterPayoutDestinations.Clone(),
		IssuedAtUnix:               session.IssuedAt.UTC().Unix(), ExpiresAtUnix: session.ExpiresAt.UTC().Unix(),
		IssuerPublicKey: append([]byte(nil), rawPublicKey...),
	}, nil
}

// SignableBytes returns a domain-separated deterministic JSON payload.
func (e SellerAffiliateReferralEvidence) SignableBytes() ([]byte, error) {
	body := sellerAffiliateReferralEvidenceBody{
		Domain: e.Domain, Version: e.Version, Network: e.Network,
		ReferralSessionID: e.ReferralSessionID, ProgramID: e.ProgramID, PolicyVersion: e.PolicyVersion,
		AffiliateLinkID: e.AffiliateLinkID, SellerPeerID: e.SellerPeerID, PromoterPeerID: e.PromoterPeerID,
		CommissionRateBPS: e.CommissionRateBPS, AttributionWindowSeconds: e.AttributionWindowSeconds,
		PromoterPayoutDestinations: e.PromoterPayoutDestinations.Clone(),
		IssuedAtUnix:               e.IssuedAtUnix, ExpiresAtUnix: e.ExpiresAtUnix,
		IssuerPublicKey: append([]byte(nil), e.IssuerPublicKey...),
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return append([]byte(SellerAffiliateReferralEvidenceDomain+"\n"), payload...), nil
}

// Verify checks domain, network, Peer-key binding, frozen policy, freshness,
// and the seller identity signature. Replay prevention is intentionally not a
// property of a standalone verifier and must be enforced during order commit.
func (e SellerAffiliateReferralEvidence) Verify(expectedSellerPeerID, expectedNetwork string, now time.Time) error {
	for _, value := range []string{
		e.Domain, e.Version, e.Network, e.ReferralSessionID, e.ProgramID, e.PolicyVersion,
		e.AffiliateLinkID, e.SellerPeerID, e.PromoterPeerID,
	} {
		if value == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("seller affiliate referral evidence identity and policy fields must be canonical")
		}
	}
	if e.Domain != SellerAffiliateReferralEvidenceDomain || e.Version != SellerAffiliateReferralEvidenceVersionV1 {
		return fmt.Errorf("seller affiliate referral evidence domain or version is unsupported")
	}
	if expectedNetwork == "" || e.Network != strings.TrimSpace(expectedNetwork) ||
		(e.Network != SellerAffiliateNetworkMainnet && e.Network != SellerAffiliateNetworkTestnet) {
		return fmt.Errorf("seller affiliate referral evidence network mismatch")
	}
	if expectedSellerPeerID == "" || e.SellerPeerID != strings.TrimSpace(expectedSellerPeerID) {
		return fmt.Errorf("seller affiliate referral evidence seller mismatch")
	}
	if e.CommissionRateBPS == 0 || e.CommissionRateBPS > 10000 || e.AttributionWindowSeconds == 0 ||
		len(e.PromoterPayoutDestinations.Destinations) == 0 || !e.PromoterPayoutDestinations.Valid() {
		return fmt.Errorf("seller affiliate referral evidence frozen terms are invalid")
	}
	if e.PolicyVersion != sellerAffiliateReferralPolicyVersion(
		e.ProgramID, e.SellerPeerID, e.CommissionRateBPS, e.AttributionWindowSeconds,
	) {
		return fmt.Errorf("seller affiliate referral evidence policy version mismatch")
	}
	issuedAt := time.Unix(e.IssuedAtUnix, 0).UTC()
	expiresAt := time.Unix(e.ExpiresAtUnix, 0).UTC()
	if e.IssuedAtUnix <= 0 || !expiresAt.After(issuedAt) ||
		uint64(expiresAt.Sub(issuedAt)/time.Second) != e.AttributionWindowSeconds ||
		now.Before(issuedAt.Add(-time.Minute)) || !now.Before(expiresAt) {
		return fmt.Errorf("seller affiliate referral evidence is not fresh")
	}
	if len(e.IssuerPublicKey) != ed25519.PublicKeySize || len(e.Signature) != ed25519.SignatureSize {
		return fmt.Errorf("seller affiliate referral evidence public key or signature size is invalid")
	}
	publicKey, err := libp2pcrypto.UnmarshalEd25519PublicKey(e.IssuerPublicKey)
	if err != nil {
		return fmt.Errorf("decode seller affiliate referral evidence public key: %w", err)
	}
	issuerID, err := peer.IDFromPublicKey(publicKey)
	if err != nil || issuerID.String() != e.SellerPeerID {
		return fmt.Errorf("seller affiliate referral evidence public key does not match seller")
	}
	signable, err := e.SignableBytes()
	if err != nil {
		return err
	}
	verified, err := publicKey.Verify(signable, e.Signature)
	if err != nil || !verified {
		return fmt.Errorf("seller affiliate referral evidence signature is invalid")
	}
	return nil
}

func sellerAffiliateReferralPolicyVersion(programID, sellerPeerID string, rateBPS uint32, windowSeconds uint64) string {
	payload, _ := json.Marshal(struct {
		ProgramID     string `json:"programID"`
		SellerPeerID  string `json:"sellerPeerID"`
		RateBPS       uint32 `json:"commissionRateBPS"`
		WindowSeconds uint64 `json:"attributionWindowSeconds"`
	}{
		ProgramID: strings.TrimSpace(programID), SellerPeerID: strings.TrimSpace(sellerPeerID),
		RateBPS: rateBPS, WindowSeconds: windowSeconds,
	})
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}

// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	SellerAffiliatePromoterEnrollmentDomain    = "mobazha:seller-affiliate-promoter-enrollment:v1"
	SellerAffiliatePromoterEnrollmentVersionV1 = "1"
	SellerAffiliatePromoterEnrollmentMaxTTL    = 10 * time.Minute
)

// SellerAffiliatePromoterEnrollmentEvidence is a short-lived promoter-Peer
// authorization to enroll one immutable payout-destination snapshot into one
// seller program. Hosting may transport it but cannot change its Peer identity,
// audience, program, network, or payout terms.
type SellerAffiliatePromoterEnrollmentEvidence struct {
	Domain                     string               `json:"domain" enum:"mobazha:seller-affiliate-promoter-enrollment:v1"`
	Version                    string               `json:"version" enum:"1"`
	Network                    string               `json:"network" enum:"mainnet,testnet"`
	EvidenceID                 string               `json:"evidenceID"`
	IssuerPromoterPeerID       string               `json:"issuerPromoterPeerID"`
	AudienceSellerPeerID       string               `json:"audienceSellerPeerID"`
	ProgramID                  string               `json:"programID"`
	PromoterPayoutDestinations PayoutDestinationSet `json:"promoterPayoutDestinations"`
	IssuedAtUnix               int64                `json:"issuedAtUnix"`
	ExpiresAtUnix              int64                `json:"expiresAtUnix"`
	IssuerPublicKey            []byte               `json:"issuerPublicKey"`
	Signature                  []byte               `json:"signature"`
}

type sellerAffiliatePromoterEnrollmentBody struct {
	Domain                     string               `json:"domain"`
	Version                    string               `json:"version"`
	Network                    string               `json:"network"`
	EvidenceID                 string               `json:"evidenceID"`
	IssuerPromoterPeerID       string               `json:"issuerPromoterPeerID"`
	AudienceSellerPeerID       string               `json:"audienceSellerPeerID"`
	ProgramID                  string               `json:"programID"`
	PromoterPayoutDestinations PayoutDestinationSet `json:"promoterPayoutDestinations"`
	IssuedAtUnix               int64                `json:"issuedAtUnix"`
	ExpiresAtUnix              int64                `json:"expiresAtUnix"`
	IssuerPublicKey            []byte               `json:"issuerPublicKey"`
}

func NewSellerAffiliatePromoterEnrollmentEvidence(
	evidenceID,
	issuerPromoterPeerID,
	audienceSellerPeerID,
	programID,
	network string,
	payoutDestinations PayoutDestinationSet,
	issuedAt time.Time,
) (SellerAffiliatePromoterEnrollmentEvidence, error) {
	evidenceID = strings.TrimSpace(evidenceID)
	issuerPromoterPeerID = strings.TrimSpace(issuerPromoterPeerID)
	audienceSellerPeerID = strings.TrimSpace(audienceSellerPeerID)
	programID = strings.TrimSpace(programID)
	network = strings.TrimSpace(network)
	if !validAffiliateID(evidenceID) || !validAffiliateID(programID) ||
		!validAffiliatePeerID(issuerPromoterPeerID) || !validAffiliatePeerID(audienceSellerPeerID) ||
		issuerPromoterPeerID == audienceSellerPeerID ||
		(network != SellerAffiliateNetworkMainnet && network != SellerAffiliateNetworkTestnet) ||
		len(payoutDestinations.Destinations) == 0 || !payoutDestinations.Valid() || issuedAt.IsZero() {
		return SellerAffiliatePromoterEnrollmentEvidence{}, ErrInvalidSellerAffiliate
	}
	issuerID, err := peer.Decode(issuerPromoterPeerID)
	if err != nil {
		return SellerAffiliatePromoterEnrollmentEvidence{}, ErrInvalidSellerAffiliate
	}
	publicKey, err := issuerID.ExtractPublicKey()
	if err != nil {
		return SellerAffiliatePromoterEnrollmentEvidence{}, ErrInvalidSellerAffiliate
	}
	rawPublicKey, err := publicKey.Raw()
	if err != nil || len(rawPublicKey) != ed25519.PublicKeySize {
		return SellerAffiliatePromoterEnrollmentEvidence{}, ErrInvalidSellerAffiliate
	}
	issuedAt = issuedAt.UTC().Truncate(time.Second)
	return SellerAffiliatePromoterEnrollmentEvidence{
		Domain: SellerAffiliatePromoterEnrollmentDomain, Version: SellerAffiliatePromoterEnrollmentVersionV1,
		Network: network, EvidenceID: evidenceID, IssuerPromoterPeerID: issuerPromoterPeerID,
		AudienceSellerPeerID: audienceSellerPeerID, ProgramID: programID,
		PromoterPayoutDestinations: payoutDestinations.Clone(),
		IssuedAtUnix:               issuedAt.Unix(), ExpiresAtUnix: issuedAt.Add(SellerAffiliatePromoterEnrollmentMaxTTL).Unix(),
		IssuerPublicKey: append([]byte(nil), rawPublicKey...),
	}, nil
}

func (e SellerAffiliatePromoterEnrollmentEvidence) SignableBytes() ([]byte, error) {
	body := sellerAffiliatePromoterEnrollmentBody{
		Domain: e.Domain, Version: e.Version, Network: e.Network, EvidenceID: e.EvidenceID,
		IssuerPromoterPeerID: e.IssuerPromoterPeerID, AudienceSellerPeerID: e.AudienceSellerPeerID,
		ProgramID: e.ProgramID, PromoterPayoutDestinations: e.PromoterPayoutDestinations.Clone(),
		IssuedAtUnix: e.IssuedAtUnix, ExpiresAtUnix: e.ExpiresAtUnix,
		IssuerPublicKey: append([]byte(nil), e.IssuerPublicKey...),
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return append([]byte(SellerAffiliatePromoterEnrollmentDomain+"\n"), payload...), nil
}

// Verify validates the promoter identity proof for one selected seller Node.
// Replaying the same valid evidence is safe only while link enrollment remains
// idempotent; it must never reactivate a seller-revoked link.
func (e SellerAffiliatePromoterEnrollmentEvidence) Verify(expectedSellerPeerID, expectedNetwork string, now time.Time) error {
	for _, value := range []string{
		e.Domain, e.Version, e.Network, e.EvidenceID, e.IssuerPromoterPeerID,
		e.AudienceSellerPeerID, e.ProgramID,
	} {
		if value == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("seller affiliate promoter enrollment identity fields must be canonical")
		}
	}
	if e.Domain != SellerAffiliatePromoterEnrollmentDomain || e.Version != SellerAffiliatePromoterEnrollmentVersionV1 {
		return fmt.Errorf("seller affiliate promoter enrollment domain or version is unsupported")
	}
	if expectedNetwork == "" || e.Network != strings.TrimSpace(expectedNetwork) ||
		(e.Network != SellerAffiliateNetworkMainnet && e.Network != SellerAffiliateNetworkTestnet) {
		return fmt.Errorf("seller affiliate promoter enrollment network mismatch")
	}
	if expectedSellerPeerID == "" || e.AudienceSellerPeerID != strings.TrimSpace(expectedSellerPeerID) ||
		e.IssuerPromoterPeerID == e.AudienceSellerPeerID {
		return fmt.Errorf("seller affiliate promoter enrollment audience mismatch")
	}
	if !validAffiliateID(e.EvidenceID) || !validAffiliateID(e.ProgramID) ||
		!validAffiliatePeerID(e.IssuerPromoterPeerID) || !validAffiliatePeerID(e.AudienceSellerPeerID) ||
		len(e.PromoterPayoutDestinations.Destinations) == 0 || !e.PromoterPayoutDestinations.Valid() {
		return fmt.Errorf("seller affiliate promoter enrollment facts are invalid")
	}
	issuedAt := time.Unix(e.IssuedAtUnix, 0).UTC()
	expiresAt := time.Unix(e.ExpiresAtUnix, 0).UTC()
	if e.IssuedAtUnix <= 0 || !expiresAt.After(issuedAt) ||
		expiresAt.Sub(issuedAt) > SellerAffiliatePromoterEnrollmentMaxTTL ||
		now.Before(issuedAt.Add(-time.Minute)) || !now.Before(expiresAt) {
		return fmt.Errorf("seller affiliate promoter enrollment is not fresh")
	}
	if len(e.IssuerPublicKey) != ed25519.PublicKeySize || len(e.Signature) != ed25519.SignatureSize {
		return fmt.Errorf("seller affiliate promoter enrollment public key or signature size is invalid")
	}
	publicKey, err := libp2pcrypto.UnmarshalEd25519PublicKey(e.IssuerPublicKey)
	if err != nil {
		return fmt.Errorf("decode seller affiliate promoter enrollment public key: %w", err)
	}
	issuerID, err := peer.IDFromPublicKey(publicKey)
	if err != nil || issuerID.String() != e.IssuerPromoterPeerID {
		return fmt.Errorf("seller affiliate promoter enrollment public key does not match promoter")
	}
	signable, err := e.SignableBytes()
	if err != nil {
		return err
	}
	verified, err := publicKey.Verify(signable, e.Signature)
	if err != nil || !verified {
		return fmt.Errorf("seller affiliate promoter enrollment signature is invalid")
	}
	return nil
}

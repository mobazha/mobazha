// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"crypto/rand"
	"testing"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func signedSellerAffiliateReferralEvidence(t *testing.T, now time.Time) SellerAffiliateReferralEvidence {
	t.Helper()
	privateKey, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	sellerPeerID, err := peer.IDFromPublicKey(publicKey)
	require.NoError(t, err)
	promoterPrivate, promoterPublic, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	_ = promoterPrivate
	promoterPeerID, err := peer.IDFromPublicKey(promoterPublic)
	require.NoError(t, err)

	session := &AffiliateReferralSession{
		ID: "session-1", AffiliateLinkID: "link-1", ProgramID: "program-1",
		SellerPeerID: sellerPeerID.String(), PromoterPeerID: promoterPeerID.String(),
		CommissionRateBPSSnapshot: 500,
		PromoterPayoutDestinations: PayoutDestinationSet{Destinations: []PayoutDestination{
			{RailID: "crypto:bip122:000000000019d6689c085ae165831e93:native", Address: "bc1qpromoter", Version: 1},
		}},
		IssuedAt: now, ExpiresAt: now.Add(30 * 24 * time.Hour), CreatedAt: now,
	}
	evidence, err := NewSellerAffiliateReferralEvidence(session, SellerAffiliateNetworkTestnet)
	require.NoError(t, err)
	signable, err := evidence.SignableBytes()
	require.NoError(t, err)
	evidence.Signature, err = privateKey.Sign(signable)
	require.NoError(t, err)
	evidence.IssuerPublicKey, err = publicKey.Raw()
	require.NoError(t, err)
	return evidence
}

func TestSellerAffiliateReferralEvidenceVerify(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	evidence := signedSellerAffiliateReferralEvidence(t, now)
	require.NoError(t, evidence.Verify(evidence.SellerPeerID, SellerAffiliateNetworkTestnet, now))

	t.Run("tampered frozen terms", func(t *testing.T) {
		tampered := evidence
		tampered.CommissionRateBPS++
		require.ErrorContains(t, tampered.Verify(tampered.SellerPeerID, SellerAffiliateNetworkTestnet, now), "policy version")
	})

	t.Run("wrong seller", func(t *testing.T) {
		_, otherPublic, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
		require.NoError(t, err)
		otherPeerID, err := peer.IDFromPublicKey(otherPublic)
		require.NoError(t, err)
		require.ErrorContains(t, evidence.Verify(otherPeerID.String(), SellerAffiliateNetworkTestnet, now), "seller mismatch")
	})

	t.Run("wrong network", func(t *testing.T) {
		require.ErrorContains(t, evidence.Verify(evidence.SellerPeerID, SellerAffiliateNetworkMainnet, now), "network mismatch")
	})

	t.Run("expired", func(t *testing.T) {
		require.ErrorContains(t, evidence.Verify(evidence.SellerPeerID, SellerAffiliateNetworkTestnet, time.Unix(evidence.ExpiresAtUnix, 0)), "not fresh")
	})

	t.Run("signature", func(t *testing.T) {
		tampered := evidence
		tampered.Signature = append([]byte(nil), evidence.Signature...)
		tampered.Signature[0] ^= 0xff
		require.ErrorContains(t, tampered.Verify(tampered.SellerPeerID, SellerAffiliateNetworkTestnet, now), "signature is invalid")
	})

	t.Run("issuer key binding", func(t *testing.T) {
		tampered := evidence
		_, otherPublic, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
		require.NoError(t, err)
		tampered.IssuerPublicKey, err = otherPublic.Raw()
		require.NoError(t, err)
		require.ErrorContains(t, tampered.Verify(tampered.SellerPeerID, SellerAffiliateNetworkTestnet, now), "does not match seller")
	})
}

func TestSellerAffiliateReferralEvidenceRejectsNonCanonicalConstruction(t *testing.T) {
	_, err := NewSellerAffiliateReferralEvidence(nil, SellerAffiliateNetworkTestnet)
	require.ErrorIs(t, err, ErrInvalidSellerAffiliate)

	now := time.Now().UTC().Truncate(time.Second)
	evidence := signedSellerAffiliateReferralEvidence(t, now)
	evidence.Domain = "other-domain"
	require.ErrorContains(t, evidence.Verify(evidence.SellerPeerID, SellerAffiliateNetworkTestnet, now), "domain or version")
}

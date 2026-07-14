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

func signedSellerAffiliatePromoterEnrollment(t *testing.T, now time.Time) SellerAffiliatePromoterEnrollmentEvidence {
	t.Helper()
	promoterPrivate, promoterPublic, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	promoterPeerID, err := peer.IDFromPublicKey(promoterPublic)
	require.NoError(t, err)
	_, sellerPublic, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	sellerPeerID, err := peer.IDFromPublicKey(sellerPublic)
	require.NoError(t, err)
	evidence, err := NewSellerAffiliatePromoterEnrollmentEvidence(
		"enrollment-1", promoterPeerID.String(), sellerPeerID.String(), "program-1",
		SellerAffiliateNetworkTestnet,
		PayoutDestinationSet{Destinations: []PayoutDestination{
			{RailID: "crypto:bip122:000000000019d6689c085ae165831e93:native", Address: "bc1qpromoter", Version: 1},
		}},
		now,
	)
	require.NoError(t, err)
	signable, err := evidence.SignableBytes()
	require.NoError(t, err)
	evidence.Signature, err = promoterPrivate.Sign(signable)
	require.NoError(t, err)
	return evidence
}

func TestSellerAffiliatePromoterEnrollmentEvidenceVerify(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	evidence := signedSellerAffiliatePromoterEnrollment(t, now)
	require.NoError(t, evidence.Verify(evidence.AudienceSellerPeerID, SellerAffiliateNetworkTestnet, now))

	t.Run("payout tampering", func(t *testing.T) {
		tampered := evidence
		tampered.PromoterPayoutDestinations = evidence.PromoterPayoutDestinations.Clone()
		tampered.PromoterPayoutDestinations.Destinations[0].Address = "bc1qattacker"
		require.ErrorContains(t, tampered.Verify(tampered.AudienceSellerPeerID, SellerAffiliateNetworkTestnet, now), "signature is invalid")
	})

	t.Run("wrong audience", func(t *testing.T) {
		_, otherPublic, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
		require.NoError(t, err)
		otherPeerID, err := peer.IDFromPublicKey(otherPublic)
		require.NoError(t, err)
		require.ErrorContains(t, evidence.Verify(otherPeerID.String(), SellerAffiliateNetworkTestnet, now), "audience mismatch")
	})

	t.Run("wrong network", func(t *testing.T) {
		require.ErrorContains(t, evidence.Verify(evidence.AudienceSellerPeerID, SellerAffiliateNetworkMainnet, now), "network mismatch")
	})

	t.Run("expired", func(t *testing.T) {
		require.ErrorContains(t, evidence.Verify(
			evidence.AudienceSellerPeerID, SellerAffiliateNetworkTestnet, time.Unix(evidence.ExpiresAtUnix, 0),
		), "not fresh")
	})

	t.Run("signature", func(t *testing.T) {
		tampered := evidence
		tampered.Signature = append([]byte(nil), evidence.Signature...)
		tampered.Signature[0] ^= 0xff
		require.ErrorContains(t, tampered.Verify(tampered.AudienceSellerPeerID, SellerAffiliateNetworkTestnet, now), "signature is invalid")
	})
}

func TestSellerAffiliatePromoterEnrollmentEvidenceRejectsSelfEnrollment(t *testing.T) {
	_, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	peerID, err := peer.IDFromPublicKey(publicKey)
	require.NoError(t, err)
	_, err = NewSellerAffiliatePromoterEnrollmentEvidence(
		"enrollment-1", peerID.String(), peerID.String(), "program-1", SellerAffiliateNetworkTestnet,
		PayoutDestinationSet{Destinations: []PayoutDestination{{RailID: "rail", Address: "address", Version: 1}}},
		time.Now().UTC(),
	)
	require.ErrorIs(t, err, ErrInvalidSellerAffiliate)
}

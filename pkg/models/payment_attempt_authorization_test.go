// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"crypto/rand"
	"strings"
	"testing"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestNewSettlementAuthorizationContextID_IsRandomCanonical32Bytes(t *testing.T) {
	first, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	second, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	require.Len(t, first, 64)
	require.NotEqual(t, first, second)
	require.True(t, validSettlementAuthorizationContextID(first))
}

func TestSettlementKeyOffer_VerifiesIdentityBoundScope(t *testing.T) {
	contextID, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	privateKey, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	participant, err := peer.IDFromPublicKey(publicKey)
	require.NoError(t, err)
	offer := SettlementKeyOffer{
		Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: "order-1", AttemptID: "attempt-1", ParticipantPeerID: participant.String(),
		ParticipantRole: SettlementParticipantSeller, RailID: "crypto:eip155:1:native",
		Purpose: "standard-order-participant:seller", PublicKey: []byte("settlement-public-key"),
	}
	payload, err := offer.SigningPayload()
	require.NoError(t, err)
	offer.Signature, err = privateKey.Sign(payload)
	require.NoError(t, err)
	require.NoError(t, offer.Verify())

	tampered := offer
	tampered.RailID = "crypto:eip155:137:native"
	require.Error(t, tampered.Verify())
}

func TestPaymentAttemptAuthorizationBundle_CanonicalizesAndRequiresCompleteOffers(t *testing.T) {
	contextID, err := NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	privateKey, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	seller, err := peer.IDFromPublicKey(publicKey)
	require.NoError(t, err)
	buyerPrivateKey, buyerPublicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	buyer, err := peer.IDFromPublicKey(buyerPublicKey)
	require.NoError(t, err)
	offer := SettlementKeyOffer{
		Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: "order-1", AttemptID: "attempt-1", ParticipantPeerID: seller.String(),
		ParticipantRole: SettlementParticipantSeller, RailID: "crypto:eip155:1:native",
		Purpose: "standard-order-participant:seller", PublicKey: []byte("settlement-public-key"),
	}
	payload, err := offer.SigningPayload()
	require.NoError(t, err)
	offer.Signature, err = privateKey.Sign(payload)
	require.NoError(t, err)
	buyerOffer := SettlementKeyOffer{
		Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: "order-1", AttemptID: "attempt-1", ParticipantPeerID: buyer.String(),
		ParticipantRole: SettlementParticipantBuyer, RailID: "crypto:eip155:1:native",
		Purpose: "standard-order-participant:buyer", PublicKey: []byte("buyer-settlement-public-key"),
	}
	buyerPayload, err := buyerOffer.SigningPayload()
	require.NoError(t, err)
	buyerOffer.Signature, err = buyerPrivateKey.Sign(buyerPayload)
	require.NoError(t, err)
	bundle := PaymentAttemptAuthorizationBundle{
		Version: SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: "order-1", AttemptID: "attempt-1", RailID: "crypto:eip155:1:native",
		SettlementTermsHash: strings.Repeat("a", 64), FundingTargetHash: strings.Repeat("b", 64),
		RequiredRoles: []SettlementParticipantRole{SettlementParticipantBuyer, SettlementParticipantSeller},
		Offers:        []SettlementKeyOffer{buyerOffer, offer}, SellerTermsSigner: seller.String(),
		SellerTermsSignature: []byte("seller-terms-signature"),
	}
	first, firstHash, err := bundle.CanonicalBytesAndHash()
	require.NoError(t, err)
	second, secondHash, err := bundle.CanonicalBytesAndHash()
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, firstHash, secondHash)

	duplicateKey := bundle
	duplicateKey.Offers = append([]SettlementKeyOffer(nil), bundle.Offers...)
	duplicateKey.Offers[1].PublicKey = append([]byte(nil), duplicateKey.Offers[0].PublicKey...)
	duplicatePayload, err := duplicateKey.Offers[1].SigningPayload()
	require.NoError(t, err)
	duplicateKey.Offers[1].Signature, err = privateKey.Sign(duplicatePayload)
	require.NoError(t, err)
	require.ErrorContains(t, duplicateKey.Validate(), "public key is reused")

	bundle.RequiredRoles = append(bundle.RequiredRoles, SettlementParticipantModerator)
	require.Error(t, bundle.Validate())
}

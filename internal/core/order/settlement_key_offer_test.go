// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package order

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

func TestBuildSettlementKeyOfferOrderMessage_BindsOuterAndInnerIdentity(t *testing.T) {
	keyPair, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	peerID, err := identity.PeerIDFromPublicKey(keyPair.PubKey)
	require.NoError(t, err)
	signer := contracts.NewKeyPairSigner(keyPair, peerID)
	contextID, err := models.NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	offer := models.SettlementKeyOffer{
		Version: models.SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: "order-1", AttemptID: "attempt-1", ParticipantPeerID: peerID.String(),
		ParticipantRole: models.SettlementParticipantSeller, RailID: "crypto:eip155:1:native",
		Purpose: "standard-order-participant:seller", PublicKey: []byte("seller-attempt-public-key"),
	}
	payload, err := offer.SigningPayload()
	require.NoError(t, err)
	offer.Signature, err = signer.Sign(payload)
	require.NoError(t, err)

	message, err := buildSettlementKeyOfferOrderMessage(offer, signer)
	require.NoError(t, err)
	require.Equal(t, npb.OrderMessage_SETTLEMENT_KEY_OFFER, message.MessageType)
	require.Equal(t, offer.OrderID, message.OrderID)
	require.Equal(t, offer.ParticipantPeerID, message.SenderPeerID)
	require.NotEmpty(t, message.Signature)
	wire := new(pb.SettlementKeyOffer)
	require.NoError(t, message.Message.UnmarshalTo(wire))
	roundTrip, err := paymentintent.SettlementKeyOfferFromProto(wire)
	require.NoError(t, err)
	require.Equal(t, offer, roundTrip)
}

func TestValidateSettlementKeyOfferTarget_RequiresOrderCounterparty(t *testing.T) {
	orderOpen := &pb.OrderOpen{
		BuyerID:  &pb.ID{PeerID: "buyer"},
		Listings: []*pb.SignedListing{{Listing: &pb.Listing{VendorID: &pb.ID{PeerID: "seller"}}}},
	}
	tests := []struct {
		name      string
		role      models.SettlementParticipantRole
		target    string
		wantError bool
	}{
		{name: "buyer to seller", role: models.SettlementParticipantBuyer, target: "seller"},
		{name: "seller to buyer", role: models.SettlementParticipantSeller, target: "buyer"},
		{name: "moderator to buyer", role: models.SettlementParticipantModerator, target: "buyer"},
		{name: "moderator to seller", role: models.SettlementParticipantModerator, target: "seller"},
		{name: "buyer to stranger", role: models.SettlementParticipantBuyer, target: "stranger", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateSettlementKeyOfferTarget(orderOpen, test.role, test.target)
			if test.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

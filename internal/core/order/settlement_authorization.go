// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package order

import (
	"context"
	"fmt"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/types/known/anypb"
)

// PublishSettlementAuthorization durably sends the seller-frozen public
// authorization snapshot to the signed order buyer.
func (s *OrderAppService) PublishSettlementAuthorization(
	ctx context.Context,
	targetPeerID peer.ID,
	authorization models.PaymentAttemptSettlementAuthorization,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.messenger == nil || s.signer == nil {
		return fmt.Errorf("publish settlement authorization: messenger and signer are required")
	}
	if targetPeerID == "" || authorization.Terms.SellerPeerID != s.signer.PeerID().String() ||
		(authorization.Terms.BuyerPeerID != targetPeerID.String() &&
			authorization.Terms.ModeratorPeerID != targetPeerID.String()) {
		return fmt.Errorf("publish settlement authorization: signed participants do not match sender and target")
	}
	if _, _, err := authorization.CanonicalBytesAndHash(); err != nil {
		return err
	}
	order, err := s.fetchOrder(authorization.Terms.OrderID)
	if err != nil {
		return fmt.Errorf("publish settlement authorization: fetch order: %w", err)
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("publish settlement authorization: load order open: %w", err)
	}
	if err := validateSettlementAuthorizationParticipants(
		orderOpen, authorization.Terms.BuyerPeerID, authorization.Terms.SellerPeerID,
	); err != nil {
		return err
	}
	message, err := buildSettlementAuthorizationOrderMessage(authorization, s.signer)
	if err != nil {
		return err
	}
	payload, err := anypb.New(message)
	if err != nil {
		return fmt.Errorf("publish settlement authorization: marshal order message: %w", err)
	}
	netMessage := newMessageWithID()
	netMessage.MessageType = npb.Message_ORDER
	netMessage.Payload = payload
	return s.db.Update(func(tx database.Tx) error {
		if err := s.messenger.ReliablySendMessage(tx, targetPeerID, netMessage, nil); err != nil {
			return fmt.Errorf("publish settlement authorization: durable handoff: %w", err)
		}
		return nil
	})
}

func buildSettlementAuthorizationOrderMessage(
	authorization models.PaymentAttemptSettlementAuthorization,
	signer contracts.Signer,
) (*npb.OrderMessage, error) {
	if signer == nil || authorization.Terms.SellerPeerID != signer.PeerID().String() {
		return nil, fmt.Errorf("build settlement authorization message: signer does not match seller")
	}
	wire, err := paymentintent.SettlementAuthorizationToProto(authorization)
	if err != nil {
		return nil, err
	}
	wireAny, err := anypb.New(wire)
	if err != nil {
		return nil, fmt.Errorf("build settlement authorization message: marshal authorization: %w", err)
	}
	message := &npb.OrderMessage{
		OrderID:     authorization.Terms.OrderID,
		MessageType: npb.OrderMessage_SETTLEMENT_AUTHORIZATION,
		Message:     wireAny,
	}
	if err := utils.SignOrderMessage(message, signer); err != nil {
		return nil, fmt.Errorf("build settlement authorization message: sign order message: %w", err)
	}
	return message, nil
}

func validateSettlementAuthorizationParticipants(orderOpen *pb.OrderOpen, buyerPeerID, sellerPeerID string) error {
	if orderOpen == nil || orderOpen.BuyerID == nil || orderOpen.BuyerID.PeerID != buyerPeerID ||
		len(orderOpen.Listings) == 0 {
		return fmt.Errorf("settlement authorization participants do not match signed order")
	}
	for _, signedListing := range orderOpen.Listings {
		if signedListing == nil || signedListing.Listing == nil || signedListing.Listing.VendorID == nil ||
			signedListing.Listing.VendorID.PeerID != sellerPeerID {
			return fmt.Errorf("settlement authorization participants do not match signed order")
		}
	}
	return nil
}

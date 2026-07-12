// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package order

import (
	"context"
	"fmt"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

// PublishSettlementKeyOffer stores the local participant's offer and
// durably hands the signed order message to the target peer in one transaction.
func (s *OrderAppService) PublishSettlementKeyOffer(
	ctx context.Context,
	targetPeerID peer.ID,
	offer models.SettlementKeyOffer,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.orderProcessor == nil || s.messenger == nil || s.signer == nil {
		return fmt.Errorf("publish settlement key offer: order processor, messenger, and signer are required")
	}
	if targetPeerID == "" {
		return fmt.Errorf("publish settlement key offer: target peer is required")
	}
	if offer.ParticipantPeerID != s.signer.PeerID().String() {
		return fmt.Errorf("publish settlement key offer: participant does not match local identity")
	}
	order, err := s.fetchOrder(offer.OrderID)
	if err != nil {
		return fmt.Errorf("publish settlement key offer: fetch order: %w", err)
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("publish settlement key offer: load order open: %w", err)
	}
	if err := validateSettlementKeyOfferTarget(orderOpen, offer.ParticipantRole, targetPeerID.String()); err != nil {
		return err
	}
	message, err := buildSettlementKeyOfferOrderMessage(offer, s.signer)
	if err != nil {
		return err
	}
	payload, err := anypb.New(message)
	if err != nil {
		return fmt.Errorf("publish settlement key offer: marshal order message: %w", err)
	}
	netMessage := newMessageWithID()
	netMessage.MessageType = npb.Message_ORDER
	netMessage.Payload = payload
	return s.db.Update(func(tx database.Tx) error {
		if _, err := s.orderProcessor.ProcessMessage(tx, message); err != nil {
			return fmt.Errorf("publish settlement key offer: persist local offer: %w", err)
		}
		if err := s.messenger.ReliablySendMessage(tx, targetPeerID, netMessage, nil); err != nil {
			return fmt.Errorf("publish settlement key offer: durable handoff: %w", err)
		}
		return nil
	})
}

func buildSettlementKeyOfferOrderMessage(
	offer models.SettlementKeyOffer,
	signer contracts.Signer,
) (*npb.OrderMessage, error) {
	if signer == nil || offer.ParticipantPeerID != signer.PeerID().String() {
		return nil, fmt.Errorf("build settlement key offer message: signer does not match participant")
	}
	wire, err := paymentintent.SettlementKeyOfferToProto(offer)
	if err != nil {
		return nil, err
	}
	wireAny, err := anypb.New(wire)
	if err != nil {
		return nil, fmt.Errorf("build settlement key offer message: marshal offer: %w", err)
	}
	message := &npb.OrderMessage{
		OrderID: offer.OrderID, MessageType: npb.OrderMessage_SETTLEMENT_KEY_OFFER, Message: wireAny,
	}
	if err := utils.SignOrderMessage(message, signer); err != nil {
		return nil, fmt.Errorf("build settlement key offer message: sign order message: %w", err)
	}
	return message, nil
}

func validateSettlementKeyOfferTarget(
	orderOpen *pb.OrderOpen,
	role models.SettlementParticipantRole,
	targetPeerID string,
) error {
	if orderOpen == nil || targetPeerID == "" {
		return fmt.Errorf("publish settlement key offer: order participants are required")
	}
	buyerPeerID := ""
	if orderOpen.BuyerID != nil {
		buyerPeerID = orderOpen.BuyerID.PeerID
	}
	sellerPeerID := ""
	if len(orderOpen.Listings) > 0 && orderOpen.Listings[0] != nil && orderOpen.Listings[0].Listing != nil &&
		orderOpen.Listings[0].Listing.VendorID != nil {
		sellerPeerID = orderOpen.Listings[0].Listing.VendorID.PeerID
	}
	switch role {
	case models.SettlementParticipantBuyer:
		if sellerPeerID == "" || targetPeerID != sellerPeerID {
			return fmt.Errorf("publish settlement buyer offer: target is not order seller")
		}
	case models.SettlementParticipantSeller:
		if buyerPeerID == "" || targetPeerID != buyerPeerID {
			return fmt.Errorf("publish settlement seller offer: target is not order buyer")
		}
	case models.SettlementParticipantModerator:
		if targetPeerID != buyerPeerID && targetPeerID != sellerPeerID {
			return fmt.Errorf("publish settlement moderator offer: target is not an order participant")
		}
	default:
		return fmt.Errorf("publish settlement key offer: unsupported participant role %q", role)
	}
	return nil
}

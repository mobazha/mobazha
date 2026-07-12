// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package orders

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/types/known/anypb"
	"gorm.io/gorm"
)

func (op *OrderProcessor) processSettlementKeyOfferMessage(
	dbtx database.Tx,
	order *models.Order,
	message *npb.OrderMessage,
) (interface{}, error) {
	wire := new(pb.SettlementKeyOffer)
	if err := message.Message.UnmarshalTo(wire); err != nil {
		return nil, err
	}
	offer, err := paymentintent.SettlementKeyOfferFromProto(wire)
	if err != nil {
		return nil, err
	}
	if order == nil || offer.OrderID != message.OrderID || offer.OrderID != order.ID.String() {
		return nil, fmt.Errorf("settlement key offer sender or order does not match signed order message")
	}
	if offer.ParticipantRole != models.SettlementParticipantModerator &&
		offer.ParticipantPeerID != message.SenderPeerID {
		return nil, fmt.Errorf("settlement key offer sender or order does not match signed order message")
	}
	orderOpen, err := order.OrderOpenMessage()
	if models.IsMessageNotExistError(err) {
		if parkErr := order.ParkMessage(message); parkErr != nil {
			return nil, parkErr
		}
		return nil, ErrMessageParked
	}
	if err != nil {
		return nil, err
	}
	switch offer.ParticipantRole {
	case models.SettlementParticipantBuyer:
		if orderOpen.BuyerID == nil || orderOpen.BuyerID.PeerID != offer.ParticipantPeerID {
			return nil, fmt.Errorf("settlement buyer offer does not match order buyer")
		}
	case models.SettlementParticipantSeller:
		if len(orderOpen.Listings) == 0 || orderOpen.Listings[0] == nil || orderOpen.Listings[0].Listing == nil ||
			orderOpen.Listings[0].Listing.VendorID == nil ||
			orderOpen.Listings[0].Listing.VendorID.PeerID != offer.ParticipantPeerID {
			return nil, fmt.Errorf("settlement seller offer does not match order seller")
		}
	case models.SettlementParticipantModerator:
		buyerPeerID := ""
		if orderOpen.BuyerID != nil {
			buyerPeerID = orderOpen.BuyerID.PeerID
		}
		if message.SenderPeerID != offer.ParticipantPeerID && message.SenderPeerID != buyerPeerID {
			return nil, fmt.Errorf("settlement moderator offer sender is neither moderator nor buyer relay")
		}
		var attempt models.PaymentAttempt
		if err := dbtx.Read().Session(&gorm.Session{NewDB: true}).
			Where("tenant_id = ? AND attempt_id = ?", order.TenantID, offer.AttemptID).
			First(&attempt).Error; err != nil {
			return nil, fmt.Errorf("settlement moderator offer requires a local authorization draft: %w", err)
		}
		if attempt.State != models.PaymentAttemptAuthorizationDraft ||
			attempt.ExpectedModeratorPeerID == "" || attempt.ExpectedModeratorPeerID != offer.ParticipantPeerID {
			return nil, fmt.Errorf("settlement moderator offer does not match selected moderator")
		}
	default:
		return nil, fmt.Errorf("unsupported settlement key offer participant role %q", offer.ParticipantRole)
	}
	if err := paymentintent.RetainReceivedSettlementKeyOfferInTransaction(
		dbtx.Read(), order.TenantID, offer,
	); err != nil {
		return nil, err
	}
	if offer.ParticipantRole == models.SettlementParticipantModerator &&
		order.Role() == models.RoleBuyer && message.SenderPeerID == offer.ParticipantPeerID {
		if err := op.relayModeratorSettlementKeyOffer(dbtx, orderOpen, offer); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (op *OrderProcessor) relayModeratorSettlementKeyOffer(
	dbtx database.Tx,
	orderOpen *pb.OrderOpen,
	offer models.SettlementKeyOffer,
) error {
	if op == nil || op.signer == nil || op.messenger == nil || orderOpen == nil || len(orderOpen.Listings) == 0 ||
		orderOpen.Listings[0] == nil || orderOpen.Listings[0].Listing == nil ||
		orderOpen.Listings[0].Listing.VendorID == nil {
		return fmt.Errorf("relay moderator settlement key offer: buyer relay dependencies are incomplete")
	}
	seller, err := peer.Decode(orderOpen.Listings[0].Listing.VendorID.PeerID)
	if err != nil {
		return fmt.Errorf("relay moderator settlement key offer: decode seller: %w", err)
	}
	wire, err := paymentintent.SettlementKeyOfferToProto(offer)
	if err != nil {
		return err
	}
	wireAny, err := anypb.New(wire)
	if err != nil {
		return err
	}
	relay := &npb.OrderMessage{OrderID: offer.OrderID, MessageType: npb.OrderMessage_SETTLEMENT_KEY_OFFER, Message: wireAny}
	if err := utils.SignOrderMessage(relay, op.signer); err != nil {
		return fmt.Errorf("relay moderator settlement key offer: sign relay: %w", err)
	}
	payload, err := anypb.New(relay)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(offer.AttemptID + "\x00" + offer.ParticipantPeerID + "\x00" + seller.String()))
	netMessage := &npb.Message{MessageID: hex.EncodeToString(digest[:20]), MessageType: npb.Message_ORDER, Payload: payload}
	if err := op.messenger.ReliablySendMessage(dbtx, seller, netMessage, nil); err != nil {
		return fmt.Errorf("relay moderator settlement key offer: durable handoff: %w", err)
	}
	return nil
}

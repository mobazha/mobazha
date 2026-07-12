// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package orders

import (
	"fmt"

	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
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
	if order == nil || offer.OrderID != message.OrderID || offer.OrderID != order.ID.String() ||
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
		// The selected moderator is frozen later in settlement terms. At this
		// stage the outer and inner Identity signatures already bind the sender.
	default:
		return nil, fmt.Errorf("unsupported settlement key offer participant role %q", offer.ParticipantRole)
	}
	if err := paymentintent.StoreCryptoPaymentAttemptSettlementKeyOfferInTransaction(
		dbtx.Read(), order.TenantID, offer.AttemptID, offer,
	); err != nil {
		return nil, err
	}
	return nil, nil
}

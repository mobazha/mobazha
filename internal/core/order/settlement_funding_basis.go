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
)

// PublishSettlementFundingBasisProposal durably sends the buyer-authored,
// non-actionable quote snapshot to the signed order seller.
func (s *OrderAppService) PublishSettlementFundingBasisProposal(
	ctx context.Context,
	targetPeerID peer.ID,
	basis models.PaymentAttemptFundingBasis,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.messenger == nil || s.signer == nil || targetPeerID == "" {
		return fmt.Errorf("publish settlement funding basis: messenger, signer, and target are required")
	}
	if basis.QuoteIssuer != s.signer.PeerID().String() {
		return fmt.Errorf("publish settlement funding basis: local buyer is not proposal issuer")
	}
	order, err := s.fetchOrder(basis.OrderID)
	if err != nil {
		return fmt.Errorf("publish settlement funding basis: fetch order: %w", err)
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil || orderOpen.BuyerID == nil || orderOpen.BuyerID.PeerID != basis.QuoteIssuer || len(orderOpen.Listings) == 0 ||
		orderOpen.Listings[0] == nil || orderOpen.Listings[0].Listing == nil || orderOpen.Listings[0].Listing.VendorID == nil ||
		orderOpen.Listings[0].Listing.VendorID.PeerID != targetPeerID.String() {
		return fmt.Errorf("publish settlement funding basis: signed order participants do not match")
	}
	message, err := buildSettlementFundingBasisOrderMessage(basis, s.signer)
	if err != nil {
		return err
	}
	payload, err := anypb.New(message)
	if err != nil {
		return fmt.Errorf("publish settlement funding basis: marshal order message: %w", err)
	}
	netMessage := newMessageWithID()
	netMessage.MessageType = npb.Message_ORDER
	netMessage.Payload = payload
	return s.db.Update(func(tx database.Tx) error {
		if err := s.messenger.ReliablySendMessage(tx, targetPeerID, netMessage, nil); err != nil {
			return fmt.Errorf("publish settlement funding basis: durable handoff: %w", err)
		}
		return nil
	})
}

func buildSettlementFundingBasisOrderMessage(
	basis models.PaymentAttemptFundingBasis,
	signer contracts.Signer,
) (*npb.OrderMessage, error) {
	if signer == nil || basis.QuoteIssuer != signer.PeerID().String() {
		return nil, fmt.Errorf("build settlement funding basis message: signer does not match buyer issuer")
	}
	wire, err := paymentintent.FundingBasisProposalToProto(basis)
	if err != nil {
		return nil, err
	}
	wireAny, err := anypb.New(wire)
	if err != nil {
		return nil, fmt.Errorf("build settlement funding basis message: marshal proposal: %w", err)
	}
	message := &npb.OrderMessage{
		OrderID: basis.OrderID, MessageType: npb.OrderMessage_SETTLEMENT_FUNDING_BASIS, Message: wireAny,
	}
	if err := utils.SignOrderMessage(message, signer); err != nil {
		return nil, fmt.Errorf("build settlement funding basis message: sign order message: %w", err)
	}
	return message, nil
}

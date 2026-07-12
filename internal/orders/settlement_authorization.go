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

func (op *OrderProcessor) processSettlementAuthorizationMessage(
	dbtx database.Tx,
	order *models.Order,
	message *npb.OrderMessage,
) (interface{}, error) {
	wire := new(pb.SettlementAuthorization)
	if err := message.Message.UnmarshalTo(wire); err != nil {
		return nil, err
	}
	authorization, err := paymentintent.SettlementAuthorizationFromProto(wire)
	if err != nil {
		return nil, err
	}
	if order == nil || order.Role() != models.RoleBuyer || authorization.Terms.OrderID != message.OrderID ||
		authorization.Terms.OrderID != order.ID.String() || authorization.Terms.SellerPeerID != message.SenderPeerID {
		return nil, fmt.Errorf("settlement authorization sender or order does not match signed order message")
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
	if orderOpen.BuyerID == nil || orderOpen.BuyerID.PeerID != authorization.Terms.BuyerPeerID ||
		len(orderOpen.Listings) == 0 {
		return nil, fmt.Errorf("settlement authorization participants do not match signed order")
	}
	for _, signedListing := range orderOpen.Listings {
		if signedListing == nil || signedListing.Listing == nil || signedListing.Listing.VendorID == nil ||
			signedListing.Listing.VendorID.PeerID != authorization.Terms.SellerPeerID {
			return nil, fmt.Errorf("settlement authorization participants do not match signed order")
		}
	}
	if err := paymentintent.RetainReceivedSettlementAuthorizationInTransaction(
		dbtx.Read(), order.TenantID, authorization,
	); err != nil {
		return nil, err
	}
	return nil, nil
}

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

func (op *OrderProcessor) processSettlementFundingBasisMessage(
	dbtx database.Tx,
	order *models.Order,
	message *npb.OrderMessage,
) (interface{}, error) {
	wire := new(pb.SettlementFundingBasisProposal)
	if err := message.Message.UnmarshalTo(wire); err != nil {
		return nil, err
	}
	basis, err := paymentintent.FundingBasisProposalFromProto(wire)
	if err != nil {
		return nil, err
	}
	if order == nil || order.Role() != models.RoleVendor || basis.OrderID != message.OrderID ||
		basis.OrderID != order.ID.String() || basis.QuoteIssuer != message.SenderPeerID {
		return nil, fmt.Errorf("settlement funding basis sender or order does not match signed order message")
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
	orderHash, err := order.OrderOpenCanonicalHash()
	if err != nil || orderOpen.BuyerID == nil || orderOpen.BuyerID.PeerID != basis.QuoteIssuer ||
		basis.OrderOpenHash != orderHash {
		return nil, fmt.Errorf("settlement funding basis does not match signed order buyer or hash")
	}
	if err := paymentintent.RetainReceivedFundingBasisProposalInTransaction(dbtx.Read(), order.TenantID, basis); err != nil {
		return nil, err
	}
	return nil, nil
}

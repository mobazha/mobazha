// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"fmt"

	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

// SettlementKeyOfferToProto verifies and converts an internal offer to its
// public order-message representation.
func SettlementKeyOfferToProto(offer models.SettlementKeyOffer) (*pb.SettlementKeyOffer, error) {
	if err := offer.Verify(); err != nil {
		return nil, err
	}
	return &pb.SettlementKeyOffer{
		Version: offer.Version, AuthorizationContextID: offer.AuthorizationContextID,
		OrderID: offer.OrderID, AttemptID: offer.AttemptID, ParticipantPeerID: offer.ParticipantPeerID,
		ParticipantRole: string(offer.ParticipantRole), RailID: offer.RailID, Purpose: offer.Purpose,
		KeyAlgorithm: offer.KeyAlgorithm,
		PublicKey:    append([]byte(nil), offer.PublicKey...), Signature: append([]byte(nil), offer.Signature...),
		ExpectedModeratorPeerID: offer.ExpectedModeratorPeerID, AmountAtomic: offer.AmountAtomic,
		ModeratorPayoutAddress: offer.ModeratorPayoutAddress, ModeratorFeeAmount: offer.ModeratorFeeAmount,
		BuyerRefundAddress: offer.BuyerRefundAddress,
		EscrowTimeoutHours: offer.EscrowTimeoutHours,
		EscrowUnlockUnix:   offer.EscrowUnlockUnix,
	}, nil
}

// SettlementKeyOfferFromProto converts and verifies a public order-message
// offer before it reaches persistence.
func SettlementKeyOfferFromProto(wire *pb.SettlementKeyOffer) (models.SettlementKeyOffer, error) {
	if wire == nil {
		return models.SettlementKeyOffer{}, fmt.Errorf("settlement key offer payload is required")
	}
	offer := models.SettlementKeyOffer{
		Version: wire.Version, AuthorizationContextID: wire.AuthorizationContextID,
		OrderID: wire.OrderID, AttemptID: wire.AttemptID, ParticipantPeerID: wire.ParticipantPeerID,
		ParticipantRole: models.SettlementParticipantRole(wire.ParticipantRole),
		RailID:          wire.RailID, Purpose: wire.Purpose,
		KeyAlgorithm: wire.KeyAlgorithm,
		PublicKey:    append([]byte(nil), wire.PublicKey...), Signature: append([]byte(nil), wire.Signature...),
		ExpectedModeratorPeerID: wire.ExpectedModeratorPeerID, AmountAtomic: wire.AmountAtomic,
		ModeratorPayoutAddress: wire.ModeratorPayoutAddress, ModeratorFeeAmount: wire.ModeratorFeeAmount,
		BuyerRefundAddress: wire.BuyerRefundAddress,
		EscrowTimeoutHours: wire.EscrowTimeoutHours,
		EscrowUnlockUnix:   wire.EscrowUnlockUnix,
	}
	if err := offer.Verify(); err != nil {
		return models.SettlementKeyOffer{}, err
	}
	return offer, nil
}

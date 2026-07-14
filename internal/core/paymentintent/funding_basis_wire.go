// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"encoding/json"
	"fmt"

	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

// FundingBasisProposalToProto verifies and converts a buyer proposal to its
// canonical order-message wire value.
func FundingBasisProposalToProto(basis models.PaymentAttemptFundingBasis) (*pb.SettlementFundingBasisProposal, error) {
	canonical, _, err := basis.CanonicalBytesAndHash()
	if err != nil {
		return nil, err
	}
	return &pb.SettlementFundingBasisProposal{
		Version: basis.Version, OrderID: basis.OrderID, AttemptID: basis.AttemptID, FundingBasis: canonical,
	}, nil
}

// FundingBasisProposalFromProto decodes and rejects non-canonical proposal
// payloads or mismatched envelope identity.
func FundingBasisProposalFromProto(wire *pb.SettlementFundingBasisProposal) (models.PaymentAttemptFundingBasis, error) {
	if wire == nil || len(wire.FundingBasis) == 0 {
		return models.PaymentAttemptFundingBasis{}, fmt.Errorf("settlement funding-basis payload is required")
	}
	var basis models.PaymentAttemptFundingBasis
	if err := json.Unmarshal(wire.FundingBasis, &basis); err != nil {
		return models.PaymentAttemptFundingBasis{}, fmt.Errorf("decode settlement funding basis: %w", err)
	}
	canonical, _, err := basis.CanonicalBytesAndHash()
	if err != nil {
		return models.PaymentAttemptFundingBasis{}, err
	}
	if wire.Version != basis.Version || wire.OrderID != basis.OrderID || wire.AttemptID != basis.AttemptID ||
		string(wire.FundingBasis) != string(canonical) {
		return models.PaymentAttemptFundingBasis{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	return basis, nil
}

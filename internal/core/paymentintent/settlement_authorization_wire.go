// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"encoding/json"
	"fmt"

	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

// SettlementAuthorizationToProto verifies and converts one complete public
// settlement authorization snapshot to its order-message wire value.
func SettlementAuthorizationToProto(
	authorization models.PaymentAttemptSettlementAuthorization,
) (*pb.SettlementAuthorization, error) {
	canonical, _, err := authorization.CanonicalBytesAndHash()
	if err != nil {
		return nil, err
	}
	return &pb.SettlementAuthorization{
		Version: authorization.Version, OrderID: authorization.Terms.OrderID,
		AttemptID: authorization.Terms.AttemptID, Authorization: canonical,
	}, nil
}

// SettlementAuthorizationFromProto decodes, verifies, and rejects any
// non-canonical complete settlement authorization snapshot.
func SettlementAuthorizationFromProto(
	wire *pb.SettlementAuthorization,
) (models.PaymentAttemptSettlementAuthorization, error) {
	if wire == nil || len(wire.Authorization) == 0 {
		return models.PaymentAttemptSettlementAuthorization{}, fmt.Errorf("settlement authorization payload is required")
	}
	var authorization models.PaymentAttemptSettlementAuthorization
	if err := json.Unmarshal(wire.Authorization, &authorization); err != nil {
		return models.PaymentAttemptSettlementAuthorization{}, fmt.Errorf("decode settlement authorization: %w", err)
	}
	canonical, _, err := authorization.CanonicalBytesAndHash()
	if err != nil {
		return models.PaymentAttemptSettlementAuthorization{}, err
	}
	if wire.Version != authorization.Version || wire.OrderID != authorization.Terms.OrderID ||
		wire.AttemptID != authorization.Terms.AttemptID || string(wire.Authorization) != string(canonical) {
		return models.PaymentAttemptSettlementAuthorization{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	return authorization, nil
}

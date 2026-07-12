// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"fmt"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
)

// NewSettlementSignRequest constructs a chain-adapter signing request only
// from a frozen crypto attempt. It prevents execution code from independently
// choosing a participant purpose, key context, or terms hash.
func NewSettlementSignRequest(
	attempt models.PaymentAttempt,
	keyRef contracts.SettlementKeyRef,
	role models.SettlementParticipantRole,
	domain, action string,
	sequence uint64,
	payload []byte,
) (contracts.SettlementSignRequest, error) {
	if attempt.Kind != models.PaymentAttemptKindCryptoFundingTarget || !role.Valid() {
		return contracts.SettlementSignRequest{}, fmt.Errorf("frozen crypto payment attempt and participant role are required")
	}
	if err := keyRef.Validate(); err != nil {
		return contracts.SettlementSignRequest{}, err
	}
	bundle, err := attempt.GetAuthorizationBundle()
	if err != nil {
		return contracts.SettlementSignRequest{}, err
	}
	if bundle == nil || keyRef.TenantID != attempt.TenantID ||
		keyRef.RailID != bundle.RailID || keyRef.ReferenceID != bundle.AuthorizationContextID {
		return contracts.SettlementSignRequest{}, fmt.Errorf("settlement key reference does not match frozen payment attempt")
	}
	for _, offer := range bundle.Offers {
		if offer.ParticipantRole == role {
			if keyRef.Purpose != offer.Purpose {
				return contracts.SettlementSignRequest{}, fmt.Errorf("settlement key reference purpose does not match frozen participant offer")
			}
			request := contracts.SettlementSignRequest{
				KeyRef: keyRef, Domain: domain, OrderID: bundle.OrderID, AttemptID: bundle.AttemptID,
				Action: action, Sequence: sequence, TermsHash: bundle.SettlementTermsHash, Payload: payload,
			}
			if err := request.Validate(); err != nil {
				return contracts.SettlementSignRequest{}, err
			}
			return request, nil
		}
	}
	return contracts.SettlementSignRequest{}, fmt.Errorf("frozen payment attempt has no offer for participant role %q", role)
}

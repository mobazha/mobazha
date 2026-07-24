// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package order

import (
	"bytes"
	"context"
	"strings"

	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type frozenSettlementAttemptActionContext struct {
	attempt       models.PaymentAttempt
	authorization models.PaymentAttemptSettlementAuthorization
	localOffer    models.SettlementKeyOffer
}

func (s *OrderAppService) frozenSettlementAttemptActionContext(
	ctx context.Context,
	order *models.Order,
	coin iwallet.CoinType,
) (*frozenSettlementAttemptActionContext, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, true, err
	}
	if s == nil || s.db == nil || s.signer == nil || s.settlementSigner == nil || order == nil {
		return nil, false, nil
	}
	tenantID := strings.TrimSpace(order.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(s.nodeID)
	}
	var attempts []models.PaymentAttempt
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"tenant_id = ? AND order_id = ? AND kind = ? AND state = ?",
			tenantID, order.ID.String(), models.PaymentAttemptKindCryptoFundingTarget,
			models.PaymentAttemptFundingTargetReady,
		).Find(&attempts).Error
	}); err != nil {
		return nil, true, err
	}
	if len(attempts) == 0 {
		return nil, false, nil
	}
	if len(attempts) != 1 || attempts[0].Currency != string(coin) {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	attempt := attempts[0]
	terms, err := attempt.GetSettlementTerms()
	if err != nil || terms == nil {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	target, err := attempt.GetFundingTarget()
	if err != nil || target == nil {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	bundle, err := attempt.GetAuthorizationBundle()
	if err != nil || bundle == nil {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	fundingBasis, err := attempt.GetFundingBasis()
	if err != nil {
		return nil, true, err
	}
	authorization := models.NewPaymentAttemptSettlementAuthorization(*terms, *target, *bundle, fundingBasis)
	if err := authorization.Validate(); err != nil {
		return nil, true, err
	}
	var localOffer *models.SettlementKeyOffer
	for i := range bundle.Offers {
		if bundle.Offers[i].ParticipantPeerID == s.signer.PeerID().String() {
			localOffer = &bundle.Offers[i]
			break
		}
	}
	if localOffer == nil {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	keyRef := contracts.SettlementKeyRef{
		TenantID: attempt.TenantID, RailID: attempt.Currency,
		Purpose:     contracts.StandardOrderSettlementKeyPurpose + ":" + string(localOffer.ParticipantRole),
		ReferenceID: attempt.AuthorizationContextID,
	}
	publicKey, _, err := paymentintent.SettlementPublicKeyForRail(ctx, s.settlementSigner, keyRef)
	if err != nil {
		return nil, true, err
	}
	if !bytes.Equal(publicKey, localOffer.PublicKey) {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	return &frozenSettlementAttemptActionContext{
		attempt: attempt, authorization: authorization, localOffer: *localOffer,
	}, true, nil
}

func applyFrozenSettlementAttemptActionParams(
	params *payment.ActionParams,
	context *frozenSettlementAttemptActionContext,
) {
	if params == nil || context == nil {
		return
	}
	authorization := context.authorization
	params.AttemptAuthorization = &authorization
	params.AttemptTenantID = context.attempt.TenantID
	params.AttemptLocalRole = context.localOffer.ParticipantRole
	params.AttemptSequence = 1
}

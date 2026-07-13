// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package payment

import (
	"strings"

	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
)

// loadOnrampFundingSource loads the attempt's onramp purchase history and
// selects the single record the session projection surfaces (ADR-019). It is
// nil-safe and fail-open in the read direction: no table, no attempt, or no
// relevant record all yield nil, which downstream refinement treats as a
// strict no-op.
func (p *PaymentSessionProjector) loadOnrampFundingSource(
	tenantID string,
	attempt *models.PaymentAttempt,
) *payment.OnrampFundingSourceView {
	if p == nil || p.db == nil || attempt == nil || strings.TrimSpace(attempt.AttemptID) == "" {
		return nil
	}
	var rows []models.PaymentAttemptOnrampFundingSource
	err := p.db.View(func(tx database.Tx) error {
		if !tx.Read().Migrator().HasTable(&models.PaymentAttemptOnrampFundingSource{}) {
			return nil
		}
		return tx.Read().
			Where("tenant_id = ? AND attempt_id = ?", tenantID, attempt.AttemptID).
			Order("updated_at DESC").
			Find(&rows).Error
	})
	if err != nil || len(rows) == 0 {
		return nil
	}
	views := make([]payment.OnrampFundingSourceView, 0, len(rows))
	for i := range rows {
		views = append(views, onrampSourceView(&rows[i]))
	}
	return payment.SelectOnrampFundingSource(views)
}

// onrampSourceView converts the durable record to its projection view.
func onrampSourceView(s *models.PaymentAttemptOnrampFundingSource) payment.OnrampFundingSourceView {
	updatedAt := s.UpdatedAt
	return payment.OnrampFundingSourceView{
		ProviderID:           s.ProviderID,
		OnrampOrderID:        s.OnrampOrderID,
		Status:               s.Status,
		DeliverToBuyerWallet: s.DeliverToBuyerWallet,
		BuyerWalletAddress:   s.BuyerWalletAddress,
		Disclosure:           s.Disclosure,
		UpdatedAt:            &updatedAt,
	}
}

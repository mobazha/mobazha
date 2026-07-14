// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BindCryptoPaymentAttemptFundingBasis atomically freezes a proposal on an
// existing local draft. Identical retries are accepted.
func BindCryptoPaymentAttemptFundingBasis(db *gorm.DB, tenantID, attemptID string, basis models.PaymentAttemptFundingBasis) (models.PaymentAttempt, error) {
	if db == nil {
		return models.PaymentAttempt{}, fmt.Errorf("bind payment attempt funding basis: database is required")
	}
	var attempt models.PaymentAttempt
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"tenant_id = ? AND attempt_id = ?", tenantID, attemptID,
		).First(&attempt).Error; err != nil {
			return fmt.Errorf("load payment attempt funding-basis draft: %w", err)
		}
		if attempt.State != models.PaymentAttemptAuthorizationDraft {
			return models.ErrPaymentAttemptSettlementTermsConflict
		}
		if err := attempt.SetFundingBasis(basis); err != nil {
			return err
		}
		result := tx.Model(&models.PaymentAttempt{}).Where(
			"tenant_id = ? AND attempt_id = ? AND state = ?", tenantID, attemptID, models.PaymentAttemptAuthorizationDraft,
		).Updates(map[string]interface{}{
			"funding_basis": attempt.FundingBasis, "funding_basis_hash": attempt.FundingBasisHash,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return models.ErrPaymentAttemptSettlementTermsConflict
		}
		return nil
	})
	return attempt, err
}

// RetainReceivedFundingBasisProposalInTransaction stores a verified buyer
// proposal before the seller creates or adopts its local attempt draft.
func RetainReceivedFundingBasisProposalInTransaction(tx *gorm.DB, tenantID string, basis models.PaymentAttemptFundingBasis) error {
	if tx == nil {
		return fmt.Errorf("retain settlement funding basis: transaction is required")
	}
	canonical, hash, err := basis.CanonicalBytesAndHash()
	if err != nil {
		return err
	}
	var existing models.PaymentAttemptFundingBasisProposalRecord
	err = tx.Session(&gorm.Session{NewDB: true}).Where(
		"tenant_id = ? AND attempt_id = ?", tenantID, basis.AttemptID,
	).First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return tx.Session(&gorm.Session{NewDB: true}).Create(&models.PaymentAttemptFundingBasisProposalRecord{
			TenantID: tenantID, AttemptID: basis.AttemptID, OrderID: basis.OrderID,
			FundingBasis: canonical, FundingBasisHash: hash, CreatedAt: time.Now().UTC(),
		}).Error
	case err != nil:
		return fmt.Errorf("load retained settlement funding basis: %w", err)
	case existing.FundingBasisHash != hash || string(existing.FundingBasis) != string(canonical):
		return models.ErrPaymentAttemptSettlementTermsConflict
	default:
		return nil
	}
}

// LoadRetainedFundingBasisProposal returns a re-verified seller inbox value.
func LoadRetainedFundingBasisProposal(db *gorm.DB, tenantID, attemptID string) (models.PaymentAttemptFundingBasis, error) {
	if db == nil || strings.TrimSpace(attemptID) == "" {
		return models.PaymentAttemptFundingBasis{}, fmt.Errorf("load retained settlement funding basis: invalid request")
	}
	var record models.PaymentAttemptFundingBasisProposalRecord
	if err := db.Session(&gorm.Session{NewDB: true}).Where(
		"tenant_id = ? AND attempt_id = ?", tenantID, attemptID,
	).First(&record).Error; err != nil {
		return models.PaymentAttemptFundingBasis{}, fmt.Errorf("load retained settlement funding basis: %w", err)
	}
	basis, err := record.Proposal()
	if err != nil {
		return models.PaymentAttemptFundingBasis{}, err
	}
	return *basis, nil
}

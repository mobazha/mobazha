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
)

// RetainReceivedSettlementAuthorizationInTransaction stores a verified final
// authorization snapshot for post-commit buyer adoption. Identical delivery
// is idempotent and replacement of an attempt snapshot fails closed.
func RetainReceivedSettlementAuthorizationInTransaction(
	tx *gorm.DB,
	tenantID string,
	authorization models.PaymentAttemptSettlementAuthorization,
) error {
	if tx == nil {
		return fmt.Errorf("retain received settlement authorization: transaction is required")
	}
	canonical, hash, err := authorization.CanonicalBytesAndHash()
	if err != nil {
		return err
	}
	var attempt models.PaymentAttempt
	if err := tx.Session(&gorm.Session{NewDB: true}).Where(
		"tenant_id = ? AND attempt_id = ?", tenantID, authorization.Terms.AttemptID,
	).First(&attempt).Error; err != nil {
		return fmt.Errorf("load local draft for settlement authorization: %w", err)
	}
	if attempt.OrderID != authorization.Terms.OrderID || attempt.AuthorizationContextID != authorization.Authorization.AuthorizationContextID ||
		attempt.Currency != authorization.Terms.AssetID || attempt.AmountValue != authorization.Terms.FundingAmount ||
		(attempt.State != models.PaymentAttemptAuthorizationDraft && attempt.State != models.PaymentAttemptFundingTargetReady) {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	localHasFundingBasis := len(attempt.FundingBasis) != 0 || strings.TrimSpace(attempt.FundingBasisHash) != ""
	if localHasFundingBasis != (authorization.FundingBasis != nil) {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	if authorization.FundingBasis != nil {
		basis, err := attempt.GetFundingBasis()
		if err != nil || basis == nil {
			return models.ErrPaymentAttemptSettlementTermsConflict
		}
		localBytes, _, err := basis.CanonicalBytesAndHash()
		if err != nil {
			return err
		}
		receivedBytes, _, err := authorization.FundingBasis.CanonicalBytesAndHash()
		if err != nil || string(localBytes) != string(receivedBytes) {
			return models.ErrPaymentAttemptSettlementTermsConflict
		}
	}
	var existing models.PaymentAttemptSettlementAuthorizationRecord
	err = tx.Session(&gorm.Session{NewDB: true}).Where(
		"tenant_id = ? AND attempt_id = ?", tenantID, attempt.AttemptID,
	).First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return tx.Session(&gorm.Session{NewDB: true}).Create(&models.PaymentAttemptSettlementAuthorizationRecord{
			TenantID: tenantID, AttemptID: attempt.AttemptID, OrderID: attempt.OrderID,
			Authorization: canonical, AuthorizationHash: hash, CreatedAt: time.Now().UTC(),
		}).Error
	case err != nil:
		return fmt.Errorf("load retained settlement authorization: %w", err)
	case existing.AuthorizationHash != hash || string(existing.Authorization) != string(canonical):
		return models.ErrPaymentAttemptSettlementTermsConflict
	default:
		return nil
	}
}

// LoadRetainedSettlementAuthorization returns one re-verified buyer inbox
// snapshot for post-commit adoption.
func LoadRetainedSettlementAuthorization(
	db *gorm.DB,
	tenantID, attemptID string,
) (models.PaymentAttemptSettlementAuthorization, error) {
	if db == nil || strings.TrimSpace(attemptID) == "" {
		return models.PaymentAttemptSettlementAuthorization{}, fmt.Errorf("load retained settlement authorization: invalid request")
	}
	var record models.PaymentAttemptSettlementAuthorizationRecord
	if err := db.Session(&gorm.Session{NewDB: true}).Where(
		"tenant_id = ? AND attempt_id = ?", tenantID, attemptID,
	).First(&record).Error; err != nil {
		return models.PaymentAttemptSettlementAuthorization{}, fmt.Errorf("load retained settlement authorization: %w", err)
	}
	authorization, err := record.SettlementAuthorization()
	if err != nil {
		return models.PaymentAttemptSettlementAuthorization{}, err
	}
	return *authorization, nil
}

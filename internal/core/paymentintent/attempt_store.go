// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateCryptoPaymentAttemptDraft persists a non-actionable crypto attempt
// before any seller or moderator key-offer request. Retried provisioning
// returns the same immutable authorization context.
func CreateCryptoPaymentAttemptDraft(
	db *gorm.DB,
	attempt models.PaymentAttempt,
	route models.PaymentRouteBinding,
) (models.PaymentAttempt, error) {
	if db == nil {
		return models.PaymentAttempt{}, fmt.Errorf("create crypto payment attempt draft: database is required")
	}
	if err := validateCryptoAttemptIdentity(attempt, route); err != nil {
		return models.PaymentAttempt{}, err
	}
	if attempt.AuthorizationContextID != "" {
		if err := attempt.SetAuthorizationContextID(attempt.AuthorizationContextID); err != nil {
			return models.PaymentAttempt{}, err
		}
	}
	requestedContextID := attempt.AuthorizationContextID
	attempt.State = models.PaymentAttemptAuthorizationDraft
	err := db.Transaction(func(tx *gorm.DB) error {
		var existing models.PaymentAttempt
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND attempt_id = ?", attempt.TenantID, attempt.AttemptID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if attempt.AuthorizationContextID == "" {
				contextID, err := models.NewSettlementAuthorizationContextID()
				if err != nil {
					return err
				}
				if err := attempt.SetAuthorizationContextID(contextID); err != nil {
					return err
				}
			}
			if err := tx.Create(&route).Error; err != nil {
				return fmt.Errorf("persist crypto payment route binding: %w", err)
			}
			return tx.Create(&attempt).Error
		}
		if err != nil {
			return fmt.Errorf("load crypto payment attempt draft: %w", err)
		}
		if !sameCryptoPaymentAttemptDraftRequest(existing, attempt) {
			return models.ErrPaymentAttemptSettlementTermsConflict
		}
		var existingRoute models.PaymentRouteBinding
		if err := tx.Where("tenant_id = ? AND route_binding_id = ?", route.TenantID, route.RouteBindingID).First(&existingRoute).Error; err != nil {
			return fmt.Errorf("load crypto payment route binding: %w", err)
		}
		if !samePaymentRouteBinding(existingRoute, route) {
			return models.ErrPaymentAttemptSettlementTermsConflict
		}
		attempt = existing
		return nil
	})
	if err != nil {
		// A concurrent creator may have committed the durable draft after this
		// transaction's initial read. In that case return the winner instead of
		// leaking a unique-constraint error to an idempotent caller.
		attempt.AuthorizationContextID = requestedContextID
		var existing models.PaymentAttempt
		if loadErr := db.Where("tenant_id = ? AND attempt_id = ?", attempt.TenantID, attempt.AttemptID).First(&existing).Error; loadErr == nil &&
			sameCryptoPaymentAttemptDraftRequest(existing, attempt) {
			var existingRoute models.PaymentRouteBinding
			if routeErr := db.Where("tenant_id = ? AND route_binding_id = ?", route.TenantID, route.RouteBindingID).First(&existingRoute).Error; routeErr == nil &&
				samePaymentRouteBinding(existingRoute, route) {
				return existing, nil
			}
		}
		return models.PaymentAttempt{}, err
	}
	return attempt, nil
}

// FreezeCryptoPaymentAttempt atomically upgrades one persisted authorization
// draft with seller-authorized terms, its verified key offers, and an
// actionable funding target. Identical retries are accepted; any mutation of
// the frozen attempt fails.
func FreezeCryptoPaymentAttempt(
	db *gorm.DB,
	attempt models.PaymentAttempt,
	route models.PaymentRouteBinding,
	terms models.PaymentAttemptSettlementTerms,
	sellerSigner string,
	sellerSignature []byte,
	authorization models.PaymentAttemptAuthorizationBundle,
	target models.PaymentAttemptFundingTarget,
) error {
	if db == nil {
		return fmt.Errorf("freeze crypto payment attempt: database is required")
	}
	if err := validateCryptoAttemptIdentity(attempt, route); err != nil {
		return err
	}
	if err := attempt.SetSettlementTerms(terms); err != nil {
		return fmt.Errorf("freeze crypto payment attempt terms: %w", err)
	}
	if attempt.AmountValue != terms.FundingAmount || attempt.Currency != terms.AssetID ||
		route.AssetID != terms.AssetID || route.RouteBindingID != terms.RouteBindingID {
		return fmt.Errorf("crypto payment attempt does not match settlement terms")
	}
	if err := attempt.SetSellerTermsAuthorization(sellerSigner, sellerSignature); err != nil {
		return fmt.Errorf("freeze crypto payment attempt seller authorization: %w", err)
	}
	if err := attempt.SetAuthorizationBundle(authorization); err != nil {
		return fmt.Errorf("freeze crypto payment attempt authorization bundle: %w", err)
	}
	if err := attempt.SetFundingTarget(target); err != nil {
		return fmt.Errorf("freeze crypto payment attempt target: %w", err)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var existing models.PaymentAttempt
		err := tx.Where("tenant_id = ? AND attempt_id = ?", attempt.TenantID, attempt.AttemptID).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			return fmt.Errorf("crypto payment authorization draft is required before freezing")
		case err != nil:
			return fmt.Errorf("load crypto payment attempt: %w", err)
		}
		var existingRoute models.PaymentRouteBinding
		if err := tx.Where("tenant_id = ? AND route_binding_id = ?", route.TenantID, route.RouteBindingID).First(&existingRoute).Error; err != nil {
			return fmt.Errorf("load crypto payment route binding: %w", err)
		}
		if !samePaymentRouteBinding(existingRoute, route) {
			return models.ErrPaymentAttemptSettlementTermsConflict
		}
		if existing.State == models.PaymentAttemptAuthorizationDraft {
			if !sameCryptoPaymentAttemptIdentity(existing, attempt) {
				return models.ErrPaymentAttemptSettlementTermsConflict
			}
			result := tx.Model(&models.PaymentAttempt{}).
				Where("tenant_id = ? AND attempt_id = ? AND state = ?", attempt.TenantID, attempt.AttemptID, models.PaymentAttemptAuthorizationDraft).
				Select("*").Updates(&attempt)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				return deleteCryptoPaymentAttemptDraftOffers(tx, attempt.TenantID, attempt.AttemptID)
			}
			// Another transaction froze this draft after our read. Never overwrite
			// it: accept only an identical durable snapshot for an idempotent retry.
			if err := tx.Where("tenant_id = ? AND attempt_id = ?", attempt.TenantID, attempt.AttemptID).First(&existing).Error; err != nil {
				return fmt.Errorf("reload concurrently frozen crypto payment attempt: %w", err)
			}
			if err := verifyFrozenCryptoAttempt(existing, attempt); err != nil {
				return err
			}
			return deleteCryptoPaymentAttemptDraftOffers(tx, attempt.TenantID, attempt.AttemptID)
		}
		if err := verifyFrozenCryptoAttempt(existing, attempt); err != nil {
			return err
		}
		return deleteCryptoPaymentAttemptDraftOffers(tx, attempt.TenantID, attempt.AttemptID)
	})
}

func deleteCryptoPaymentAttemptDraftOffers(tx *gorm.DB, tenantID, attemptID string) error {
	result := tx.Where("tenant_id = ? AND attempt_id = ?", tenantID, attemptID).
		Delete(&models.PaymentAttemptSettlementOffer{})
	if result.Error != nil {
		return fmt.Errorf("delete frozen payment attempt draft offers: %w", result.Error)
	}
	return nil
}

func sameCryptoPaymentAttemptIdentity(left, right models.PaymentAttempt) bool {
	return left.TenantID == right.TenantID && left.AttemptID == right.AttemptID &&
		left.Kind == right.Kind && left.PaymentSessionID == right.PaymentSessionID &&
		left.OrderID == right.OrderID && left.AmountValue == right.AmountValue &&
		left.Currency == right.Currency && left.RouteBindingID == right.RouteBindingID &&
		left.IdempotencyKey == right.IdempotencyKey &&
		left.AuthorizationContextID == right.AuthorizationContextID
}

func sameCryptoPaymentAttemptDraftRequest(existing, requested models.PaymentAttempt) bool {
	if requested.AuthorizationContextID != "" && existing.AuthorizationContextID != requested.AuthorizationContextID {
		return false
	}
	requested.AuthorizationContextID = existing.AuthorizationContextID
	return sameCryptoPaymentAttemptIdentity(existing, requested)
}

func samePaymentRouteBinding(left, right models.PaymentRouteBinding) bool {
	return left.TenantID == right.TenantID && left.RouteBindingID == right.RouteBindingID &&
		left.AttemptID == right.AttemptID && left.ContributionID == right.ContributionID &&
		left.ModuleID == right.ModuleID && left.ImplementationGeneration == right.ImplementationGeneration &&
		left.RailKind == right.RailKind && left.NetworkID == right.NetworkID &&
		left.AssetID == right.AssetID && left.ProtocolVersion == right.ProtocolVersion &&
		left.StateSchemaVersion == right.StateSchemaVersion &&
		left.ProviderBindingID == right.ProviderBindingID &&
		left.ExternalAccountReference == right.ExternalAccountReference
}

func validateCryptoAttemptIdentity(attempt models.PaymentAttempt, route models.PaymentRouteBinding) error {
	if attempt.Kind != models.PaymentAttemptKindCryptoFundingTarget ||
		strings.TrimSpace(attempt.AttemptID) == "" || strings.TrimSpace(attempt.PaymentSessionID) == "" ||
		strings.TrimSpace(attempt.OrderID) == "" || strings.TrimSpace(attempt.RouteBindingID) == "" ||
		strings.TrimSpace(attempt.IdempotencyKey) == "" ||
		attempt.RouteBindingID != route.RouteBindingID || attempt.AttemptID != route.AttemptID ||
		attempt.TenantID != route.TenantID || strings.TrimSpace(route.AssetID) == "" ||
		strings.TrimSpace(route.ContributionID) == "" || strings.TrimSpace(route.ModuleID) == "" ||
		strings.TrimSpace(route.ImplementationGeneration) == "" || strings.TrimSpace(route.RailKind) == "" ||
		strings.TrimSpace(route.NetworkID) == "" || strings.TrimSpace(route.ProtocolVersion) == "" ||
		strings.TrimSpace(route.StateSchemaVersion) == "" {
		return fmt.Errorf("invalid crypto payment attempt route identity")
	}
	return nil
}

func verifyFrozenCryptoAttempt(existing, expected models.PaymentAttempt) error {
	if existing.Kind != expected.Kind || existing.PaymentSessionID != expected.PaymentSessionID ||
		existing.OrderID != expected.OrderID || existing.AmountValue != expected.AmountValue ||
		existing.Currency != expected.Currency || existing.RouteBindingID != expected.RouteBindingID ||
		existing.IdempotencyKey != expected.IdempotencyKey || existing.State != expected.State ||
		existing.SettlementTermsHash != expected.SettlementTermsHash ||
		string(existing.SettlementTerms) != string(expected.SettlementTerms) ||
		existing.SellerTermsSigner != expected.SellerTermsSigner ||
		string(existing.SellerTermsSignature) != string(expected.SellerTermsSignature) ||
		existing.AuthorizationContextID != expected.AuthorizationContextID ||
		existing.AuthorizationBundleHash != expected.AuthorizationBundleHash ||
		string(existing.AuthorizationBundle) != string(expected.AuthorizationBundle) ||
		existing.FundingTargetHash != expected.FundingTargetHash ||
		string(existing.FundingTarget) != string(expected.FundingTarget) {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	return nil
}

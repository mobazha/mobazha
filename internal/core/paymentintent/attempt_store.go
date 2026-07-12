// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
)

// FreezeCryptoPaymentAttempt atomically persists one shared crypto route,
// seller-authorized settlement terms, and its actionable funding target.
// Identical retries are accepted; any mutation of the frozen attempt fails.
func FreezeCryptoPaymentAttempt(
	db *gorm.DB,
	attempt models.PaymentAttempt,
	route models.PaymentRouteBinding,
	terms models.PaymentAttemptSettlementTerms,
	sellerSigner string,
	sellerSignature []byte,
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
	if err := attempt.SetFundingTarget(target); err != nil {
		return fmt.Errorf("freeze crypto payment attempt target: %w", err)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var existing models.PaymentAttempt
		err := tx.Where("tenant_id = ? AND attempt_id = ?", attempt.TenantID, attempt.AttemptID).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := tx.Create(&route).Error; err != nil {
				return fmt.Errorf("persist crypto payment route binding: %w", err)
			}
			if err := tx.Create(&attempt).Error; err != nil {
				return fmt.Errorf("persist crypto payment attempt: %w", err)
			}
			return nil
		case err != nil:
			return fmt.Errorf("load crypto payment attempt: %w", err)
		}

		if err := verifyFrozenCryptoAttempt(existing, attempt); err != nil {
			return err
		}
		var existingRoute models.PaymentRouteBinding
		if err := tx.Where("tenant_id = ? AND route_binding_id = ?", route.TenantID, route.RouteBindingID).First(&existingRoute).Error; err != nil {
			return fmt.Errorf("load crypto payment route binding: %w", err)
		}
		if !samePaymentRouteBinding(existingRoute, route) {
			return models.ErrPaymentAttemptSettlementTermsConflict
		}
		return nil
	})
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
		existing.FundingTargetHash != expected.FundingTargetHash ||
		string(existing.FundingTarget) != string(expected.FundingTarget) {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	return nil
}

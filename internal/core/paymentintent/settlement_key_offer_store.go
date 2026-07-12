// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StoreCryptoPaymentAttemptSettlementKeyOffer retains a verified participant
// offer for a persisted authorization draft. Retrying the exact signed offer
// is idempotent; replacing a role or reusing another role's public key fails.
func StoreCryptoPaymentAttemptSettlementKeyOffer(
	db *gorm.DB,
	tenantID, attemptID string,
	offer models.SettlementKeyOffer,
) error {
	if db == nil {
		return fmt.Errorf("store settlement key offer: database is required")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		return StoreCryptoPaymentAttemptSettlementKeyOfferInTransaction(tx, tenantID, attemptID, offer)
	})
}

// StoreCryptoPaymentAttemptSettlementKeyOfferInTransaction retains an offer
// using the caller's existing order transaction.
func StoreCryptoPaymentAttemptSettlementKeyOfferInTransaction(
	tx *gorm.DB,
	tenantID, attemptID string,
	offer models.SettlementKeyOffer,
) error {
	if tx == nil {
		return fmt.Errorf("store settlement key offer: transaction is required")
	}
	canonical, hash, err := offer.CanonicalBytesAndHash()
	if err != nil {
		return err
	}
	var attempt models.PaymentAttempt
	if err := tx.Session(&gorm.Session{NewDB: true}).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND attempt_id = ?", tenantID, attemptID).First(&attempt).Error; err != nil {
		return fmt.Errorf("load crypto payment attempt for settlement key offer: %w", err)
	}
	if attempt.Kind != models.PaymentAttemptKindCryptoFundingTarget || attempt.State != models.PaymentAttemptAuthorizationDraft ||
		offer.OrderID != attempt.OrderID || offer.AttemptID != attempt.AttemptID ||
		offer.AuthorizationContextID != attempt.AuthorizationContextID || offer.RailID != attempt.Currency {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}

	var existing models.PaymentAttemptSettlementOffer
	err = tx.Session(&gorm.Session{NewDB: true}).
		Where("tenant_id = ? AND attempt_id = ? AND participant_role = ?", tenantID, attemptID, offer.ParticipantRole).
		First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		var records []models.PaymentAttemptSettlementOffer
		if err := tx.Session(&gorm.Session{NewDB: true}).
			Where("tenant_id = ? AND attempt_id = ?", tenantID, attemptID).Find(&records).Error; err != nil {
			return fmt.Errorf("list existing settlement key offers: %w", err)
		}
		for _, record := range records {
			stored, err := record.SettlementKeyOffer()
			if err != nil {
				return err
			}
			if string(stored.PublicKey) == string(offer.PublicKey) {
				return fmt.Errorf("settlement key offer public key is already retained for role %q", stored.ParticipantRole)
			}
		}
		publicKeyDigest := sha256.Sum256(offer.PublicKey)
		return tx.Session(&gorm.Session{NewDB: true}).Create(&models.PaymentAttemptSettlementOffer{
			TenantID: tenantID, AttemptID: attemptID, ParticipantRole: offer.ParticipantRole,
			Offer: canonical, OfferHash: hash, PublicKeyHash: hex.EncodeToString(publicKeyDigest[:]),
		}).Error
	case err != nil:
		return fmt.Errorf("load existing settlement key offer: %w", err)
	case string(existing.Offer) != string(canonical) || existing.OfferHash != hash:
		return models.ErrPaymentAttemptSettlementTermsConflict
	default:
		return nil
	}
}

// ListCryptoPaymentAttemptSettlementKeyOffers returns the verified draft
// offers in role order. It never treats draft offers as an actionable bundle.
func ListCryptoPaymentAttemptSettlementKeyOffers(
	db *gorm.DB,
	tenantID, attemptID string,
) ([]models.SettlementKeyOffer, error) {
	if db == nil {
		return nil, fmt.Errorf("list settlement key offers: database is required")
	}
	var attempt models.PaymentAttempt
	if err := db.Where("tenant_id = ? AND attempt_id = ?", tenantID, attemptID).First(&attempt).Error; err != nil {
		return nil, fmt.Errorf("load crypto payment attempt for settlement key offers: %w", err)
	}
	if attempt.Kind != models.PaymentAttemptKindCryptoFundingTarget || attempt.State != models.PaymentAttemptAuthorizationDraft {
		return nil, models.ErrPaymentAttemptSettlementTermsConflict
	}
	var records []models.PaymentAttemptSettlementOffer
	if err := db.Where("tenant_id = ? AND attempt_id = ?", tenantID, attemptID).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list settlement key offers: %w", err)
	}
	offers := make([]models.SettlementKeyOffer, 0, len(records))
	publicKeys := make(map[string]models.SettlementParticipantRole, len(records))
	for _, record := range records {
		offer, err := record.SettlementKeyOffer()
		if err != nil {
			return nil, err
		}
		if offer.OrderID != attempt.OrderID || offer.AttemptID != attempt.AttemptID ||
			offer.AuthorizationContextID != attempt.AuthorizationContextID || offer.RailID != attempt.Currency {
			return nil, models.ErrPaymentAttemptSettlementTermsConflict
		}
		if otherRole, exists := publicKeys[string(offer.PublicKey)]; exists {
			return nil, fmt.Errorf("settlement key offer public key is retained for both %q and %q", otherRole, offer.ParticipantRole)
		}
		publicKeys[string(offer.PublicKey)] = offer.ParticipantRole
		offers = append(offers, *offer)
	}
	sort.Slice(offers, func(i, j int) bool { return offers[i].ParticipantRole < offers[j].ParticipantRole })
	return offers, nil
}

// BuildCryptoPaymentAttemptAuthorizationBundle creates the only complete
// authorization bundle from a draft's retained offers and supplied immutable
// settlement terms. It does not freeze the attempt; callers pass the result to
// FreezeCryptoPaymentAttempt for the atomic state transition.
func BuildCryptoPaymentAttemptAuthorizationBundle(
	db *gorm.DB,
	tenantID, attemptID string,
	terms models.PaymentAttemptSettlementTerms,
	sellerSigner string,
	sellerSignature []byte,
	target models.PaymentAttemptFundingTarget,
) (models.PaymentAttemptAuthorizationBundle, error) {
	if db == nil {
		return models.PaymentAttemptAuthorizationBundle{}, fmt.Errorf("build authorization bundle: database is required")
	}
	var attempt models.PaymentAttempt
	if err := db.Where("tenant_id = ? AND attempt_id = ?", tenantID, attemptID).First(&attempt).Error; err != nil {
		return models.PaymentAttemptAuthorizationBundle{}, fmt.Errorf("load crypto payment attempt for authorization bundle: %w", err)
	}
	if attempt.Kind != models.PaymentAttemptKindCryptoFundingTarget || attempt.State != models.PaymentAttemptAuthorizationDraft ||
		terms.OrderID != attempt.OrderID || terms.AttemptID != attempt.AttemptID ||
		terms.AssetID != attempt.Currency || target.AttemptID != attempt.AttemptID ||
		target.AssetID != terms.AssetID || target.AmountAtomic != terms.FundingAmount || target.Address != terms.FundingTargetAddress {
		return models.PaymentAttemptAuthorizationBundle{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	_, termsHash, err := terms.CanonicalBytesAndHash()
	if err != nil {
		return models.PaymentAttemptAuthorizationBundle{}, err
	}
	if err := terms.VerifySellerAuthorization(sellerSigner, sellerSignature); err != nil {
		return models.PaymentAttemptAuthorizationBundle{}, err
	}
	_, targetHash, err := target.CanonicalBytesAndHash()
	if err != nil {
		return models.PaymentAttemptAuthorizationBundle{}, err
	}
	offers, err := ListCryptoPaymentAttemptSettlementKeyOffers(db, tenantID, attemptID)
	if err != nil {
		return models.PaymentAttemptAuthorizationBundle{}, err
	}
	expectedPeers := map[models.SettlementParticipantRole]string{
		models.SettlementParticipantBuyer:  terms.BuyerPeerID,
		models.SettlementParticipantSeller: terms.SellerPeerID,
	}
	if terms.ModeratorPeerID != "" {
		expectedPeers[models.SettlementParticipantModerator] = terms.ModeratorPeerID
	}
	if len(offers) != len(expectedPeers) {
		return models.PaymentAttemptAuthorizationBundle{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	requiredRoles := make([]models.SettlementParticipantRole, 0, len(expectedPeers))
	seenRoles := make(map[models.SettlementParticipantRole]struct{}, len(offers))
	for _, offer := range offers {
		expectedPeerID, ok := expectedPeers[offer.ParticipantRole]
		if !ok || offer.ParticipantPeerID != expectedPeerID {
			return models.PaymentAttemptAuthorizationBundle{}, models.ErrPaymentAttemptSettlementTermsConflict
		}
		seenRoles[offer.ParticipantRole] = struct{}{}
	}
	for role := range expectedPeers {
		if _, exists := seenRoles[role]; !exists {
			return models.PaymentAttemptAuthorizationBundle{}, models.ErrPaymentAttemptSettlementTermsConflict
		}
		requiredRoles = append(requiredRoles, role)
	}
	sort.Slice(requiredRoles, func(i, j int) bool { return requiredRoles[i] < requiredRoles[j] })
	bundle := models.PaymentAttemptAuthorizationBundle{
		Version:                models.SettlementAuthorizationVersion,
		AuthorizationContextID: attempt.AuthorizationContextID,
		OrderID:                attempt.OrderID,
		AttemptID:              attempt.AttemptID,
		RailID:                 attempt.Currency,
		SettlementTermsHash:    termsHash,
		FundingTargetHash:      targetHash,
		RequiredRoles:          requiredRoles,
		Offers:                 offers,
		SellerTermsSigner:      sellerSigner,
		SellerTermsSignature:   append([]byte(nil), sellerSignature...),
	}
	if _, _, err := bundle.CanonicalBytesAndHash(); err != nil {
		return models.PaymentAttemptAuthorizationBundle{}, err
	}
	return bundle, nil
}

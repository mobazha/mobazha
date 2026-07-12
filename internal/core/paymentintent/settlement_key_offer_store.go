// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RetainedSettlementKeyOfferMaxAge is the local inbox retention period for a
// verified offer that arrived before a matching local attempt exists. It is
// storage hygiene only: it does not add a protocol TTL to SettlementKeyOffer
// and does not invalidate an offer retained by an active authorization draft.
const RetainedSettlementKeyOfferMaxAge = 7 * 24 * time.Hour

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
		offer.AuthorizationContextID != attempt.AuthorizationContextID || offer.RailID != attempt.Currency ||
		strings.TrimSpace(offer.ExpectedModeratorPeerID) != strings.TrimSpace(attempt.ExpectedModeratorPeerID) ||
		(attempt.ExpectedModeratorPeerID != "" && strings.TrimSpace(offer.AmountAtomic) != strings.TrimSpace(attempt.AmountValue)) {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	return retainSettlementKeyOfferRecord(tx, tenantID, offer, canonical, hash)
}

// RetainReceivedSettlementKeyOfferInTransaction retains a verified inbound
// offer even when the receiving node has not yet materialized its local draft.
// If the draft already exists, the stricter draft-state checks are applied.
func RetainReceivedSettlementKeyOfferInTransaction(
	tx *gorm.DB,
	tenantID string,
	offer models.SettlementKeyOffer,
) error {
	if tx == nil {
		return fmt.Errorf("retain received settlement key offer: transaction is required")
	}
	canonical, hash, err := offer.CanonicalBytesAndHash()
	if err != nil {
		return err
	}
	var attempt models.PaymentAttempt
	err = tx.Session(&gorm.Session{NewDB: true}).
		Where("tenant_id = ? AND attempt_id = ?", tenantID, offer.AttemptID).First(&attempt).Error
	switch {
	case err == nil:
		return StoreCryptoPaymentAttemptSettlementKeyOfferInTransaction(tx, tenantID, offer.AttemptID, offer)
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return fmt.Errorf("load local draft for received settlement key offer: %w", err)
	default:
		return retainSettlementKeyOfferRecord(tx, tenantID, offer, canonical, hash)
	}
}

// LoadRetainedSettlementKeyOffer returns one re-verified inbox offer without
// requiring a local attempt draft. Seller-side responders use it to adopt the
// buyer's already-persisted attempt and authorization context.
func LoadRetainedSettlementKeyOffer(
	db *gorm.DB,
	tenantID, attemptID string,
	role models.SettlementParticipantRole,
) (models.SettlementKeyOffer, error) {
	if db == nil || strings.TrimSpace(attemptID) == "" || !role.Valid() {
		return models.SettlementKeyOffer{}, fmt.Errorf("load retained settlement key offer: invalid request")
	}
	var record models.PaymentAttemptSettlementOffer
	if err := db.Session(&gorm.Session{NewDB: true}).Where(
		"tenant_id = ? AND attempt_id = ? AND participant_role = ?", tenantID, attemptID, role,
	).First(&record).Error; err != nil {
		return models.SettlementKeyOffer{}, fmt.Errorf("load retained settlement key offer: %w", err)
	}
	offer, err := record.SettlementKeyOffer()
	if err != nil {
		return models.SettlementKeyOffer{}, err
	}
	return *offer, nil
}

// PruneStaleRetainedSettlementKeyOffers removes local inbox records that can
// no longer contribute to an authorization draft. It removes records older
// than before only when no matching local attempt exists, and always removes
// residual records for attempts that are terminal or already frozen. Active
// authorization drafts are deliberately retained regardless of age because
// SettlementKeyOffer has no protocol expiry in the first release.
func PruneStaleRetainedSettlementKeyOffers(
	db *gorm.DB,
	before time.Time,
) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("prune settlement key offers: database is required")
	}
	var deleted int64
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		deleted, err = PruneStaleRetainedSettlementKeyOffersInTransaction(tx, before)
		return err
	}); err != nil {
		return 0, err
	}
	return deleted, nil
}

// PruneStaleRetainedSettlementKeyOffersInTransaction is the transactional
// form of PruneStaleRetainedSettlementKeyOffers.
func PruneStaleRetainedSettlementKeyOffersInTransaction(
	tx *gorm.DB,
	before time.Time,
) (int64, error) {
	if tx == nil {
		return 0, fmt.Errorf("prune settlement key offers: transaction is required")
	}
	if before.IsZero() {
		return 0, fmt.Errorf("prune settlement key offers: cutoff is required")
	}

	const attemptMissing = `
NOT EXISTS (
	SELECT 1 FROM payment_attempts
	WHERE payment_attempts.tenant_id = payment_attempt_settlement_offers.tenant_id
	  AND payment_attempts.attempt_id = payment_attempt_settlement_offers.attempt_id
)`
	const terminalOrFrozenAttempt = `
EXISTS (
	SELECT 1 FROM payment_attempts
	WHERE payment_attempts.tenant_id = payment_attempt_settlement_offers.tenant_id
	  AND payment_attempts.attempt_id = payment_attempt_settlement_offers.attempt_id
	  AND payment_attempts.state IN ?
)`

	// Keep any tenant filter carried by the caller's transaction. Unlike the
	// offer read/write paths this statement targets only one model, so a fresh
	// NewDB session is neither needed nor safe: it would widen a tenant-scoped
	// cleanup into a cross-tenant delete.
	result := tx.Where(
		`(payment_attempt_settlement_offers.created_at < ? AND `+attemptMissing+") OR "+terminalOrFrozenAttempt,
		before,
		[]string{
			models.PaymentAttemptExpired,
			models.PaymentAttemptAbandoned,
			models.PaymentAttemptFundingTargetReady,
		},
	).Delete(&models.PaymentAttemptSettlementOffer{})
	if result.Error != nil {
		return 0, fmt.Errorf("prune stale settlement key offers: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func retainSettlementKeyOfferRecord(
	tx *gorm.DB,
	tenantID string,
	offer models.SettlementKeyOffer,
	canonical []byte,
	hash string,
) error {
	var existing models.PaymentAttemptSettlementOffer
	err := tx.Session(&gorm.Session{NewDB: true}).
		Where("tenant_id = ? AND attempt_id = ? AND participant_role = ?", tenantID, offer.AttemptID, offer.ParticipantRole).
		First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		var records []models.PaymentAttemptSettlementOffer
		if err := tx.Session(&gorm.Session{NewDB: true}).
			Where("tenant_id = ? AND attempt_id = ?", tenantID, offer.AttemptID).Find(&records).Error; err != nil {
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
			TenantID: tenantID, AttemptID: offer.AttemptID, ParticipantRole: offer.ParticipantRole,
			OrderID: offer.OrderID, AuthorizationContextID: offer.AuthorizationContextID, RailID: offer.RailID,
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
		terms.AssetID != attempt.Currency || terms.ModeratorPeerID != attempt.ExpectedModeratorPeerID ||
		target.AttemptID != attempt.AttemptID ||
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
	if err := models.ValidateSettlementTermsOfferBindings(terms, offers); err != nil {
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

// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PaymentAttemptSettlementOffer is one verified, inbound participant offer
// retained while a crypto payment attempt is still an authorization draft.
// The final PaymentAttemptAuthorizationBundle remains the only actionable
// authorization record.
type PaymentAttemptSettlementOffer struct {
	TenantID               string                    `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:idx_payment_attempt_offer_public_key,priority:1"`
	AttemptID              string                    `gorm:"column:attempt_id;primaryKey;size:64;uniqueIndex:idx_payment_attempt_offer_public_key,priority:2"`
	ParticipantRole        SettlementParticipantRole `gorm:"column:participant_role;primaryKey;size:16"`
	OrderID                string                    `gorm:"column:order_id;size:255;not null;index:idx_payment_attempt_offer_scope,priority:1"`
	AuthorizationContextID string                    `gorm:"column:authorization_context_id;size:64;not null;index:idx_payment_attempt_offer_scope,priority:2"`
	RailID                 string                    `gorm:"column:rail_id;size:255;not null"`
	Offer                  []byte                    `gorm:"column:offer;type:text;not null"`
	OfferHash              string                    `gorm:"column:offer_hash;size:64;not null"`
	PublicKeyHash          string                    `gorm:"column:public_key_hash;size:64;not null;uniqueIndex:idx_payment_attempt_offer_public_key,priority:3"`
	CreatedAt              time.Time
}

func (PaymentAttemptSettlementOffer) TableName() string { return "payment_attempt_settlement_offers" }

// SettlementKeyOffer decodes and re-verifies the offer retained by this row.
func (r PaymentAttemptSettlementOffer) SettlementKeyOffer() (*SettlementKeyOffer, error) {
	if strings.TrimSpace(r.AttemptID) == "" || strings.TrimSpace(r.OrderID) == "" ||
		strings.TrimSpace(r.AuthorizationContextID) == "" || strings.TrimSpace(r.RailID) == "" ||
		!r.ParticipantRole.Valid() || len(r.Offer) == 0 ||
		strings.TrimSpace(r.OfferHash) == "" || strings.TrimSpace(r.PublicKeyHash) == "" {
		return nil, fmt.Errorf("invalid payment attempt settlement offer record")
	}
	var offer SettlementKeyOffer
	if err := json.Unmarshal(r.Offer, &offer); err != nil {
		return nil, fmt.Errorf("decode payment attempt settlement offer: %w", err)
	}
	canonical, hash, err := offer.CanonicalBytesAndHash()
	if err != nil {
		return nil, err
	}
	publicKeyDigest := sha256.Sum256(offer.PublicKey)
	if offer.OrderID != r.OrderID || offer.AttemptID != r.AttemptID ||
		offer.AuthorizationContextID != r.AuthorizationContextID || offer.RailID != r.RailID ||
		offer.ParticipantRole != r.ParticipantRole || string(canonical) != string(r.Offer) || hash != r.OfferHash ||
		hex.EncodeToString(publicKeyDigest[:]) != r.PublicKeyHash {
		return nil, ErrPaymentAttemptSettlementTermsConflict
	}
	return &offer, nil
}

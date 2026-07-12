// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
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
	TenantID        string                    `gorm:"column:tenant_id;primaryKey;default:''"`
	AttemptID       string                    `gorm:"column:attempt_id;primaryKey;size:64"`
	ParticipantRole SettlementParticipantRole `gorm:"column:participant_role;primaryKey;size:16"`
	Offer           []byte                    `gorm:"column:offer;type:text;not null"`
	OfferHash       string                    `gorm:"column:offer_hash;size:64;not null"`
	CreatedAt       time.Time
}

func (PaymentAttemptSettlementOffer) TableName() string { return "payment_attempt_settlement_offers" }

// SettlementKeyOffer decodes and re-verifies the offer retained by this row.
func (r PaymentAttemptSettlementOffer) SettlementKeyOffer() (*SettlementKeyOffer, error) {
	if strings.TrimSpace(r.AttemptID) == "" || !r.ParticipantRole.Valid() || len(r.Offer) == 0 || strings.TrimSpace(r.OfferHash) == "" {
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
	if offer.ParticipantRole != r.ParticipantRole || string(canonical) != string(r.Offer) || hash != r.OfferHash {
		return nil, ErrPaymentAttemptSettlementTermsConflict
	}
	return &offer, nil
}

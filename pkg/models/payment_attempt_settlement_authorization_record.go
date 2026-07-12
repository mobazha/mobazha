// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"encoding/json"
	"strings"
	"time"
)

// PaymentAttemptSettlementAuthorizationRecord is the buyer inbox copy of one
// seller-frozen public authorization snapshot. The attempt adopts it only
// after chain-specific deterministic verification outside the message tx.
type PaymentAttemptSettlementAuthorizationRecord struct {
	TenantID          string    `gorm:"column:tenant_id;primaryKey;default:''"`
	AttemptID         string    `gorm:"column:attempt_id;primaryKey;size:128"`
	OrderID           string    `gorm:"column:order_id;size:255;not null;index"`
	Authorization     []byte    `gorm:"column:authorization;type:text;not null"`
	AuthorizationHash string    `gorm:"column:authorization_hash;size:64;not null;index"`
	CreatedAt         time.Time `gorm:"column:created_at;not null;index"`
}

// TableName returns the stable settlement authorization inbox table name.
func (PaymentAttemptSettlementAuthorizationRecord) TableName() string {
	return "payment_attempt_settlement_authorizations"
}

// SettlementAuthorization decodes and re-verifies the canonical inbox value.
func (r PaymentAttemptSettlementAuthorizationRecord) SettlementAuthorization() (*PaymentAttemptSettlementAuthorization, error) {
	var authorization PaymentAttemptSettlementAuthorization
	if err := json.Unmarshal(r.Authorization, &authorization); err != nil {
		return nil, ErrPaymentAttemptSettlementTermsConflict
	}
	canonical, hash, err := authorization.CanonicalBytesAndHash()
	if err != nil || string(canonical) != string(r.Authorization) ||
		hash != strings.TrimSpace(r.AuthorizationHash) || authorization.Terms.AttemptID != r.AttemptID ||
		authorization.Terms.OrderID != r.OrderID {
		return nil, ErrPaymentAttemptSettlementTermsConflict
	}
	return &authorization, nil
}

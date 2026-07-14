// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"encoding/json"
	"strings"
	"time"
)

// PaymentAttemptFundingBasisProposalRecord is the seller inbox copy of one
// buyer-authored funding-basis proposal. It is non-actionable until the seller
// validates and binds it into seller-signed settlement terms.
type PaymentAttemptFundingBasisProposalRecord struct {
	TenantID         string    `gorm:"column:tenant_id;primaryKey;default:''"`
	AttemptID        string    `gorm:"column:attempt_id;primaryKey;size:128"`
	OrderID          string    `gorm:"column:order_id;size:255;not null;index"`
	FundingBasis     []byte    `gorm:"column:funding_basis;type:text;not null"`
	FundingBasisHash string    `gorm:"column:funding_basis_hash;size:64;not null;index"`
	CreatedAt        time.Time `gorm:"column:created_at;not null;index"`
}

// TableName returns the stable funding-basis proposal inbox table name.
func (PaymentAttemptFundingBasisProposalRecord) TableName() string {
	return "payment_attempt_funding_basis_proposals"
}

// Proposal decodes and verifies the canonical inbox value.
func (r PaymentAttemptFundingBasisProposalRecord) Proposal() (*PaymentAttemptFundingBasis, error) {
	var basis PaymentAttemptFundingBasis
	if err := json.Unmarshal(r.FundingBasis, &basis); err != nil {
		return nil, ErrPaymentAttemptSettlementTermsConflict
	}
	canonical, hash, err := basis.CanonicalBytesAndHash()
	if err != nil || string(canonical) != string(r.FundingBasis) ||
		hash != strings.TrimSpace(r.FundingBasisHash) || basis.AttemptID != r.AttemptID || basis.OrderID != r.OrderID {
		return nil, ErrPaymentAttemptSettlementTermsConflict
	}
	return &basis, nil
}

// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"strings"
	"time"
)

// Onramp funding source statuses. They mirror contracts.OnrampStatus by value
// (pkg/models must stay import-light); keep in sync with
// pkg/contracts/onramp_provider.go.
const (
	OnrampSourceStatusCreated         = "created"
	OnrampSourceStatusAwaitingPayment = "awaiting_payment"
	OnrampSourceStatusProcessing      = "processing"
	OnrampSourceStatusDelivering      = "delivering"
	OnrampSourceStatusDelivered       = "delivered"
	OnrampSourceStatusFailed          = "failed"
	OnrampSourceStatusReversed        = "reversed"
)

// PaymentAttemptOnrampFundingSource is the durable record of one onramp
// purchase funding a payment attempt (RFC-0012 Proposal 5; ADR-019). It is a
// funding-source descriptor, never a settlement mode: the attempt's frozen
// funding target and on-chain observation remain the only truth for
// funded/verified.
//
// Cardinality is deliberately 1:N per attempt: failed or reversed purchases
// are retained for reconciliation and dispute forensics (a fiat-leg reversal
// after on-chain delivery must stay auditable), while the partial unique index
// on (tenant_id, attempt_id) WHERE active enforces at most one purchase in
// flight at a time.
type PaymentAttemptOnrampFundingSource struct {
	TenantID      string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:idx_onramp_source_idem,priority:1;uniqueIndex:idx_onramp_source_active,priority:1,where:active"`
	AttemptID     string `gorm:"column:attempt_id;primaryKey;size:64;uniqueIndex:idx_onramp_source_idem,priority:2;uniqueIndex:idx_onramp_source_active,priority:2"`
	OnrampOrderID string `gorm:"column:onramp_order_id;primaryKey;size:128"`

	OrderID    string `gorm:"column:order_id;size:255;not null;index:idx_onramp_source_order"`
	ProviderID string `gorm:"column:provider_id;size:64;not null"`
	// FiatCurrency is part of the provider purchase's commercial identity.
	// Persist it so an idempotent resume cannot silently switch currencies when
	// a client locale or provider discovery response changes.
	FiatCurrency string `gorm:"column:fiat_currency;size:16;not null"`

	// Status mirrors contracts.OnrampStatus. Active is derived from Status by
	// SetStatus and backs the at-most-one-active partial unique index; writers
	// must use SetStatus rather than assigning Status directly.
	//
	// The index must stay scoped to (tenant_id, attempt_id): the invariant is
	// one in-flight purchase per attempt. Scoped to (active) alone it becomes a
	// single global row lock — the first buyer with an in-flight purchase makes
	// every other buyer's initiate fail with a duplicate-key 500, across every
	// tenant on the node.
	Status string `gorm:"column:status;size:32;not null;index:idx_onramp_source_status"`
	Active bool   `gorm:"column:active;not null;default:false;uniqueIndex:idx_onramp_source_active,priority:3"`

	// IdempotencyKey makes initiate/resume safe: re-entry for the same attempt
	// and key returns this record instead of creating a second onramp order.
	IdempotencyKey string `gorm:"column:idempotency_key;size:128;not null;uniqueIndex:idx_onramp_source_idem,priority:3"`

	DeliveryTarget       string `gorm:"column:delivery_target;size:255;not null;default:''"`
	DeliverToBuyerWallet bool   `gorm:"column:deliver_to_buyer_wallet;not null;default:false"`
	BuyerWalletAddress   string `gorm:"column:buyer_wallet_address;size:255;not null;default:''"`
	BuyerActionURL       string `gorm:"column:buyer_action_url;size:1024;not null;default:''"`
	Disclosure           string `gorm:"column:disclosure;size:2048;not null;default:''"`

	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime;index:idx_onramp_source_updated"`
}

// TableName pins the table name.
func (PaymentAttemptOnrampFundingSource) TableName() string {
	return "payment_attempt_onramp_funding_sources"
}

// onrampSourceActiveStatuses are the in-flight statuses backing Active.
func onrampSourceStatusActive(status string) bool {
	switch status {
	case OnrampSourceStatusCreated, OnrampSourceStatusAwaitingPayment,
		OnrampSourceStatusProcessing, OnrampSourceStatusDelivering:
		return true
	default:
		return false
	}
}

// SetStatus updates Status and keeps the Active flag consistent. It is the
// only supported way to change Status.
func (s *PaymentAttemptOnrampFundingSource) SetStatus(status string) {
	s.Status = strings.TrimSpace(status)
	s.Active = onrampSourceStatusActive(s.Status)
}

// IsActive reports whether the purchase is still progressing toward delivery.
func (s *PaymentAttemptOnrampFundingSource) IsActive() bool {
	return s != nil && onrampSourceStatusActive(s.Status)
}

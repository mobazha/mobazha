// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	PaymentAttemptKindProviderSession       = "provider_session"
	PaymentAttemptKindDirectObservedAddress = "direct_observed_address"

	PaymentAttemptPendingExternal     = "pending_external"
	PaymentAttemptExternalDispatching = "external_dispatching"
	PaymentAttemptExternalCreated     = "external_created"
	PaymentAttemptLinked              = "linked"
	PaymentAttemptReconcileRequired   = "reconcile_required"
	PaymentAttemptExpired             = "expired"
	PaymentAttemptAbandoning          = "abandoning"
	PaymentAttemptAbandoned           = "abandoned"
)

// PaymentAttempt is Core's durable claim for one concrete payment provisioning
// operation. It must exist before any external provider or chain create call.
type PaymentAttempt struct {
	TenantID  string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:idx_payment_attempt_idempotency,priority:1"`
	AttemptID string `gorm:"column:attempt_id;primaryKey;size:64"`
	// Kind defaults to provider_session so databases created before attempt
	// kinds were introduced upgrade without losing the meaning of existing rows.
	Kind              string `gorm:"column:kind;size:64;not null;default:provider_session;index:idx_payment_attempt_kind_state,priority:1"`
	PaymentSessionID  string `gorm:"column:payment_session_id;size:255;not null;index:idx_payment_attempt_session"`
	OrderID           string `gorm:"column:order_id;size:255;not null;index:idx_payment_attempt_order"`
	ProviderID        string `gorm:"column:provider_id;size:64;not null;default:''"`
	Amount            int64  `gorm:"column:amount;not null;default:0"`
	AmountValue       string `gorm:"column:amount_value;type:text;not null;default:''"`
	Currency          string `gorm:"column:currency;size:16;not null;default:''"`
	RouteBindingID    string `gorm:"column:route_binding_id;size:64;not null"`
	IdempotencyKey    string `gorm:"column:idempotency_key;size:128;not null;uniqueIndex:idx_payment_attempt_idempotency,priority:2"`
	State             string `gorm:"column:state;size:32;not null;index:idx_payment_attempt_state;index:idx_payment_attempt_kind_state,priority:2"`
	ExternalReference string `gorm:"column:external_reference;size:255"`
	ExternalIndex     uint32 `gorm:"column:external_index;not null;default:0"`
	RequiredConfs     int    `gorm:"column:required_confirmations;not null;default:0"`
	// SettlementTerms is the canonical, immutable economic allocation owned by
	// this attempt. It must be committed before an actionable funding target is
	// exposed. SettlementTermsHash binds settlement actions to these exact bytes.
	SettlementTerms        []byte     `gorm:"column:settlement_terms;type:text"`
	SettlementTermsHash    string     `gorm:"column:settlement_terms_hash;size:64;not null;default:'';index"`
	SellerTermsSignature   []byte     `gorm:"column:seller_terms_signature"`
	PlatformTermsSignature []byte     `gorm:"column:platform_terms_signature"`
	ExpiresAt              *time.Time `gorm:"column:expires_at;index"`
	LastError              string     `gorm:"column:last_error;size:2048"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

func (PaymentAttempt) TableName() string { return "payment_attempts" }

// SetSettlementTerms freezes canonical settlement terms on an attempt. A
// retry may supply the same terms, but changing an already-frozen hash fails.
func (a *PaymentAttempt) SetSettlementTerms(terms PaymentAttemptSettlementTerms) error {
	if a == nil {
		return fmt.Errorf("payment attempt is nil")
	}
	if strings.TrimSpace(a.AttemptID) == "" {
		return fmt.Errorf("payment attempt ID is required")
	}
	if terms.AttemptID != a.AttemptID || terms.OrderID != a.OrderID {
		return fmt.Errorf("settlement terms do not belong to payment attempt")
	}
	canonical, hash, err := terms.CanonicalBytesAndHash()
	if err != nil {
		return err
	}
	existingHash := strings.TrimSpace(a.SettlementTermsHash)
	if len(a.SettlementTerms) > 0 || existingHash != "" {
		if existingHash != hash || string(a.SettlementTerms) != string(canonical) {
			return ErrPaymentAttemptSettlementTermsConflict
		}
		return nil
	}
	a.SettlementTerms = canonical
	a.SettlementTermsHash = hash
	return nil
}

// GetSettlementTerms decodes and verifies the attempt's frozen terms.
func (a *PaymentAttempt) GetSettlementTerms() (*PaymentAttemptSettlementTerms, error) {
	if a == nil || len(a.SettlementTerms) == 0 {
		return nil, nil
	}
	var terms PaymentAttemptSettlementTerms
	if err := json.Unmarshal(a.SettlementTerms, &terms); err != nil {
		return nil, fmt.Errorf("decode payment attempt settlement terms: %w", err)
	}
	canonical, hash, err := terms.CanonicalBytesAndHash()
	if err != nil {
		return nil, err
	}
	if string(canonical) != string(a.SettlementTerms) || hash != strings.TrimSpace(a.SettlementTermsHash) {
		return nil, ErrPaymentAttemptSettlementTermsConflict
	}
	return &terms, nil
}

// PaymentRouteBinding is immutable routing identity for an accepted attempt.
// A provider/account/configuration switch creates another attempt and binding.
type PaymentRouteBinding struct {
	TenantID                 string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:idx_payment_route_attempt,priority:1"`
	RouteBindingID           string `gorm:"column:route_binding_id;primaryKey;size:64"`
	AttemptID                string `gorm:"column:attempt_id;size:64;not null;uniqueIndex:idx_payment_route_attempt,priority:2"`
	ContributionID           string `gorm:"column:contribution_id;size:128;not null;index:idx_payment_route_contribution"`
	ModuleID                 string `gorm:"column:module_id;size:128;not null"`
	ImplementationGeneration string `gorm:"column:implementation_generation;size:64;not null"`
	RailKind                 string `gorm:"column:rail_kind;size:32;not null"`
	NetworkID                string `gorm:"column:network_id;size:128;not null"`
	AssetID                  string `gorm:"column:asset_id;size:255;not null"`
	ProtocolVersion          string `gorm:"column:protocol_version;size:64;not null"`
	StateSchemaVersion       string `gorm:"column:state_schema_version;size:64;not null"`
	ProviderBindingID        string `gorm:"column:provider_binding_id;size:128"`
	ExternalAccountReference string `gorm:"column:external_account_reference;size:255"`
	CreatedAt                time.Time
}

func (PaymentRouteBinding) TableName() string { return "payment_route_bindings" }

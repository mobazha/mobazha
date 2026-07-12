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
	PaymentAttemptKindCryptoFundingTarget   = "crypto_funding_target"

	PaymentAttemptPendingExternal     = "pending_external"
	PaymentAttemptExternalDispatching = "external_dispatching"
	PaymentAttemptExternalCreated     = "external_created"
	PaymentAttemptLinked              = "linked"
	PaymentAttemptReconcileRequired   = "reconcile_required"
	PaymentAttemptExpired             = "expired"
	PaymentAttemptAbandoning          = "abandoning"
	PaymentAttemptAbandoned           = "abandoned"
	PaymentAttemptAuthorizationDraft  = "authorization_draft"
	PaymentAttemptFundingTargetReady  = "funding_target_ready"
)

// PaymentAttempt is Core's durable claim for one concrete payment provisioning
// operation. It must exist before any external provider or chain create call.
type PaymentAttempt struct {
	TenantID  string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:idx_payment_attempt_idempotency,priority:1;uniqueIndex:idx_payment_attempt_authorization_context,priority:1"`
	AttemptID string `gorm:"column:attempt_id;primaryKey;size:64"`
	// Kind defaults to provider_session so databases created before attempt
	// kinds were introduced upgrade without losing the meaning of existing rows.
	Kind              string `gorm:"column:kind;size:64;not null;default:provider_session;index:idx_payment_attempt_kind_state,priority:1"`
	PaymentSessionID  string `gorm:"column:payment_session_id;size:255;not null;index:idx_payment_attempt_session"`
	OrderID           string `gorm:"column:order_id;size:255;not null;index:idx_payment_attempt_order"`
	ProviderID        string `gorm:"column:provider_id;size:64;not null;default:''"`
	Amount            int64  `gorm:"column:amount;not null;default:0"`
	AmountValue       string `gorm:"column:amount_value;type:text;not null;default:''"`
	Currency          string `gorm:"column:currency;size:255;not null;default:''"`
	RouteBindingID    string `gorm:"column:route_binding_id;size:64;not null"`
	IdempotencyKey    string `gorm:"column:idempotency_key;size:128;not null;uniqueIndex:idx_payment_attempt_idempotency,priority:2"`
	State             string `gorm:"column:state;size:32;not null;index:idx_payment_attempt_state;index:idx_payment_attempt_kind_state,priority:2"`
	ExternalReference string `gorm:"column:external_reference;size:255"`
	ExternalIndex     uint32 `gorm:"column:external_index;not null;default:0"`
	RequiredConfs     int    `gorm:"column:required_confirmations;not null;default:0"`
	FundingTarget     []byte `gorm:"column:funding_target;type:text"`
	FundingTargetHash string `gorm:"column:funding_target_hash;size:64;not null;default:'';index"`
	// AuthorizationContextID is the non-secret, immutable key-locating input
	// for attempt-scoped Settlement participant keys.
	AuthorizationContextID  string `gorm:"column:authorization_context_id;size:64;not null;default:'';uniqueIndex:idx_payment_attempt_authorization_context,priority:2,where:authorization_context_id <> ''"`
	AuthorizationBundle     []byte `gorm:"column:authorization_bundle;type:text"`
	AuthorizationBundleHash string `gorm:"column:authorization_bundle_hash;size:64;not null;default:'';index"`
	// SettlementTerms is the canonical, immutable economic allocation owned by
	// this attempt. It must be committed before an actionable funding target is
	// exposed. SettlementTermsHash binds settlement actions to these exact bytes.
	SettlementTerms        []byte     `gorm:"column:settlement_terms;type:text"`
	SettlementTermsHash    string     `gorm:"column:settlement_terms_hash;size:64;not null;default:'';index"`
	SellerTermsSignature   []byte     `gorm:"column:seller_terms_signature"`
	SellerTermsSigner      string     `gorm:"column:seller_terms_signer;size:255;not null;default:''"`
	PlatformTermsSignature []byte     `gorm:"column:platform_terms_signature"`
	ExpiresAt              *time.Time `gorm:"column:expires_at;index"`
	LastError              string     `gorm:"column:last_error;size:2048"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// SetAuthorizationContextID freezes the random context that locates this
// attempt's Settlement keys. Retried provisioning must reuse the same value.
func (a *PaymentAttempt) SetAuthorizationContextID(contextID string) error {
	if a == nil || !validSettlementAuthorizationContextID(contextID) {
		return fmt.Errorf("invalid payment attempt authorization context")
	}
	if a.AuthorizationContextID != "" && a.AuthorizationContextID != contextID {
		return ErrPaymentAttemptSettlementTermsConflict
	}
	a.AuthorizationContextID = contextID
	return nil
}

// SetAuthorizationBundle freezes verified participant offers, seller terms
// authorization, and the target commitment for this crypto attempt.
func (a *PaymentAttempt) SetAuthorizationBundle(bundle PaymentAttemptAuthorizationBundle) error {
	if a == nil || a.Kind != PaymentAttemptKindCryptoFundingTarget {
		return fmt.Errorf("authorization bundles require a crypto payment attempt")
	}
	canonical, hash, err := bundle.CanonicalBytesAndHash()
	if err != nil {
		return err
	}
	if err := a.validateAuthorizationBundle(bundle); err != nil {
		return err
	}
	if len(a.AuthorizationBundle) > 0 || a.AuthorizationBundleHash != "" {
		if string(a.AuthorizationBundle) != string(canonical) || a.AuthorizationBundleHash != hash {
			return ErrPaymentAttemptSettlementTermsConflict
		}
		return nil
	}
	a.AuthorizationBundle = canonical
	a.AuthorizationBundleHash = hash
	return nil
}

// GetAuthorizationBundle decodes and verifies the immutable attempt bundle.
func (a *PaymentAttempt) GetAuthorizationBundle() (*PaymentAttemptAuthorizationBundle, error) {
	if a == nil || len(a.AuthorizationBundle) == 0 {
		return nil, nil
	}
	var bundle PaymentAttemptAuthorizationBundle
	if err := json.Unmarshal(a.AuthorizationBundle, &bundle); err != nil {
		return nil, fmt.Errorf("decode payment attempt authorization bundle: %w", err)
	}
	canonical, hash, err := bundle.CanonicalBytesAndHash()
	if err != nil {
		return nil, err
	}
	if string(canonical) != string(a.AuthorizationBundle) || hash != strings.TrimSpace(a.AuthorizationBundleHash) {
		return nil, ErrPaymentAttemptSettlementTermsConflict
	}
	if err := a.validateAuthorizationBundle(bundle); err != nil {
		return nil, err
	}
	return &bundle, nil
}

func (a *PaymentAttempt) validateAuthorizationBundle(bundle PaymentAttemptAuthorizationBundle) error {
	terms, err := a.GetSettlementTerms()
	if err != nil {
		return err
	}
	if terms == nil || a.AuthorizationContextID == "" || a.SellerTermsSigner == "" || len(a.SellerTermsSignature) == 0 {
		return fmt.Errorf("authorization context, terms, and seller authorization are required before authorization bundle")
	}
	if err := terms.VerifySellerAuthorization(a.SellerTermsSigner, a.SellerTermsSignature); err != nil {
		return err
	}
	if bundle.AuthorizationContextID != a.AuthorizationContextID || bundle.OrderID != a.OrderID ||
		bundle.AttemptID != a.AttemptID || bundle.RailID != a.Currency ||
		bundle.SettlementTermsHash != a.SettlementTermsHash || bundle.SellerTermsSigner != a.SellerTermsSigner ||
		string(bundle.SellerTermsSignature) != string(a.SellerTermsSignature) {
		return fmt.Errorf("authorization bundle does not match payment attempt")
	}
	expectedOfferPeers := map[SettlementParticipantRole]string{
		SettlementParticipantBuyer:  terms.BuyerPeerID,
		SettlementParticipantSeller: terms.SellerPeerID,
	}
	if terms.ModeratorPeerID != "" {
		expectedOfferPeers[SettlementParticipantModerator] = terms.ModeratorPeerID
	}
	if len(bundle.RequiredRoles) != len(expectedOfferPeers) {
		return fmt.Errorf("authorization bundle roles do not match settlement terms")
	}
	for _, offer := range bundle.Offers {
		if expectedPeerID, ok := expectedOfferPeers[offer.ParticipantRole]; !ok || offer.ParticipantPeerID != expectedPeerID {
			return fmt.Errorf("authorization bundle offer does not match settlement terms")
		}
	}
	return nil
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
	if terms.AttemptID != a.AttemptID || terms.OrderID != a.OrderID {
		return nil, ErrPaymentAttemptSettlementTermsConflict
	}
	return &terms, nil
}

// SetSellerTermsAuthorization freezes a verified seller identity signature for
// the attempt's already-frozen canonical settlement terms.
func (a *PaymentAttempt) SetSellerTermsAuthorization(signerPeerID string, signature []byte) error {
	if a == nil {
		return fmt.Errorf("payment attempt is nil")
	}
	terms, err := a.GetSettlementTerms()
	if err != nil {
		return err
	}
	if terms == nil {
		return fmt.Errorf("payment attempt settlement terms are required before seller authorization")
	}
	if err := terms.VerifySellerAuthorization(signerPeerID, signature); err != nil {
		return err
	}
	if a.SellerTermsSigner != "" || len(a.SellerTermsSignature) > 0 {
		if a.SellerTermsSigner != strings.TrimSpace(signerPeerID) || string(a.SellerTermsSignature) != string(signature) {
			return ErrPaymentAttemptSettlementTermsConflict
		}
		return nil
	}
	a.SellerTermsSigner = strings.TrimSpace(signerPeerID)
	a.SellerTermsSignature = append([]byte(nil), signature...)
	return nil
}

// SetFundingTarget freezes an actionable crypto target only after canonical
// settlement terms and seller authorization have been verified.
func (a *PaymentAttempt) SetFundingTarget(target PaymentAttemptFundingTarget) error {
	if a == nil {
		return fmt.Errorf("payment attempt is nil")
	}
	terms, err := a.GetSettlementTerms()
	if err != nil {
		return err
	}
	if terms == nil || strings.TrimSpace(a.SellerTermsSigner) == "" || len(a.SellerTermsSignature) == 0 {
		return fmt.Errorf("verified settlement terms are required before funding target")
	}
	if err := terms.VerifySellerAuthorization(a.SellerTermsSigner, a.SellerTermsSignature); err != nil {
		return err
	}
	canonical, hash, err := target.CanonicalBytesAndHash()
	if err != nil {
		return err
	}
	if a.Kind == PaymentAttemptKindCryptoFundingTarget {
		bundle, err := a.GetAuthorizationBundle()
		if err != nil {
			return err
		}
		if bundle == nil || bundle.AuthorizationContextID != a.AuthorizationContextID || bundle.FundingTargetHash != hash {
			return fmt.Errorf("%w: verified authorization bundle is required before funding target", ErrPaymentAttemptSettlementTermsConflict)
		}
	}
	if target.AttemptID != a.AttemptID || target.AssetID != terms.AssetID ||
		target.AmountAtomic != terms.FundingAmount || target.Address != terms.FundingTargetAddress {
		return fmt.Errorf("%w: funding target does not match settlement terms", ErrPaymentAttemptSettlementTermsConflict)
	}
	if len(a.FundingTarget) > 0 || strings.TrimSpace(a.FundingTargetHash) != "" {
		if string(a.FundingTarget) != string(canonical) || strings.TrimSpace(a.FundingTargetHash) != hash {
			return ErrPaymentAttemptSettlementTermsConflict
		}
		return nil
	}
	a.FundingTarget = canonical
	a.FundingTargetHash = hash
	a.State = PaymentAttemptFundingTargetReady
	return nil
}

// GetFundingTarget decodes and verifies the attempt's immutable target.
func (a *PaymentAttempt) GetFundingTarget() (*PaymentAttemptFundingTarget, error) {
	if a == nil || len(a.FundingTarget) == 0 {
		return nil, nil
	}
	var target PaymentAttemptFundingTarget
	if err := json.Unmarshal(a.FundingTarget, &target); err != nil {
		return nil, fmt.Errorf("decode payment attempt funding target: %w", err)
	}
	canonical, hash, err := target.CanonicalBytesAndHash()
	if err != nil {
		return nil, err
	}
	if string(canonical) != string(a.FundingTarget) || hash != strings.TrimSpace(a.FundingTargetHash) {
		return nil, ErrPaymentAttemptSettlementTermsConflict
	}
	terms, err := a.GetSettlementTerms()
	if err != nil {
		return nil, err
	}
	if terms == nil || target.AttemptID != a.AttemptID || target.AssetID != terms.AssetID ||
		target.AmountAtomic != terms.FundingAmount || target.Address != terms.FundingTargetAddress {
		return nil, ErrPaymentAttemptSettlementTermsConflict
	}
	if a.Kind == PaymentAttemptKindCryptoFundingTarget {
		bundle, err := a.GetAuthorizationBundle()
		if err != nil {
			return nil, err
		}
		if bundle == nil || bundle.FundingTargetHash != hash {
			return nil, ErrPaymentAttemptSettlementTermsConflict
		}
	}
	return &target, nil
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

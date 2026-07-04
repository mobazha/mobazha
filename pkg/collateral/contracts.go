// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Package collateral defines the public, Core-owned resource-collateral
// contract. These types describe intent and durable references only; an
// extension cannot use them to mark funds locked, released, or slashed.
package collateral

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

type State string

const (
	StatePendingFunding State = "pending-funding"
	StateActive         State = "active"
	StateReleasePending State = "release-pending"
	StateReleased       State = "released"
	StateSlashPending   State = "slash-pending"
	StateSlashed        State = "slashed"
	StateFailed         State = "failed"
)

type AllocationState string

const (
	AllocationActive   AllocationState = "active"
	AllocationReleased AllocationState = "released"
	AllocationClaimed  AllocationState = "claimed"
)

// OpenRequest declares the minimum single-asset collateral required for one
// provider-owned resource and principal. Funding is a later Core transition.
type OpenRequest struct {
	TenantID       string    `json:"tenantID"`
	ProviderID     string    `json:"providerID"`
	ResourceID     string    `json:"resourceID"`
	PrincipalID    string    `json:"principalID"`
	AssetID        string    `json:"assetID"`
	RequiredAmount string    `json:"requiredAmount"`
	PolicyID       string    `json:"policyID"`
	PolicyVersion  string    `json:"policyVersion"`
	IdempotencyKey string    `json:"idempotencyKey"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

func (r OpenRequest) Validate(now time.Time) error {
	if missing(r.TenantID, r.ProviderID, r.ResourceID, r.PrincipalID, r.AssetID, r.PolicyID, r.PolicyVersion, r.IdempotencyKey) {
		return fmt.Errorf("collateral tenant, provider, resource, principal, asset, policy, policy version, and idempotency key are required")
	}
	if err := validateBaseUnits(r.RequiredAmount, false); err != nil {
		return fmt.Errorf("collateral required amount: %w", err)
	}
	if r.ExpiresAt.IsZero() || !r.ExpiresAt.After(now) {
		return fmt.Errorf("collateral requirement expiry must be in the future")
	}
	return nil
}

// Account is the Core-issued projection of a collateral financial aggregate.
// FundingReference is an opaque reference owned by the selected payment rail.
type Account struct {
	CollateralID     string     `json:"collateralID"`
	TenantID         string     `json:"tenantID"`
	ProviderID       string     `json:"providerID"`
	ResourceID       string     `json:"resourceID"`
	PrincipalID      string     `json:"principalID"`
	AssetID          string     `json:"assetID"`
	RequiredAmount   string     `json:"requiredAmount"`
	FundedAmount     string     `json:"fundedAmount"`
	AvailableAmount  string     `json:"availableAmount"`
	PolicyID         string     `json:"policyID"`
	PolicyVersion    string     `json:"policyVersion"`
	FundingReference string     `json:"fundingReference,omitempty"`
	Revision         uint64     `json:"revision"`
	State            State      `json:"state"`
	ActivatedAt      *time.Time `json:"activatedAt,omitempty"`
	ExpiresAt        time.Time  `json:"expiresAt"`
}

func (a Account) Validate() error {
	if missing(a.CollateralID, a.TenantID, a.ProviderID, a.ResourceID, a.PrincipalID, a.AssetID, a.PolicyID, a.PolicyVersion) {
		return fmt.Errorf("collateral account identity, scope, asset, and policy are required")
	}
	if a.Revision == 0 || a.ExpiresAt.IsZero() {
		return fmt.Errorf("collateral account revision and expiry are required")
	}
	if !validState(a.State) {
		return fmt.Errorf("collateral account state %q is unsupported", a.State)
	}
	required, err := parseBaseUnits(a.RequiredAmount, false)
	if err != nil {
		return fmt.Errorf("collateral required amount: %w", err)
	}
	funded, err := parseBaseUnits(a.FundedAmount, true)
	if err != nil {
		return fmt.Errorf("collateral funded amount: %w", err)
	}
	available, err := parseBaseUnits(a.AvailableAmount, true)
	if err != nil {
		return fmt.Errorf("collateral available amount: %w", err)
	}
	if available.Cmp(funded) > 0 {
		return fmt.Errorf("collateral available amount exceeds funded amount")
	}
	if a.State == StateActive {
		if a.ActivatedAt == nil || a.ActivatedAt.IsZero() || strings.TrimSpace(a.FundingReference) == "" {
			return fmt.Errorf("active collateral requires activation time and funding reference")
		}
		if funded.Cmp(required) < 0 {
			return fmt.Errorf("active collateral funded amount is below the requirement")
		}
	}
	return nil
}

// AllocationRequest asks Core to reserve active collateral coverage for one
// order extension. It does not move or release funds.
type AllocationRequest struct {
	CollateralID               string `json:"collateralID"`
	TenantID                   string `json:"tenantID"`
	ProviderID                 string `json:"providerID"`
	ResourceID                 string `json:"resourceID"`
	PrincipalID                string `json:"principalID"`
	OrderID                    string `json:"orderID"`
	ExtensionID                string `json:"extensionID"`
	Amount                     string `json:"amount"`
	ExpectedCollateralRevision uint64 `json:"expectedCollateralRevision"`
	IdempotencyKey             string `json:"idempotencyKey"`
}

func (r AllocationRequest) Validate() error {
	if missing(r.CollateralID, r.TenantID, r.ProviderID, r.ResourceID, r.PrincipalID, r.OrderID, r.ExtensionID, r.IdempotencyKey) {
		return fmt.Errorf("collateral allocation identity, scope, order, extension, and idempotency key are required")
	}
	if r.ExpectedCollateralRevision == 0 {
		return fmt.Errorf("collateral allocation expected revision is required")
	}
	if err := validateBaseUnits(r.Amount, false); err != nil {
		return fmt.Errorf("collateral allocation amount: %w", err)
	}
	return nil
}

// AllocationReference is a Core-issued, revision-bound reference suitable for
// a future Order Extension contract. Providers cannot mint this reference.
type AllocationReference struct {
	AllocationID       string          `json:"allocationID"`
	CollateralID       string          `json:"collateralID"`
	TenantID           string          `json:"tenantID"`
	OrderID            string          `json:"orderID"`
	ExtensionID        string          `json:"extensionID"`
	AssetID            string          `json:"assetID"`
	Amount             string          `json:"amount"`
	CollateralRevision uint64          `json:"collateralRevision"`
	AllocationRevision uint64          `json:"allocationRevision"`
	State              AllocationState `json:"state"`
}

func (r AllocationReference) Validate() error {
	if missing(r.AllocationID, r.CollateralID, r.TenantID, r.OrderID, r.ExtensionID, r.AssetID) {
		return fmt.Errorf("collateral allocation reference identity, scope, order, extension, and asset are required")
	}
	if r.CollateralRevision == 0 || r.AllocationRevision == 0 {
		return fmt.Errorf("collateral and allocation revisions are required")
	}
	if r.State != AllocationActive && r.State != AllocationReleased && r.State != AllocationClaimed {
		return fmt.Errorf("collateral allocation state %q is unsupported", r.State)
	}
	if err := validateBaseUnits(r.Amount, false); err != nil {
		return fmt.Errorf("collateral allocation reference amount: %w", err)
	}
	return nil
}

// ClaimAttestation is bounded provider evidence for a Core-owned claim
// command. It intentionally has no payout address or financial execution
// handle; Core derives any beneficiary after policy and dispute validation.
type ClaimAttestation struct {
	AttestationID              string    `json:"attestationID"`
	IdempotencyKey             string    `json:"idempotencyKey"`
	Issuer                     string    `json:"issuer"`
	TenantID                   string    `json:"tenantID"`
	CollateralID               string    `json:"collateralID"`
	AllocationID               string    `json:"allocationID"`
	OrderID                    string    `json:"orderID"`
	ExtensionID                string    `json:"extensionID"`
	ExpectedCollateralRevision uint64    `json:"expectedCollateralRevision"`
	ExpectedAllocationRevision uint64    `json:"expectedAllocationRevision"`
	ConditionType              string    `json:"conditionType"`
	ConditionVersion           string    `json:"conditionVersion"`
	EvidenceDigest             string    `json:"evidenceDigest"`
	ObservedAt                 time.Time `json:"observedAt"`
	ExpiresAt                  time.Time `json:"expiresAt"`
}

func (a ClaimAttestation) Validate(now time.Time) error {
	if missing(a.AttestationID, a.IdempotencyKey, a.Issuer, a.TenantID, a.CollateralID, a.AllocationID, a.OrderID, a.ExtensionID, a.ConditionType, a.ConditionVersion, a.EvidenceDigest) {
		return fmt.Errorf("collateral claim identity, issuer, scope, binding, condition, evidence, and idempotency key are required")
	}
	if a.ExpectedCollateralRevision == 0 || a.ExpectedAllocationRevision == 0 {
		return fmt.Errorf("collateral claim expected revisions are required")
	}
	if a.ObservedAt.IsZero() || a.ExpiresAt.IsZero() || !a.ExpiresAt.After(now) || !a.ExpiresAt.After(a.ObservedAt) || a.ObservedAt.After(now.Add(time.Minute)) {
		return fmt.Errorf("collateral claim time window is invalid")
	}
	return nil
}

func validState(state State) bool {
	switch state {
	case StatePendingFunding, StateActive, StateReleasePending, StateReleased, StateSlashPending, StateSlashed, StateFailed:
		return true
	default:
		return false
	}
}

func missing(values ...string) bool {
	for _, value := range values {
		if value == "" || value != strings.TrimSpace(value) {
			return true
		}
	}
	return false
}

func validateBaseUnits(value string, allowZero bool) error {
	_, err := parseBaseUnits(value, allowZero)
	return err
}

func parseBaseUnits(value string, allowZero bool) (*big.Int, error) {
	if value == "" || value != strings.TrimSpace(value) || strings.HasPrefix(value, "+") {
		return nil, fmt.Errorf("must be canonical integer base units")
	}
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok || parsed.Sign() < 0 || (!allowZero && parsed.Sign() == 0) || parsed.String() != value {
		return nil, fmt.Errorf("must be canonical %s integer base units", map[bool]string{true: "non-negative", false: "positive"}[allowZero])
	}
	return parsed, nil
}

// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const maxRailPayloadBytes = 16 * 1024

type RailActionState string

const (
	RailActionPending   RailActionState = "pending"
	RailActionConfirmed RailActionState = "confirmed"
	RailActionFailed    RailActionState = "failed"
)

// RailDescriptor advertises an implementation's exact collateral authority.
// The v1 gate requires the complete lifecycle so a funding-only observer can
// never be mistaken for a release/slash-capable collateral rail.
type RailDescriptor struct {
	ID                       string   `json:"id"`
	Version                  string   `json:"version"`
	CustodyModel             string   `json:"custodyModel"`
	Assets                   []string `json:"assets"`
	SupportsFundingTargets   bool     `json:"supportsFundingTargets"`
	SupportsFundingObserve   bool     `json:"supportsFundingObserve"`
	SupportsPrincipalRelease bool     `json:"supportsPrincipalRelease"`
	SupportsClaimSlash       bool     `json:"supportsClaimSlash"`
	SupportsReconciliation   bool     `json:"supportsReconciliation"`
	HasReceiptVerification   bool     `json:"hasReceiptVerification"`
}

func (d RailDescriptor) ValidateForAsset(assetID string) error {
	if missing(d.ID, d.Version, d.CustodyModel, assetID) {
		return fmt.Errorf("collateral rail identity, version, custody model, and asset are required")
	}
	previous := ""
	found := false
	for _, asset := range d.Assets {
		if asset == "" || asset != strings.TrimSpace(asset) || (previous != "" && asset <= previous) {
			return fmt.Errorf("collateral rail assets must be canonical, unique, and sorted")
		}
		if asset == assetID {
			found = true
		}
		previous = asset
	}
	if !found {
		return fmt.Errorf("collateral rail does not support asset %q", assetID)
	}
	if !d.SupportsFundingTargets || !d.SupportsFundingObserve || !d.SupportsPrincipalRelease ||
		!d.SupportsClaimSlash || !d.SupportsReconciliation || !d.HasReceiptVerification {
		return fmt.Errorf("collateral rail does not implement the complete v1 lifecycle")
	}
	return nil
}

// FundingTargetRequest asks a rail for a principal-funded destination or
// instruction set bound to one unopened collateral obligation.
type FundingTargetRequest struct {
	TenantID     string `json:"tenantID"`
	CollateralID string `json:"collateralID"`
	PrincipalID  string `json:"principalID"`
	// PrincipalDestination is the Core-resolved rail identity that funds the
	// obligation and receives a normal release. For an EVM vault it is an
	// address; other rails may define a different canonical destination form.
	PrincipalDestination string    `json:"principalDestination"`
	AssetID              string    `json:"assetID"`
	Amount               string    `json:"amount"`
	IdempotencyKey       string    `json:"idempotencyKey"`
	ExpiresAt            time.Time `json:"expiresAt"`
}

func (r FundingTargetRequest) Validate(now time.Time) error {
	if missing(r.TenantID, r.CollateralID, r.PrincipalID, r.PrincipalDestination, r.AssetID, r.IdempotencyKey) {
		return fmt.Errorf("collateral funding target tenant, account, principal, destination, asset, and idempotency key are required")
	}
	if err := validateBaseUnits(r.Amount, false); err != nil {
		return fmt.Errorf("collateral funding target amount: %w", err)
	}
	if r.ExpiresAt.IsZero() || !r.ExpiresAt.After(now) {
		return fmt.Errorf("collateral funding target expiry must be in the future")
	}
	return nil
}

// FundingTarget is a Core-scoped rail target. Payload may contain bounded,
// chain-specific wallet instructions; it is never treated as funding proof.
type FundingTarget struct {
	RailID               string          `json:"railID"`
	TenantID             string          `json:"-"`
	CollateralID         string          `json:"collateralID"`
	PrincipalDestination string          `json:"-"`
	IdempotencyKey       string          `json:"-"`
	AssetID              string          `json:"assetID"`
	Amount               string          `json:"amount"`
	Destination          string          `json:"destination,omitempty"`
	Payload              json.RawMessage `json:"payload,omitempty"`
	ExpiresAt            time.Time       `json:"expiresAt"`
}

func (t FundingTarget) Validate(now time.Time) error {
	if missing(t.RailID, t.TenantID, t.CollateralID, t.PrincipalDestination, t.IdempotencyKey, t.AssetID) || (strings.TrimSpace(t.Destination) == "" && len(t.Payload) == 0) {
		return fmt.Errorf("collateral funding target rail, tenant, account, principal destination, idempotency key, asset, and destination or payload are required")
	}
	if len(t.Payload) > maxRailPayloadBytes {
		return fmt.Errorf("collateral funding target payload exceeds %d bytes", maxRailPayloadBytes)
	}
	if err := validateBaseUnits(t.Amount, false); err != nil {
		return fmt.Errorf("collateral funding target amount: %w", err)
	}
	if t.ExpiresAt.IsZero() || !t.ExpiresAt.After(now) {
		return fmt.Errorf("collateral funding target is expired")
	}
	return nil
}

// RailFundingStatus is a receipt-verified funding projection returned during
// polling/reconciliation. Only Confirmed status may become FundingObservation.
type RailFundingStatus struct {
	State         RailActionState `json:"state"`
	Reference     string          `json:"reference,omitempty"`
	AssetID       string          `json:"assetID"`
	Amount        string          `json:"amount"`
	Confirmations uint64          `json:"confirmations,omitempty"`
	ObservedAt    time.Time       `json:"observedAt,omitempty"`
	LastError     string          `json:"lastError,omitempty"`
}

func (s RailFundingStatus) Validate() error {
	if s.State != RailActionPending && s.State != RailActionConfirmed && s.State != RailActionFailed {
		return fmt.Errorf("collateral funding status %q is unsupported", s.State)
	}
	if strings.TrimSpace(s.AssetID) == "" {
		return fmt.Errorf("collateral funding status asset is required")
	}
	if err := validateBaseUnits(s.Amount, false); err != nil {
		return fmt.Errorf("collateral funding status amount: %w", err)
	}
	if s.State == RailActionConfirmed && (strings.TrimSpace(s.Reference) == "" || s.ObservedAt.IsZero()) {
		return fmt.Errorf("confirmed collateral funding requires reference and observation time")
	}
	if s.State == RailActionFailed && strings.TrimSpace(s.LastError) == "" {
		return fmt.Errorf("failed collateral funding requires error")
	}
	return nil
}

// RailExecutionRequest is created only after Core has entered release-pending
// or slash-pending. Destination is derived by Core policy, never by extension
// evidence or an API caller.
type RailExecutionRequest struct {
	ActionID         string        `json:"actionID"`
	TenantID         string        `json:"tenantID"`
	CollateralID     string        `json:"collateralID"`
	ClaimID          string        `json:"claimID,omitempty"`
	Kind             ExecutionKind `json:"kind"`
	AssetID          string        `json:"assetID"`
	Amount           string        `json:"amount"`
	Destination      string        `json:"destination"`
	ExpectedRevision uint64        `json:"expectedRevision"`
	IdempotencyKey   string        `json:"idempotencyKey"`
}

func (r RailExecutionRequest) Validate() error {
	if missing(r.ActionID, r.TenantID, r.CollateralID, r.AssetID, r.Destination, r.IdempotencyKey) {
		return fmt.Errorf("collateral rail action, tenant, account, asset, destination, and idempotency key are required")
	}
	if r.Kind != ExecutionRelease && r.Kind != ExecutionSlash {
		return fmt.Errorf("collateral rail execution kind %q is unsupported", r.Kind)
	}
	if r.Kind == ExecutionSlash && strings.TrimSpace(r.ClaimID) == "" {
		return fmt.Errorf("collateral rail slash requires claim")
	}
	if r.Kind == ExecutionRelease && strings.TrimSpace(r.ClaimID) != "" {
		return fmt.Errorf("collateral rail release cannot include claim")
	}
	if r.ExpectedRevision == 0 {
		return fmt.Errorf("collateral rail expected revision is required")
	}
	if err := validateBaseUnits(r.Amount, false); err != nil {
		return fmt.Errorf("collateral rail amount: %w", err)
	}
	return nil
}

type RailActionResult struct {
	ActionID   string          `json:"actionID"`
	State      RailActionState `json:"state"`
	Reference  string          `json:"reference,omitempty"`
	LastError  string          `json:"lastError,omitempty"`
	ObservedAt time.Time       `json:"observedAt,omitempty"`
}

func (r RailActionResult) Validate() error {
	if strings.TrimSpace(r.ActionID) == "" {
		return fmt.Errorf("collateral rail action ID is required")
	}
	if r.State != RailActionPending && r.State != RailActionConfirmed && r.State != RailActionFailed {
		return fmt.Errorf("collateral rail action state %q is unsupported", r.State)
	}
	if r.State == RailActionConfirmed && (strings.TrimSpace(r.Reference) == "" || r.ObservedAt.IsZero()) {
		return fmt.Errorf("confirmed collateral rail action requires reference and observation time")
	}
	if r.State == RailActionFailed && strings.TrimSpace(r.LastError) == "" {
		return fmt.Errorf("failed collateral rail action requires error")
	}
	return nil
}

// Rail is the dedicated collateral payment boundary. Existing order
// settlement adapters do not satisfy this interface implicitly.
type Rail interface {
	Descriptor() RailDescriptor
	// PrepareFunding and SubmitExecution are create-or-retrieve operations.
	// Repeating the same validated idempotency identity must not create a
	// second external target or transfer.
	PrepareFunding(context.Context, FundingTargetRequest) (FundingTarget, error)
	// Status results are receipt-verified rail projections. Core still checks
	// every immutable binding before accepting a confirmed transition.
	FundingStatus(context.Context, FundingTarget) (RailFundingStatus, error)
	SubmitExecution(context.Context, RailExecutionRequest) (RailActionResult, error)
	// ExecutionStatus receives the immutable request so receipt verification
	// can bind action, collateral, claim, amount, and destination instead of
	// confirming an action identifier in isolation.
	ExecutionStatus(context.Context, RailExecutionRequest) (RailActionResult, error)
}

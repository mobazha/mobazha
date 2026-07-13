// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrOperatorUnavailable means no reviewed collateral rail is configured
	// for the selected node. Callers must not synthesize a funding target.
	ErrOperatorUnavailable = errors.New("collateral operator is unavailable")
	// ErrOperatorConflict identifies an idempotency or state conflict that is
	// safe to expose without leaking rail or persistence details.
	ErrOperatorConflict = errors.New("collateral operator conflict")
	// ErrOperatorInvalid identifies a rejected operator command.
	ErrOperatorInvalid = errors.New("collateral operator request is invalid")
)

// OperatorOpenRequest is tenant- and principal-neutral input. The Core service
// binds both values from the selected node instead of trusting an API caller.
type OperatorOpenRequest struct {
	ProviderID     string    `json:"providerID"`
	ResourceID     string    `json:"resourceID"`
	AssetID        string    `json:"assetID"`
	RequiredAmount string    `json:"requiredAmount"`
	PolicyID       string    `json:"policyID"`
	PolicyVersion  string    `json:"policyVersion"`
	IdempotencyKey string    `json:"idempotencyKey"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

// OperatorPrepareFundingRequest asks Core to create or retrieve one immutable
// funding target. PrincipalDestination is resolved by the owning distribution;
// it is never returned by the read-only status projection.
type OperatorPrepareFundingRequest struct {
	CollateralID         string `json:"collateralID"`
	PrincipalDestination string `json:"principalDestination"`
	IdempotencyKey       string `json:"idempotencyKey"`
}

// OperatorFundingStatus is the safe operational projection of a durable
// funding-target record. It deliberately omits tenant, principal destination,
// idempotency identity, raw rail payload, credentials, and private evidence.
type OperatorFundingStatus struct {
	RailID           string          `json:"railID"`
	RailVersion      string          `json:"railVersion"`
	State            RailActionState `json:"state"`
	AssetID          string          `json:"assetID"`
	Amount           string          `json:"amount"`
	Destination      string          `json:"destination,omitempty"`
	FundingReference string          `json:"fundingReference,omitempty"`
	Attempts         uint64          `json:"attempts"`
	LastErrorCode    string          `json:"lastErrorCode,omitempty"`
	ObservedAt       *time.Time      `json:"observedAt,omitempty"`
	ExpiresAt        time.Time       `json:"expiresAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

// OperatorAccountStatus combines the Core aggregate with its optional funding
// target. The API layer further removes tenant-scoped fields from Account.
type OperatorAccountStatus struct {
	Account Account                `json:"account"`
	Funding *OperatorFundingStatus `json:"funding,omitempty"`
}

// OperatorService is the narrow onboarding surface. Financial release, claim,
// slash, evidence, and credential authority are intentionally absent.
type OperatorService interface {
	Capabilities(context.Context) (RailDescriptor, bool)
	Open(context.Context, OperatorOpenRequest) (Account, error)
	ListAccounts(context.Context, string, string) ([]Account, error)
	Status(context.Context, string) (OperatorAccountStatus, error)
	PrepareFunding(context.Context, OperatorPrepareFundingRequest) (FundingTarget, error)
	ReconcileFunding(context.Context, string) (OperatorAccountStatus, error)
}

// OperatorServiceProvider is optional so distributions without an approved
// collateral composition keep the capability unavailable.
type OperatorServiceProvider interface {
	CollateralOperator() OperatorService
}

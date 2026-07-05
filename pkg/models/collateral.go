// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import "time"

// CollateralAccountRecord is the Core-owned financial aggregate projection.
type CollateralAccountRecord struct {
	TenantID           string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:uidx_collateral_open_idempotency,priority:1" json:"-"`
	CollateralID       string `gorm:"primaryKey;type:varchar(96)"`
	ProviderID         string `gorm:"type:varchar(160);not null;index:idx_collateral_resource,priority:1"`
	ResourceID         string `gorm:"type:varchar(256);not null;index:idx_collateral_resource,priority:2"`
	PrincipalID        string `gorm:"type:varchar(192);not null;index"`
	AssetID            string `gorm:"type:varchar(160);not null"`
	RequiredAmount     string `gorm:"type:varchar(128);not null"`
	FundedAmount       string `gorm:"type:varchar(128);not null"`
	AvailableAmount    string `gorm:"type:varchar(128);not null"`
	PolicyID           string `gorm:"type:varchar(160);not null"`
	PolicyVersion      string `gorm:"type:varchar(32);not null"`
	OpenIdempotencyKey string `gorm:"type:varchar(192);not null;uniqueIndex:uidx_collateral_open_idempotency,priority:2"`
	FundingReference   string `gorm:"type:varchar(256);index"`
	Revision           uint64 `gorm:"not null"`
	State              string `gorm:"type:varchar(32);not null;index"`
	ActivatedAt        *time.Time
	ExpiresAt          time.Time `gorm:"index"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CollateralFundingRecord claims one externally confirmed funding reference
// exactly once within a tenant and asset. This prevents one transfer from
// activating multiple collateral accounts.
type CollateralFundingRecord struct {
	TenantID         string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:uidx_collateral_funding_reference,priority:1" json:"-"`
	FundingID        string `gorm:"primaryKey;type:varchar(96)"`
	CollateralID     string `gorm:"type:varchar(96);not null;index"`
	AssetID          string `gorm:"type:varchar(160);not null;uniqueIndex:uidx_collateral_funding_reference,priority:2"`
	Amount           string `gorm:"type:varchar(128);not null"`
	FundingReference string `gorm:"type:varchar(256);not null;uniqueIndex:uidx_collateral_funding_reference,priority:3"`
	ObservedAt       time.Time
	CreatedAt        time.Time
}

// CollateralExecutionRecord claims one externally confirmed release or slash
// reference exactly once within a tenant and asset.
type CollateralExecutionRecord struct {
	TenantID           string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:uidx_collateral_execution_reference,priority:1" json:"-"`
	ExecutionID        string `gorm:"primaryKey;type:varchar(96)"`
	CollateralID       string `gorm:"type:varchar(96);not null;index"`
	ClaimID            string `gorm:"type:varchar(96);index"`
	Kind               string `gorm:"type:varchar(32);not null"`
	AssetID            string `gorm:"type:varchar(160);not null;uniqueIndex:uidx_collateral_execution_reference,priority:2"`
	Amount             string `gorm:"type:varchar(128);not null"`
	ExecutionReference string `gorm:"type:varchar(256);not null;uniqueIndex:uidx_collateral_execution_reference,priority:3"`
	ObservedAt         time.Time
	CreatedAt          time.Time
}

// CollateralAllocationRecord binds available coverage to one order extension.
type CollateralAllocationRecord struct {
	TenantID           string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:uidx_collateral_order_extension,priority:1;uniqueIndex:uidx_collateral_allocation_idempotency,priority:1" json:"-"`
	AllocationID       string `gorm:"primaryKey;type:varchar(96)"`
	CollateralID       string `gorm:"type:varchar(96);not null;index;uniqueIndex:uidx_collateral_order_extension,priority:2"`
	ProviderID         string `gorm:"type:varchar(160);not null"`
	ResourceID         string `gorm:"type:varchar(256);not null"`
	PrincipalID        string `gorm:"type:varchar(192);not null"`
	OrderID            string `gorm:"type:varchar(128);not null;index;uniqueIndex:uidx_collateral_order_extension,priority:3"`
	ExtensionID        string `gorm:"type:varchar(96);not null;index;uniqueIndex:uidx_collateral_order_extension,priority:4"`
	AssetID            string `gorm:"type:varchar(160);not null"`
	Amount             string `gorm:"type:varchar(128);not null"`
	CollateralRevision uint64 `gorm:"not null"`
	AllocationRevision uint64 `gorm:"not null"`
	State              string `gorm:"type:varchar(32);not null;index"`
	IdempotencyKey     string `gorm:"type:varchar(192);not null;uniqueIndex:uidx_collateral_allocation_idempotency,priority:2"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CollateralOrderExtensionBindingRecord stores the Core-issued v2 projection
// admitted for one persisted order extension. It is a reference, not a copy
// of provider evidence and not authority to mutate collateral.
type CollateralOrderExtensionBindingRecord struct {
	TenantID           string `gorm:"column:tenant_id;primaryKey;default:''" json:"-"`
	OrderID            string `gorm:"primaryKey;type:varchar(128);index"`
	ExtensionID        string `gorm:"primaryKey;type:varchar(96);index"`
	ContractVersion    string `gorm:"type:varchar(32);not null"`
	ExtensionRevision  uint64 `gorm:"primaryKey;not null"`
	AllocationID       string `gorm:"type:varchar(96);not null;index"`
	CollateralID       string `gorm:"type:varchar(96);not null;index"`
	ProviderID         string `gorm:"type:varchar(160);not null"`
	ResourceID         string `gorm:"type:varchar(256);not null"`
	PrincipalID        string `gorm:"type:varchar(192);not null"`
	AssetID            string `gorm:"type:varchar(160);not null"`
	Amount             string `gorm:"type:varchar(128);not null"`
	CollateralRevision uint64 `gorm:"not null"`
	AllocationRevision uint64 `gorm:"not null"`
	AllocationState    string `gorm:"type:varchar(32);not null"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CollateralAllocationCredentialRecord stores a signed seller-Core credential
// as issued or imported evidence. It never becomes a buyer-local allocation.
type CollateralAllocationCredentialRecord struct {
	TenantID          string    `gorm:"column:tenant_id;primaryKey;default:'';index:idx_collateral_credential_binding,priority:1" json:"-"`
	CredentialID      string    `gorm:"primaryKey;type:varchar(96)"`
	Direction         string    `gorm:"type:varchar(16);not null"`
	OrderID           string    `gorm:"type:varchar(128);not null;index;index:idx_collateral_credential_binding,priority:2"`
	ExtensionID       string    `gorm:"type:varchar(96);not null;index;index:idx_collateral_credential_binding,priority:3"`
	ExtensionRevision uint64    `gorm:"not null;index:idx_collateral_credential_binding,priority:4"`
	AudiencePeerID    string    `gorm:"type:varchar(192);not null;index:idx_collateral_credential_binding,priority:5"`
	IssuerPeerID      string    `gorm:"type:varchar(192);not null;index"`
	CredentialDigest  string    `gorm:"type:varchar(96);not null"`
	Credential        []byte    `gorm:"not null"`
	IssuedAt          time.Time `gorm:"index"`
	ExpiresAt         time.Time `gorm:"index"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// CollateralCredentialRefreshRecord is the buyer-side durable cursor for one
// extension revision. It throttles requests independently of transient ACK
// state and tracks only the newest imported credential for scheduler scans.
type CollateralCredentialRefreshRecord struct {
	TenantID             string    `gorm:"column:tenant_id;primaryKey;default:''" json:"-"`
	OrderID              string    `gorm:"primaryKey;type:varchar(128)"`
	ExtensionID          string    `gorm:"primaryKey;type:varchar(96)"`
	ExtensionRevision    uint64    `gorm:"primaryKey"`
	AudiencePeerID       string    `gorm:"primaryKey;type:varchar(192);index:idx_collateral_refresh_due,priority:1"`
	CredentialID         string    `gorm:"type:varchar(96)"`
	CredentialIssuedAt   time.Time `gorm:"index"`
	CredentialExpiresAt  time.Time `gorm:"index:idx_collateral_refresh_due,priority:2"`
	AccountExpiresAt     time.Time
	LastRequestMessageID string    `gorm:"type:varchar(96)"`
	LastRequestedAt      time.Time `gorm:"index:idx_collateral_refresh_due,priority:3"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// CollateralClaimRecord stores a Core-accepted, revision-bound claim. The
// record authorizes a later payment action; it is not itself proof of slash.
type CollateralClaimRecord struct {
	TenantID                   string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:uidx_collateral_claim_idempotency,priority:1;uniqueIndex:uidx_collateral_claim_attestation,priority:1;uniqueIndex:uidx_collateral_claim_attestation_idempotency,priority:1;uniqueIndex:uidx_collateral_claim_replay,priority:1" json:"-"`
	ClaimID                    string `gorm:"primaryKey;type:varchar(96)"`
	CollateralID               string `gorm:"type:varchar(96);not null;index"`
	AllocationID               string `gorm:"type:varchar(96);not null;index"`
	OrderID                    string `gorm:"type:varchar(128);not null;index"`
	ExtensionID                string `gorm:"type:varchar(96);not null;index"`
	AttestationID              string `gorm:"type:varchar(192);not null;uniqueIndex:uidx_collateral_claim_attestation,priority:2"`
	AttestationIdempotencyKey  string `gorm:"type:varchar(192);not null;uniqueIndex:uidx_collateral_claim_attestation_idempotency,priority:2"`
	ReplayFingerprint          string `gorm:"type:varchar(96);not null;uniqueIndex:uidx_collateral_claim_replay,priority:2"`
	IdempotencyKey             string `gorm:"type:varchar(192);not null;uniqueIndex:uidx_collateral_claim_idempotency,priority:2"`
	Issuer                     string `gorm:"type:varchar(160);not null"`
	Amount                     string `gorm:"type:varchar(128);not null"`
	Reason                     string `gorm:"type:varchar(256);not null"`
	ConditionType              string `gorm:"type:varchar(160);not null"`
	ConditionVersion           string `gorm:"type:varchar(32);not null"`
	EvidenceDigest             string `gorm:"type:varchar(256);not null"`
	ExpectedCollateralRevision uint64 `gorm:"not null"`
	ExpectedAllocationRevision uint64 `gorm:"not null"`
	ExpectedOrderStateVersion  string `gorm:"type:varchar(96);not null"`
	CollateralRevision         uint64 `gorm:"not null"`
	AllocationRevision         uint64 `gorm:"not null"`
	State                      string `gorm:"type:varchar(32);not null;index"`
	ExecutionReference         string `gorm:"type:varchar(256)"`
	ObservedAt                 time.Time
	ExpiresAt                  time.Time
	AcceptedAt                 time.Time
	UpdatedAt                  time.Time
}

// CollateralActionRecord is an append-only audit record for accepted Core
// collateral transitions. External rail execution will add its own durable
// action records in C2.
type CollateralActionRecord struct {
	TenantID                   string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:uidx_collateral_action_idempotency,priority:1" json:"-"`
	ActionID                   string `gorm:"primaryKey;type:varchar(96)"`
	CollateralID               string `gorm:"type:varchar(96);not null;index"`
	AllocationID               string `gorm:"type:varchar(96);index"`
	Kind                       string `gorm:"type:varchar(48);not null;index"`
	IdempotencyKey             string `gorm:"type:varchar(192);not null;uniqueIndex:uidx_collateral_action_idempotency,priority:2"`
	ExpectedCollateralRevision uint64 `gorm:"not null"`
	ResultCollateralRevision   uint64 `gorm:"not null"`
	ExpectedAllocationRevision uint64 `gorm:"not null;default:0"`
	ResultAllocationRevision   uint64 `gorm:"not null;default:0"`
	Amount                     string `gorm:"type:varchar(128)"`
	AssetID                    string `gorm:"type:varchar(160)"`
	Reason                     string `gorm:"type:varchar(256)"`
	Reference                  string `gorm:"type:varchar(256)"`
	CreatedAt                  time.Time
}

// CollateralFundingTargetRecord persists a funding-target request before the
// selected rail performs external work. A pending row without a destination
// is deliberately recoverable by repeating the same idempotent request.
type CollateralFundingTargetRecord struct {
	TenantID             string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:uidx_collateral_funding_target_idempotency,priority:1" json:"-"`
	CollateralID         string `gorm:"primaryKey;type:varchar(96)"`
	RailID               string `gorm:"type:varchar(160);not null"`
	RailVersion          string `gorm:"type:varchar(32);not null"`
	PrincipalID          string `gorm:"type:varchar(192);not null"`
	PrincipalDestination string `gorm:"type:varchar(512);not null;default:''"`
	AssetID              string `gorm:"type:varchar(160);not null"`
	Amount               string `gorm:"type:varchar(128);not null"`
	IdempotencyKey       string `gorm:"type:varchar(192);not null;uniqueIndex:uidx_collateral_funding_target_idempotency,priority:2"`
	ExpectedRevision     uint64 `gorm:"not null"`
	Destination          string `gorm:"type:varchar(512)"`
	Payload              []byte
	State                string `gorm:"type:varchar(32);not null;index"`
	FundingReference     string `gorm:"type:varchar(256)"`
	ObservedAmount       string `gorm:"type:varchar(128)"`
	Attempts             uint64 `gorm:"not null"`
	LastError            string `gorm:"type:text"`
	ObservedAt           *time.Time
	ExpiresAt            time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// CollateralRailActionRecord is the durable C2 execution intent. The immutable
// request is committed before SubmitExecution is called, so a crash or timeout
// can only leave recoverable pending work and never an untracked transfer.
type CollateralRailActionRecord struct {
	TenantID         string `gorm:"column:tenant_id;primaryKey;default:'';uniqueIndex:uidx_collateral_rail_action_idempotency,priority:1" json:"-"`
	ActionID         string `gorm:"primaryKey;type:varchar(96)"`
	CollateralID     string `gorm:"type:varchar(96);not null;index"`
	ClaimID          string `gorm:"type:varchar(96);index"`
	RailID           string `gorm:"type:varchar(160);not null"`
	RailVersion      string `gorm:"type:varchar(32);not null"`
	Kind             string `gorm:"type:varchar(32);not null"`
	AssetID          string `gorm:"type:varchar(160);not null"`
	Amount           string `gorm:"type:varchar(128);not null"`
	Destination      string `gorm:"type:varchar(512);not null"`
	ExpectedRevision uint64 `gorm:"not null"`
	IdempotencyKey   string `gorm:"type:varchar(192);not null;uniqueIndex:uidx_collateral_rail_action_idempotency,priority:2"`
	State            string `gorm:"type:varchar(32);not null;index"`
	Reference        string `gorm:"type:varchar(256)"`
	Attempts         uint64 `gorm:"not null"`
	LastError        string `gorm:"type:text"`
	ObservedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

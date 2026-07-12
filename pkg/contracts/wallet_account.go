// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import "context"

// WalletAccountRole identifies an isolated receiving account within the
// node-controlled wallet domain. Roles are stable business-independent names;
// each chain adapter owns the derivation scheme behind a role.
type WalletAccountRole string

const (
	AccountMain      WalletAccountRole = "main"
	AccountGuest     WalletAccountRole = "guest"
	AccountAffiliate WalletAccountRole = "affiliate"
)

// Valid reports whether the role is defined by the wallet-domain contract.
func (r WalletAccountRole) Valid() bool {
	switch r {
	case AccountMain, AccountGuest, AccountAffiliate:
		return true
	default:
		return false
	}
}

// Destination is a chain-specific receiving destination. RailID is the
// canonical network-qualified rail identifier; Tag carries memo or destination
// tag data when an adapter requires it.
type Destination struct {
	RailID  string
	Address string
	Tag     string
	Version uint32
}

// ReservedDestination is a durable address allocation. Index is adapter
// metadata and must not be interpreted as a derivation path by callers.
type ReservedDestination struct {
	Destination
	Index uint32
}

// WalletCapabilities declares which wallet-domain operations are complete for
// one rail. Guest must remain false until receive, watch, spend, confirmation,
// and restart recovery are all available together.
type WalletCapabilities struct {
	Receive      bool
	Watch        bool
	Spend        bool
	AutoTransfer bool
	Guest        bool
	Affiliate    bool
}

// WalletTransferRequest asks the wallet domain to move funds from one durable
// reservation. Amount is expressed in the rail's smallest native unit. When
// SweepAll is true Amount must be zero and the entire spendable balance is
// moved minus the network fee.
type WalletTransferRequest struct {
	RailID         string
	Role           WalletAccountRole
	ReferenceID    string
	Destination    string
	Amount         uint64
	SweepAll       bool
	IdempotencyKey string
}

// WalletTransferState is infrastructure state derived from transaction
// construction, broadcast, and chain confirmation.
type WalletTransferState string

const (
	WalletTransferPending   WalletTransferState = "pending"
	WalletTransferBuilt     WalletTransferState = "built"
	WalletTransferSubmitted WalletTransferState = "submitted"
	WalletTransferConfirmed WalletTransferState = "confirmed"
	WalletTransferReorged   WalletTransferState = "reorged"
)

// WalletTransfer is an opaque transfer receipt. Business services must not
// interpret it as commission, payout, or order state.
type WalletTransfer struct {
	IdempotencyKey string
	State          WalletTransferState
	TxHash         string
	Confirmations  int
	LastError      string
}

// WalletAccountService reserves receiving destinations without exposing key
// material or derivation paths to business services. A repeated reservation
// for the same rail, role, and reference returns the original destination.
type WalletAccountService interface {
	Capabilities(ctx context.Context, railID string) (WalletCapabilities, error)
	ReserveAddress(ctx context.Context, railID string, role WalletAccountRole, referenceID string) (ReservedDestination, error)
	Transfer(ctx context.Context, request WalletTransferRequest) (WalletTransfer, error)
	ReconcileTransfers(ctx context.Context) error
}

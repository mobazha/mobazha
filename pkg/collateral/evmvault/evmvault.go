// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Package evmvault exposes the reviewed EVM ERC-20 collateral rail to
// composition roots without making it part of Open Core's default runtime.
package evmvault

import internal "github.com/mobazha/mobazha/internal/collateral/evmvault"

const (
	DefaultRailID = internal.DefaultRailID
	VaultVersion  = internal.VaultVersion
)

type (
	Config              = internal.Config
	Backend             = internal.Backend
	TransactOptsFactory = internal.TransactOptsFactory
	VaultClient         = internal.VaultClient
	BindingClient       = internal.BindingClient
	Rail                = internal.Rail
	ObligationCommand   = internal.ObligationCommand
	FundingQuery        = internal.FundingQuery
	ExecutionCommand    = internal.ExecutionCommand
)

var (
	NewBindingClient = internal.NewBindingClient
	NewRail          = internal.NewRail
)

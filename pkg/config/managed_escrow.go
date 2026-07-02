// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package config

import iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"

const managedEscrowReleaseFeeUSDCentsPrefix = "managedEscrow.releaseFeeUSDCents."

func ManagedEscrowReleaseFeeUSDCentsKey(chainType iwallet.ChainType) string {
	return managedEscrowReleaseFeeUSDCentsPrefix + chainType.String()
}

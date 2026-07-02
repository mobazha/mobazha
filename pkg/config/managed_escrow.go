package config

import iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"

const managedEscrowReleaseFeeUSDCentsPrefix = "managedEscrow.releaseFeeUSDCents."

func ManagedEscrowReleaseFeeUSDCentsKey(chainType iwallet.ChainType) string {
	return managedEscrowReleaseFeeUSDCentsPrefix + chainType.String()
}

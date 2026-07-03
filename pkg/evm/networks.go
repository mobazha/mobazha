package evm

import iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"

// Network identifies the EIP-155 domains supported by Mobazha's EVM wallet
// layer. It deliberately contains no payment-provider readiness or deployment
// policy; commercial payment modules apply those checks themselves.
type Network struct {
	Chain        iwallet.ChainType
	MainnetID    uint64
	PublicTestID uint64
}

var networks = []Network{
	{Chain: iwallet.ChainEthereum, MainnetID: 1, PublicTestID: 11155111},
	{Chain: iwallet.ChainOptimism, MainnetID: 10},
	{Chain: iwallet.ChainBSC, MainnetID: 56, PublicTestID: 97},
	{Chain: iwallet.ChainGnosis, MainnetID: 100},
	{Chain: iwallet.ChainPolygon, MainnetID: 137, PublicTestID: 80002},
	{Chain: iwallet.ChainZkSyncEra, MainnetID: 324},
	{Chain: iwallet.ChainMantle, MainnetID: 5000},
	{Chain: iwallet.ChainBase, MainnetID: 8453, PublicTestID: 84532},
	{Chain: iwallet.ChainArbitrum, MainnetID: 42161},
	{Chain: iwallet.ChainCelo, MainnetID: 42220},
	{Chain: iwallet.ChainAvalanche, MainnetID: 43114},
	{Chain: iwallet.ChainLinea, MainnetID: 59144},
	{Chain: iwallet.ChainScroll, MainnetID: 534352},
}

// ChainIDForNetwork returns the EIP-155 domain for a runtime network. Public
// testnet lookup fails closed when Mobazha has no explicit mapping.
func ChainIDForNetwork(chain iwallet.ChainType, testnet bool) (uint64, bool) {
	for _, network := range networks {
		if network.Chain != chain {
			continue
		}
		if testnet {
			return network.PublicTestID, network.PublicTestID != 0
		}
		return network.MainnetID, true
	}
	return 0, false
}

// ChainTypeForID maps a known mainnet or public-testnet EIP-155 domain to its
// Mobazha chain type.
func ChainTypeForID(chainID uint64) (iwallet.ChainType, bool) {
	for _, network := range networks {
		if network.MainnetID == chainID || (network.PublicTestID != 0 && network.PublicTestID == chainID) {
			return network.Chain, true
		}
	}
	return "", false
}

package core

import (
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/chains"
	"github.com/mobazha/mobazha3.0/internal/chains/base"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/evm"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// evmChains lists the EVM chains that need shared client injection.
var evmChains = []iwallet.ChainType{
	iwallet.ChainBSC,
	iwallet.ChainEthereum,
	iwallet.ChainPolygon,
	iwallet.ChainBase,
	iwallet.ChainConflux,
}

// extractEVMConfigs converts multiwallet ChainAPIs to evm.EVMClientConfig slice.
// This bridges the multiwallet config (which has richer data like WSS URLs, multiple
// RPC fallbacks) to the simpler EVMClientConfig needed for chain client creation.
// The first RPC URL in the list is used as the primary endpoint.
func extractEVMConfigs(chainAPIs map[iwallet.ChainType]chains.APIUrls, testnet bool) []evm.EVMClientConfig {
	var configs []evm.EVMClientConfig
	for _, chain := range evmChains {
		api, ok := chainAPIs[chain]
		if !ok {
			continue
		}

		var rpcURL, registry, escrow string
		if testnet {
			if len(api.TestnetRpc) > 0 {
				rpcURL = api.TestnetRpc[0]
			}
			registry = api.TestnetRegistryAddress
			escrow = api.TestnetEscrowAddress
		} else {
			if len(api.MainnetRpc) > 0 {
				rpcURL = api.MainnetRpc[0]
			}
			registry = api.MainnetRegistryAddress
			escrow = api.MainnetEscrowAddress
		}

		if rpcURL == "" {
			continue
		}

		configs = append(configs, evm.EVMClientConfig{
			ChainType:       chain,
			RpcURL:          rpcURL,
			RegistryAddress: registry,
			EscrowAddress:   escrow,
			Testnet:         testnet,
		})
	}
	return configs
}

// startEVMChainClients injects EVM chain clients into wallets during Start().
// This is symmetric with startUTXOPaymentMonitor() — both follow the pattern:
//
//	Construction: nil ChainClient → Start(): inject based on mode
//
// SaaS mode:       shared clients from HostService (one RPC per chain, shared across tenants)
// Standalone mode: individual clients created via pkg/evm factory, using configs derived
//
//	from the node's multiwallet ChainAPIs (respecting user-configured RPC URLs)
func (n *MobazhaNode) startEVMChainClients() {
	if n.hostService != nil {
		// SaaS mode: use shared EVM clients from HostService
		configureEVMWallets(n.nodeID, n.multiwallet, n.hostService)
		return
	}

	// Standalone mode: use configs from node's ChainAPIs (set during construction),
	// falling back to compiled-in defaults if none were stored.
	configs := n.evmChainConfigs
	if len(configs) == 0 {
		configs = evm.GetDefaultConfigs(n.walletTestnet)
	}
	if len(configs) == 0 {
		logger.LogWarningWithIDf(log, n.nodeID, "No EVM chain configs available")
		return
	}

	configured := 0
	for _, cfg := range configs {
		wallet, ok := n.multiwallet.WalletForChain(cfg.ChainType)
		if !ok {
			continue
		}

		setter, ok := wallet.(base.ChainClientSetter)
		if !ok {
			continue
		}

		client, err := evm.NewSharedClient(cfg)
		if err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Failed to create EVM client for %s: %v", cfg.ChainType, err)
			continue
		}

		setter.SetChainClient(client)
		configured++
		logger.LogInfoWithIDf(log, n.nodeID, "Created standalone EVM client for %s (rpc=%s)", cfg.ChainType, cfg.RpcURL)
	}

	if configured > 0 {
		logger.LogInfoWithIDf(log, n.nodeID, "Configured %d EVM wallets with standalone chain clients", configured)
	}
}

// configureEVMWallets injects shared EVM chain clients from HostService into
// wallets that were created with nil ChainClient.
//
// The shared EVM client is a full *EthClient (with RPC connection) that:
//   - Supports GetRecommendedContractVersion() for dynamic escrow address lookup
//   - Supports GetTransaction() for payment verification
//   - Is shared across all SaaS tenant nodes (one per chain)
func configureEVMWallets(nodeID string, mw walletProvider, hs coreiface.HostService) {
	if mw == nil || hs == nil {
		return
	}

	configured := 0
	for _, chain := range evmChains {
		client := hs.GetEVMChainClient(chain)
		if client == nil {
			continue
		}

		wallet, ok := mw.WalletForChain(chain)
		if !ok {
			continue
		}

		if setter, ok := wallet.(base.ChainClientSetter); ok {
			setter.SetChainClient(client)
			configured++
			logger.LogInfoWithIDf(log, nodeID, "Configured %s wallet with shared EVM client", chain)
		}
	}

	if configured > 0 {
		logger.LogInfoWithIDf(log, nodeID, "Configured %d EVM wallets with shared chain clients", configured)
	}
}

// walletProvider is the minimal interface needed by configureEVMWallets.
// Both *Multiwallet and contracts.WalletOperator satisfy this.
type walletProvider interface {
	WalletForChain(chain iwallet.ChainType) (iwallet.Wallet, bool)
}

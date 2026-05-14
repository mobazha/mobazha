package evm

import (
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// defaultEVMChainDef holds the canonical RPC URLs and registry addresses for an EVM chain.
// Used by hosting's initSharedEVMClients() and as fallback for standalone nodes
// when ChainAPIs are not available (see startEVMChainClients in chain_evm.go).
type defaultEVMChainDef struct {
	Chain           iwallet.ChainType
	MainnetRpc      string
	MainnetRegistry string
	TestnetRpc      string
	TestnetRegistry string
}

// defaultEVMChains contains compiled-in defaults for all supported EVM chains.
var defaultEVMChains = []defaultEVMChainDef{
	{
		Chain:           iwallet.ChainBSC,
		MainnetRpc:      "https://56.rpc.thirdweb.com",
		MainnetRegistry: "0x4c1a1b21c4471ca57145ee08404cbaf9c8b83991",
		TestnetRpc:      "https://data-seed-prebsc-2-s2.binance.org:8545",
		TestnetRegistry: "0x8EC3ec712fd24b11c2Ce369a04249e7C439CB339",
	},
	{
		Chain:           iwallet.ChainEthereum,
		MainnetRpc:      "https://sepolia.drpc.org",
		MainnetRegistry: "0x24C0f3049Cc8188631bF74A9DE944223C9772156",
		TestnetRpc:      "https://sepolia.drpc.org",
		TestnetRegistry: "0x24C0f3049Cc8188631bF74A9DE944223C9772156",
	},
	{
		Chain:           iwallet.ChainPolygon,
		MainnetRpc:      "https://polygon-rpc.com",
		MainnetRegistry: "0x97EA76863a5843408E2727DCacCDa6A499BA747F",
		TestnetRpc:      "https://rpc-amoy.polygon.technology",
		TestnetRegistry: "0xb46a91f9546b6650453F2B54705E0e8e25C85247",
	},
	{
		Chain:           iwallet.ChainBase,
		MainnetRpc:      "https://mainnet.base.org",
		MainnetRegistry: "0x4c1a1b21c4471ca57145ee08404cbaf9c8b83991",
		TestnetRpc:      "https://sepolia.base.org",
		TestnetRegistry: "0x24C0f3049Cc8188631bF74A9DE944223C9772156",
	},
	{
		Chain:           iwallet.ChainConflux,
		MainnetRpc:      "https://evm.confluxrpc.com/GGna2h7aru3XSNFpeLrfT2ahYqM3YFeiX4FfgCgChdSfM9CbMXPik9762LBpKrzbC4c7kENDz2ikAYdyHQWjGiDvJ",
		MainnetRegistry: "0x17ebC8FeE90E7556E1E12Aa42604477D6A243324",
		TestnetRpc:      "https://evmtestnet.confluxrpc.com",
		TestnetRegistry: "0x93ecc969ff6C9e822F4AFD80acb59848eB9b9bf7",
	},
	// ── Phase EVM-ManagedEscrow v0.3.0 Sprint 1 D8 — promoted EVM L2 set ──
	// V1 ContractManager Registry is intentionally zero-address.
	// internal/chains/evm/client.go rejects GetRecommendedContractVersion
	// when the registry slot is empty/zero so V1 paths fail closed
	// (CHAIN_NOT_SUPPORTED). V2 ManagedEscrowAdapter shadow registration uses
	// the same chain client without invoking the V1 registry binding.
	// Public RPC endpoints sourced from chainlist.org; operators
	// SHOULD override in standalone repo config for production load.
	{Chain: iwallet.ChainArbitrum, MainnetRpc: "https://arb1.arbitrum.io/rpc", MainnetRegistry: zeroEVMRegistry},
	{Chain: iwallet.ChainOptimism, MainnetRpc: "https://mainnet.optimism.io", MainnetRegistry: zeroEVMRegistry},
	{Chain: iwallet.ChainAvalanche, MainnetRpc: "https://api.avax.network/ext/bc/C/rpc", MainnetRegistry: zeroEVMRegistry},
	{Chain: iwallet.ChainGnosis, MainnetRpc: "https://rpc.gnosischain.com", MainnetRegistry: zeroEVMRegistry},
	{Chain: iwallet.ChainCelo, MainnetRpc: "https://forno.celo.org", MainnetRegistry: zeroEVMRegistry},
	{Chain: iwallet.ChainMantle, MainnetRpc: "https://rpc.mantle.xyz", MainnetRegistry: zeroEVMRegistry},
	{Chain: iwallet.ChainZkSyncEra, MainnetRpc: "https://mainnet.era.zksync.io", MainnetRegistry: zeroEVMRegistry},
	{Chain: iwallet.ChainScroll, MainnetRpc: "https://rpc.scroll.io", MainnetRegistry: zeroEVMRegistry},
	{Chain: iwallet.ChainLinea, MainnetRpc: "https://rpc.linea.build", MainnetRegistry: zeroEVMRegistry},
}

// zeroEVMRegistry is the sentinel registry address marking a chain
// whose V1 ContractManager Registry has NOT been deployed (Phase
// EVM-ManagedEscrow v0.3.0 Sprint 1 D8 promoted set). The EVM client guard
// in internal/chains/evm/client.go treats this exact value as a
// V1-unsupported signal: any GetRecommendedContractVersion call
// returns ErrChainNotSupported instead of triggering an empty
// contract eth_call. The literal must stay verbatim — paste-typing
// alternative all-zero formats (e.g. "0x0") would bypass the guard.
const zeroEVMRegistry = "0x0000000000000000000000000000000000000000"

// GetDefaultConfigs returns default EVM client configs for all supported EVM chains.
// This is a standalone function — it does not depend on the factory being registered.
// Used by hosting's initSharedEVMClients() and as fallback for standalone nodes.
func GetDefaultConfigs(testnet bool) []EVMClientConfig {
	var configs []EVMClientConfig
	for _, dc := range defaultEVMChains {
		rpcURL := dc.MainnetRpc
		registryAddr := dc.MainnetRegistry
		if testnet {
			rpcURL = dc.TestnetRpc
			registryAddr = dc.TestnetRegistry
		}
		if rpcURL == "" {
			continue
		}
		configs = append(configs, EVMClientConfig{
			ChainType:       dc.Chain,
			RpcURL:          rpcURL,
			RegistryAddress: registryAddr,
			Testnet:         testnet,
		})
	}
	return configs
}

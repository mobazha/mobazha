package evm

import (
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// defaultEVMChainDef holds the canonical RPC URLs and registry addresses for an EVM chain.
// Used by hosting's initSharedEVMClients() and as fallback for standalone nodes
// when ChainAPIs are not available (see startEVMChainClients in evm_configure.go).
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
}

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

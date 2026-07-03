package core

import (
	"github.com/mobazha/mobazha/internal/chains"
	"github.com/mobazha/mobazha/internal/logger"
	tronWal "github.com/mobazha/mobazha/internal/chains/tron"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// TronChainConfig holds TRON chain configuration derived from multiwallet ChainAPIs.
type TronChainConfig struct {
	RpcEndpoints    []string
	EscrowAddress   string
	Testnet         bool
}

// extractTronConfig converts multiwallet ChainAPIs to TronChainConfig.
// Returns nil if no TRON config is available (no RPC URL).
func extractTronConfig(chainAPIs map[iwallet.ChainType]chains.APIUrls, testnet bool) *TronChainConfig {
	api, ok := chainAPIs[iwallet.ChainTRON]
	if !ok {
		return nil
	}

	var endpoints []string
	var escrow string
	if testnet {
		endpoints = api.TestnetRpc
		escrow = api.TestnetEscrowAddress
	} else {
		endpoints = api.MainnetRpc
		escrow = api.MainnetEscrowAddress
	}

	if len(endpoints) == 0 {
		return nil
	}

	return &TronChainConfig{
		RpcEndpoints:  endpoints,
		EscrowAddress: escrow,
		Testnet:       testnet,
	}
}

// startTRONChainClients creates TronClient from config and injects it into the TronWallet.
// Symmetric with startSolanaChainClients() and startEVMChainClients().
//
// The TronClient is also stored on the node for injection into TRONChainOps
// during registerPaymentStrategies().
func (n *MobazhaNode) startTRONChainClients() {
	cfg := n.tronChainConfig
	if cfg == nil {
		logger.LogDebugWithIDf(log, n.nodeID, "No TRON chain config available, skipping client creation")
		return
	}

	wallet, ok := n.multiwallet.WalletForChain(iwallet.ChainTRON)
	if !ok {
		return
	}
	tronW, ok := wallet.(*tronWal.TronWallet)
	if !ok {
		return
	}

	client := tronWal.NewTronClient(cfg.RpcEndpoints, tronWal.DefaultRetryConfig())
	tronW.ConfigureTronClient(client)
	n.tronClient = client

	logger.LogInfoWithIDf(log, n.nodeID, "Configured TRON wallet with client (endpoints=%v)", cfg.RpcEndpoints)
}

package core

import (
	"github.com/mobazha/mobazha3.0/internal/chains/base"
	internalutxo "github.com/mobazha/mobazha3.0/internal/chains/utxo"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// isSweepableP2WPKHChain reports whether a chain's default derivation type
// matches the current sweep signing capability (P2WPKH / BIP-143). BCH and
// ZEC use P2PKH; until the per-chain UTXOSweeper supports them, callers
// should NOT enable guest checkout for those chains because auto-sweep
// would fail after a buyer has already paid.
func isSweepableP2WPKHChain(chain iwallet.ChainType) bool {
	return chain.GetDefaultDerivationType() == iwallet.DerivationNativeSegwit
}

// configureUTXOWallets injects a UTXOChainClient into every UTXO wallet held
// by the node's multiwallet. Each wallet receives its chain-specific client
// backed by the shared ChainOperations (typically the UTXO Monitor).
//
// The standard self-hosted profile initializes this infrastructure;
// restricted profiles leave the multiwallet absent.
func (n *MobazhaNode) configureUTXOWallets(ops utxo.ChainOperations) {
	if n.multiwallet == nil || ops == nil {
		return
	}

	for _, chain := range n.multiwallet.SupportedChains() {
		if !chain.IsUTXOChain() {
			continue
		}

		wallet, ok := n.multiwallet.WalletForChain(chain)
		if !ok {
			continue
		}

		if setter, ok := wallet.(base.ChainClientSetter); ok {
			client := internalutxo.NewUTXOChainClient(ops, chain)
			setter.SetChainClient(client)
			logger.LogInfoWithIDf(log, n.nodeID, "Configured %s wallet with UTXOChainClient", chain)
		}
	}
}

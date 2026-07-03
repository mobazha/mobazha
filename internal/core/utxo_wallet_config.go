package core

import (
	"github.com/mobazha/mobazha/internal/chains/base"
	internalutxo "github.com/mobazha/mobazha/internal/chains/utxo"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/utxo"
)

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

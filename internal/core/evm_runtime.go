package core

import (
	evmchain "github.com/mobazha/mobazha3.0/internal/chains/evm"
	pkgEVM "github.com/mobazha/mobazha3.0/pkg/evm"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func (n *MobazhaNode) runtimeEVMChainID(chain iwallet.ChainType) uint64 {
	chainID, ok := pkgEVM.ChainIDForNetwork(chain, n.runtimeEVMUsesTestnet(chain))
	if !ok {
		return 0
	}
	return chainID
}

func (n *MobazhaNode) runtimeEVMUsesTestnet(chain iwallet.ChainType) bool {
	if n.walletTestnet {
		return true
	}
	if n.multiwallet == nil {
		return false
	}
	wallet, ok := n.multiwallet.WalletForChain(chain)
	if !ok || wallet == nil {
		return false
	}
	if wallet.IsTestnet() {
		return true
	}

	evmWallet, ok := wallet.(*evmchain.ETHWallet)
	if !ok || evmWallet == nil {
		return false
	}
	client, ok := evmWallet.ChainClient.(*evmchain.EthClient)
	return ok && client != nil && client.Testnet
}

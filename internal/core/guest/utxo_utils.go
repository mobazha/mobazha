package guest

import (
	"fmt"

	"github.com/mobazha/mobazha/pkg/contracts"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// utxoAddressUtilsFor resolves the UTXOAddressUtilities implementation for a
// given UTXO chain from the node's multiwallet. Each UTXO wallet
// (BTC/LTC/BCH/ZEC) implements this interface so chain-specific address
// encoding (HRP, network params, P2WPKH vs P2PKH) lives with the wallet.
func utxoAddressUtilsFor(mw contracts.WalletOperator, chainType iwallet.ChainType) (iwallet.UTXOAddressUtilities, error) {
	if mw == nil {
		return nil, fmt.Errorf("multiwallet not initialised; cannot derive %s address", chainType)
	}
	wallet, ok := mw.WalletForChain(chainType)
	if !ok {
		return nil, fmt.Errorf("wallet for %s not loaded", chainType)
	}
	utils, ok := wallet.(iwallet.UTXOAddressUtilities)
	if !ok {
		return nil, fmt.Errorf("wallet for %s does not implement UTXOAddressUtilities", chainType)
	}
	return utils, nil
}

// utxoSweeperFor resolves the UTXOSweeper implementation for a given UTXO chain
// from the node's multiwallet. Each UTXO wallet (BTC/LTC/BCH/ZEC) implements
// UTXOSweeper with chain-specific signing (P2WPKH for BTC/LTC, P2PKH for BCH/ZEC).
func utxoSweeperFor(mw contracts.WalletOperator, chainType iwallet.ChainType) (iwallet.UTXOSweeper, error) {
	if mw == nil {
		return nil, fmt.Errorf("multiwallet not initialised; cannot sweep %s", chainType)
	}
	wallet, ok := mw.WalletForChain(chainType)
	if !ok {
		return nil, fmt.Errorf("wallet for %s not loaded", chainType)
	}
	sweeper, ok := wallet.(iwallet.UTXOSweeper)
	if !ok {
		return nil, fmt.Errorf("wallet for %s does not implement UTXOSweeper", chainType)
	}
	return sweeper, nil
}

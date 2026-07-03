// Package contracts — WalletOperator abstracts multi-currency wallet operations.
//
// Standalone mode: backed by a real Multiwallet (UTXO + EVM + Solana chains).
// SaaS mode: backed by KeyVault signing + shared chain services.
package contracts

import (
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// WalletOperator abstracts multi-currency wallet operations.
//
// The core method is WalletForCurrencyCode, which returns an iwallet.Wallet
// for the given currency. All subsequent signing, transaction, and balance
// operations go through the iwallet.Wallet interface.
type WalletOperator interface {
	// WalletForCurrencyCode returns the wallet implementation for the given
	// currency code (e.g. "BTC", "ETH", "LTC", "BCH", "ZEC", "SOL", "STRIPE").
	WalletForCurrencyCode(currencyCode string) (iwallet.Wallet, error)

	// SupportedChains returns all chain types that have active wallets.
	// This allows callers to enumerate wallets without knowing the concrete type.
	SupportedChains() []iwallet.ChainType

	// WalletForChain returns the wallet for a specific chain type.
	// The boolean return indicates whether the chain is supported.
	WalletForChain(chain iwallet.ChainType) (iwallet.Wallet, bool)

	// Start initializes all configured wallets and begins background
	// monitoring (UTXO watching, block subscriptions, etc.).
	// SaaS implementations may use a no-op.
	Start() error

	// Close shuts down all wallets and background processes.
	// SaaS implementations may use a no-op.
	Close() error
}

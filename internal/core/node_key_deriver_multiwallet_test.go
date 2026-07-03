package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/mobazha/mobazha/internal/chains"
	"github.com/mobazha/mobazha/internal/config"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// testUTXOChains is the chain set exercised by NodeKeyDeriver tests.
var testUTXOChains = []iwallet.ChainType{
	iwallet.ChainBitcoin,
	iwallet.ChainLitecoin,
	iwallet.ChainBitcoinCash,
	iwallet.ChainZCash,
}

// loadTestMultiwallet builds a UTXO-only multiwallet for address-derivation
// tests and
// initializes each wallet's BIP-44 keys from the provided master key.
//
// Returns a multiwallet containing exactly the chains in testUTXOChains.
// ChainClient is intentionally left nil on every wallet — it is injected
// later by configureUTXOWallets() once Electrum sources are connected.
//
// This is intentionally narrower than production InitializeMultiwallet:
// it skips Solana, Fiat, and EVM because these tests only exercise UTXO
// BIP-44 derivation.
func loadTestMultiwallet(
	bip44Key *hdkeychain.ExtendedKey,
	cfg *repo.Config,
	netConfig *config.NetConfig,
	walletTestnet bool,
	dataDir string,
) (chains.Multiwallet, error) {
	if netConfig == nil {
		netConfig = &config.NetConfig{}
	}

	opts := []chains.Option{
		chains.DataDir(dataDir),
		chains.LogDir(cfg.LogDir),
		chains.Chains(testUTXOChains),
		chains.LogLevel(repo.LogLevelMap[strings.ToLower(defaultIfEmpty(cfg.LogLevel, "info"))]),
		chains.NetConfig(netConfig),
		chains.Testnet(walletTestnet),
		chains.Regtest(cfg.Regtest),
	}

	mw, _, err := chains.NewMultiwallet(opts...)
	if err != nil {
		return nil, fmt.Errorf("test multiwallet: %w", err)
	}

	// Initialize each UTXO wallet's BIP-44 keys. Mirrors the UTXO branch of
	// InitializeMultiwallet (full mode) but skips Solana/Fiat keypath.
	creationDate := time.Now()
	for chain, wallet := range mw {
		if !chain.IsUTXOChain() {
			continue
		}
		if wallet.WalletExists() {
			continue
		}

		canonicalNative, err := iwallet.RequireCanonicalNativeCoinType(chain)
		if err != nil {
			return nil, fmt.Errorf("canonical coin for %s: %w", chain, err)
		}
		pricingCode, err := canonicalNative.PricingCurrencyCode()
		if err != nil {
			return nil, fmt.Errorf("pricing code for %s: %w", chain, err)
		}
		def, err := models.CurrencyDefinitions.Lookup(pricingCode)
		if err != nil {
			return nil, fmt.Errorf("currency definition for %s: %w", chain, err)
		}

		coinTypeKey, err := bip44Key.Derive(hdkeychain.HardenedKeyStart + uint32(def.Bip44Code))
		if err != nil {
			return nil, fmt.Errorf("derive coin-type key for %s: %w", chain, err)
		}
		accountKey, err := coinTypeKey.Derive(hdkeychain.HardenedKeyStart + 0)
		if err != nil {
			return nil, fmt.Errorf("derive account key for %s: %w", chain, err)
		}

		if err := wallet.CreateWallet(*accountKey, creationDate); err != nil {
			return nil, fmt.Errorf("create %s wallet: %w", chain, err)
		}
	}

	return mw, nil
}

// defaultIfEmpty returns s if non-empty, otherwise fallback.
func defaultIfEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

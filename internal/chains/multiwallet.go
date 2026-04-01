package chains

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/mobazha/mobazha3.0/internal/chains/base"
	"github.com/mobazha/mobazha3.0/internal/chains/database"
	"github.com/mobazha/mobazha3.0/internal/chains/database/sqlitedb"
	"github.com/mobazha/mobazha3.0/internal/chains/evm"
	"github.com/mobazha/mobazha3.0/internal/chains/fiat/shim"
	"github.com/mobazha/mobazha3.0/internal/chains/solana"
	tronWal "github.com/mobazha/mobazha3.0/internal/chains/tron"
	"github.com/mobazha/mobazha3.0/internal/chains/utxo/bitcoin"
	"github.com/mobazha/mobazha3.0/internal/chains/utxo/bitcoincash"
	"github.com/mobazha/mobazha3.0/internal/chains/utxo/litecoin"
	"github.com/mobazha/mobazha3.0/internal/chains/utxo/zcash"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/natefinch/lumberjack"
)

var (
	defaultLogFilename = "multiwallet.log"
	ErrUnsuppertedCoin = errors.New("multiwallet does not contain an implementation for the given coin")
)

// Compile-time check: *Multiwallet implements contracts.WalletOperator.
var _ contracts.WalletOperator = (*Multiwallet)(nil)

// Multiwallet is the basic wallet map
type Multiwallet map[iwallet.ChainType]iwallet.Wallet

func NewMultiwallet(opts ...Option) (Multiwallet, error) {
	var cfg Config
	if err := cfg.Apply(append([]Option{Defaults}, opts...)...); err != nil {
		return nil, err
	}

	writers := []io.Writer{os.Stdout}
	if cfg.LogDir != "" {
		rotator := &lumberjack.Logger{
			Filename:   path.Join(cfg.LogDir, defaultLogFilename),
			MaxSize:    10, // Megabytes
			MaxBackups: 3,
			MaxAge:     30, // Days
		}
		writers = append(writers, rotator)
	}
	logging.Configure(logging.Config{Level: cfg.LogLevel, Format: logging.FormatText, Writers: writers})
	logger := logging.MustGetLogger("multiwallet").With("node_id", cfg.NodeID)

	os.MkdirAll(cfg.DataDir, os.ModePerm)
	db, err := sqlitedb.NewSqliteDB(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	if err := database.InitializeDatabase(db); err != nil {
		return nil, err
	}

	multiwallet := make(map[iwallet.ChainType]iwallet.Wallet)
	for _, chain := range cfg.Chains {
		switch chain {
		case iwallet.ChainBitcoinCash:
			w, err := bitcoincash.NewBitcoinCashWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				DB:        db,
				Testnet:   cfg.UseTestnet,
				Regtest:   cfg.UseRegtest,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, err
			}

			multiwallet[chain] = w
		case iwallet.ChainBitcoin:
			w, err := bitcoin.NewBitcoinWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				DB:        db,
				Testnet:   cfg.UseTestnet,
				Regtest:   cfg.UseRegtest,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, err
			}

			multiwallet[chain] = w
		case iwallet.ChainLitecoin:
			w, err := litecoin.NewLitecoinWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				DB:        db,
				Testnet:   cfg.UseTestnet,
				Regtest:   cfg.UseRegtest,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, err
			}

			multiwallet[chain] = w
		case iwallet.ChainZCash:
			w, err := zcash.NewZCashWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				DB:        db,
				Testnet:   cfg.UseTestnet,
				Regtest:   cfg.UseRegtest,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, err
			}

			multiwallet[chain] = w

		case iwallet.ChainBSC, iwallet.ChainEthereum, iwallet.ChainPolygon, iwallet.ChainBase, iwallet.ChainConflux:
			// ChainClient always nil at construction — unified with UTXO pattern.
			// Injected during MobazhaNode.Start() via startEVMChainClients():
			//   - Standalone: creates own EthClient per chain via pkg/evm factory
			//   - SaaS: gets shared EthClient from HostService
			coinType, err := iwallet.RequireCanonicalNativeCoinType(chain)
			if err != nil {
				return nil, err
			}
			w, err := evm.NewETHWallet(coinType, nil, &base.WalletConfig{
				Logger:    logger,
				DB:        db,
				Testnet:   cfg.UseTestnet,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, err
			}

			multiwallet[chain] = w

		case iwallet.ChainSolana:
			// ChainClient always nil at construction — unified with EVM/UTXO pattern.
			// Injected during MobazhaNode.Start() via startSolanaChainClients():
			//   - Standalone: creates own SolanaClient + resolves escrow from ContractManager
			//   - SaaS: gets shared SolanaClient + pre-resolved escrow from HostService
			w, err := solana.NewSolanaWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				DB:        db,
				Testnet:   cfg.UseTestnet,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, err
			}
			multiwallet[chain] = w
		case iwallet.ChainTRON:
			// TronClient nil at construction — injected during MobazhaNode.Start()
			// via startTRONChainClients(), symmetric with Solana/EVM pattern.
			w, err := tronWal.NewTronWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				DB:        db,
				Testnet:   cfg.UseTestnet,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, err
			}
			multiwallet[chain] = w
		case iwallet.ChainFiat:
			w, err := shim.NewFiatWalletShim(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				DB:        db,
				Testnet:   cfg.UseTestnet,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, err
			}

			multiwallet[chain] = w
		default:
			return nil, fmt.Errorf("a wallet implementation for %s does not exist", chain)
		}
	}

	return multiwallet, nil
}

func (w *Multiwallet) Start() error {
	for _, wallet := range *w {
		if err := wallet.OpenWallet(); err != nil {
			return err
		}
	}
	return nil
}

func (w *Multiwallet) Close() error {
	for _, wallet := range *w {
		if err := wallet.CloseWallet(); err != nil {
			return err
		}
	}
	return nil
}

func (w *Multiwallet) SupportedChains() []iwallet.ChainType {
	chains := make([]iwallet.ChainType, 0, len(*w))
	for chain := range *w {
		chains = append(chains, chain)
	}
	return chains
}

func (w *Multiwallet) WalletForChain(chain iwallet.ChainType) (iwallet.Wallet, bool) {
	wallet, ok := (*w)[chain]
	return wallet, ok
}

func (w *Multiwallet) WalletForCurrencyCode(currencyCode string) (iwallet.Wallet, error) {
	coinType := iwallet.CoinType(currencyCode)
	coinInfo, err := coinType.CoinInfo()
	if err != nil {
		return nil, err
	}

	chainType := coinInfo.Chain
	if coinType.IsFiatPayment() {
		chainType = iwallet.ChainFiat
	}

	if wallet, ok := (*w)[chainType]; ok {
		return wallet, nil
	}
	return nil, ErrUnsuppertedCoin
}

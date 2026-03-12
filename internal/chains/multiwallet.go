package chains

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/mobazha/mobazha3.0/internal/chains/base"
	"github.com/mobazha/mobazha3.0/internal/chains/utxo/bitcoin"
	"github.com/mobazha/mobazha3.0/internal/chains/utxo/bitcoincash"
	"github.com/mobazha/mobazha3.0/internal/chains/evm"
	"github.com/mobazha/mobazha3.0/internal/chains/utxo/litecoin"
	"github.com/mobazha/mobazha3.0/internal/chains/solana"
	"github.com/mobazha/mobazha3.0/internal/chains/fiat/stripe"
	"github.com/mobazha/mobazha3.0/internal/chains/utxo/zcash"
	"github.com/mobazha/mobazha3.0/internal/chains/database"
	"github.com/mobazha/mobazha3.0/internal/chains/database/sqlitedb"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/natefinch/lumberjack"
	"github.com/op/go-logging"
)

var (
	defaultLogFilename = "multiwallet.log"
	ErrUnsuppertedCoin = errors.New("multiwallet does not contain an implementation for the given coin")
	fileLogFormat      = logging.MustStringFormatter(`%{time:2006-01-02 T15:04:05.000} [%{level}] [%{module}] %{message}`)
	stdoutLogFormat    = logging.MustStringFormatter(`%{color:reset}%{color}%{time:15:04:05} [%{level}] [%{module}] %{message}`)
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

	logger := logging.MustGetLogger("multiwallet")

	backendStdout := logging.NewLogBackend(os.Stdout, fmt.Sprintf("[%s] ", cfg.NodeID), 0)
	backendStdoutFormatter := logging.NewBackendFormatter(backendStdout, stdoutLogFormat)

	if cfg.LogDir != "" {
		rotator := &lumberjack.Logger{
			Filename:   path.Join(cfg.LogDir, defaultLogFilename),
			MaxSize:    10, // Megabytes
			MaxBackups: 3,
			MaxAge:     30, // Days
		}

		backendFile := logging.NewLogBackend(rotator, fmt.Sprintf("[%s] ", cfg.NodeID), 0)
		backendFileFormatter := logging.NewBackendFormatter(backendFile, fileLogFormat)
		leveledBackend := logging.MultiLogger(backendStdoutFormatter, backendFileFormatter)
		leveledBackend.SetLevel(cfg.LogLevel, "")
		logger.SetBackend(leveledBackend)
	} else {
		leveledBackend := logging.AddModuleLevel(backendStdoutFormatter)
		leveledBackend.SetLevel(cfg.LogLevel, "")
		logger.SetBackend(leveledBackend)
	}

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
			clientURL := cfg.ChainAPIs[chain].MainnetWss
			if cfg.UseTestnet {
				clientURL = cfg.ChainAPIs[chain].TestnetWss
			}
			w, err := bitcoincash.NewBitcoinCashWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				DB:        db,
				ClientURL: clientURL,
				Testnet:   cfg.UseTestnet,
				FeeURL:    cfg.NetConfig.GetFeeUrl(chain),
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, err
			}

			multiwallet[chain] = w
		case iwallet.ChainBitcoin:
			clientURL := cfg.ChainAPIs[chain].MainnetWss
			if cfg.UseTestnet {
				clientURL = cfg.ChainAPIs[chain].TestnetWss
			}
			w, err := bitcoin.NewBitcoinWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				DB:        db,
				ClientURL: clientURL,
				Testnet:   cfg.UseTestnet,
				FeeURL:    cfg.NetConfig.GetFeeUrl(chain),
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, err
			}

			multiwallet[chain] = w
		case iwallet.ChainLitecoin:
			clientURL := cfg.ChainAPIs[chain].MainnetWss
			if cfg.UseTestnet {
				clientURL = cfg.ChainAPIs[chain].TestnetWss
			}
			w, err := litecoin.NewLitecoinWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				DB:        db,
				ClientURL: clientURL,
				Testnet:   cfg.UseTestnet,
				FeeURL:    cfg.NetConfig.GetFeeUrl(chain),
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, err
			}

			multiwallet[chain] = w
		case iwallet.ChainZCash:
			clientURL := cfg.ChainAPIs[chain].MainnetWss
			if cfg.UseTestnet {
				clientURL = cfg.ChainAPIs[chain].TestnetWss
			}
			w, err := zcash.NewZCashWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				DB:        db,
				ClientURL: clientURL,
				Testnet:   cfg.UseTestnet,
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
			w, err := evm.NewETHWallet(iwallet.CoinType(chain), nil, &base.WalletConfig{
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
		case iwallet.ChainStripe:
			w, err := stripe.NewStripeWallet(&base.WalletConfig{
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
		chainType = iwallet.ChainStripe
	}

	if wallet, ok := (*w)[chainType]; ok {
		return wallet, nil
	}
	return nil, ErrUnsuppertedCoin
}

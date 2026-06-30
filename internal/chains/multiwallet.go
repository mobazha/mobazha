package chains

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/mobazha/mobazha3.0/internal/chains/base"
	"github.com/mobazha/mobazha3.0/internal/chains/evm"
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

func NewMultiwallet(opts ...Option) (Multiwallet, *base.KeyStore, error) {
	var cfg Config
	if err := cfg.Apply(append([]Option{Defaults}, opts...)...); err != nil {
		return nil, nil, err
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

	keyStore := base.NewKeyStore()

	multiwallet := make(map[iwallet.ChainType]iwallet.Wallet)
	for _, chain := range cfg.Chains {
		switch chain {
		case iwallet.ChainBitcoinCash:
			w, err := bitcoincash.NewBitcoinCashWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				KeyStore:  keyStore,
				Testnet:   cfg.UseTestnet,
				Regtest:   cfg.UseRegtest,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, nil, err
			}

			multiwallet[chain] = w
		case iwallet.ChainBitcoin:
			w, err := bitcoin.NewBitcoinWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				KeyStore:  keyStore,
				Testnet:   cfg.UseTestnet,
				Regtest:   cfg.UseRegtest,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, nil, err
			}

			multiwallet[chain] = w
		case iwallet.ChainLitecoin:
			w, err := litecoin.NewLitecoinWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				KeyStore:  keyStore,
				Testnet:   cfg.UseTestnet,
				Regtest:   cfg.UseRegtest,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, nil, err
			}

			multiwallet[chain] = w
		case iwallet.ChainZCash:
			w, err := zcash.NewZCashWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				KeyStore:  keyStore,
				Testnet:   cfg.UseTestnet,
				Regtest:   cfg.UseRegtest,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, nil, err
			}

			multiwallet[chain] = w

		case iwallet.ChainBSC, iwallet.ChainEthereum, iwallet.ChainPolygon, iwallet.ChainBase, iwallet.ChainConflux,
			// Phase EVM-ManagedEscrow v0.3.0 Sprint 1 D8 — promoted EVM L2 set.
			// Wallets construct successfully but their chain client's V1
			// ContractManager Registry is intentionally not deployed
			// (RegistryAddress zero-address sentinel in chain config).
			// Order creation on V1 paths fails closed via the EVM client
			// guard in internal/chains/evm/client.go; V2 ManagedEscrowAdapter
			// shadow registration handles the V2 path.
			iwallet.ChainArbitrum, iwallet.ChainOptimism, iwallet.ChainAvalanche,
			iwallet.ChainGnosis, iwallet.ChainCelo, iwallet.ChainMantle,
			iwallet.ChainZkSyncEra, iwallet.ChainScroll, iwallet.ChainLinea:
			coinType, err := iwallet.RequireCanonicalNativeCoinType(chain)
			if err != nil {
				return nil, nil, err
			}
			w, err := evm.NewETHWallet(coinType, nil, &base.WalletConfig{
				Logger:    logger,
				KeyStore:  keyStore,
				Testnet:   cfg.UseTestnet,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, nil, err
			}

			multiwallet[chain] = w

		case iwallet.ChainSolana:
			// Solana is recognized for wire compatibility but its wallet/runtime
			// implementation is supplied only by the commercial module.
			continue
		case iwallet.ChainTRON:
			w, err := tronWal.NewTronWallet(&base.WalletConfig{
				NodeID:    cfg.NodeID,
				Logger:    logger,
				KeyStore:  keyStore,
				Testnet:   cfg.UseTestnet,
				NetConfig: cfg.NetConfig,
			})
			if err != nil {
				return nil, nil, err
			}
			multiwallet[chain] = w
		case iwallet.ChainFiat:
			// Fiat is intentionally not part of Multiwallet, aligned with
			// ChainExternalPayment handling below:
			//   - no cryptographic keys (Stripe/PayPal manage tokens
			//     server-side), so the iwallet.Wallet contract
			//     (Spend/Sweep/HasKey/Balance/HD-derive) is a category
			//     mismatch;
			//   - no on-chain semantics (GetTransaction /
			//     SubscribeTransactions / EstimateFee do not apply to
			//     PaymentIntents);
			//   - all fiat verification flows through
			//     FiatPaymentAppService + FiatProviderRegistry instead of
			//     iwallet.Wallet.
			// GetAllSupportedChainTypes() retains ChainFiat so that
			// ChainType.IsValid() and unrelated enumeration paths still treat
			// it as a recognised chain; we explicitly skip wallet
			// instantiation here.
			continue
		case iwallet.ChainExternalPayment:
			// ExternalPayment is intentionally not part of Multiwallet:
			//   - keys live inside the external_payment-wallet-rpc sidecar (not a shared
			//     in-process KeyStore), so the iwallet.Wallet contract
			//     (Spend/Sweep/HasKey/Balance) cannot be honoured here without
			//     leaking abstractions or breaking EXTERNAL_PAYMENT's privacy model;
			//   - on-chain transactions are not publicly visible (no
			//     GetTransaction semantics);
			//   - guest-checkout integration goes through
			//     pkg/external_payment.Source (impl: internal/chains/external_payment.Client) +
			//     pkg/external_payment.Monitor + DirectPaymentService instead.
			// GetAllSupportedChainTypes() includes ChainExternalPayment so that
			// ChainType.IsValid() and unrelated enumeration paths still treat
			// it as a recognised chain, but we explicitly skip it here rather
			// than fall through to the "implementation missing" error.
			continue
		default:
			return nil, nil, fmt.Errorf("a wallet implementation for %s does not exist", chain)
		}
	}

	return multiwallet, keyStore, nil
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

	if wallet, ok := (*w)[coinInfo.Chain]; ok {
		return wallet, nil
	}
	return nil, ErrUnsuppertedCoin
}

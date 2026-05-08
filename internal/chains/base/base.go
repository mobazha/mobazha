package base

import (
	"fmt"
	"sync"
	"time"

	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/mobazha/mobazha3.0/internal/config"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// WalletConfig is struct that can be used pass into the constructor
// for each coin's wallet.
type WalletConfig struct {
	NodeID    string
	KeyStore  *KeyStore
	Logger    *logging.Logger
	Testnet   bool
	Regtest   bool
	NetConfig *config.NetConfig
}

// DBTx satisfies the iwallet.Tx interface.
type DBTx struct {
	isClosed bool
	mtx      *sync.Mutex

	OnCommit func() error
}

// Commit will commit the transaction.
func (tx *DBTx) Commit() error {
	if tx.isClosed {
		panic("dbtx is closed")
	}
	if tx.OnCommit != nil {
		if err := tx.OnCommit(); err != nil {
			tx.Rollback()
			return err
		}
	}
	tx.isClosed = true
	tx.mtx.Unlock()
	return nil
}

// Rollback will rollback the transaction.
func (tx *DBTx) Rollback() error {
	if tx.isClosed {
		panic("dbtx is closed")
	}
	tx.OnCommit = nil
	tx.isClosed = true
	tx.mtx.Unlock()
	return nil
}

// WalletPostInitFunc is called after the wallet is initialized.
type WalletPostInitFunc func(masterKey *hd.ExtendedKey) error

// WalletBase is a base class that wallets can extended by the individual
// wallets. It contains a little over half the interface methods so the only
// remaining methods that need to be implemented by each coin's package are
// the methods specific to signing and building transactions.
type WalletBase struct {
	ChainClient  iwallet.ChainClient
	Keychain     *Keychain
	KeychainOpts []KeychainOption
	KeyStore     *KeyStore
	CoinType     iwallet.CoinType
	Logger       *logging.Logger
	PostInitFunc WalletPostInitFunc
	NetConfig    *config.NetConfig

	txMtx sync.Mutex

	Done chan struct{}

	featureManager *pkgconfig.FeatureManager
}

func (w *WalletBase) Init() {
	w.featureManager = pkgconfig.GetGlobalFeatureManager()
}

// SetChainClient replaces the ChainClient for this wallet.
// Used to inject the shared UTXOChainClient (backed by Electrum/Mempool Monitor)
// into UTXO wallets after construction.
func (w *WalletBase) SetChainClient(client iwallet.ChainClient) {
	w.ChainClient = client
}

// ChainClientSetter is an interface for wallets that support replacing ChainClient
type ChainClientSetter interface {
	SetChainClient(client iwallet.ChainClient)
}

// IsTestnet returns whether the wallet is using testnet.
// Note: This is a default implementation that should be overridden
// by specific wallet implementations that have testnet field.
func (w *WalletBase) IsTestnet() bool {
	return true
}

// Begin returns a new database transaction. A transaction must only be used
// once. After Commit() or Rollback() is called the transaction can be discarded.
func (w *WalletBase) Begin() (iwallet.Tx, error) {
	w.txMtx.Lock()
	return &DBTx{mtx: &w.txMtx}, nil
}

// WalletExists should return whether the wallet exits or has been
// initialized.
func (w *WalletBase) WalletExists() bool {
	return w.KeyStore.Has(w.CoinType)
}

// CreateWallet initializes the wallet by storing key material in the in-memory KeyStore.
func (w *WalletBase) CreateWallet(xpriv hd.ExtendedKey, birthday time.Time) error {
	xpub, err := xpriv.Neuter()
	if err != nil {
		return err
	}

	if w.KeyStore.Has(w.CoinType) {
		return fmt.Errorf("wallet already exists for coin %s", w.CoinType.CurrencyCode())
	}

	if w.PostInitFunc != nil {
		if err := w.PostInitFunc(&xpriv); err != nil {
			return err
		}
	}

	w.KeyStore.Put(w.CoinType, &KeyMaterial{
		AccountPriv: &xpriv,
		AccountPub:  xpub,
	})
	return nil
}

// OpenWallet will be called each time on Mobazha start. It
// will also be called after CreateWallet().
func (w *WalletBase) OpenWallet() error {
	km, ok := w.KeyStore.Get(w.CoinType)
	if !ok {
		return fmt.Errorf("key material not found for coin %s", w.CoinType.CurrencyCode())
	}
	keychain, err := NewKeychain(km, w.CoinType, w.KeychainOpts...)
	if err != nil {
		return err
	}
	w.Keychain = keychain
	w.txMtx = sync.Mutex{}

	return nil
}

// CloseWallet will be called when Mobazha shuts down.
func (w *WalletBase) CloseWallet() error {
	close(w.Done)
	return nil
}

// BlockchainInfo returns the best hash and height of the chain.
func (w *WalletBase) BlockchainInfo() (iwallet.BlockInfo, error) {
	return iwallet.BlockInfo{}, nil
}

// Whether the wallet is for coin or token of ETH like. Default false
func (w *WalletBase) CoinCategory() iwallet.CoinCategory {
	return iwallet.CoinCategoryBitcoin
}

// GetTransaction returns a transaction given it's ID.
func (w *WalletBase) GetTransaction(id iwallet.TransactionID, coinType iwallet.CoinType) (*iwallet.Transaction, error) {
	if w.ChainClient == nil {
		return nil, fmt.Errorf("transaction %s: chain client not available", id)
	}
	return w.ChainClient.GetTransaction(id, coinType)
}

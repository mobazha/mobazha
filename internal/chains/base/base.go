package base

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/mobazha/mobazha3.0/internal/chains/database"
	"github.com/mobazha/mobazha3.0/internal/config"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

// WalletConfig is struct that can be used pass into the constructor
// for each coin's wallet.
type WalletConfig struct {
	NodeID    string
	DB        database.Database
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
	DB           database.Database
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
	// Default implementation returns true
	// Each wallet should override this method to return their actual testnet status
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
	err := w.DB.View(func(tx database.Tx) error {
		var rec database.CoinRecord
		return tx.Read().Where("coin = ?", w.CoinType).First(&rec).Error
	})
	return !errors.Is(err, gorm.ErrRecordNotFound)
}

// CreateWallet should initialize the wallet. This will be called by
// Mobazha if WalletExists() returns false.
//
// The xPriv may be used to create a bip44 keychain. The xPriv is
// `cointype` level in the bip44 path. For example in the following
// path the wallet should only derive the paths after `account` as
// m, purpose', and coin_type' are kept private by Mobazha so this
// wallet cannot derive keys from other wallets.
//
// m / purpose' / coin_type' / account' / change / address_index
//
// The birthday can be used determine where to sync state from if
// appropriate.
//
// If the wallet does not implement WalletCrypter then pw will be
// nil. Otherwise it should be used to encrypt the private keys.
func (w *WalletBase) CreateWallet(xpriv hd.ExtendedKey, pw []byte, birthday time.Time) error {
	xpub, err := xpriv.Neuter()
	if err != nil {
		return err
	}

	err = w.DB.View(func(tx database.Tx) error {
		var rec database.CoinRecord
		return tx.Read().Where("coin = ?", w.CoinType).First(&rec).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	} else if err == nil {
		return fmt.Errorf("wallet already exists for coin %s", w.CoinType.CurrencyCode())
	}

	err = w.PostInitFunc(&xpriv)
	if err != nil {
		return err
	}

	return w.DB.Update(func(tx database.Tx) error {
		return tx.Save(&database.CoinRecord{
			MasterPriv:         xpriv.String(),
			EncryptedMasterKey: false,
			MasterPub:          xpub.String(),
			Coin:               w.CoinType.String(),
			Birthday:           birthday,
			BestBlockHeight:    0,
			BestBlockID:        strings.Repeat("0", 64),
		})
	})
}

// Open wallet will be called each time on Mobazha start. It
// will also be called after CreateWallet().
func (w *WalletBase) OpenWallet() error {
	keychain, err := NewKeychain(w.DB, w.CoinType, w.KeychainOpts...)
	if err != nil {
		return err
	}
	w.Keychain = keychain
	w.txMtx = sync.Mutex{}

	return nil
}

// CloseWallet will be called when Mobazha shuts down.
func (w *WalletBase) CloseWallet() error {
	if err := w.DB.Close(); err != nil {
		return err
	}

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

// SetPassphase is called after creating the wallet. It gives the wallet
// the opportunity to set up encryption of the private keys.
func (w *WalletBase) SetPassphase(pw []byte) error {
	return w.Keychain.SetPassphase(pw)
}

// ChangePassphrase is called in response to user action requesting the
// passphrase be changed. It is expected that this will return an error
// if the old password is incorrect.
func (w *WalletBase) ChangePassphrase(old, new []byte) error {
	return w.Keychain.ChangePassphrase(old, new)
}

// RemovePassphrase is called in response to user action requesting the
// passphrase be removed. It is expected that this will return an error
// if the old password is incorrect.
func (w *WalletBase) RemovePassphrase(pw []byte) error {
	return w.Keychain.RemovePassphrase(pw)
}

// Unlock is called just prior to calling Spend(). The wallet should
// decrypt the private key and hold the decrypted key in memory for
// the provided duration after which it should be purged from memory.
// If the provided password is incorrect it should error.
func (w *WalletBase) Unlock(pw []byte, howLong time.Duration) error {
	return w.Keychain.Unlock(pw, howLong)
}

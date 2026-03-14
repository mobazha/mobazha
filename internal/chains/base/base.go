package base

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/internal/chains/database"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/op/go-logging"
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

// AddrFunc is a function to convert an HD key to an address.
type AddrFunc func(key *hd.ExtendedKey) (iwallet.Address, error)

// PostInitFunc is a function to convert an HD key to an address.
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

// CurrentAddress is called when requesting this wallet's receiving
// address. It is customary that the wallet return the first unused
// address and only return a different address after funds have been
// received on the address. This, however, is just a wallet implementation
// detail.
func (w *WalletBase) CurrentAddress() (iwallet.Address, error) {
	return w.Keychain.CurrentAddress(false)
}

// GetTransaction returns a transaction given it's ID.
func (w *WalletBase) GetTransaction(id iwallet.TransactionID, coinType iwallet.CoinType) (*iwallet.Transaction, error) {
	var record database.TransactionRecord
	err := w.DB.View(func(tx database.Tx) error {
		return tx.Read().Where("coin=?", coinType.CurrencyCode()).Where("txid=?", id.String()).First(&record).Error
	})
	if err == nil {
		// We need to return the input metadata with this transaction. If it isn't stored with this
		// transaction in the database then we will need to use the API to get a copy of the transaction
		// with the input metadata.
		tx, err := record.Transaction()
		if err == nil {
			missingInputMetadata := false
			for _, in := range tx.From {
				if in.Address.String() == "" || in.Amount.String() == "" || in.Amount.Cmp(iwallet.NewAmount(0)) == 0 {
					missingInputMetadata = true
				}
			}
			if !missingInputMetadata {
				return &tx, nil
			}
		}
	}

	// Use ChainClient (which is now ElectrumChainClient for UTXO chains).
	// ChainClient may be nil before Start() injects it (keys-only wallet).
	if w.ChainClient == nil {
		if err != nil {
			return nil, fmt.Errorf("transaction %s not found in database and chain client not available: %w", id, err)
		}
		return nil, fmt.Errorf("transaction %s found in database but has incomplete metadata and chain client not available", id)
	}
	return w.ChainClient.GetTransaction(id, coinType)
}

// Transactions returns a slice of this wallet's transactions. The transactions should
// be sorted last to first and the limit and offset respected. The offsetID means
// 'return transactions starting with the transaction after offsetID in the sorted list'
func (w *WalletBase) Transactions(limit int, offsetID iwallet.TransactionID) ([]iwallet.Transaction, error) {
	var records []database.TransactionRecord
	err := w.DB.View(func(tx database.Tx) error {
		if offsetID != "" {
			var rec database.TransactionRecord
			err := tx.Read().Where("coin=?", w.CoinType.CurrencyCode()).Where("txid=?", offsetID.String()).First(&rec).Error
			if err != nil {
				return err
			}
			return tx.Read().Where("coin=?", w.CoinType.CurrencyCode()).Where("timestamp < ?", rec.Timestamp).Order("timestamp desc").Limit(limit).Find(&records).Error
		}

		return tx.Read().Where("coin=?", w.CoinType.CurrencyCode()).Order("timestamp desc").Limit(limit).Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	txs := make([]iwallet.Transaction, len(records))
	for i, rec := range records {
		txs[i], err = rec.Transaction()
		if err != nil {
			return nil, err
		}
	}
	return txs, nil
}

// Balance should return the confirmed and unconfirmed balance for the wallet.
func (w *WalletBase) Balance() (unconfirmed iwallet.Amount, confirmed iwallet.Amount, err error) {
	err = w.DB.View(func(dbtx database.Tx) error {
		var (
			utxoRecords []database.UtxoRecord
			txRecords   []database.TransactionRecord
			txMap       = make(map[iwallet.TransactionID]iwallet.Transaction)
		)
		err := dbtx.Read().Where("coin=?", w.CoinType.CurrencyCode()).Find(&utxoRecords).Error
		if err != nil {
			return err
		}
		err = dbtx.Read().Where("coin=?", w.CoinType.CurrencyCode()).Find(&txRecords).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		for _, record := range txRecords {
			tx, err := record.Transaction()
			if err != nil {
				return err
			}
			txMap[tx.ID] = tx
		}

		for _, utxo := range utxoRecords {
			if utxo.Height > 0 {
				confirmed = confirmed.Add(iwallet.NewAmount(utxo.Amount))
			} else {
				if checkIfStxoIsConfirmed(iwallet.TransactionID(utxo.Outpoint[:64]), txMap) {
					confirmed = confirmed.Add(iwallet.NewAmount(utxo.Amount))
				} else {
					unconfirmed = unconfirmed.Add(iwallet.NewAmount(utxo.Amount))
				}
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return unconfirmed, confirmed, nil
	}
	return unconfirmed, confirmed, nil
}

func checkIfStxoIsConfirmed(txid iwallet.TransactionID, txMap map[iwallet.TransactionID]iwallet.Transaction) bool {
	tx, ok := txMap[txid]
	if !ok {
		return false
	}

	// For each input, recursively check if confirmed
	inputsConfirmed := true
	for _, from := range tx.From {
		checkTx, ok := txMap[iwallet.TransactionID(hex.EncodeToString(from.ID[:32]))]
		if ok { // Is an stxo. If confirmed we can return true. If no, we need to check the dependency.
			if checkTx.Height == 0 {
				if !checkIfStxoIsConfirmed(iwallet.TransactionID(hex.EncodeToString(from.ID[:32])), txMap) {
					inputsConfirmed = false
				}
			}
		} else { // We don't have the tx in our db so it can't be an stxo. Return false.
			return false
		}
	}
	return inputsConfirmed
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


package base

import (
	"fmt"
	"testing"
	"time"

	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/mobazha/mobazha3.0/internal/chains/database"
	"github.com/mobazha/mobazha3.0/internal/chains/database/sqlitedb"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func setupWallet() (*WalletBase, error) {
	db, err := sqlitedb.NewMemoryDB()
	if err != nil {
		return nil, err
	}
	if err := database.InitializeDatabase(db); err != nil {
		return nil, err
	}
	logger, err := logging.GetLogger("test")
	if err != nil {
		return nil, err
	}
	w := &WalletBase{
		ChainClient: NewMockChainClient(),
		Done:        make(chan struct{}),
		DB:          db,
		Logger:      logger,
		CoinType:    iwallet.CtMock,
		PostInitFunc: func(xpriv *hd.ExtendedKey) error {
			return nil
		},
	}

	return w, nil
}

func TestWalletBase_WalletExists(t *testing.T) {
	w, err := setupWallet()
	if err != nil {
		t.Fatal(err)
	}
	if w.WalletExists() {
		t.Error("Wallet exists")
	}

	xpriv, err := hd.NewKeyFromString("tprv8ZgxMBicQKsPeghT19pungdFLMJM2hMs3EEn5WtgobD7wuQSFQu4VNaEJXH9HS3RhhLT4wgZ3hj31m3kafuxhL9vfGTRtBVLSog4zjxW3L1")
	if err != nil {
		t.Fatal(err)
	}

	if err := w.CreateWallet(*xpriv, nil, time.Now()); err != nil {
		t.Fatal(err)
	}

	if !w.WalletExists() {
		t.Error("Wallet does not exist")
	}
}

func TestWalletBase_OpenCloseWallet(t *testing.T) {
	w, err := setupWallet()
	if err != nil {
		t.Fatal(err)
	}

	xpriv, err := hd.NewKeyFromString("tprv8ZgxMBicQKsPeghT19pungdFLMJM2hMs3EEn5WtgobD7wuQSFQu4VNaEJXH9HS3RhhLT4wgZ3hj31m3kafuxhL9vfGTRtBVLSog4zjxW3L1")
	if err != nil {
		t.Fatal(err)
	}

	if err := w.CreateWallet(*xpriv, nil, time.Now()); err != nil {
		t.Fatal(err)
	}

	if err := w.OpenWallet(); err != nil {
		t.Fatal(err)
	}

	if err := w.CloseWallet(); err != nil {
		t.Fatal(err)
	}
}

func TestWalletBase_CurrentAddress(t *testing.T) {
	w, err := setupWallet()
	if err != nil {
		t.Fatal(err)
	}
	xpriv, err := hd.NewKeyFromString("tprv8ZgxMBicQKsPeghT19pungdFLMJM2hMs3EEn5WtgobD7wuQSFQu4VNaEJXH9HS3RhhLT4wgZ3hj31m3kafuxhL9vfGTRtBVLSog4zjxW3L1")
	if err != nil {
		t.Fatal(err)
	}

	if err := w.CreateWallet(*xpriv, nil, time.Now()); err != nil {
		t.Fatal(err)
	}

	if err := w.OpenWallet(); err != nil {
		t.Fatal(err)
	}

	current, err := w.CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}

	expected := "9324aa9a2c341003a4880f70aad70868b2c9b82d84032751ae7ce73b80a19bd9"
	if expected != current.String() {
		t.Errorf("Expected address %s, got %s", expected, current.String())
	}

	err = w.DB.View(func(tx database.Tx) error {
		current, err = w.Keychain.CurrentAddressWithTx(tx, false)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	if expected != current.String() {
		t.Errorf("Expected address %s, got %s", expected, current.String())
	}
}

func TestWalletBase_EncryptDecrypt(t *testing.T) {
	w, err := setupWallet()
	if err != nil {
		t.Fatal(err)
	}
	xpriv, err := hd.NewKeyFromString("tprv8ZgxMBicQKsPeghT19pungdFLMJM2hMs3EEn5WtgobD7wuQSFQu4VNaEJXH9HS3RhhLT4wgZ3hj31m3kafuxhL9vfGTRtBVLSog4zjxW3L1")
	if err != nil {
		t.Fatal(err)
	}

	if err := w.CreateWallet(*xpriv, nil, time.Now()); err != nil {
		t.Fatal(err)
	}

	if err := w.OpenWallet(); err != nil {
		t.Fatal(err)
	}

	addrs, err := w.Keychain.GetAddresses()
	if err != nil {
		t.Fatal(err)
	}

	if w.Keychain.IsEncrypted() {
		t.Fatal("Keychain is encrypted")
	}

	pw := []byte("let me in")
	if err := w.SetPassphase(pw); err != nil {
		t.Fatal(err)
	}

	if !w.Keychain.IsEncrypted() {
		t.Fatal("Keychain is not encrypted")
	}
	err = w.DB.Update(func(tx database.Tx) error {
		_, err := w.Keychain.KeyForAddress(tx, addrs[0], nil)
		return err
	})
	if err != ErrEncryptedKeychain {
		t.Errorf("Expected ErrEncryptedKeychain, got %s", err)
	}

	tryUnlockWithWrongPassword := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("recovered from %v", r)
			}
		}()

		err = w.Unlock([]byte("wrong password"), time.Second)
		return
	}

	if err := tryUnlockWithWrongPassword(); err == nil {
		t.Errorf("Expected decryption error got nil")
	}

	if err := w.Unlock(pw, time.Second); err != nil {
		t.Fatal(err)
	}

	if w.Keychain.IsEncrypted() {
		t.Fatal("Keychain is encrypted")
	}
	err = w.DB.Update(func(tx database.Tx) error {
		_, err := w.Keychain.KeyForAddress(tx, addrs[0], nil)
		return err
	})
	if err != nil {
		t.Errorf("Expected nil, got %s", err)
	}

	<-time.After(time.Second * 2)
	if !w.Keychain.IsEncrypted() {
		t.Fatal("Keychain is not encrypted")
	}
	err = w.DB.Update(func(tx database.Tx) error {
		_, err := w.Keychain.KeyForAddress(tx, addrs[0], nil)
		return err
	})
	if err != ErrEncryptedKeychain {
		t.Errorf("Expected ErrEncryptedKeychain, got %s", err)
	}
}

func TestWalletBase_ChangeRemovePassphrase(t *testing.T) {
	w, err := setupWallet()
	if err != nil {
		t.Fatal(err)
	}
	xpriv, err := hd.NewKeyFromString("tprv8ZgxMBicQKsPeghT19pungdFLMJM2hMs3EEn5WtgobD7wuQSFQu4VNaEJXH9HS3RhhLT4wgZ3hj31m3kafuxhL9vfGTRtBVLSog4zjxW3L1")
	if err != nil {
		t.Fatal(err)
	}

	if err := w.CreateWallet(*xpriv, nil, time.Now()); err != nil {
		t.Fatal(err)
	}

	if err := w.OpenWallet(); err != nil {
		t.Fatal(err)
	}

	addrs, err := w.Keychain.GetAddresses()
	if err != nil {
		t.Fatal(err)
	}

	if w.Keychain.IsEncrypted() {
		t.Fatal("Keychain is encrypted")
	}

	pw := []byte("let me in")
	if err := w.SetPassphase(pw); err != nil {
		t.Fatal(err)
	}

	if !w.Keychain.IsEncrypted() {
		t.Fatal("Keychain is not encrypted")
	}
	err = w.DB.Update(func(tx database.Tx) error {
		_, err := w.Keychain.KeyForAddress(tx, addrs[0], nil)
		return err
	})
	if err != ErrEncryptedKeychain {
		t.Errorf("Expected ErrEncryptedKeychain, got %s", err)
	}

	pw2 := []byte("let me in 2")
	if err := w.ChangePassphrase(pw, pw2); err != nil {
		t.Fatal(err)
	}

	if err := w.Unlock(pw2, time.Millisecond); err != nil {
		t.Fatal(err)
	}

	if w.Keychain.IsEncrypted() {
		t.Fatal("Keychain is encrypted")
	}
	err = w.DB.Update(func(tx database.Tx) error {
		_, err := w.Keychain.KeyForAddress(tx, addrs[0], nil)
		return err
	})
	if err != nil {
		t.Errorf("Expected nil, got %s", err)
	}

	<-time.After(time.Second)
	if !w.Keychain.IsEncrypted() {
		t.Fatal("Keychain is not encrypted")
	}
	err = w.DB.Update(func(tx database.Tx) error {
		_, err := w.Keychain.KeyForAddress(tx, addrs[0], nil)
		return err
	})
	if err != ErrEncryptedKeychain {
		t.Errorf("Expected ErrEncryptedKeychain, got %s", err)
	}

	if err := w.RemovePassphrase(pw2); err != nil {
		t.Fatal(err)
	}

	if w.Keychain.IsEncrypted() {
		t.Fatal("Keychain is encrypted")
	}
}

package base

import (
	"strings"
	"testing"
	"time"

	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/mobazha/mobazha3.0/internal/chains/database"
	"github.com/mobazha/mobazha3.0/internal/chains/database/sqlitedb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func setupKeychain() (*Keychain, error) {
	db, err := sqlitedb.NewMemoryDB()
	if err != nil {
		return nil, err
	}

	if err := database.InitializeDatabase(db); err != nil {
		return nil, err
	}

	xpriv, err := hd.NewKeyFromString("tprv8ZgxMBicQKsPeghT19pungdFLMJM2hMs3EEn5WtgobD7wuQSFQu4VNaEJXH9HS3RhhLT4wgZ3hj31m3kafuxhL9vfGTRtBVLSog4zjxW3L1")
	if err != nil {
		return nil, err
	}

	xpub, err := xpriv.Neuter()
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx database.Tx) error {
		return tx.Save(&database.CoinRecord{
			MasterPriv:         xpriv.String(),
			EncryptedMasterKey: false,
			MasterPub:          xpub.String(),
			Coin:               iwallet.CtMock.String(),
			Birthday:           time.Now(),
			BestBlockHeight:    0,
			BestBlockID:        strings.Repeat("0", 64),
		})
	})
	if err != nil {
		return nil, err
	}

	return NewKeychain(db, iwallet.CtMock)
}

func TestNewKeychain(t *testing.T) {
	keychain, err := setupKeychain()
	if err != nil {
		t.Fatal(err)
	}

	var addrs []database.AddressRecord
	err = keychain.db.View(func(tx database.Tx) error {
		return tx.Read().Find(&addrs).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 20 {
		t.Errorf("Expected 20 addresses got %d", len(addrs))
	}
}

func TestKeychain_EncryptDecrypt(t *testing.T) {
	keychain, err := setupKeychain()
	if err != nil {
		t.Fatal(err)
	}

	addrs, err := keychain.GetAddresses()
	if err != nil {
		t.Fatal(err)
	}

	if keychain.IsEncrypted() {
		t.Fatal("Keychain is encrypted")
	}

	pw := []byte("let me in")
	if err := keychain.SetPassphase(pw); err != nil {
		t.Fatal(err)
	}

	if !keychain.IsEncrypted() {
		t.Fatal("Keychain is not encrypted")
	}
	err = keychain.db.Update(func(tx database.Tx) error {
		_, err := keychain.KeyForAddress(tx, addrs[0], nil)
		return err
	})
	if err != ErrEncryptedKeychain {
		t.Errorf("Expected ErrEncryptedKeychain, got %s", err)
	}

	if err := keychain.Unlock([]byte("wrong password"), time.Second); err == nil {
		t.Errorf("Expected decryption error got nil")
	}

	if err := keychain.Unlock(pw, time.Millisecond); err != nil {
		t.Fatal(err)
	}

	if keychain.IsEncrypted() {
		t.Fatal("Keychain is encrypted")
	}
	err = keychain.db.Update(func(tx database.Tx) error {
		_, err := keychain.KeyForAddress(tx, addrs[0], nil)
		return err
	})
	if err != nil {
		t.Errorf("Expected nil, got %s", err)
	}

	<-time.After(time.Second)
	if !keychain.IsEncrypted() {
		t.Fatal("Keychain is not encrypted")
	}
	err = keychain.db.Update(func(tx database.Tx) error {
		_, err := keychain.KeyForAddress(tx, addrs[0], nil)
		return err
	})
	if err != ErrEncryptedKeychain {
		t.Errorf("Expected ErrEncryptedKeychain, got %s", err)
	}
}

func TestKeychain_ChangeRemovePassphrase(t *testing.T) {
	keychain, err := setupKeychain()
	if err != nil {
		t.Fatal(err)
	}

	addrs, err := keychain.GetAddresses()
	if err != nil {
		t.Fatal(err)
	}

	if keychain.IsEncrypted() {
		t.Fatal("Keychain is encrypted")
	}

	pw := []byte("let me in")
	if err := keychain.SetPassphase(pw); err != nil {
		t.Fatal(err)
	}

	if !keychain.IsEncrypted() {
		t.Fatal("Keychain is not encrypted")
	}
	err = keychain.db.Update(func(tx database.Tx) error {
		_, err := keychain.KeyForAddress(tx, addrs[0], nil)
		return err
	})
	if err != ErrEncryptedKeychain {
		t.Errorf("Expected ErrEncryptedKeychain, got %s", err)
	}

	pw2 := []byte("let me in 2")
	if err := keychain.ChangePassphrase(pw, pw2); err != nil {
		t.Fatal(err)
	}

	if err := keychain.Unlock(pw2, time.Millisecond); err != nil {
		t.Fatal(err)
	}

	if keychain.IsEncrypted() {
		t.Fatal("Keychain is encrypted")
	}
	err = keychain.db.Update(func(tx database.Tx) error {
		_, err := keychain.KeyForAddress(tx, addrs[0], nil)
		return err
	})
	if err != nil {
		t.Errorf("Expected nil, got %s", err)
	}

	<-time.After(time.Second)
	if !keychain.IsEncrypted() {
		t.Fatal("Keychain is not encrypted")
	}
	err = keychain.db.Update(func(tx database.Tx) error {
		_, err := keychain.KeyForAddress(tx, addrs[0], nil)
		return err
	})
	if err != ErrEncryptedKeychain {
		t.Errorf("Expected ErrEncryptedKeychain, got %s", err)
	}

	if err := keychain.RemovePassphrase(pw2); err != nil {
		t.Fatal(err)
	}

	if keychain.IsEncrypted() {
		t.Fatal("Keychain is encrypted")
	}
}

func TestKeychain_CurrentAddress(t *testing.T) {
	keychain, err := setupKeychain()
	if err != nil {
		t.Fatal(err)
	}

	current, err := keychain.CurrentAddress(false)
	if err != nil {
		t.Fatal(err)
	}

	expected := "9324aa9a2c341003a4880f70aad70868b2c9b82d84032751ae7ce73b80a19bd9"
	if expected != current.String() {
		t.Errorf("Expected address %s, got %s", expected, current.String())
	}

	err = keychain.db.View(func(tx database.Tx) error {
		current, err = keychain.CurrentAddressWithTx(tx, false)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	if expected != current.String() {
		t.Errorf("Expected address %s, got %s", expected, current.String())
	}
}

func TestKeychain_MarkAddressAsUsed(t *testing.T) {
	keychain, err := setupKeychain()
	if err != nil {
		t.Fatal(err)
	}
	addrs, err := keychain.GetAddresses()
	if err != nil {
		t.Fatal(err)
	}

	err = keychain.db.Update(func(tx database.Tx) error {
		return keychain.MarkAddressAsUsed(tx, addrs[0])
	})
	if err != nil {
		t.Fatal(err)
	}

	addrs, err = keychain.GetAddresses()
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 21 {
		t.Errorf("Expected 21 addresses got %d", len(addrs))
	}

	var recs []database.AddressRecord
	err = keychain.db.View(func(tx database.Tx) error {
		return tx.Read().Find(&recs).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	var (
		numUsed    = 0
		addrExists = false
	)

	for _, rec := range recs {
		if rec.Used {
			numUsed++
		}
		if rec.Addr == addrs[0].String() {
			addrExists = true
			if !rec.Used {
				t.Error("Failed to mark address as used")
			}
		}
	}
	if !addrExists {
		t.Error("Address does not exist in results")
	}
	if numUsed != 1 {
		t.Errorf("Expected 1 used got %d", numUsed)
	}
}

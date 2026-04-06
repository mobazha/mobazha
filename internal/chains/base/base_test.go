package base

import (
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

// TECHDEBT(TD-013): EncryptDecrypt and ChangeRemovePassphrase tests removed —
// they depended on KeyForAddress which requires address derivation (disabled).
// These tests should be restored when a new address derivation mechanism is implemented.

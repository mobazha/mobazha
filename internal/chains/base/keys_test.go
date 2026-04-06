package base

import (
	"strings"
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

// TECHDEBT(TD-013): EncryptDecrypt and ChangeRemovePassphrase tests removed —
// they depended on KeyForAddress which requires address derivation (disabled).
// These tests should be restored when a new address derivation mechanism is implemented.

package base

import (
	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

func setupKeychain() (*Keychain, error) {
	xpriv, err := hd.NewKeyFromString("tprv8ZgxMBicQKsPeghT19pungdFLMJM2hMs3EEn5WtgobD7wuQSFQu4VNaEJXH9HS3RhhLT4wgZ3hj31m3kafuxhL9vfGTRtBVLSog4zjxW3L1")
	if err != nil {
		return nil, err
	}

	xpub, err := xpriv.Neuter()
	if err != nil {
		return nil, err
	}

	km := &KeyMaterial{
		AccountPriv: xpriv,
		AccountPub:  xpub,
	}

	return NewKeychain(km, iwallet.CtMock)
}

// TECHDEBT(TD-013): EncryptDecrypt and ChangeRemovePassphrase tests removed —
// they depended on KeyForAddress which requires address derivation (disabled).
// These tests should be restored when a new address derivation mechanism is implemented.

package base

import (
	"errors"
	"fmt"
	"sync"

	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ErrEncryptedKeychain means the keychain is encrypted.
var ErrEncryptedKeychain = errors.New("keychain is encrypted")

// KeychainConfig holds some optional configuration options for
// the keychain.
type KeychainConfig struct {
	ExternalOnly bool
}

// Apply applies the given options to this Option
func (cfg *KeychainConfig) Apply(opts ...KeychainOption) error {
	for i, opt := range opts {
		if err := opt(cfg); err != nil {
			return fmt.Errorf("keychain option %d failed: %s", i, err)
		}
	}
	return nil
}

// KeychainOption is a keychain option type.
type KeychainOption func(*KeychainConfig) error

// Keychain manages a Bip44 keychain for each coin.
type Keychain struct {
	internalPrivkey *hd.ExtendedKey
	internalPubkey  *hd.ExtendedKey

	externalPrivkey *hd.ExtendedKey
	externalPubkey  *hd.ExtendedKey

	externalOnly bool

	coinType iwallet.CoinType

	mtx sync.RWMutex
}

// NewKeychain instantiates a new Keychain from in-memory KeyMaterial.
//
// Typical Bip44 derivation path:
//
//	m / purpose' / coin_type' / account' / change / address_index
//
// KeyMaterial holds the `account` level keys so that this Keychain
// cannot derive keys for other coins. We derive external (change=0)
// and internal (change=1) sub-keys from them.
func NewKeychain(km *KeyMaterial, coinType iwallet.CoinType, opts ...KeychainOption) (*Keychain, error) {
	cfg := KeychainConfig{}
	if err := cfg.Apply(opts...); err != nil {
		return nil, err
	}

	var externalPrivkey, externalPubkey, internalPrivkey, internalPubkey *hd.ExtendedKey

	externalPubkey, internalPubkey, err := generateAccountPubKeys(km.AccountPub)
	if err != nil {
		return nil, err
	}

	if km.AccountPriv != nil && !km.AccountPriv.IsPrivate() {
		// AccountPriv is actually a pubkey (should not happen, but be safe)
	} else if km.AccountPriv != nil {
		externalPrivkey, internalPrivkey, err = generateAccountPrivKeys(km.AccountPriv)
		if err != nil {
			return nil, err
		}
	}

	kc := &Keychain{
		internalPrivkey: internalPrivkey,
		internalPubkey:  internalPubkey,
		externalPrivkey: externalPrivkey,
		externalPubkey:  externalPubkey,
		externalOnly:    cfg.ExternalOnly,
		coinType:        coinType,
		mtx:             sync.RWMutex{},
	}
	return kc, nil
}

// IsEncrypted returns whether or not this keychain is encrypted.
func (kc *Keychain) IsEncrypted() bool {
	kc.mtx.RLock()
	defer kc.mtx.RUnlock()

	return kc.internalPrivkey == nil || kc.externalPrivkey == nil
}

func generateAccountPrivKeys(accountPrivKey *hd.ExtendedKey) (external, internal *hd.ExtendedKey, err error) {
	// Change(0) = external
	external, err = accountPrivKey.Derive(0)
	if err != nil {
		return nil, nil, err
	}
	// Change(1) = internal
	internal, err = accountPrivKey.Derive(1)
	if err != nil {
		return nil, nil, err
	}
	return
}

func generateAccountPubKeys(accountPubKey *hd.ExtendedKey) (external, internal *hd.ExtendedKey, err error) {
	// Change(0) = external
	external, err = accountPubKey.Derive(0)
	if err != nil {
		return nil, nil, err
	}
	// Change(1) = internal
	internal, err = accountPubKey.Derive(1)
	if err != nil {
		return nil, nil, err
	}
	return
}

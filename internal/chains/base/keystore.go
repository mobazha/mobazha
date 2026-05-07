package base

import (
	"sync"

	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// KeyMaterial holds derived BIP44 key material for one UTXO/EVM/TRON coin.
// Replaces CoinRecord persistence — keys are held in memory only.
type KeyMaterial struct {
	AccountPriv *hd.ExtendedKey
	AccountPub  *hd.ExtendedKey
}

// SolanaKeyMaterial holds raw ed25519 private key bytes for Solana.
// Uses []byte instead of solana.PrivateKey to avoid pulling solana-go
// into the base package (keeps PrivateDistribution binary lean).
type SolanaKeyMaterial struct {
	PrivKey []byte
}

// KeyStore provides in-memory key access for all coin wallets.
// One KeyStore is shared across all wallets in a Multiwallet instance.
type KeyStore struct {
	keys      map[iwallet.CoinType]*KeyMaterial
	solanaKey *SolanaKeyMaterial
	mu        sync.RWMutex
}

func NewKeyStore() *KeyStore {
	return &KeyStore{keys: make(map[iwallet.CoinType]*KeyMaterial)}
}

func (ks *KeyStore) Put(coinType iwallet.CoinType, km *KeyMaterial) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.keys[coinType] = km
}

func (ks *KeyStore) Get(coinType iwallet.CoinType) (*KeyMaterial, bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	km, ok := ks.keys[coinType]
	return km, ok
}

func (ks *KeyStore) Has(coinType iwallet.CoinType) bool {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	_, ok := ks.keys[coinType]
	return ok
}

func (ks *KeyStore) PutSolana(km *SolanaKeyMaterial) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.solanaKey = km
}

func (ks *KeyStore) GetSolana() (*SolanaKeyMaterial, bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.solanaKey, ks.solanaKey != nil
}

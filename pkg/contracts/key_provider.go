package contracts

import (
	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/gagliardetto/solana-go"
)

// KeyProvider abstracts access to the node's cryptographic master keys.
//
// Standalone mode: fileKeyProvider reads keys from node fields on disk.
// SaaS mode: keyVaultProvider fetches keys from a centralized KeyVault.
//
// Each method returns the master key for a specific domain. Consumers derive
// per-order keys from these masters using chain-specific derivation (HKDF,
// BIP32, etc.). The error return allows SaaS implementations to handle
// network failures or missing key entries gracefully.
type KeyProvider interface {
	// EVMMasterKey returns the secp256k1 private key used for EVM chain
	// escrow operations (signing, address derivation).
	EVMMasterKey() (*btcec.PrivateKey, error)

	// SolanaMasterKey returns the ed25519 private key used for Solana
	// escrow operations.
	SolanaMasterKey() (*solana.PrivateKey, error)

	// EscrowMasterKey returns the secp256k1 private key used for UTXO
	// multisig escrow key derivation.
	EscrowMasterKey() (*btcec.PrivateKey, error)

	// RatingMasterKey returns the secp256k1 private key used for
	// generating per-order rating signing keys.
	RatingMasterKey() (*btcec.PrivateKey, error)
}

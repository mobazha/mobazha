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

	// TRONMasterKey returns the secp256k1 private key used for TRON
	// escrow operations. Derived from an independent HD path
	// (m/44'/195'/0'/0/0) to avoid cross-chain key reuse with EVM.
	TRONMasterKey() (*btcec.PrivateKey, error)

	// DigitalContentMasterKey returns the 32-byte master key for digital
	// asset encryption at the given version. Per-asset keys are derived via
	// HKDF at runtime and never stored.
	//
	// Standalone: deterministic derivation from node mnemonic (BIP32).
	// SaaS: KeyVault per-tenant, supports multiple version coexistence.
	//
	// Version is 1-based and monotonically increasing. Old versions MUST
	// remain accessible until all assets have been re-encrypted.
	DigitalContentMasterKey(version int) ([]byte, error)
}

// ProviderCredentialKeyProvider supplies versioned, tenant-scoped encryption
// keys for payment-provider credentials. It is deliberately separate from
// KeyProvider so hosted distributions can back it with a dedicated KMS/Vault
// without exposing payment secrets to chain-signing components.
type ProviderCredentialKeyProvider interface {
	ProviderCredentialMasterKey(version uint64) ([]byte, error)
}

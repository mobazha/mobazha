package guest

import (
	"fmt"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// NodeKeyDeriver implements BIP44KeyDeriver using the node's BIP44
// master key. masterKey is at the m/44' level (purpose already derived),
// consistent with builder.go's bip44Key. UTXO address encoding is delegated to
// the per-chain wallet (via iwallet.UTXOAddressUtilities) so chain-specific
// HRP/network rules live with the wallet implementation, not in core.
//
// Address encoding for non-UTXO chains (EVM, TRON) is provided by the full
// build via DeriveAddress in key_deriver_full.go. The private_distribution build
// restricts DeriveAddress to UTXO chains only.
type NodeKeyDeriver struct {
	masterKey   *hdkeychain.ExtendedKey
	multiwallet contracts.WalletOperator
}

// NewNodeKeyDeriver creates a BIP44KeyDeriver backed by the node's BIP-44
// master key. multiwallet is required for UTXO address derivation; pass nil
// only if no UTXO chains will be derived.
func NewNodeKeyDeriver(bip44Key *hdkeychain.ExtendedKey, multiwallet contracts.WalletOperator) *NodeKeyDeriver {
	return &NodeKeyDeriver{
		masterKey:   bip44Key,
		multiwallet: multiwallet,
	}
}

// DerivePrivateKey derives the raw private key bytes at the given HD index.
// This is chain-agnostic — the BIP44 path determines the key, and all
// supported chains use secp256k1.
func (d *NodeKeyDeriver) DerivePrivateKey(chainType iwallet.ChainType, index uint32) ([]byte, error) {
	childKey, err := d.deriveChildKey(chainType, index)
	if err != nil {
		return nil, err
	}

	privKey, err := childKey.ECPrivKey()
	if err != nil {
		return nil, fmt.Errorf("extract private key: %w", err)
	}

	return privKey.Serialize(), nil
}

// deriveChildKey derives the extended key at {coinType}'/0'/0/{index}
// from the m/44' master key.
func (d *NodeKeyDeriver) deriveChildKey(chainType iwallet.ChainType, index uint32) (*hdkeychain.ExtendedKey, error) {
	coinType, ok := iwallet.CanonicalBIP44CoinType(chainType)
	if !ok {
		return nil, fmt.Errorf("unsupported chain type for HD derivation: %s", chainType)
	}

	coinTypeKey, err := d.masterKey.Derive(hdkeychain.HardenedKeyStart + coinType)
	if err != nil {
		return nil, fmt.Errorf("derive coinType key: %w", err)
	}

	accountKey, err := coinTypeKey.Derive(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return nil, fmt.Errorf("derive account key: %w", err)
	}

	changeKey, err := accountKey.Derive(0) // external chain
	if err != nil {
		return nil, fmt.Errorf("derive change key: %w", err)
	}

	childKey, err := changeKey.Derive(index)
	if err != nil {
		return nil, fmt.Errorf("derive child key at index %d: %w", index, err)
	}

	return childKey, nil
}

// encodeUTXOAddress converts an extended key to a UTXO chain address using
// the per-chain wallet's UTXOAddressUtilities.
func (d *NodeKeyDeriver) encodeUTXOAddress(key *hdkeychain.ExtendedKey, chainType iwallet.ChainType) (string, error) {
	pubKey, err := key.ECPubKey()
	if err != nil {
		return "", fmt.Errorf("get UTXO public key: %w", err)
	}

	utils, err := utxoAddressUtilsFor(d.multiwallet, chainType)
	if err != nil {
		return "", err
	}

	addr, _, err := utils.DerivePaymentAddressFromPubKey(pubKey)
	if err != nil {
		return "", fmt.Errorf("encode UTXO address: %w", err)
	}
	return addr, nil
}

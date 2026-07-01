package guest

import (
	"fmt"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"

	"github.com/mobazha/mobazha3.0/internal/chains/tron"
)

// NodeKeyDeriver implements BIP44KeyDeriver using the node's BIP44 master
// key. UTXO address encoding remains delegated to the chain wallet so network
// rules do not leak into Core.
type NodeKeyDeriver struct {
	masterKey   *hdkeychain.ExtendedKey
	multiwallet contracts.WalletOperator
}

func NewNodeKeyDeriver(bip44Key *hdkeychain.ExtendedKey, multiwallet contracts.WalletOperator) *NodeKeyDeriver {
	return &NodeKeyDeriver{masterKey: bip44Key, multiwallet: multiwallet}
}

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

func (d *NodeKeyDeriver) deriveChildKey(chainType iwallet.ChainType, index uint32) (*hdkeychain.ExtendedKey, error) {
	coinType, ok := iwallet.CanonicalBIP44CoinType(chainType)
	if !ok {
		return nil, fmt.Errorf("unsupported chain type for HD derivation: %s", chainType)
	}
	coinTypeKey, err := d.masterKey.Derive(hdkeychain.HardenedKeyStart + coinType)
	if err != nil {
		return nil, fmt.Errorf("derive coinType key: %w", err)
	}
	accountKey, err := coinTypeKey.Derive(hdkeychain.HardenedKeyStart)
	if err != nil {
		return nil, fmt.Errorf("derive account key: %w", err)
	}
	changeKey, err := accountKey.Derive(0)
	if err != nil {
		return nil, fmt.Errorf("derive change key: %w", err)
	}
	childKey, err := changeKey.Derive(index)
	if err != nil {
		return nil, fmt.Errorf("derive child key at index %d: %w", index, err)
	}
	return childKey, nil
}

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

// DeriveAddress derives a chain-specific address at the given HD index.
// Full build supports UTXO, EVM, and TRON chains.
func (d *NodeKeyDeriver) DeriveAddress(chainType iwallet.ChainType, index uint32) (string, error) {
	childKey, err := d.deriveChildKey(chainType, index)
	if err != nil {
		return "", err
	}
	return d.encodeAddress(childKey, chainType)
}

// isGuestEVMChain returns true for EVM-compatible chains supported by Guest Checkout.
func isGuestEVMChain(ct iwallet.ChainType) bool {
	switch ct {
	case iwallet.ChainEthereum, iwallet.ChainBSC, iwallet.ChainPolygon, iwallet.ChainBase:
		return true
	default:
		return false
	}
}

func (d *NodeKeyDeriver) encodeAddress(key *hdkeychain.ExtendedKey, chainType iwallet.ChainType) (string, error) {
	switch {
	case chainType.IsUTXOChain():
		return d.encodeUTXOAddress(key, chainType)
	case isGuestEVMChain(chainType):
		return d.encodeEVMAddress(key)
	case chainType == iwallet.ChainTRON:
		return d.encodeTRONAddress(key)
	default:
		return "", fmt.Errorf("unsupported chain type for address encoding: %s", chainType)
	}
}

func (d *NodeKeyDeriver) encodeEVMAddress(key *hdkeychain.ExtendedKey) (string, error) {
	pubKey, err := key.ECPubKey()
	if err != nil {
		return "", fmt.Errorf("get EVM public key: %w", err)
	}
	addr := ethcrypto.PubkeyToAddress(*pubKey.ToECDSA())
	return addr.Hex(), nil
}

func (d *NodeKeyDeriver) encodeTRONAddress(key *hdkeychain.ExtendedKey) (string, error) {
	pubKey, err := key.ECPubKey()
	if err != nil {
		return "", fmt.Errorf("get TRON public key: %w", err)
	}
	return tron.PubKeyToTRONAddress(pubKey.ToECDSA()), nil
}

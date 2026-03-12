package utxo

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// AddressProcessor handles payment address derivation for UTXO-based chains (BTC, LTC, BCH, ZEC)
type AddressProcessor struct {
	chainType      iwallet.ChainType
	chainParams    *chaincfg.Params
	derivationType iwallet.DerivationType
}

// NewAddressProcessor creates a new UTXO address processor
func NewAddressProcessor(chainType iwallet.ChainType) (*AddressProcessor, error) {
	params, err := getChainParams(chainType)
	if err != nil {
		return nil, err
	}

	return &AddressProcessor{
		chainType:      chainType,
		chainParams:    params,
		derivationType: chainType.GetDefaultDerivationType(),
	}, nil
}

// getChainParams returns the chain parameters for a chain type
func getChainParams(chainType iwallet.ChainType) (*chaincfg.Params, error) {
	switch chainType {
	case iwallet.ChainBitcoin:
		return &chaincfg.MainNetParams, nil
	case iwallet.ChainLitecoin:
		// Litecoin uses modified parameters
		params := chaincfg.MainNetParams
		params.Name = "litecoin"
		params.Bech32HRPSegwit = "ltc"
		params.PubKeyHashAddrID = 0x30 // L
		params.ScriptHashAddrID = 0x32 // M
		return &params, nil
	case iwallet.ChainBitcoinCash:
		// BCH uses legacy Bitcoin parameters for transparent addresses
		params := chaincfg.MainNetParams
		params.Name = "bitcoincash"
		return &params, nil
	case iwallet.ChainZCash:
		// ZEC transparent addresses use 2-byte prefixes
		// For now, we use Bitcoin params and handle encoding separately
		params := chaincfg.MainNetParams
		params.Name = "zcash"
		return &params, nil
	default:
		return nil, fmt.Errorf("unsupported chain type: %s", chainType)
	}
}

// DerivePaymentAddress derives a payment address from escrow public key and chaincode
// This uses the same derivation as GenerateEscrowPublicKey in internal/orders/utils/keys.go
// so both buyer and seller can derive the same address deterministically.
func (p *AddressProcessor) DerivePaymentAddress(escrowPubKey *btcec.PublicKey, chaincode []byte) (string, []byte, error) {
	if escrowPubKey == nil {
		return "", nil, errors.New("escrow public key cannot be nil")
	}
	if len(chaincode) != 32 {
		return "", nil, errors.New("chaincode must be 32 bytes")
	}

	// Use the same derivation method as GenerateEscrowPublicKey
	hdKey := hdkeychain.NewExtendedKey(
		chaincfg.MainNetParams.HDPublicKeyID[:],
		escrowPubKey.SerializeCompressed(),
		chaincode,
		[]byte{0x00, 0x00, 0x00, 0x00},
		0,
		0,
		false)

	// Derive child key at index 0
	childKey, err := hdKey.Derive(0)
	if err != nil {
		return "", nil, fmt.Errorf("failed to derive child key: %w", err)
	}

	pubKey, err := childKey.ECPubKey()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get public key: %w", err)
	}

	return p.generateAddress(pubKey)
}

// DerivePaymentAddressFromPrivKey derives a payment address from escrow private key and chaincode
// This is used by the seller's node to get both address and signing capability
func (p *AddressProcessor) DerivePaymentAddressFromPrivKey(escrowPrivKey *btcec.PrivateKey, chaincode []byte) (string, []byte, *btcec.PrivateKey, error) {
	if escrowPrivKey == nil {
		return "", nil, nil, errors.New("escrow private key cannot be nil")
	}
	if len(chaincode) != 32 {
		return "", nil, nil, errors.New("chaincode must be 32 bytes")
	}

	hdKey := hdkeychain.NewExtendedKey(
		chaincfg.MainNetParams.HDPrivateKeyID[:],
		escrowPrivKey.Serialize(),
		chaincode,
		[]byte{0x00, 0x00, 0x00, 0x00},
		0,
		0,
		true)

	childKey, err := hdKey.Derive(0)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to derive child key: %w", err)
	}

	derivedPrivKey, err := childKey.ECPrivKey()
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to get private key: %w", err)
	}

	pubKey := derivedPrivKey.PubKey()
	address, scriptPubKey, err := p.generateAddress(pubKey)
	if err != nil {
		return "", nil, nil, err
	}

	return address, scriptPubKey, derivedPrivKey, nil
}

// DerivePaymentAddressFromXpub derives a payment address from an xpub and index
func (p *AddressProcessor) DerivePaymentAddressFromXpub(xpub string, index uint32) (string, []byte, error) {
	extendedKey, err := hdkeychain.NewKeyFromString(xpub)
	if err != nil {
		return "", nil, fmt.Errorf("invalid xpub: %w", err)
	}

	// Derive using standard path: xpub/0/index (external chain)
	externalKey, err := extendedKey.Derive(0)
	if err != nil {
		return "", nil, fmt.Errorf("failed to derive external chain: %w", err)
	}

	childKey, err := externalKey.Derive(index)
	if err != nil {
		return "", nil, fmt.Errorf("failed to derive child key: %w", err)
	}

	pubKey, err := childKey.ECPubKey()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get public key: %w", err)
	}

	return p.generateAddress(pubKey)
}

// DerivePaymentAddressFromPubKey derives a payment address directly from a public key
func (p *AddressProcessor) DerivePaymentAddressFromPubKey(pubKey *btcec.PublicKey) (string, []byte, error) {
	if pubKey == nil {
		return "", nil, errors.New("public key cannot be nil")
	}
	return p.generateAddress(pubKey)
}

func (p *AddressProcessor) generateAddress(pubKey *btcec.PublicKey) (string, []byte, error) {
	switch p.derivationType {
	case iwallet.DerivationNativeSegwit:
		return p.generateNativeSegwitAddress(pubKey)
	case iwallet.DerivationSegwit:
		return p.generateSegwitAddress(pubKey)
	case iwallet.DerivationLegacy:
		return p.generateLegacyAddress(pubKey)
	case iwallet.DerivationCashAddr:
		return p.generateCashAddr(pubKey)
	case iwallet.DerivationTransparent:
		return p.generateTransparentAddress(pubKey)
	default:
		return "", nil, fmt.Errorf("unsupported derivation type: %s", p.derivationType)
	}
}

func (p *AddressProcessor) generateNativeSegwitAddress(pubKey *btcec.PublicKey) (string, []byte, error) {
	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())

	scriptPubKey, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_0).
		AddData(pubKeyHash).
		Script()
	if err != nil {
		return "", nil, err
	}

	addr, err := btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, p.chainParams)
	if err != nil {
		return "", nil, err
	}

	return addr.EncodeAddress(), scriptPubKey, nil
}

func (p *AddressProcessor) generateSegwitAddress(pubKey *btcec.PublicKey) (string, []byte, error) {
	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())

	witnessScript, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_0).
		AddData(pubKeyHash).
		Script()
	if err != nil {
		return "", nil, err
	}

	witnessScriptHash := btcutil.Hash160(witnessScript)

	addr, err := btcutil.NewAddressScriptHashFromHash(witnessScriptHash, p.chainParams)
	if err != nil {
		return "", nil, err
	}

	scriptPubKey, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return "", nil, err
	}

	return addr.EncodeAddress(), scriptPubKey, nil
}

func (p *AddressProcessor) generateLegacyAddress(pubKey *btcec.PublicKey) (string, []byte, error) {
	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())

	addr, err := btcutil.NewAddressPubKeyHash(pubKeyHash, p.chainParams)
	if err != nil {
		return "", nil, err
	}

	scriptPubKey, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return "", nil, err
	}

	return addr.EncodeAddress(), scriptPubKey, nil
}

func (p *AddressProcessor) generateCashAddr(pubKey *btcec.PublicKey) (string, []byte, error) {
	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())

	addr, err := btcutil.NewAddressPubKeyHash(pubKeyHash, p.chainParams)
	if err != nil {
		return "", nil, err
	}

	scriptPubKey, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return "", nil, err
	}

	// TODO: Use proper CashAddr library for correct format
	cashAddr := "bitcoincash:" + addr.EncodeAddress()

	return cashAddr, scriptPubKey, nil
}

func (p *AddressProcessor) generateTransparentAddress(pubKey *btcec.PublicKey) (string, []byte, error) {
	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())

	addr, err := btcutil.NewAddressPubKeyHash(pubKeyHash, p.chainParams)
	if err != nil {
		return "", nil, err
	}

	scriptPubKey, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return "", nil, err
	}

	// TODO: Use proper ZEC library for t1... address format
	return addr.EncodeAddress(), scriptPubKey, nil
}

// ValidateXpub validates an extended public key
func (p *AddressProcessor) ValidateXpub(xpub string) error {
	extKey, err := hdkeychain.NewKeyFromString(xpub)
	if err != nil {
		return fmt.Errorf("invalid xpub format: %w", err)
	}

	if extKey.IsPrivate() {
		return errors.New("xpub cannot be a private key")
	}

	_, err = extKey.Derive(0)
	if err != nil {
		return fmt.Errorf("xpub derivation failed: %w", err)
	}

	return nil
}

// SetDerivationType sets the derivation type for address generation
func (p *AddressProcessor) SetDerivationType(derivationType iwallet.DerivationType) {
	p.derivationType = derivationType
}

// GetDerivationType returns the current derivation type
func (p *AddressProcessor) GetDerivationType() iwallet.DerivationType {
	return p.derivationType
}

// GetChainType returns the chain type
func (p *AddressProcessor) GetChainType() iwallet.ChainType {
	return p.chainType
}

// ============================================================================
// Package-level utility functions
// ============================================================================

// AddressToScriptHash converts scriptPubKey to Electrum scripthash format (reversed SHA256)
func AddressToScriptHash(scriptPubKey []byte) string {
	hash := sha256.Sum256(scriptPubKey)
	reversed := make([]byte, 32)
	for i := 0; i < 32; i++ {
		reversed[i] = hash[31-i]
	}
	return hex.EncodeToString(reversed)
}

// GeneratePaymentURI generates a BIP21 payment URI
func GeneratePaymentURI(chainType iwallet.ChainType, address string, amount float64) string {
	var scheme string
	switch chainType {
	case iwallet.ChainBitcoin:
		scheme = "bitcoin"
	case iwallet.ChainLitecoin:
		scheme = "litecoin"
	case iwallet.ChainBitcoinCash:
		scheme = "bitcoincash"
	case iwallet.ChainZCash:
		scheme = "zcash"
	default:
		scheme = "bitcoin"
	}

	if amount > 0 {
		return fmt.Sprintf("%s:%s?amount=%.8f", scheme, address, amount)
	}
	return fmt.Sprintf("%s:%s", scheme, address)
}

// DerivePaymentAddress is a convenience function to derive address from public key and chaincode
func DerivePaymentAddress(chainType iwallet.ChainType, escrowPubKey *btcec.PublicKey, chaincode []byte) (string, []byte, error) {
	proc, err := NewAddressProcessor(chainType)
	if err != nil {
		return "", nil, err
	}
	return proc.DerivePaymentAddress(escrowPubKey, chaincode)
}

// DerivePaymentAddressFromPubKey is a convenience function to derive address directly from public key
func DerivePaymentAddressFromPubKey(chainType iwallet.ChainType, pubKey *btcec.PublicKey) (string, []byte, error) {
	proc, err := NewAddressProcessor(chainType)
	if err != nil {
		return "", nil, err
	}
	return proc.DerivePaymentAddressFromPubKey(pubKey)
}

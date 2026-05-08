package utxo

// Pure utility helpers shared across UTXO chains.
//
// All chain-specific address derivation lives in iwallet.UTXOAddressUtilities,
// implemented per chain in internal/chains/utxo/{bitcoin,litecoin,bitcoincash,zcash}.
// New callers must go through that interface (via the multiwallet) rather than
// reintroduce switch-case branches here.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// AddressToScriptHash converts a scriptPubKey to the Electrum scripthash format
// (reversed SHA-256). Pure cryptographic transform — no chain dependencies.
func AddressToScriptHash(scriptPubKey []byte) string {
	hash := sha256.Sum256(scriptPubKey)
	reversed := make([]byte, 32)
	for i := 0; i < 32; i++ {
		reversed[i] = hash[31-i]
	}
	return hex.EncodeToString(reversed)
}

// GeneratePaymentURI generates a BIP-21 / CIP-1 payment URI string. Only the
// scheme name is chain-specific; address encoding is the caller's
// responsibility (use iwallet.UTXOAddressUtilities to obtain a properly
// encoded address first).
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

package tron

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/btcsuite/btcd/btcutil/base58"
	"golang.org/x/crypto/sha3"
)

const (
	tronAddressPrefix   = byte(0x41) // mainnet prefix
	base58AddressLength = 34
	hexAddressLength    = 42 // "41" + 40 hex chars
)

var (
	ErrInvalidHexAddress    = errors.New("tron: invalid hex address format")
	ErrInvalidBase58Address = errors.New("tron: invalid base58 address format")
	ErrChecksumMismatch     = errors.New("tron: address checksum mismatch")
)

// HexToBase58 converts a TRON hex address (41-prefixed) to base58check (T-prefixed).
func HexToBase58(hexAddr string) (string, error) {
	hexAddr = strings.TrimPrefix(hexAddr, "0x")
	if len(hexAddr) != hexAddressLength || !strings.HasPrefix(hexAddr, "41") {
		return "", ErrInvalidHexAddress
	}
	addrBytes, err := hex.DecodeString(hexAddr)
	if err != nil {
		return "", ErrInvalidHexAddress
	}
	checksum := doubleHash(addrBytes)[:4]
	return base58.Encode(append(addrBytes, checksum...)), nil
}

// Base58ToHex converts a TRON base58check address (T-prefixed) to hex (41-prefixed).
func Base58ToHex(base58Addr string) (string, error) {
	decoded := base58.Decode(base58Addr)
	if len(decoded) != 25 { // 21 bytes address + 4 bytes checksum
		return "", ErrInvalidBase58Address
	}
	addr := decoded[:21]
	checksum := decoded[21:]
	expected := doubleHash(addr)[:4]
	if checksum[0] != expected[0] || checksum[1] != expected[1] ||
		checksum[2] != expected[2] || checksum[3] != expected[3] {
		return "", ErrChecksumMismatch
	}
	if addr[0] != tronAddressPrefix {
		return "", ErrInvalidBase58Address
	}
	return hex.EncodeToString(addr), nil
}

// ValidateAddress checks whether the given string is a valid TRON base58check address.
func ValidateAddress(addr string) bool {
	_, err := Base58ToHex(addr)
	return err == nil
}

// PubKeyToTRONAddress derives a T-prefixed TRON address from an ECDSA public key.
func PubKeyToTRONAddress(pubKey *ecdsa.PublicKey) string {
	pubBytes := append(pubKey.X.Bytes(), pubKey.Y.Bytes()...)
	// Pad X and Y to 32 bytes each
	if len(pubKey.X.Bytes()) < 32 {
		padded := make([]byte, 32-len(pubKey.X.Bytes()))
		pubBytes = append(padded, pubBytes...)
	}
	if len(pubKey.Y.Bytes()) < 32 {
		yPadded := make([]byte, 32)
		copy(yPadded[32-len(pubKey.Y.Bytes()):], pubKey.Y.Bytes())
		pubBytes = append(pubKey.X.Bytes(), yPadded...)
		if len(pubKey.X.Bytes()) < 32 {
			xPadded := make([]byte, 32)
			copy(xPadded[32-len(pubKey.X.Bytes()):], pubKey.X.Bytes())
			pubBytes = append(xPadded, yPadded...)
		}
	}

	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(pubBytes)
	hash := hasher.Sum(nil)

	// Take last 20 bytes, prepend 0x41
	addrBytes := append([]byte{tronAddressPrefix}, hash[12:]...)
	checksum := doubleHash(addrBytes)[:4]
	return base58.Encode(append(addrBytes, checksum...))
}

// doubleHash performs SHA256(SHA256(data)).
func doubleHash(data []byte) []byte {
	h1 := sha256.Sum256(data)
	h2 := sha256.Sum256(h1[:])
	return h2[:]
}

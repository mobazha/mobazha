// Package crypto provides cryptographic signing and encryption for Mobazha.
package crypto

import (
	"crypto/ed25519"
	"fmt"
)

// Signature represents a cryptographic signature.
type Signature []byte

// Sign signs a message using an Ed25519 private key.
func Sign(privateKey ed25519.PrivateKey, message []byte) (Signature, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: expected %d, got %d",
			ed25519.PrivateKeySize, len(privateKey))
	}

	signature := ed25519.Sign(privateKey, message)
	return Signature(signature), nil
}

// Verify verifies a signature against a message using an Ed25519 public key.
func Verify(publicKey ed25519.PublicKey, message []byte, signature Signature) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}

	if len(signature) != ed25519.SignatureSize {
		return false
	}

	return ed25519.Verify(publicKey, message, signature)
}

// SignatureSize returns the size of an Ed25519 signature.
func SignatureSize() int {
	return ed25519.SignatureSize
}

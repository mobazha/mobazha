// Package crypto provides cryptographic functions for Mobazha.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// KeySize is the size of AES-256 keys.
const KeySize = 32

// NonceSize is the size of GCM nonce.
const NonceSize = 12

// EncryptAESGCM encrypts plaintext using AES-256-GCM.
// Returns: nonce || ciphertext || tag
func EncryptAESGCM(plaintext, key []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("invalid key length: expected %d bytes, got %d", KeySize, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal appends the encrypted data to nonce, so the result is nonce || ciphertext || tag
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// DecryptAESGCM decrypts ciphertext encrypted with AES-256-GCM.
// Input format: nonce || ciphertext || tag
func DecryptAESGCM(ciphertext, key []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("invalid key length: expected %d bytes, got %d", KeySize, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// DeriveKeyOptions contains options for key derivation.
type DeriveKeyOptions struct {
	// Secret is the input key material (e.g., private key bytes).
	Secret []byte
	// Salt provides additional entropy (can be public).
	Salt []byte
	// Info is the context/application-specific info.
	Info []byte
}

// DeriveKey derives a key using HKDF-SHA256.
// Returns a 32-byte key suitable for AES-256.
func DeriveKey(opts DeriveKeyOptions) ([]byte, error) {
	if len(opts.Secret) == 0 {
		return nil, fmt.Errorf("secret is required")
	}

	kdf := hkdf.New(sha256.New, opts.Secret, opts.Salt, opts.Info)

	key := make([]byte, KeySize)
	if _, err := io.ReadFull(kdf, key); err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	return key, nil
}

// DeriveListingKey derives a listing encryption key.
// This is a convenience function that formats the info parameter correctly.
func DeriveListingKey(privateKeyBytes []byte, peerID string, slug string, version int) ([]byte, error) {
	return DeriveKey(DeriveKeyOptions{
		Secret: privateKeyBytes,
		Salt:   []byte("mobazha-phase2-listing-encryption-v1"),
		Info:   []byte(fmt.Sprintf("listing:%s:%s:v%d", peerID, slug, version)),
	})
}

// GenerateRandomKey generates a cryptographically secure random key.
func GenerateRandomKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	return key, nil
}

// GenerateRandomNonce generates a random nonce for AES-GCM.
func GenerateRandomNonce() ([]byte, error) {
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	return nonce, nil
}

// Hash computes the SHA-256 hash of data.
func Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

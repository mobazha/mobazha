package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/hkdf"
)

const (
	// matrixEncryptionSalt is the domain-separation salt for AES key derivation.
	matrixEncryptionSalt = "mobazha-matrix-e2ee-v1"

	// matrixPasswordSalt is the HKDF salt for Matrix login password derivation.
	matrixPasswordSalt = "mobazha-matrix-password-v1"
	// matrixPasswordInfo is the HKDF info for Matrix login password derivation.
	matrixPasswordInfo = "matrix-login-password"
	// passwordLength is the derived password length in bytes.
	passwordLength = 32
)

// DeriveMatrixEncryptionKey derives an AES-256 key from raw private key bytes
// using SHA-256 with a domain-separation salt.
func DeriveMatrixEncryptionKey(privKeyBytes []byte) []byte {
	h := sha256.New()
	h.Write([]byte(matrixEncryptionSalt))
	h.Write(privKeyBytes)
	return h.Sum(nil)
}

// DeriveMatrixPassword derives a URL-safe base64 Matrix login password
// from raw private key bytes using HKDF-SHA256.
func DeriveMatrixPassword(privKeyBytes []byte) (string, error) {
	hkdfReader := hkdf.New(sha256.New, privKeyBytes, []byte(matrixPasswordSalt), []byte(matrixPasswordInfo))
	password := make([]byte, passwordLength)
	if _, err := hkdfReader.Read(password); err != nil {
		return "", fmt.Errorf("failed to derive password: %w", err)
	}
	return base64.URLEncoding.EncodeToString(password), nil
}

// EncryptAESGCM encrypts plaintext using AES-256-GCM.
// The returned ciphertext has the nonce prepended.
func EncryptAESGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptAESGCM decrypts ciphertext that was encrypted with EncryptAESGCM.
// Expects nonce prepended to the ciphertext.
func DecryptAESGCM(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce := ciphertext[:gcm.NonceSize()]
	return gcm.Open(nil, nonce, ciphertext[gcm.NonceSize():], nil)
}

// CountKeysInJSON counts occurrences of "room_id" in JSON as a heuristic
// for the number of Matrix room keys.
func CountKeysInJSON(keysJSON string) int {
	count := 0
	search := `"room_id"`
	for i := 0; i <= len(keysJSON)-len(search); i++ {
		if keysJSON[i:i+len(search)] == search {
			count++
		}
	}
	return count
}

// MatrixUserIDFromPeerID constructs the expected Matrix user ID for a peer.
func MatrixUserIDFromPeerID(peerID, serverName string) string {
	return fmt.Sprintf("@peer_%s:%s", strings.ToLower(peerID), serverName)
}

package encryption

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/hkdf"
)

const (
	// matrixPasswordSalt is the HKDF salt for Matrix login password derivation.
	matrixPasswordSalt = "mobazha-matrix-password-v1"
	// matrixPasswordInfo is the HKDF info for Matrix login password derivation.
	matrixPasswordInfo = "matrix-login-password"
	// passwordLength is the derived password length in bytes.
	passwordLength = 32

	// matrixPickleSalt is the HKDF salt for pickle key derivation.
	matrixPickleSalt = "mobazha-matrix-pickle-v1"
	// matrixPickleInfo is the HKDF info for pickle key derivation.
	matrixPickleInfo = "matrix-crypto-pickle-key"
	// pickleKeyLength is the derived pickle key length in bytes.
	pickleKeyLength = 32
)

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

// DeriveMatrixPickleKey derives a 32-byte pickle key from raw private key bytes
// using HKDF-SHA256. Used to encrypt the OlmAccount stored in the crypto DB.
func DeriveMatrixPickleKey(privKeyBytes []byte) []byte {
	r := hkdf.New(sha256.New, privKeyBytes, []byte(matrixPickleSalt), []byte(matrixPickleInfo))
	key := make([]byte, pickleKeyLength)
	if _, err := r.Read(key); err != nil {
		panic("hkdf read failed: " + err.Error())
	}
	return key
}

// MatrixUserIDFromPeerID constructs the expected Matrix user ID for a peer.
func MatrixUserIDFromPeerID(peerID, serverName string) string {
	return fmt.Sprintf("@peer_%s:%s", strings.ToLower(peerID), serverName)
}

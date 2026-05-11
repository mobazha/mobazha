package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"strconv"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"golang.org/x/crypto/hkdf"
)

// DigitalCrypto provides HKDF key derivation and AES-256-GCM
// encryption for digital assets (files, license keys, download tokens).
//
// Key hierarchy (derived at runtime, never stored):
//
//	DigitalContentMasterKey(version)
//	  └─ HKDF(salt=assetID, info="digital-asset-v{ver}") → assetKey
//	  └─ HKDF(salt=assetID, info="license-wrap-v{ver}")  → licenseWrapKey
//	  └─ HKDF(salt="",      info="download-token-v1")    → downloadTokenSecret
type DigitalCrypto struct {
	keys contracts.KeyProvider
}

// NewDigitalCrypto creates a new DigitalCrypto instance.
func NewDigitalCrypto(keys contracts.KeyProvider) *DigitalCrypto {
	return &DigitalCrypto{keys: keys}
}

// DeriveAssetKey derives a per-asset AES-256 key from the master key.
func (dc *DigitalCrypto) DeriveAssetKey(assetID string, keyVersion int) ([]byte, error) {
	master, err := dc.keys.DigitalContentMasterKey(keyVersion)
	if err != nil {
		return nil, fmt.Errorf("get master key v%d: %w", keyVersion, err)
	}
	defer ZeroBytes(master)

	return deriveHKDF(master, []byte(assetID), fmt.Sprintf("digital-asset-v%d", keyVersion), 32)
}

// DeriveLicenseWrapKey derives a per-asset key used to encrypt license keys.
func (dc *DigitalCrypto) DeriveLicenseWrapKey(assetID string, keyVersion int) ([]byte, error) {
	master, err := dc.keys.DigitalContentMasterKey(keyVersion)
	if err != nil {
		return nil, fmt.Errorf("get master key v%d: %w", keyVersion, err)
	}
	defer ZeroBytes(master)

	return deriveHKDF(master, []byte(assetID), fmt.Sprintf("license-wrap-v%d", keyVersion), 32)
}

// DeriveDownloadTokenSecret derives the HMAC secret for signing download URLs.
func (dc *DigitalCrypto) DeriveDownloadTokenSecret(keyVersion int) ([]byte, error) {
	master, err := dc.keys.DigitalContentMasterKey(keyVersion)
	if err != nil {
		return nil, fmt.Errorf("get master key v%d: %w", keyVersion, err)
	}
	defer ZeroBytes(master)

	return deriveHKDF(master, nil, "download-token-v1", 32)
}

// EncryptFile encrypts plaintext using AES-256-GCM with a per-asset key.
func (dc *DigitalCrypto) EncryptFile(plaintext []byte, assetID string, keyVersion int) ([]byte, error) {
	key, err := dc.DeriveAssetKey(assetID, keyVersion)
	if err != nil {
		return nil, err
	}
	defer ZeroBytes(key)

	return aesGCMEncrypt(plaintext, key)
}

// DecryptFile decrypts ciphertext using AES-256-GCM with a per-asset key.
func (dc *DigitalCrypto) DecryptFile(ciphertext []byte, assetID string, keyVersion int) ([]byte, error) {
	key, err := dc.DeriveAssetKey(assetID, keyVersion)
	if err != nil {
		return nil, err
	}
	defer ZeroBytes(key)

	return aesGCMDecrypt(ciphertext, key)
}

// EncryptLicenseKey encrypts a single license key value.
func (dc *DigitalCrypto) EncryptLicenseKey(plainKey []byte, assetID string, keyVersion int) ([]byte, error) {
	wrapKey, err := dc.DeriveLicenseWrapKey(assetID, keyVersion)
	if err != nil {
		return nil, err
	}
	defer ZeroBytes(wrapKey)

	return aesGCMEncrypt(plainKey, wrapKey)
}

// DecryptLicenseKey decrypts a single license key value.
func (dc *DigitalCrypto) DecryptLicenseKey(cipherKey []byte, assetID string, keyVersion int) ([]byte, error) {
	wrapKey, err := dc.DeriveLicenseWrapKey(assetID, keyVersion)
	if err != nil {
		return nil, err
	}
	defer ZeroBytes(wrapKey)

	return aesGCMDecrypt(cipherKey, wrapKey)
}

// SignDownloadURL computes an HMAC-SHA256 over the download URL parameters.
// Fields are length-prefixed (BE uint16) so colons or other characters in
// IDs cannot produce collisions.
func (dc *DigitalCrypto) SignDownloadURL(orderID, grantNonce, assetID string, expiryTs int64, grantVersion, keyVersion int) ([]byte, error) {
	secret, err := dc.DeriveDownloadTokenSecret(keyVersion)
	if err != nil {
		return nil, err
	}
	defer ZeroBytes(secret)

	mac := hmac.New(sha256.New, secret)
	for _, field := range []string{
		orderID,
		grantNonce,
		assetID,
		strconv.FormatInt(expiryTs, 10),
		strconv.Itoa(grantVersion),
	} {
		b := []byte(field)
		mac.Write([]byte{byte(len(b) >> 8), byte(len(b))})
		mac.Write(b)
	}
	return mac.Sum(nil), nil
}

// VerifyDownloadURL verifies an HMAC signature over download URL parameters.
func (dc *DigitalCrypto) VerifyDownloadURL(orderID, grantNonce, assetID string, expiryTs int64, grantVersion, keyVersion int, sig []byte) (bool, error) {
	expected, err := dc.SignDownloadURL(orderID, grantNonce, assetID, expiryTs, grantVersion, keyVersion)
	if err != nil {
		return false, err
	}
	return hmac.Equal(expected, sig), nil
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func deriveHKDF(ikm, salt []byte, info string, keyLen int) ([]byte, error) {
	kdf := hkdf.New(sha256.New, ikm, salt, []byte(info))
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(kdf, key); err != nil {
		return nil, fmt.Errorf("HKDF expand: %w", err)
	}
	return key, nil
}

func aesGCMEncrypt(plaintext, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: expected 32, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce generation: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func aesGCMDecrypt(ciphertext, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: expected 32, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}
	return plaintext, nil
}

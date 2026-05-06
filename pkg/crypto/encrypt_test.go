package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptAESGCM(t *testing.T) {
	key, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("GenerateRandomKey() error = %v", err)
	}

	plaintext := []byte("Hello, Mobazha! This is a secret message.")

	// Encrypt
	ciphertext, err := EncryptAESGCM(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptAESGCM() error = %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Error("Ciphertext should not equal plaintext")
	}

	// Decrypt
	decrypted, err := DecryptAESGCM(ciphertext, key)
	if err != nil {
		t.Fatalf("DecryptAESGCM() error = %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted text = %s, want %s", decrypted, plaintext)
	}
}

func TestEncryptAESGCM_InvalidKey(t *testing.T) {
	invalidKey := make([]byte, 16) // Wrong size

	_, err := EncryptAESGCM([]byte("test"), invalidKey)
	if err == nil {
		t.Error("EncryptAESGCM() should error on invalid key size")
	}
}

func TestDecryptAESGCM_InvalidKey(t *testing.T) {
	key, _ := GenerateRandomKey()
	ciphertext, _ := EncryptAESGCM([]byte("test"), key)

	invalidKey := make([]byte, 16) // Wrong size
	_, err := DecryptAESGCM(ciphertext, invalidKey)
	if err == nil {
		t.Error("DecryptAESGCM() should error on invalid key size")
	}
}

func TestDecryptAESGCM_WrongKey(t *testing.T) {
	key1, _ := GenerateRandomKey()
	key2, _ := GenerateRandomKey()

	ciphertext, _ := EncryptAESGCM([]byte("test"), key1)

	_, err := DecryptAESGCM(ciphertext, key2)
	if err == nil {
		t.Error("DecryptAESGCM() should error with wrong key")
	}
}

func TestDecryptAESGCM_TamperedCiphertext(t *testing.T) {
	key, _ := GenerateRandomKey()
	ciphertext, _ := EncryptAESGCM([]byte("test"), key)

	// Tamper with the ciphertext
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err := DecryptAESGCM(ciphertext, key)
	if err == nil {
		t.Error("DecryptAESGCM() should error with tampered ciphertext")
	}
}

func TestDeriveKey(t *testing.T) {
	secret := []byte("my-secret-key-material")
	salt := []byte("application-salt")
	info := []byte("context-info")

	key1, err := DeriveKey(DeriveKeyOptions{
		Secret: secret,
		Salt:   salt,
		Info:   info,
	})
	if err != nil {
		t.Fatalf("DeriveKey() error = %v", err)
	}

	if len(key1) != KeySize {
		t.Errorf("Key size = %d, want %d", len(key1), KeySize)
	}

	// Same inputs should produce same key
	key2, _ := DeriveKey(DeriveKeyOptions{
		Secret: secret,
		Salt:   salt,
		Info:   info,
	})

	if !bytes.Equal(key1, key2) {
		t.Error("Same inputs should produce same key")
	}

	// Different info should produce different key
	key3, _ := DeriveKey(DeriveKeyOptions{
		Secret: secret,
		Salt:   salt,
		Info:   []byte("different-context"),
	})

	if bytes.Equal(key1, key3) {
		t.Error("Different info should produce different key")
	}
}

func TestDeriveKey_EmptySecret(t *testing.T) {
	_, err := DeriveKey(DeriveKeyOptions{
		Secret: nil,
		Salt:   []byte("salt"),
		Info:   []byte("info"),
	})
	if err == nil {
		t.Error("DeriveKey() should error with empty secret")
	}
}

func TestDeriveListingKey(t *testing.T) {
	privateKeyBytes := []byte("mock-private-key-bytes-32-bytes!")
	peerID := "QmTestPeerID"
	slug := "test-product"

	// Same version should produce same key
	key1, err := DeriveListingKey(privateKeyBytes, peerID, slug, 1)
	if err != nil {
		t.Fatalf("DeriveListingKey() error = %v", err)
	}

	key2, _ := DeriveListingKey(privateKeyBytes, peerID, slug, 1)
	if !bytes.Equal(key1, key2) {
		t.Error("Same inputs should produce same listing key")
	}

	// Different version should produce different key
	key3, _ := DeriveListingKey(privateKeyBytes, peerID, slug, 2)
	if bytes.Equal(key1, key3) {
		t.Error("Different version should produce different listing key")
	}

	// Different slug should produce different key
	key4, _ := DeriveListingKey(privateKeyBytes, peerID, "other-product", 1)
	if bytes.Equal(key1, key4) {
		t.Error("Different slug should produce different listing key")
	}
}

func TestGenerateRandomKey(t *testing.T) {
	key1, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("GenerateRandomKey() error = %v", err)
	}

	if len(key1) != KeySize {
		t.Errorf("Key size = %d, want %d", len(key1), KeySize)
	}

	key2, _ := GenerateRandomKey()
	if bytes.Equal(key1, key2) {
		t.Error("Random keys should be different")
	}
}

func TestGenerateRandomNonce(t *testing.T) {
	nonce1, err := GenerateRandomNonce()
	if err != nil {
		t.Fatalf("GenerateRandomNonce() error = %v", err)
	}

	if len(nonce1) != NonceSize {
		t.Errorf("Nonce size = %d, want %d", len(nonce1), NonceSize)
	}

	nonce2, _ := GenerateRandomNonce()
	if bytes.Equal(nonce1, nonce2) {
		t.Error("Random nonces should be different")
	}
}

func TestHash(t *testing.T) {
	data := []byte("test data")

	hash1 := Hash(data)
	hash2 := Hash(data)

	if !bytes.Equal(hash1, hash2) {
		t.Error("Same input should produce same hash")
	}

	hash3 := Hash([]byte("different data"))
	if bytes.Equal(hash1, hash3) {
		t.Error("Different input should produce different hash")
	}

	// SHA-256 produces 32 bytes
	if len(hash1) != 32 {
		t.Errorf("Hash size = %d, want 32", len(hash1))
	}
}

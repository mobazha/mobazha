package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestSign_and_Verify(t *testing.T) {
	// Generate a key pair
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	message := []byte("Hello, Mobazha!")

	// Sign the message
	signature, err := Sign(privateKey, message)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	if len(signature) != SignatureSize() {
		t.Errorf("Signature size = %d, want %d", len(signature), SignatureSize())
	}

	// Verify the signature
	valid := Verify(publicKey, message, signature)
	if !valid {
		t.Error("Verify() returned false for valid signature")
	}

	// Verify with wrong message
	wrongMessage := []byte("Wrong message")
	valid = Verify(publicKey, wrongMessage, signature)
	if valid {
		t.Error("Verify() returned true for wrong message")
	}
}

func TestSign_InvalidPrivateKey(t *testing.T) {
	invalidKey := make([]byte, 16) // Wrong size

	_, err := Sign(ed25519.PrivateKey(invalidKey), []byte("test"))
	if err == nil {
		t.Error("Sign() should error on invalid private key")
	}
}

func TestVerify_InvalidPublicKey(t *testing.T) {
	invalidKey := make([]byte, 16) // Wrong size
	signature := make([]byte, ed25519.SignatureSize)

	valid := Verify(ed25519.PublicKey(invalidKey), []byte("test"), signature)
	if valid {
		t.Error("Verify() should return false for invalid public key")
	}
}

func TestVerify_InvalidSignature(t *testing.T) {
	publicKey, _, _ := ed25519.GenerateKey(rand.Reader)
	invalidSignature := make([]byte, 32) // Wrong size

	valid := Verify(publicKey, []byte("test"), Signature(invalidSignature))
	if valid {
		t.Error("Verify() should return false for invalid signature size")
	}
}

func TestSignatureSize(t *testing.T) {
	if SignatureSize() != ed25519.SignatureSize {
		t.Errorf("SignatureSize() = %d, want %d", SignatureSize(), ed25519.SignatureSize)
	}
}

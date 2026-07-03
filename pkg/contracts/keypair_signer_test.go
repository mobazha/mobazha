package contracts

import (
	"testing"

	"github.com/mobazha/mobazha/pkg/identity"
)

func TestKeyPairSigner_NewAndSign(t *testing.T) {
	kp, err := identity.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	peerID, err := identity.PeerIDFromPublicKey(kp.PubKey)
	if err != nil {
		t.Fatalf("PeerIDFromPublicKey: %v", err)
	}

	signer := NewKeyPairSigner(kp, peerID)

	// Verify interface compliance
	var _ Signer = signer

	// Test PeerID
	if signer.PeerID() != peerID {
		t.Errorf("PeerID mismatch: got %s, want %s", signer.PeerID(), peerID)
	}
	if !signer.PeerID().IsValid() {
		t.Error("PeerID should be valid")
	}

	// Test Sign + Verify
	message := []byte("hello mobazha")
	sig, err := signer.Sign(message)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(sig) == 0 {
		t.Fatal("signature should not be empty")
	}

	valid, err := signer.Verify(message, sig)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !valid {
		t.Error("signature should be valid")
	}

	// Verify with wrong message
	valid, err = signer.Verify([]byte("wrong message"), sig)
	if err != nil {
		t.Fatalf("Verify wrong message: %v", err)
	}
	if valid {
		t.Error("signature should be invalid for wrong message")
	}

	// Test PublicKey
	pubKey, err := signer.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey: %v", err)
	}
	if len(pubKey) != 32 {
		t.Errorf("Ed25519 public key should be 32 bytes, got %d", len(pubKey))
	}

	// Test KeyPair accessor
	if signer.KeyPair() != kp {
		t.Error("KeyPair() should return the original key pair")
	}
}

func TestKeyPairSigner_FromMarshaledKey(t *testing.T) {
	// Generate a key pair and marshal it
	kp, err := identity.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	marshaledKey, err := kp.MarshalPrivateKey()
	if err != nil {
		t.Fatalf("MarshalPrivateKey: %v", err)
	}

	// Create signer from marshaled key
	signer, err := NewKeyPairSignerFromMarshaledKey(marshaledKey)
	if err != nil {
		t.Fatalf("NewKeyPairSignerFromMarshaledKey: %v", err)
	}

	// Verify PeerID matches
	expectedPeerID, err := identity.PeerIDFromPublicKey(kp.PubKey)
	if err != nil {
		t.Fatalf("PeerIDFromPublicKey: %v", err)
	}
	if signer.PeerID() != expectedPeerID {
		t.Errorf("PeerID mismatch: got %s, want %s", signer.PeerID(), expectedPeerID)
	}

	// Sign with original, verify with reconstructed
	message := []byte("cross-verify test")
	sig, err := kp.Sign(message)
	if err != nil {
		t.Fatalf("Sign with original: %v", err)
	}

	valid, err := signer.Verify(message, sig)
	if err != nil {
		t.Fatalf("Verify with reconstructed: %v", err)
	}
	if !valid {
		t.Error("cross-verification should succeed")
	}

	// Sign with reconstructed, verify with original
	sig2, err := signer.Sign(message)
	if err != nil {
		t.Fatalf("Sign with reconstructed: %v", err)
	}

	valid2, err := kp.Verify(message, sig2)
	if err != nil {
		t.Fatalf("Verify with original: %v", err)
	}
	if !valid2 {
		t.Error("reverse cross-verification should succeed")
	}
}

func TestKeyPairSigner_FromMarshaledKey_InvalidInput(t *testing.T) {
	_, err := NewKeyPairSignerFromMarshaledKey([]byte("invalid"))
	if err == nil {
		t.Error("expected error for invalid marshaled key")
	}

	_, err = NewKeyPairSignerFromMarshaledKey(nil)
	if err == nil {
		t.Error("expected error for nil marshaled key")
	}
}

func TestKeyPairSigner_TwoSignersDifferent(t *testing.T) {
	kp1, err := identity.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	pid1, err := identity.PeerIDFromPublicKey(kp1.PubKey)
	if err != nil {
		t.Fatalf("PeerIDFromPublicKey: %v", err)
	}
	signer1 := NewKeyPairSigner(kp1, pid1)

	kp2, err := identity.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	pid2, err := identity.PeerIDFromPublicKey(kp2.PubKey)
	if err != nil {
		t.Fatalf("PeerIDFromPublicKey: %v", err)
	}
	signer2 := NewKeyPairSigner(kp2, pid2)

	// Different PeerIDs
	if signer1.PeerID() == signer2.PeerID() {
		t.Error("two different signers should have different PeerIDs")
	}

	// Signature from signer1 should not verify with signer2
	message := []byte("test message")
	sig, err := signer1.Sign(message)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	valid, err := signer2.Verify(message, sig)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if valid {
		t.Error("signature from signer1 should not verify with signer2")
	}
}

func TestNewKeyPairSigner_NilKeyPairPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewKeyPairSigner(nil, ...) should panic")
		}
	}()
	NewKeyPairSigner(nil, "some-peer-id")
}

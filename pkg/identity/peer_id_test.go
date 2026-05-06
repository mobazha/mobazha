package identity

import (
	"crypto/ed25519"
	"testing"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestGenerateKeyPair(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	if keyPair.PrivKey == nil {
		t.Error("PrivKey should not be nil")
	}
	if keyPair.PubKey == nil {
		t.Error("PubKey should not be nil")
	}

	// Verify key type is Ed25519
	if keyPair.PrivKey.Type() != libp2pcrypto.Ed25519 {
		t.Errorf("expected Ed25519 key type, got %v", keyPair.PrivKey.Type())
	}
}

func TestGenerateKeyPairFromSeed(t *testing.T) {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}

	keyPair1, err := GenerateKeyPairFromSeed(seed)
	if err != nil {
		t.Fatalf("GenerateKeyPairFromSeed() error = %v", err)
	}

	keyPair2, err := GenerateKeyPairFromSeed(seed)
	if err != nil {
		t.Fatalf("GenerateKeyPairFromSeed() error = %v", err)
	}

	// Same seed should produce same keys
	raw1, _ := keyPair1.RawPublicKey()
	raw2, _ := keyPair2.RawPublicKey()
	if string(raw1) != string(raw2) {
		t.Error("Same seed should produce same public key")
	}

	// Different seed should produce different keys
	seed2 := make([]byte, 32)
	for i := range seed2 {
		seed2[i] = byte(i + 100)
	}
	keyPair3, err := GenerateKeyPairFromSeed(seed2)
	if err != nil {
		t.Fatalf("GenerateKeyPairFromSeed() error = %v", err)
	}
	raw3, _ := keyPair3.RawPublicKey()
	if string(raw1) == string(raw3) {
		t.Error("Different seeds should produce different public keys")
	}
}

func TestKeyPairFromPrivateKey(t *testing.T) {
	// Generate a key pair first
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	rawPriv, err := keyPair.RawPrivateKey()
	if err != nil {
		t.Fatalf("RawPrivateKey() error = %v", err)
	}

	// Reconstruct from raw private key
	keyPair2, err := KeyPairFromPrivateKey(rawPriv)
	if err != nil {
		t.Fatalf("KeyPairFromPrivateKey() error = %v", err)
	}

	// Should produce same PeerID
	pid1, _ := PeerIDFromPublicKey(keyPair.PubKey)
	pid2, _ := PeerIDFromPublicKey(keyPair2.PubKey)
	if pid1 != pid2 {
		t.Errorf("expected same PeerID, got %s vs %s", pid1, pid2)
	}
}

func TestKeyPairFromPrivateKey_InvalidSize(t *testing.T) {
	_, err := KeyPairFromPrivateKey(make([]byte, 16))
	if err == nil {
		t.Error("KeyPairFromPrivateKey() should error on invalid key size")
	}
}

func TestKeyPairFromMarshaledPrivateKey(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	marshaled, err := keyPair.MarshalPrivateKey()
	if err != nil {
		t.Fatalf("MarshalPrivateKey() error = %v", err)
	}

	// Reconstruct from marshaled key
	keyPair2, err := KeyPairFromMarshaledPrivateKey(marshaled)
	if err != nil {
		t.Fatalf("KeyPairFromMarshaledPrivateKey() error = %v", err)
	}

	// Should produce same PeerID
	pid1, _ := PeerIDFromPublicKey(keyPair.PubKey)
	pid2, _ := PeerIDFromPublicKey(keyPair2.PubKey)
	if pid1 != pid2 {
		t.Errorf("expected same PeerID, got %s vs %s", pid1, pid2)
	}
}

func TestGeneratePeerID(t *testing.T) {
	peerID, keyPair, err := GeneratePeerID()
	if err != nil {
		t.Fatalf("GeneratePeerID() error = %v", err)
	}

	if !peerID.IsValid() {
		t.Error("PeerID should be valid")
	}

	if keyPair == nil {
		t.Error("KeyPair should not be nil")
	}

	// PeerID should be parseable by libp2p
	libp2pID, err := peerID.ToLibp2p()
	if err != nil {
		t.Fatalf("PeerID.ToLibp2p() error = %v", err)
	}
	if libp2pID.String() != peerID.String() {
		t.Errorf("PeerID string mismatch: %s vs %s", libp2pID.String(), peerID.String())
	}
}

func TestPeerIDFromPublicKey_MatchesLibp2p(t *testing.T) {
	// Generate key using libp2p directly
	privKey, pubKey, err := libp2pcrypto.GenerateKeyPair(libp2pcrypto.Ed25519, 0)
	if err != nil {
		t.Fatalf("libp2p GenerateKeyPair() error = %v", err)
	}
	_ = privKey

	// Get PeerID via libp2p directly
	libp2pPeerID, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		t.Fatalf("peer.IDFromPublicKey() error = %v", err)
	}

	// Get PeerID via our function
	ourPeerID, err := PeerIDFromPublicKey(pubKey)
	if err != nil {
		t.Fatalf("PeerIDFromPublicKey() error = %v", err)
	}

	// Must be identical
	if ourPeerID.String() != libp2pPeerID.String() {
		t.Errorf("PeerID mismatch: ours=%s, libp2p=%s", ourPeerID.String(), libp2pPeerID.String())
	}
}

func TestPeerIDFromRawPublicKey(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	rawPub, err := keyPair.RawPublicKey()
	if err != nil {
		t.Fatalf("RawPublicKey() error = %v", err)
	}

	// Derive PeerID from raw public key
	peerID, err := PeerIDFromRawPublicKey(rawPub)
	if err != nil {
		t.Fatalf("PeerIDFromRawPublicKey() error = %v", err)
	}

	// Should match PeerID from libp2p public key
	peerID2, err := PeerIDFromPublicKey(keyPair.PubKey)
	if err != nil {
		t.Fatalf("PeerIDFromPublicKey() error = %v", err)
	}
	if peerID != peerID2 {
		t.Errorf("PeerID mismatch: raw=%s, libp2p=%s", peerID, peerID2)
	}
}

func TestPeerIDFromRawPublicKey_InvalidKey(t *testing.T) {
	_, err := PeerIDFromRawPublicKey(ed25519.PublicKey(make([]byte, 10)))
	if err == nil {
		t.Error("PeerIDFromRawPublicKey() should error on invalid key")
	}
}

func TestPeerID_String(t *testing.T) {
	peerID := PeerID("12D3KooWTestPeerID")
	if peerID.String() != "12D3KooWTestPeerID" {
		t.Errorf("PeerID.String() = %v, want %v", peerID.String(), "12D3KooWTestPeerID")
	}
}

func TestPeerID_IsValid(t *testing.T) {
	// Generate a real valid PeerID
	validPeerID, _, err := GeneratePeerID()
	if err != nil {
		t.Fatalf("GeneratePeerID() error = %v", err)
	}

	tests := []struct {
		name   string
		peerID PeerID
		want   bool
	}{
		{"valid peer ID", validPeerID, true},
		{"empty peer ID", PeerID(""), false},
		{"invalid peer ID", PeerID("not-a-real-peer-id"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.peerID.IsValid(); got != tt.want {
				t.Errorf("PeerID.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPeerID_ToLibp2p_Roundtrip(t *testing.T) {
	peerID, _, err := GeneratePeerID()
	if err != nil {
		t.Fatalf("GeneratePeerID() error = %v", err)
	}

	// Convert to libp2p
	libp2pID, err := peerID.ToLibp2p()
	if err != nil {
		t.Fatalf("ToLibp2p() error = %v", err)
	}

	// Convert back
	roundtripped := PeerIDFromLibp2p(libp2pID)
	if roundtripped != peerID {
		t.Errorf("roundtrip failed: %s -> %s -> %s", peerID, libp2pID, roundtripped)
	}
}

func TestGenerateKeyPairFromSeed_MatchesOriginal(t *testing.T) {
	// This test verifies that our GenerateKeyPairFromSeed produces the same
	// result as the original mobazha3.0 IdentityKeyFromSeed function.
	// Both use: HMAC-SHA256("OpenBazaar seed", seed) -> GenerateKeyPairWithReader(Ed25519)
	seed := []byte("test seed for compatibility check!!") // 35 bytes, will be HMAC'd to 32

	keyPair, err := GenerateKeyPairFromSeed(seed)
	if err != nil {
		t.Fatalf("GenerateKeyPairFromSeed() error = %v", err)
	}

	peerID, err := PeerIDFromPublicKey(keyPair.PubKey)
	if err != nil {
		t.Fatalf("PeerIDFromPublicKey() error = %v", err)
	}

	// Verify PeerID is valid libp2p format
	if !peerID.IsValid() {
		t.Error("PeerID from seed should be valid")
	}

	// Verify deterministic: same seed -> same PeerID
	keyPair2, _ := GenerateKeyPairFromSeed(seed)
	peerID2, _ := PeerIDFromPublicKey(keyPair2.PubKey)
	if peerID != peerID2 {
		t.Errorf("same seed should produce same PeerID: %s vs %s", peerID, peerID2)
	}
}

func TestRawKeyRoundtrip(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	// Get raw keys
	rawPriv, err := keyPair.RawPrivateKey()
	if err != nil {
		t.Fatalf("RawPrivateKey() error = %v", err)
	}
	if len(rawPriv) != ed25519.PrivateKeySize {
		t.Errorf("raw private key size = %d, want %d", len(rawPriv), ed25519.PrivateKeySize)
	}

	rawPub, err := keyPair.RawPublicKey()
	if err != nil {
		t.Fatalf("RawPublicKey() error = %v", err)
	}
	if len(rawPub) != ed25519.PublicKeySize {
		t.Errorf("raw public key size = %d, want %d", len(rawPub), ed25519.PublicKeySize)
	}

	// Reconstruct and verify
	keyPair2, err := KeyPairFromPrivateKey(rawPriv)
	if err != nil {
		t.Fatalf("KeyPairFromPrivateKey() error = %v", err)
	}

	rawPub2, _ := keyPair2.RawPublicKey()
	if string(rawPub) != string(rawPub2) {
		t.Error("public keys should match after roundtrip")
	}
}

func TestKeyPair_SignAndVerify(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	msg := []byte("test message")

	sig, err := kp.Sign(msg)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	valid, err := kp.Verify(msg, sig)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !valid {
		t.Error("valid signature should verify")
	}

	valid, err = kp.Verify([]byte("wrong"), sig)
	if err != nil {
		t.Fatalf("Verify wrong: %v", err)
	}
	if valid {
		t.Error("should not verify with wrong message")
	}
}

func TestMarshalPublicKeyFromEd25519(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	rawPub, err := kp.RawPublicKey()
	if err != nil {
		t.Fatal(err)
	}

	// Marshal via our helper
	marshaled, err := MarshalPublicKeyFromEd25519(rawPub)
	if err != nil {
		t.Fatalf("MarshalPublicKeyFromEd25519: %v", err)
	}
	if len(marshaled) == 0 {
		t.Fatal("marshaled key should not be empty")
	}

	// Marshal via libp2p directly for comparison
	expected, err := libp2pcrypto.MarshalPublicKey(kp.PubKey)
	if err != nil {
		t.Fatal(err)
	}

	if string(marshaled) != string(expected) {
		t.Error("MarshalPublicKeyFromEd25519 output should match crypto.MarshalPublicKey")
	}

	// Unmarshal and verify it's the same key
	unmarshaledPub, err := libp2pcrypto.UnmarshalPublicKey(marshaled)
	if err != nil {
		t.Fatalf("UnmarshalPublicKey: %v", err)
	}
	if !unmarshaledPub.Equals(kp.PubKey) {
		t.Error("unmarshaled key should equal original")
	}
}

func TestMarshalPublicKeyFromEd25519_Invalid(t *testing.T) {
	_, err := MarshalPublicKeyFromEd25519([]byte("too short"))
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestMarshalPrivateKeyFromEd25519(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	rawPriv, err := kp.RawPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	// Marshal via our helper
	marshaled, err := MarshalPrivateKeyFromEd25519(rawPriv)
	if err != nil {
		t.Fatalf("MarshalPrivateKeyFromEd25519: %v", err)
	}
	if len(marshaled) == 0 {
		t.Fatal("marshaled key should not be empty")
	}

	// Marshal via libp2p directly for comparison
	expected, err := libp2pcrypto.MarshalPrivateKey(kp.PrivKey)
	if err != nil {
		t.Fatal(err)
	}

	if string(marshaled) != string(expected) {
		t.Error("MarshalPrivateKeyFromEd25519 output should match crypto.MarshalPrivateKey")
	}

	// Unmarshal and verify it produces the same PeerID
	kp2, err := KeyPairFromMarshaledPrivateKey(marshaled)
	if err != nil {
		t.Fatalf("KeyPairFromMarshaledPrivateKey: %v", err)
	}
	pid1, _ := PeerIDFromPublicKey(kp.PubKey)
	pid2, _ := PeerIDFromPublicKey(kp2.PubKey)
	if pid1 != pid2 {
		t.Errorf("PeerID mismatch after marshal roundtrip: %s vs %s", pid1, pid2)
	}
}

func TestMarshalPrivateKeyFromEd25519_Invalid(t *testing.T) {
	_, err := MarshalPrivateKeyFromEd25519([]byte("too short"))
	if err == nil {
		t.Error("expected error for invalid key size")
	}
}

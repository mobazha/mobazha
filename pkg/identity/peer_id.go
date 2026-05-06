// Package identity provides Peer ID generation and key management for Mobazha.
// It wraps libp2p's crypto and peer packages to provide a clean API for the
// Mobazha ecosystem, following the same patterns as the existing mobazha3.0 node.
package identity

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerID represents a unique identifier for a peer in the Mobazha network.
// It is derived from the public key using the libp2p multihash format.
type PeerID string

// KeyPair holds the libp2p key pair and provides access to raw Ed25519 keys.
type KeyPair struct {
	// PrivKey is the libp2p private key (protobuf-encoded Ed25519).
	PrivKey libp2pcrypto.PrivKey
	// PubKey is the libp2p public key.
	PubKey libp2pcrypto.PubKey
}

// RawPrivateKey returns the raw Ed25519 private key (64 bytes).
func (kp *KeyPair) RawPrivateKey() (ed25519.PrivateKey, error) {
	raw, err := kp.PrivKey.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw private key: %w", err)
	}
	return ed25519.PrivateKey(raw), nil
}

// RawPublicKey returns the raw Ed25519 public key (32 bytes).
func (kp *KeyPair) RawPublicKey() (ed25519.PublicKey, error) {
	raw, err := kp.PubKey.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw public key: %w", err)
	}
	return ed25519.PublicKey(raw), nil
}

// Sign signs a message using the private key.
func (kp *KeyPair) Sign(message []byte) ([]byte, error) {
	return kp.PrivKey.Sign(message)
}

// Verify verifies a signature using the public key.
func (kp *KeyPair) Verify(message []byte, signature []byte) (bool, error) {
	return kp.PubKey.Verify(message, signature)
}

// MarshalPrivateKey returns the protobuf-encoded private key bytes,
// compatible with libp2p's crypto.UnmarshalPrivateKey.
func (kp *KeyPair) MarshalPrivateKey() ([]byte, error) {
	return libp2pcrypto.MarshalPrivateKey(kp.PrivKey)
}

// GenerateKeyPair creates a new random Ed25519 key pair.
func GenerateKeyPair() (*KeyPair, error) {
	privKey, pubKey, err := libp2pcrypto.GenerateKeyPair(libp2pcrypto.Ed25519, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}
	return &KeyPair{PrivKey: privKey, PubKey: pubKey}, nil
}

// GenerateKeyPairFromSeed creates a deterministic key pair from a seed,
// using the same HMAC-SHA256 derivation as the existing Mobazha node.
// This matches mobazha3.0/internal/repo/identity.go IdentityKeyFromSeed.
func GenerateKeyPairFromSeed(seed []byte) (*KeyPair, error) {
	hm := hmac.New(sha256.New, []byte("OpenBazaar seed"))
	hm.Write(seed)
	reader := bytes.NewReader(hm.Sum(nil))
	privKey, pubKey, err := libp2pcrypto.GenerateKeyPairWithReader(libp2pcrypto.Ed25519, 0, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair from seed: %w", err)
	}
	return &KeyPair{PrivKey: privKey, PubKey: pubKey}, nil
}

// KeyPairFromPrivateKey creates a KeyPair from a raw Ed25519 private key (64 bytes).
func KeyPairFromPrivateKey(privateKey ed25519.PrivateKey) (*KeyPair, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: got %d, want %d", len(privateKey), ed25519.PrivateKeySize)
	}
	privKey, err := libp2pcrypto.UnmarshalEd25519PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal private key: %w", err)
	}
	return &KeyPair{PrivKey: privKey, PubKey: privKey.GetPublic()}, nil
}

// KeyPairFromMarshaledPrivateKey creates a KeyPair from protobuf-encoded private key bytes,
// as returned by libp2p's crypto.MarshalPrivateKey.
func KeyPairFromMarshaledPrivateKey(marshaledKey []byte) (*KeyPair, error) {
	privKey, err := libp2pcrypto.UnmarshalPrivateKey(marshaledKey)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal private key: %w", err)
	}
	return &KeyPair{PrivKey: privKey, PubKey: privKey.GetPublic()}, nil
}

// MarshalPrivateKeyFromEd25519 serializes a raw Ed25519 private key (64 bytes)
// into libp2p protobuf format, compatible with crypto.UnmarshalPrivateKey.
// This is the bridge between KeyVault (which stores ed25519.PrivateKey) and
// mobazha3.0 nodes (which use libp2p marshaled bytes for identity).
func MarshalPrivateKeyFromEd25519(privKey ed25519.PrivateKey) ([]byte, error) {
	if len(privKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: got %d, want %d", len(privKey), ed25519.PrivateKeySize)
	}
	libp2pPrivKey, err := libp2pcrypto.UnmarshalEd25519PrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ed25519 private key: %w", err)
	}
	return libp2pcrypto.MarshalPrivateKey(libp2pPrivKey)
}

// MarshalPublicKeyFromEd25519 serializes a raw Ed25519 public key into
// libp2p protobuf format, compatible with crypto.MarshalPublicKey.
// This is useful for protocol messages that require the marshaled key format.
func MarshalPublicKeyFromEd25519(pubKey ed25519.PublicKey) ([]byte, error) {
	libp2pPubKey, err := libp2pcrypto.UnmarshalEd25519PublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ed25519 public key: %w", err)
	}
	return libp2pcrypto.MarshalPublicKey(libp2pPubKey)
}

// PeerIDFromPublicKey derives a PeerID from a libp2p public key.
// This produces the same result as peer.IDFromPublicKey in libp2p.
func PeerIDFromPublicKey(pubKey libp2pcrypto.PubKey) (PeerID, error) {
	id, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return "", fmt.Errorf("failed to derive PeerID: %w", err)
	}
	return PeerID(id.String()), nil
}

// PeerIDFromRawPublicKey derives a PeerID from a raw Ed25519 public key (32 bytes).
func PeerIDFromRawPublicKey(publicKey ed25519.PublicKey) (PeerID, error) {
	pubKey, err := libp2pcrypto.UnmarshalEd25519PublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal public key: %w", err)
	}
	return PeerIDFromPublicKey(pubKey)
}

// GeneratePeerID creates a new random peer identity.
// Returns the PeerID and the associated key pair.
func GeneratePeerID() (PeerID, *KeyPair, error) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return "", nil, err
	}
	peerID, err := PeerIDFromPublicKey(keyPair.PubKey)
	if err != nil {
		return "", nil, err
	}
	return peerID, keyPair, nil
}

// String returns the string representation of the PeerID.
func (p PeerID) String() string {
	return string(p)
}

// IsValid checks if the PeerID is valid (non-empty and parseable by libp2p).
func (p PeerID) IsValid() bool {
	if len(p) == 0 {
		return false
	}
	_, err := peer.Decode(string(p))
	return err == nil
}

// ToLibp2p converts PeerID to libp2p's peer.ID type.
func (p PeerID) ToLibp2p() (peer.ID, error) {
	return peer.Decode(string(p))
}

// PeerIDFromLibp2p converts a libp2p peer.ID to our PeerID type.
func PeerIDFromLibp2p(id peer.ID) PeerID {
	return PeerID(id.String())
}

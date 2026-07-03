package contracts

import (
	"crypto/ed25519"
	"fmt"

	"github.com/mobazha/mobazha/pkg/identity"
)

// KeyPairSigner is the standard implementation of Signer using an identity.KeyPair.
// It can be used directly by both mobazha-node and mobazha-cloud.
type KeyPairSigner struct {
	keyPair *identity.KeyPair
	peerID  identity.PeerID
}

// Compile-time check: KeyPairSigner implements Signer.
var _ Signer = (*KeyPairSigner)(nil)

// NewKeyPairSigner creates a Signer from a key pair and its derived PeerID.
// Panics if keyPair is nil — callers must provide a valid key pair.
func NewKeyPairSigner(keyPair *identity.KeyPair, peerID identity.PeerID) *KeyPairSigner {
	if keyPair == nil {
		panic("contracts: NewKeyPairSigner called with nil keyPair")
	}
	return &KeyPairSigner{keyPair: keyPair, peerID: peerID}
}

// NewKeyPairSignerFromMarshaledKey creates a Signer from protobuf-encoded
// private key bytes (the format stored in the database by libp2p).
func NewKeyPairSignerFromMarshaledKey(marshaledKey []byte) (*KeyPairSigner, error) {
	keyPair, err := identity.KeyPairFromMarshaledPrivateKey(marshaledKey)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal key: %w", err)
	}

	peerID, err := identity.PeerIDFromPublicKey(keyPair.PubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive peer ID: %w", err)
	}

	return &KeyPairSigner{keyPair: keyPair, peerID: peerID}, nil
}

// --- Signer interface ---

// Sign signs a message using the private key.
func (s *KeyPairSigner) Sign(message []byte) ([]byte, error) {
	return s.keyPair.Sign(message)
}

// Verify verifies a signature against this signer's public key.
func (s *KeyPairSigner) Verify(message []byte, signature []byte) (bool, error) {
	return s.keyPair.Verify(message, signature)
}

// PublicKey returns the raw Ed25519 public key.
func (s *KeyPairSigner) PublicKey() (ed25519.PublicKey, error) {
	return s.keyPair.RawPublicKey()
}

// PeerID returns the libp2p Peer ID.
func (s *KeyPairSigner) PeerID() identity.PeerID {
	return s.peerID
}

// --- Accessors ---

// KeyPair returns the underlying identity.KeyPair.
// New code should prefer using the Signer interface methods (Sign, Verify, PublicKey, PeerID)
// instead of accessing the KeyPair directly.
func (s *KeyPairSigner) KeyPair() *identity.KeyPair {
	return s.keyPair
}

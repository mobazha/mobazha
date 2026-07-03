// Package contracts defines contract interfaces between core and implementations.
// These interfaces ensure compatibility between mobazha-node and mobazha-cloud.
package contracts

import (
	"crypto/ed25519"

	"github.com/mobazha/mobazha/pkg/identity"
)

// Signer is the contract interface for signing operations.
// This interface must be implemented by:
// - mobazha-node: using local key storage (wrapping libp2p crypto.PrivKey)
// - mobazha-cloud: using Key Vault and multi-tenant key management
type Signer interface {
	// Sign signs a message and returns the Ed25519 signature.
	Sign(message []byte) ([]byte, error)

	// Verify verifies a signature against the signer's public key.
	Verify(message []byte, signature []byte) (bool, error)

	// PublicKey returns the signer's raw Ed25519 public key.
	PublicKey() (ed25519.PublicKey, error)

	// PeerID returns the libp2p peer ID derived from the public key.
	PeerID() identity.PeerID
}

// Verifier verifies signatures from arbitrary peers.
// Unlike Signer (which verifies against its own key), Verifier resolves
// the public key from a PeerID and then verifies.
type Verifier interface {
	// VerifyFromPeer verifies a signature given the sender's PeerID.
	// The implementation extracts the public key from the PeerID and verifies.
	VerifyFromPeer(message []byte, signature []byte, senderPeerID identity.PeerID) (bool, error)
}

// SignerFactory creates signers for different tenants (cloud) or the local node.
type SignerFactory interface {
	// GetSigner returns a signer for the given peer ID.
	// In node mode, this returns the local node's signer.
	// In cloud mode, this looks up the tenant's key in Key Vault.
	GetSigner(peerID identity.PeerID) (Signer, error)
}

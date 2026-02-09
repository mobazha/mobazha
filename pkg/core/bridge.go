// Package core provides bridging utilities between mobazha3.0 and mobazha-core.
// This package enables gradual migration from legacy code to the shared core library.
package core

import (
	"crypto/ed25519"
	"fmt"

	"github.com/mobazha/mobazha-core/contracts"
	"github.com/mobazha/mobazha-core/identity"
	"github.com/mobazha/mobazha-core/orders"
	"github.com/mobazha/mobazha-core/p2p"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
)

// Compile-time check: IdentityBridge implements contracts.Signer
var _ contracts.Signer = (*IdentityBridge)(nil)

// IdentityBridge bridges legacy identity code with mobazha-core identity module.
// It implements the contracts.Signer interface, allowing it to be used by both
// mobazha-node (local keys) and mobazha-cloud (multi-tenant key vault).
type IdentityBridge struct {
	keyPair *identity.KeyPair
	peerID  identity.PeerID
}

// NewIdentityBridge creates a new identity bridge from a raw Ed25519 private key.
// If existingPrivateKey is nil, generates a new key pair.
func NewIdentityBridge(existingPrivateKey ed25519.PrivateKey) (*IdentityBridge, error) {
	var keyPair *identity.KeyPair
	var err error

	if existingPrivateKey != nil {
		keyPair, err = identity.KeyPairFromPrivateKey(existingPrivateKey)
	} else {
		keyPair, err = identity.GenerateKeyPair()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create key pair: %w", err)
	}

	peerID, err := identity.PeerIDFromPublicKey(keyPair.PubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive peer ID: %w", err)
	}

	return &IdentityBridge{keyPair: keyPair, peerID: peerID}, nil
}

// NewIdentityBridgeFromMarshaledKey creates an IdentityBridge from libp2p marshaled
// private key bytes (the format stored in the database).
func NewIdentityBridgeFromMarshaledKey(marshaledKey []byte) (*IdentityBridge, error) {
	keyPair, err := identity.KeyPairFromMarshaledPrivateKey(marshaledKey)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal key: %w", err)
	}

	peerID, err := identity.PeerIDFromPublicKey(keyPair.PubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive peer ID: %w", err)
	}

	return &IdentityBridge{keyPair: keyPair, peerID: peerID}, nil
}

// --- contracts.Signer interface ---

// Sign signs a message using the private key and returns the signature bytes.
func (b *IdentityBridge) Sign(message []byte) ([]byte, error) {
	return b.keyPair.PrivKey.Sign(message)
}

// Verify verifies a signature against this signer's public key.
func (b *IdentityBridge) Verify(message []byte, signature []byte) (bool, error) {
	return b.keyPair.PubKey.Verify(message, signature)
}

// PublicKey returns the raw Ed25519 public key.
func (b *IdentityBridge) PublicKey() (ed25519.PublicKey, error) {
	return b.keyPair.RawPublicKey()
}

// PeerID returns the libp2p Peer ID.
func (b *IdentityBridge) PeerID() identity.PeerID {
	return b.peerID
}

// --- Legacy accessors (for backward compatibility during migration) ---

// RawPrivateKey returns the raw Ed25519 private key (use with caution).
func (b *IdentityBridge) RawPrivateKey() (ed25519.PrivateKey, error) {
	return b.keyPair.RawPrivateKey()
}

// Libp2pPrivKey returns the underlying libp2p private key.
// Needed for IPFS node, Messenger, store-and-forward, and other P2P operations.
func (b *IdentityBridge) Libp2pPrivKey() libp2pcrypto.PrivKey {
	return b.keyPair.PrivKey
}

// Libp2pPubKey returns the underlying libp2p public key.
func (b *IdentityBridge) Libp2pPubKey() libp2pcrypto.PubKey {
	return b.keyPair.PubKey
}

// OrderStateBridge bridges legacy order state handling with mobazha-core order module.
type OrderStateBridge struct{}

// NewOrderStateBridge creates a new order state bridge.
func NewOrderStateBridge() *OrderStateBridge {
	return &OrderStateBridge{}
}

// ValidateTransition checks if an order state transition is valid.
func (b *OrderStateBridge) ValidateTransition(currentState, event int) (int, bool) {
	result := orders.Transition(orders.OrderState(currentState), orders.OrderEvent(event))
	return int(result.NewState), result.Valid
}

// GetAllowedEvents returns the allowed events for a given state.
func (b *OrderStateBridge) GetAllowedEvents(state int) []int {
	allowed := orders.AllowedEvents(orders.OrderState(state))
	result := make([]int, len(allowed))
	for i, e := range allowed {
		result[i] = int(e)
	}
	return result
}

// MessageBridge bridges legacy P2P messaging with mobazha-core p2p module.
type MessageBridge struct {
	signer contracts.Signer
}

// NewMessageBridge creates a new message bridge using any Signer implementation.
func NewMessageBridge(signer contracts.Signer) *MessageBridge {
	return &MessageBridge{signer: signer}
}

// CreateSignedMessage creates a signed P2P message.
func (b *MessageBridge) CreateSignedMessage(msgType int, recipient string, payload []byte) (*p2p.Message, error) {
	msg := p2p.NewMessage(
		p2p.MessageType(msgType),
		b.signer.PeerID(),
		identity.PeerID(recipient),
		payload,
	)

	signature, err := b.signer.Sign(msg.SignableBytes())
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}
	msg.Signature = signature

	return msg, nil
}

// VerifyMessage verifies a received message's signature using the sender's public key.
// Note: This uses the sender's key (not the signer's own key) for verification.
func (b *MessageBridge) VerifyMessage(msg *p2p.Message, senderPublicKey ed25519.PublicKey) bool {
	return ed25519.Verify(senderPublicKey, msg.SignableBytes(), msg.Signature)
}

// GetSenderPeerID returns the sender's PeerID from a message.
func GetSenderPeerID(msg *p2p.Message) identity.PeerID {
	return msg.SenderPeerID
}

// GetRecipientPeerID returns the recipient's PeerID from a message.
func GetRecipientPeerID(msg *p2p.Message) identity.PeerID {
	return msg.RecipientPeerID
}

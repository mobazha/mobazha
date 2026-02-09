// Package core provides bridging utilities between mobazha3.0 and mobazha-core.
// This package enables gradual migration from legacy code to the shared core library.
package core

import (
	"crypto/ed25519"
	"fmt"

	"github.com/mobazha/mobazha-core/crypto"
	"github.com/mobazha/mobazha-core/identity"
	"github.com/mobazha/mobazha-core/orders"
	"github.com/mobazha/mobazha-core/p2p"
)

// IdentityBridge bridges legacy identity code with mobazha-core identity module.
type IdentityBridge struct {
	keyPair *identity.KeyPair
}

// NewIdentityBridge creates a new identity bridge.
// If existingPrivateKey is provided, it uses that key; otherwise generates a new one.
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

	return &IdentityBridge{keyPair: keyPair}, nil
}

// PeerID returns the Peer ID derived from the public key.
func (b *IdentityBridge) PeerID() identity.PeerID {
	return identity.PeerIDFromPublicKey(b.keyPair.PublicKey)
}

// PublicKey returns the public key.
func (b *IdentityBridge) PublicKey() ed25519.PublicKey {
	return b.keyPair.PublicKey
}

// PrivateKey returns the private key (use with caution).
func (b *IdentityBridge) PrivateKey() ed25519.PrivateKey {
	return b.keyPair.PrivateKey
}

// Sign signs a message using the private key.
func (b *IdentityBridge) Sign(message []byte) (crypto.Signature, error) {
	return crypto.Sign(b.keyPair.PrivateKey, message)
}

// Verify verifies a signature against a message.
func (b *IdentityBridge) Verify(message []byte, signature crypto.Signature) bool {
	return crypto.Verify(b.keyPair.PublicKey, message, signature)
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
	identity *IdentityBridge
}

// NewMessageBridge creates a new message bridge.
func NewMessageBridge(identity *IdentityBridge) *MessageBridge {
	return &MessageBridge{identity: identity}
}

// CreateSignedMessage creates a signed P2P message.
func (b *MessageBridge) CreateSignedMessage(msgType int, recipient string, payload []byte) (*p2p.Message, error) {
	msg := p2p.NewMessage(
		p2p.MessageType(msgType),
		b.identity.PeerID(),
		identity.PeerID(recipient),
		payload,
	)

	signature, err := b.identity.Sign(msg.SignableBytes())
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}
	msg.Signature = signature

	return msg, nil
}

// VerifyMessage verifies a received message's signature.
func (b *MessageBridge) VerifyMessage(msg *p2p.Message, senderPublicKey ed25519.PublicKey) bool {
	return crypto.Verify(senderPublicKey, msg.SignableBytes(), crypto.Signature(msg.Signature))
}

// GetSenderPeerID returns the sender's PeerID from a message.
func GetSenderPeerID(msg *p2p.Message) identity.PeerID {
	return msg.SenderPeerID
}

// GetRecipientPeerID returns the recipient's PeerID from a message.
func GetRecipientPeerID(msg *p2p.Message) identity.PeerID {
	return msg.RecipientPeerID
}

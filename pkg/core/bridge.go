// Package core provides bridging utilities for order state and P2P messaging.
// It wraps pkg/orders, pkg/p2p, and pkg/identity for legacy integration points.
package core

import (
	"crypto/ed25519"
	"fmt"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/identity"
	"github.com/mobazha/mobazha3.0/pkg/orders"
	"github.com/mobazha/mobazha3.0/pkg/p2p"
)

// OrderStateBridge bridges legacy order state handling with pkg/orders.
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

// MessageBridge bridges legacy P2P messaging with pkg/p2p.
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

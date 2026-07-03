// Package contracts defines contract interfaces between core and implementations.
package contracts

import (
	"context"

	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/p2p"
)

// MessageSender is the contract interface for sending P2P messages.
// Implementations handle routing through the P2P network or via relay.
//
// In mobazha-node: wraps the Messenger with direct P2P and store-and-forward
// In mobazha-cloud: wraps a shared P2P Gateway Pool for multi-tenant sending
type MessageSender interface {
	// SendMessage sends a message to a peer.
	// Returns an error if the message cannot be delivered.
	SendMessage(ctx context.Context, recipient identity.PeerID, msg *p2p.Message) error

	// SendReliableMessage sends a message with automatic retry and persistence.
	// The implementation should persist the message and retry until delivered,
	// falling back to store-and-forward for offline peers.
	SendReliableMessage(ctx context.Context, recipient identity.PeerID, msg *p2p.Message) error
}

// MessageHandler is the contract interface for receiving P2P messages.
type MessageHandler interface {
	// HandleMessage processes an incoming message from a sender.
	HandleMessage(ctx context.Context, sender identity.PeerID, msg *p2p.Message) error
}

// MessageRouter routes messages to appropriate handlers based on type.
type MessageRouter interface {
	// RegisterHandler registers a handler for a specific message type.
	RegisterHandler(msgType p2p.MessageType, handler MessageHandler)

	// Route routes an incoming message to its handler.
	Route(ctx context.Context, sender identity.PeerID, msg *p2p.Message) error
}

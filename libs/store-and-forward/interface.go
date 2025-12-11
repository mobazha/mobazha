package storeandforward

import (
	"context"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// SNFClientInterface defines the interface for SNF client operations.
// Both Client and LocalClient implement this interface.
type SNFClientInterface interface {
	// SubscribeMessages returns a subscription for receiving relayed messages
	SubscribeMessages() *Subscription

	// GetMessages retrieves messages from all registered SNF servers
	GetMessages(ctx context.Context) ([]Message, error)

	// SendMessage sends a message through an SNF server
	SendMessage(ctx context.Context, to, server peer.ID, pubkey crypto.PubKey, encryptedMessage, metadata []byte) error

	// AckMessage acknowledges receipt of a message
	AckMessage(ctx context.Context, messageID []byte) error
}

// Ensure both Client and LocalClient implement the interface
var _ SNFClientInterface = (*Client)(nil)
var _ SNFClientInterface = (*LocalClient)(nil)

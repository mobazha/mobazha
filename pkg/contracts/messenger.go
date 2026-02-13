// Package contracts — Messenger and NetworkService abstract P2P messaging.
//
// Standalone mode: backed by libp2p streams and store-and-forward servers.
// SaaS mode: backed by the hosting's shared libp2p host and local delivery.
package contracts

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/database"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
)

// Messenger abstracts reliable message delivery.
//
// Messages are persisted to the database within the caller's transaction
// and retried until the recipient sends an ACK.
type Messenger interface {
	// ReliablySendMessage persists the message in tx and schedules delivery.
	// The done channel (if non-nil) is closed when the message is ACK'd.
	ReliablySendMessage(tx database.Tx, peer peer.ID, msg *pb.Message, done chan<- struct{}) error

	// ProcessACK marks the referenced outgoing message as delivered.
	ProcessACK(tx database.Tx, ack *pb.AckMessage) error

	// SendACK sends an acknowledgment for messageID to the given peer.
	SendACK(messageID string, peer peer.ID)

	// Start begins background retry and polling loops.
	// SaaS implementations may use a no-op.
	Start()

	// Stop shuts down background loops and waits for them to finish.
	// SaaS implementations may use a no-op.
	Stop()
}

// NetworkService abstracts low-level P2P message sending and handler
// registration.
//
// In standalone mode this wraps a libp2p host; in SaaS mode it delegates
// to the hosting process's shared libp2p host or a local delivery path.
type NetworkService interface {
	// SendMessage sends a single message to peerID.
	SendMessage(ctx context.Context, peerID peer.ID, msg *pb.Message) error

	// RegisterHandler registers a handler function for the given message type.
	RegisterHandler(messageType pb.Message_MessageType, handler func(peerID peer.ID, msg *pb.Message) error)

	// DeliverLocalMessage dispatches a message to this node's registered
	// handler without going through the network stack. Used by hosting
	// for same-process message delivery between co-located nodes.
	DeliverLocalMessage(from peer.ID, msg *pb.Message) error

	// Close shuts down the network service and releases resources.
	Close()
}

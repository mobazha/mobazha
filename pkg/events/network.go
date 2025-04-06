package events

import (
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// PeerConnected is an event that gets pushed to the bus
// whenever a new peer connects.
type PeerConnected struct {
	Peer peer.ID
}

// PeerDisconnected is an event that gets pushed to the bus
// whenever a peer disconnects.
type PeerDisconnected struct {
	Peer peer.ID
}

// MessageACK is an event that gets pushed to the bus
// whenever a message ACK is received.
type MessageACK struct {
	MessageID string
}

// MessageStore is an event that gets pushed to the bus
// whenever a STORE message is received.
type MessageStore struct {
	Peer peer.ID
	Cids []cid.Cid
}

// PublishStarted is an event that gets pushed to the bus
// when publishing starts. It contains an ID so that we can
// match up this publish with the response if there are
// concurrent publishes.
type PublishStarted struct {
	ID int
}

// PublishFinished is an event that gets pushed to the bus
// when publishing finishes. It contains an ID so that we can
// match up this event with start event.
type PublishFinished struct {
	ID int
}

// PublishingError is an event that gets pushed to the bus
// if publishing finishes with an error.
type PublishingError struct {
	Err error
}

// IPFSShutdown is an event that gets pushed when the IPFS node
// shuts down.
type IPFSShutdown struct{}

// PingReceived is an event that gets pushed when a node receives
// a PING message.
type PingReceived struct {
	Peer peer.ID
}

// PongReceived is an event that gets pushed when a node receives
// a PONG message.
type PongReceived struct {
	Peer peer.ID
}

package events

import peer "github.com/libp2p/go-libp2p/core/peer"

// TrackerStarted is the start event for the follower tracker.
type TrackerStarted struct{}

// TrackerPeerConnected represents a peer connected event
// in the follower tracker.
type TrackerPeerConnected struct {
	Peer peer.ID
}

// TrackerPeerConnected represents a peer disconnected event
// in the follower tracker.
type TrackerPeerDisconnected struct {
	Peer peer.ID
}

// TrackerPeerConnected represents a peer follow event
// in the follower tracker.
type TrackerFollow struct {
	Peer peer.ID
}

// TrackerPeerConnected represents a peer unfolow event
// in the follower tracker.
type TrackerUnfollow struct {
	Peer peer.ID
}

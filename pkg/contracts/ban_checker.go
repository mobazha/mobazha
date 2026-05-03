package contracts

import peer "github.com/libp2p/go-libp2p/core/peer"

// BanChecker abstracts the peer ban/block functionality so that consumers
// (e.g. ListingAppService, PreferencesAppService) don't depend on
// internal/net.BanManager directly.
type BanChecker interface {
	IsGlobalBanned(peerID peer.ID) bool
	AddBlockedID(peerID peer.ID)
	RemoveBlockedID(peerID peer.ID)
}

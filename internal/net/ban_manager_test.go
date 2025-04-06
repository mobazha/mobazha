package net

import (
	"testing"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

func bannedPeers() []peer.ID {
	peerStrings := []string{
		"QmY3ArotKMKaL7YGfbQfyDrib6RVraLqZYWXZvVgZktBxp",
		"QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h",
		"QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN",
	}
	peers := make([]peer.ID, 0, len(peerStrings))

	for _, p := range peerStrings {
		peer, err := peer.Decode(p)
		if err != nil {
			panic(err)
		}
		peers = append(peers, peer)
	}
	return peers
}

func TestNewBanManager(t *testing.T) {
	banned := bannedPeers()
	bm := NewBanManager(nil, banned)

	if len(bm.blockedIds) != len(banned) {
		t.Errorf("Expected to initialize the ban manager with %d peers. Got %d", len(banned), len(bm.blockedIds))
	}
}

func TestBanManager_AddBlockedId(t *testing.T) {
	banned := bannedPeers()
	bm := NewBanManager(nil, nil)

	for _, p := range banned {
		bm.AddBlockedID(p)
	}

	for _, p := range banned {
		if !bm.IsBanned(p) {
			t.Errorf("Peer %s is not banned", p)
		}
	}
}

func TestBanManager_RemoveBlockedId(t *testing.T) {
	banned := bannedPeers()
	bm := NewBanManager(nil, banned)

	for _, p := range banned {
		bm.RemoveBlockedID(p)
	}

	for _, p := range banned {
		if bm.IsBanned(p) {
			t.Errorf("Peer %s is banned when it shouldn't be", p)
		}
	}
}

func TestBanManager_SetBlockedIds(t *testing.T) {
	banned := bannedPeers()
	bm := NewBanManager(nil, nil)

	bm.SetBlockedIds(banned)

	for _, p := range banned {
		if !bm.IsBanned(p) {
			t.Errorf("Peer %s is not banned", p)
		}
	}
}

func TestBanManager_GetBlockedIds(t *testing.T) {
	banned := bannedPeers()
	bm := NewBanManager(nil, banned)

	for _, b := range banned {
		exists := false
		for _, p := range bm.GetBlockedIds() {
			if p == b {
				exists = true
				break
			}
		}
		if !exists {
			t.Errorf("Failed to return peer %s", b)
		}
	}
}

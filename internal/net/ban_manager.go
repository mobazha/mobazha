package net

import (
	"sync"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

type BanManager struct {
	globalBlockedIds map[string]bool
	blockedIds       map[string]bool
	*sync.RWMutex
}

func NewBanManager(globalBlockedIds []peer.ID, blockedIds []peer.ID) *BanManager {
	globalBlockedMap := make(map[string]bool)
	for _, pid := range globalBlockedIds {
		globalBlockedMap[pid.String()] = true
	}

	blockedMap := make(map[string]bool)
	for _, pid := range blockedIds {
		blockedMap[pid.String()] = true
	}
	return &BanManager{globalBlockedMap, blockedMap, new(sync.RWMutex)}
}

func (bm *BanManager) AddBlockedID(peerID peer.ID) {
	bm.Lock()
	defer bm.Unlock()
	bm.blockedIds[peerID.String()] = true
}

func (bm *BanManager) RemoveBlockedID(peerID peer.ID) {
	bm.Lock()
	defer bm.Unlock()
	if bm.blockedIds[peerID.String()] {
		delete(bm.blockedIds, peerID.String())
	}
}

func (bm *BanManager) SetBlockedIds(peerIDs []peer.ID) {
	bm.Lock()
	defer bm.Unlock()

	bm.blockedIds = make(map[string]bool)

	for _, pid := range peerIDs {
		bm.blockedIds[pid.String()] = true
	}
}

func (bm *BanManager) GetBlockedIds() []peer.ID {
	bm.RLock()
	defer bm.RUnlock()
	var ret []peer.ID
	for pid := range bm.blockedIds {
		id, err := peer.Decode(pid)
		if err != nil {
			continue
		}
		ret = append(ret, id)
	}
	return ret
}

func (bm *BanManager) IsBanned(peerID peer.ID) bool {
	bm.RLock()
	defer bm.RUnlock()
	return bm.blockedIds[peerID.String()] || bm.globalBlockedIds[peerID.String()]
}

func (bm *BanManager) IsGlobalBanned(peerID peer.ID) bool {
	bm.RLock()
	defer bm.RUnlock()
	return bm.globalBlockedIds[peerID.String()]
}

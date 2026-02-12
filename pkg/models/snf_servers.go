package models

import (
	"encoding/json"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

type StoreAndForwardServers struct {
	TenantID    string `gorm:"column:tenant_id;primaryKey;default:''" json:"-"`
	PeerID      string `gorm:"primaryKey"`
	SNFServers  []byte
	LastUpdated time.Time
}

// PutServers save the peer IDs into the model. This will override
// the existing peer IDs.
func (p *StoreAndForwardServers) PutServers(peerIDs []string) error {
	out, err := json.Marshal(peerIDs)
	if err != nil {
		return err
	}
	p.SNFServers = out
	return nil
}

// Servers returns the peer IDs saved in the model.
func (p *StoreAndForwardServers) Servers() ([]peer.ID, error) {
	if p.SNFServers == nil {
		return nil, nil
	}
	var inboxPeers []string
	if err := json.Unmarshal(p.SNFServers, &inboxPeers); err != nil {
		return nil, err
	}
	pids := make([]peer.ID, 0, len(inboxPeers))
	for _, peerStr := range inboxPeers {
		pid, err := peer.Decode(peerStr)
		if err != nil {
			return nil, err
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

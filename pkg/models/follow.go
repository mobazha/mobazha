package models

import peer "github.com/libp2p/go-libp2p/core/peer"

// Followers represents the nodes that are following this node.
type Followers []string

// Count returns the number of followers.
func (f *Followers) Count() int {
	return len(*f)
}

// Following represents the list of nodes this node is following.
type Following []string

// Count returns the number of following.
func (f *Following) Count() int {
	return len(*f)
}

func (f *Following) IsFollowing(peer peer.ID) bool {
	for _, following := range *f {
		if peer.String() == following {
			return true
		}
	}
	return false
}

// FollowSequence is a database model which holds the sequence
// number for our outgoing follow and unfollow notifications.
type FollowSequence struct {
	TenantID string `gorm:"column:tenant_id;primaryKey;default:''" json:"-"`
	PeerID   string `gorm:"primaryKey"`
	Num      int
}

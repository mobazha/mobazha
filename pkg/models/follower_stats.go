package models

import (
	"time"
)

// FollowerStat tracks the update of our followers. We use this
// to make decisions about which followers to push our data to.
// Ideally we only want to push to followers with strong uptime.
type FollowerStat struct {
	PeerID            string        `gorm:"primaryKey"`
	ConnectedDuration time.Duration `gorm:"index"`
	LastConnection    time.Time     `gorm:"index"`
}

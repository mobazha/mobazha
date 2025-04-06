package wallet_interface

import "time"

// BlockID represents an ID for a block on the network.
type BlockID string

// String returns the string representation of the ID.
func (b *BlockID) String() string {
	return string(*b)
}

// BlockInfo holds info about a block.
type BlockInfo struct {
	BlockID   BlockID   `json:"blockID"`
	PrevBlock BlockID   `json:",omitempty"`
	Height    uint64    `json:"height"`
	BlockTime time.Time `json:"blockTime"`
}

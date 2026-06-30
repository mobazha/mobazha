package payment

import (
	"math/big"
	"time"
)

// FundingObservation is the chain-neutral evidence for one inbound transfer
// to a watched payment address. Core validates and aggregates it before any
// order state transition.
type FundingObservation struct {
	OrderID        string
	ChainNamespace string
	ChainReference string
	TxHash         string
	TxHashSource   string
	EventIndex     int
	EventType      string
	FromAddress    string
	ToAddress      string
	TokenAddress   string
	Amount         *big.Int
	BlockNumber    int64
	BlockTime      time.Time
}

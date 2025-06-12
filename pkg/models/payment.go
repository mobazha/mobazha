package models

import (
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type InitializeEscrowData struct {
	OrderID      string           `json:"orderID"`
	PayerAddress []byte           `json:"payerAddress"` // payer pubkey bytes
	Moderator    string           `json:"moderator"`    // peerID
	CoinType     iwallet.CoinType `json:"coinType"`
	Amount       uint64           `json:"amount"`
}

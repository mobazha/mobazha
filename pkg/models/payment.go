package models

import (
	"github.com/gagliardetto/solana-go"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type InitializeSolEscrowData struct {
	OrderID   string           `json:"orderID"`
	Payer     solana.PublicKey `json:"payer"`
	Moderator string           `json:"moderator"` // peerID
	CoinType  iwallet.CoinType `json:"coinType"`
	Amount    uint64           `json:"amount"`
}

package models

import "github.com/gagliardetto/solana-go"

type InitializeSolEscrowData struct {
	OrderID   string           `json:"orderID"`
	Payer     solana.PublicKey `json:"payer"`
	Moderator string           `json:"moderator"` // peerID
	Amount    uint64           `json:"amount"`
}

// InitializeSPLTokenData 初始化SPL Token托管参数
type InitializeSPLTokenData struct {
	OrderID   string           `json:"orderID"`
	Payer     solana.PublicKey `json:"payer"`
	Moderator string           `json:"moderator"` // peerID
	Mint      solana.PublicKey `json:"mint"`
	Amount    uint64           `json:"amount"`
}

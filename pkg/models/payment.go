package models

import "github.com/gagliardetto/solana-go"

type InitializeSolEscrowData struct {
	OrderID   string           `json:"orderID"`
	Payer     solana.PublicKey `json:"payer"`
	Moderator solana.PublicKey `json:"moderator"`
	Amount    uint64           `json:"amount"`
}

// InitializeSPLTokenData 初始化SPL Token托管参数
type InitializeSPLTokenData struct {
	Payer     solana.PublicKey `json:"payer"`
	Moderator solana.PublicKey `json:"moderator"`
	Mint      solana.PublicKey `json:"mint"`
	Amount    uint64           `json:"amount"`
}

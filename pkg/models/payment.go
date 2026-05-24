package models

import (
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type OrderInfo struct {
	BuyerAddress  string
	VendorAddress string
	UniqueId      [20]byte
	UnlockHours   int
}

type InitializeEscrowData struct {
	OrderID       string           `json:"orderID"`
	PayerAddress  string           `json:"payerAddress"`  // payer pubkey bytes
	RefundAddress string           `json:"refundAddress"` // buyer-controlled refund target for crypto payments
	Moderator     string           `json:"moderator"`     // peerID
	CoinType      iwallet.CoinType `json:"coinType"`
	Amount        uint64           `json:"amount"`
}

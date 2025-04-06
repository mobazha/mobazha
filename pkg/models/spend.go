package models

import "time"

type SpendRequest struct {
	Currency Currency `json:"currency"`
	Address  string   `json:"address"`
	Amount   string   `json:"amount"`
	FeeLevel string   `json:"feeLevel"`
	Memo     string   `json:"memo"`
	OrderID  string   `json:"orderID"`
}

type SpendResponse struct {
	Txid               string    `json:"txid"`
	Amount             string    `json:"amount"`
	ConfirmedBalance   string    `json:"confirmedBalance"`
	UnconfirmedBalance string    `json:"unconfirmedBalance"`
	Currency           *Currency `json:"currency"`
	Memo               string    `json:"memo"`
	OrderID            string    `json:"orderID"`
	Timestamp          time.Time `json:"timestamp"`
}

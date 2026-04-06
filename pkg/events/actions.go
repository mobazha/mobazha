package events

import "time"

type Follow struct {
	Notification
	PeerID string `json:"peerID"`
}

type Unfollow struct {
	Notification
	PeerID string `json:"peerID"`
}

type ModeratorAdd struct {
	Notification
	PeerID string `json:"peerID"`
}

type ModeratorRemove struct {
	Notification
	PeerID string `json:"peerID"`
}

type ShoppingCartUpdate struct {
	ItemsCount int `json:"itemsCount"`
}

type IncomingTransaction struct {
	Wallet        string    `json:"wallet"`
	Txid          string    `json:"txid"`
	Value         int64     `json:"value"`
	Address       string    `json:"address"`
	Status        string    `json:"status"`
	Memo          string    `json:"memo"`
	Timestamp     time.Time `json:"timestamp"`
	Confirmations int32     `json:"confirmations"`
	OrderID       string    `json:"orderID"`
	Thumbnail     string    `json:"thumbnail"`
	Height        int32     `json:"height"`
	CanBumpFee    bool      `json:"canBumpFee"`
}

type AddressRequestResponse struct {
	PeerID  string `json:"peerID"`
	Address string `json:"address"`
	Coin    string `json:"coin"`
}

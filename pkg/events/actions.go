package events

import (
	"time"

	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

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

type ChatMessage struct {
	MessageID string             `json:"messageID"`
	PeerID    string             `json:"peerID"`
	OrderID   string             `json:"orderID"`
	Timestamp time.Time          `json:"timestamp"`
	Read      bool               `json:"read"`
	Outgoing  bool               `json:"outgoing"`
	Message   string             `json:"message"`
	File      *models.FileInChat `json:"file"`
}

func ToChatEvent(cm *models.ChatMessage) *ChatMessage {
	fileInChat, _ := cm.File()
	return &ChatMessage{
		MessageID: cm.MessageID,
		Timestamp: cm.Timestamp,
		PeerID:    cm.PeerID,
		OrderID:   cm.OrderID,
		Outgoing:  cm.Outgoing,
		Read:      cm.Read,
		Message:   cm.Message,
		File:      fileInChat,
	}
}

type ChatRead struct {
	MessageID string `json:"messageID"`
	PeerID    string `json:"peerID"`
	OrderID   string `json:"orderID"`
}

type ChatTyping struct {
	PeerID  string `json:"peerID"`
	OrderID string `json:"orderID"`
}

type ChatGroupCreate struct {
	GroupID   string    `json:"groupID"`
	OrderID   string    `json:"orderID"`
	GroupName string    `json:"groupName"`
	Owner     string    `json:"owner"`
	Peers     []string  `json:"peers"`
	Timestamp time.Time `json:"timestamp"`
}

type ChatGroupUpdate struct {
	GroupID   string    `json:"groupID"`
	OrderID   string    `json:"orderID"`
	GroupName string    `json:"groupName"`
	Owner     string    `json:"owner"`
	Peers     []string  `json:"peers"`
	Timestamp time.Time `json:"timestamp"`
}

type ChatGroupDelete struct {
	GroupID   string    `json:"groupID"`
	OrderID   string    `json:"orderID"`
	Timestamp time.Time `json:"timestamp"`
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

type ChannelMessage struct {
	PeerID    string    `json:"peerID"`
	Topic     string    `json:"topic"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Cid       string    `json:"cid"`
}

type ChannelRequestResponse struct {
	PeerID string    `json:"peerID"`
	Topic  string    `json:"topic"`
	Cids   []cid.Cid `json:"cids"`
}

type ChannelBootstrapped struct {
	Topic string
}

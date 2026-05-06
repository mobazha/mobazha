// Package p2p defines the P2P message protocol for Mobazha.
package p2p

import (
	"time"

	"github.com/mobazha/mobazha3.0/pkg/crypto"
	"github.com/mobazha/mobazha3.0/pkg/identity"
)

// MessageType represents the type of P2P message.
type MessageType int

const (
	// MessageTypeUnknown is an invalid message type.
	MessageTypeUnknown MessageType = iota
	// MessageTypeOrderOpen is sent when a buyer creates an order.
	MessageTypeOrderOpen
	// MessageTypeOrderConfirm is sent when a vendor confirms an order.
	MessageTypeOrderConfirm
	// MessageTypeOrderFulfillment is sent when a vendor fulfills an order.
	MessageTypeOrderFulfillment
	// MessageTypeOrderComplete is sent when a buyer completes an order.
	MessageTypeOrderComplete
	// MessageTypeOrderCancel is sent when an order is canceled.
	MessageTypeOrderCancel
	// MessageTypeOrderRefund is sent when a refund is issued.
	MessageTypeOrderRefund
	// MessageTypeDisputeOpen is sent when a dispute is opened.
	MessageTypeDisputeOpen
	// MessageTypeDisputeClose is sent when a dispute is resolved.
	MessageTypeDisputeClose
	// MessageTypeChat is a chat message.
	MessageTypeChat
	// MessageTypePing is a ping message.
	MessageTypePing
	// MessageTypePong is a pong response.
	MessageTypePong
)

// String returns the string representation of a MessageType.
func (m MessageType) String() string {
	switch m {
	case MessageTypeOrderOpen:
		return "ORDER_OPEN"
	case MessageTypeOrderConfirm:
		return "ORDER_CONFIRM"
	case MessageTypeOrderFulfillment:
		return "ORDER_FULFILLMENT"
	case MessageTypeOrderComplete:
		return "ORDER_COMPLETE"
	case MessageTypeOrderCancel:
		return "ORDER_CANCEL"
	case MessageTypeOrderRefund:
		return "ORDER_REFUND"
	case MessageTypeDisputeOpen:
		return "DISPUTE_OPEN"
	case MessageTypeDisputeClose:
		return "DISPUTE_CLOSE"
	case MessageTypeChat:
		return "CHAT"
	case MessageTypePing:
		return "PING"
	case MessageTypePong:
		return "PONG"
	default:
		return "UNKNOWN"
	}
}

// Message represents a P2P message in the Mobazha network.
type Message struct {
	// Type is the message type.
	Type MessageType

	// SenderPeerID is the sender's peer ID.
	SenderPeerID identity.PeerID

	// RecipientPeerID is the recipient's peer ID (empty for broadcast).
	RecipientPeerID identity.PeerID

	// Timestamp is when the message was created.
	Timestamp time.Time

	// Payload is the message content (typically protobuf encoded).
	Payload []byte

	// Signature is the sender's signature over the message.
	Signature crypto.Signature
}

// NewMessage creates a new message.
func NewMessage(msgType MessageType, sender, recipient identity.PeerID, payload []byte) *Message {
	return &Message{
		Type:            msgType,
		SenderPeerID:    sender,
		RecipientPeerID: recipient,
		Timestamp:       time.Now(),
		Payload:         payload,
	}
}

// SignableBytes returns the bytes that should be signed.
func (m *Message) SignableBytes() []byte {
	// Concatenate: type + sender + recipient + timestamp + payload
	data := make([]byte, 0)
	data = append(data, byte(m.Type))
	data = append(data, []byte(m.SenderPeerID)...)
	data = append(data, []byte(m.RecipientPeerID)...)
	// Add timestamp as bytes
	ts := m.Timestamp.Unix()
	data = append(data, byte(ts>>56), byte(ts>>48), byte(ts>>40), byte(ts>>32),
		byte(ts>>24), byte(ts>>16), byte(ts>>8), byte(ts))
	data = append(data, m.Payload...)
	return data
}

// IsValid performs basic validation on the message.
func (m *Message) IsValid() bool {
	if m.Type == MessageTypeUnknown {
		return false
	}
	if !m.SenderPeerID.IsValid() {
		return false
	}
	if m.Timestamp.IsZero() {
		return false
	}
	return true
}

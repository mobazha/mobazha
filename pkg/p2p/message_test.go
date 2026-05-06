package p2p

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/identity"
)

// testPeerID generates a real libp2p PeerID for testing.
func testPeerID(t *testing.T) identity.PeerID {
	t.Helper()
	kp, err := identity.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}
	pid, err := identity.PeerIDFromPublicKey(kp.PubKey)
	if err != nil {
		t.Fatalf("PeerIDFromPublicKey() error = %v", err)
	}
	return pid
}

func TestNewMessage(t *testing.T) {
	sender := testPeerID(t)
	recipient := testPeerID(t)
	payload := []byte("test payload")

	msg := NewMessage(MessageTypeOrderOpen, sender, recipient, payload)

	if msg.Type != MessageTypeOrderOpen {
		t.Errorf("Type = %v, want %v", msg.Type, MessageTypeOrderOpen)
	}

	if msg.SenderPeerID != sender {
		t.Errorf("SenderPeerID = %v, want %v", msg.SenderPeerID, sender)
	}

	if msg.RecipientPeerID != recipient {
		t.Errorf("RecipientPeerID = %v, want %v", msg.RecipientPeerID, recipient)
	}

	if string(msg.Payload) != string(payload) {
		t.Errorf("Payload = %v, want %v", msg.Payload, payload)
	}

	if msg.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestMessage_IsValid(t *testing.T) {
	validPeerID := testPeerID(t)

	tests := []struct {
		name string
		msg  *Message
		want bool
	}{
		{
			name: "valid message",
			msg: &Message{
				Type:         MessageTypeOrderOpen,
				SenderPeerID: validPeerID,
				Timestamp:    time.Now(),
			},
			want: true,
		},
		{
			name: "unknown type",
			msg: &Message{
				Type:         MessageTypeUnknown,
				SenderPeerID: validPeerID,
				Timestamp:    time.Now(),
			},
			want: false,
		},
		{
			name: "empty sender",
			msg: &Message{
				Type:         MessageTypeOrderOpen,
				SenderPeerID: identity.PeerID(""),
				Timestamp:    time.Now(),
			},
			want: false,
		},
		{
			name: "zero timestamp",
			msg: &Message{
				Type:         MessageTypeOrderOpen,
				SenderPeerID: validPeerID,
				Timestamp:    time.Time{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.IsValid(); got != tt.want {
				t.Errorf("Message.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_SignableBytes(t *testing.T) {
	sender := testPeerID(t)
	recipient := testPeerID(t)

	msg := &Message{
		Type:            MessageTypeOrderOpen,
		SenderPeerID:    sender,
		RecipientPeerID: recipient,
		Timestamp:       time.Unix(1234567890, 0),
		Payload:         []byte("test"),
	}

	bytes1 := msg.SignableBytes()
	bytes2 := msg.SignableBytes()

	// Same message should produce same bytes
	if string(bytes1) != string(bytes2) {
		t.Error("SignableBytes() should be deterministic")
	}

	// Different message should produce different bytes
	msg2 := &Message{
		Type:            MessageTypeOrderConfirm,
		SenderPeerID:    sender,
		RecipientPeerID: recipient,
		Timestamp:       time.Unix(1234567890, 0),
		Payload:         []byte("test"),
	}

	bytes3 := msg2.SignableBytes()
	if string(bytes1) == string(bytes3) {
		t.Error("Different messages should produce different bytes")
	}
}

func TestMessageType_String(t *testing.T) {
	tests := []struct {
		msgType MessageType
		want    string
	}{
		{MessageTypeOrderOpen, "ORDER_OPEN"},
		{MessageTypeOrderConfirm, "ORDER_CONFIRM"},
		{MessageTypeOrderFulfillment, "ORDER_FULFILLMENT"},
		{MessageTypeOrderComplete, "ORDER_COMPLETE"},
		{MessageTypeOrderCancel, "ORDER_CANCEL"},
		{MessageTypeOrderRefund, "ORDER_REFUND"},
		{MessageTypeDisputeOpen, "DISPUTE_OPEN"},
		{MessageTypeDisputeClose, "DISPUTE_CLOSE"},
		{MessageTypeChat, "CHAT"},
		{MessageTypePing, "PING"},
		{MessageTypePong, "PONG"},
		{MessageTypeUnknown, "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.msgType.String(); got != tt.want {
				t.Errorf("MessageType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

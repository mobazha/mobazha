package core

import (
	"testing"

	"github.com/mobazha/mobazha-core/orders"
	"github.com/mobazha/mobazha-core/p2p"
)

func TestIdentityBridge_NewKey(t *testing.T) {
	bridge, err := NewIdentityBridge(nil)
	if err != nil {
		t.Fatalf("NewIdentityBridge failed: %v", err)
	}

	peerID := bridge.PeerID()
	if peerID == "" {
		t.Error("Expected non-empty PeerID")
	}

	pubKey := bridge.PublicKey()
	if len(pubKey) == 0 {
		t.Error("Expected non-empty public key")
	}

	privKey := bridge.PrivateKey()
	if len(privKey) == 0 {
		t.Error("Expected non-empty private key")
	}

	t.Logf("Generated PeerID: %s", peerID)
}

func TestIdentityBridge_ExistingKey(t *testing.T) {
	// Create first bridge with new key
	bridge1, err := NewIdentityBridge(nil)
	if err != nil {
		t.Fatalf("First NewIdentityBridge failed: %v", err)
	}

	// Create second bridge with same private key
	bridge2, err := NewIdentityBridge(bridge1.PrivateKey())
	if err != nil {
		t.Fatalf("Second NewIdentityBridge failed: %v", err)
	}

	// Should have same PeerID
	if bridge1.PeerID() != bridge2.PeerID() {
		t.Errorf("PeerIDs should match: %s != %s", bridge1.PeerID(), bridge2.PeerID())
	}
}

func TestIdentityBridge_SignVerify(t *testing.T) {
	bridge, err := NewIdentityBridge(nil)
	if err != nil {
		t.Fatalf("NewIdentityBridge failed: %v", err)
	}

	message := []byte("test message to sign")

	signature, err := bridge.Sign(message)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	if !bridge.Verify(message, signature) {
		t.Error("Signature verification failed")
	}

	// Verify with wrong message should fail
	wrongMessage := []byte("different message")
	if bridge.Verify(wrongMessage, signature) {
		t.Error("Verification should fail for wrong message")
	}
}

func TestOrderStateBridge_ValidTransition(t *testing.T) {
	bridge := NewOrderStateBridge()

	// Test valid transition: Pending -> PaymentSent -> AwaitingPayment
	newState, valid := bridge.ValidateTransition(int(orders.StatePending), int(orders.EventPaymentSent))
	if !valid {
		t.Error("Expected valid transition from Pending on PaymentSent")
	}
	if newState != int(orders.StateAwaitingPayment) {
		t.Errorf("Expected AwaitingPayment state, got %d", newState)
	}
}

func TestOrderStateBridge_InvalidTransition(t *testing.T) {
	bridge := NewOrderStateBridge()

	// Test invalid transition: Completed -> PaymentSent (not allowed)
	_, valid := bridge.ValidateTransition(int(orders.StateCompleted), int(orders.EventPaymentSent))
	if valid {
		t.Error("Expected invalid transition from Completed on PaymentSent")
	}
}

func TestOrderStateBridge_AllowedEvents(t *testing.T) {
	bridge := NewOrderStateBridge()

	// Pending state should allow PaymentSent, VendorCancel, BuyerCancel
	allowed := bridge.GetAllowedEvents(int(orders.StatePending))
	if len(allowed) == 0 {
		t.Error("Expected some allowed events for Pending state")
	}

	t.Logf("Allowed events for Pending: %v", allowed)
}

func TestMessageBridge_CreateSignedMessage(t *testing.T) {
	identity, err := NewIdentityBridge(nil)
	if err != nil {
		t.Fatalf("NewIdentityBridge failed: %v", err)
	}

	bridge := NewMessageBridge(identity)

	recipientID := "12D3KooWTestRecipient"
	payload := []byte(`{"action": "test"}`)

	msg, err := bridge.CreateSignedMessage(int(p2p.MessageTypeOrderOpen), recipientID, payload)
	if err != nil {
		t.Fatalf("CreateSignedMessage failed: %v", err)
	}

	if msg == nil {
		t.Fatal("Expected non-nil message")
	}

	if GetSenderPeerID(msg) != identity.PeerID() {
		t.Errorf("Sender mismatch: expected %s, got %s", identity.PeerID(), GetSenderPeerID(msg))
	}

	if string(GetRecipientPeerID(msg)) != recipientID {
		t.Errorf("Recipient mismatch: expected %s, got %s", recipientID, GetRecipientPeerID(msg))
	}

	if len(msg.Signature) == 0 {
		t.Error("Expected non-empty signature")
	}

	// Verify the signature
	if !bridge.VerifyMessage(msg, identity.PublicKey()) {
		t.Error("Message signature verification failed")
	}
}

func TestMessageBridge_VerifyInvalidSignature(t *testing.T) {
	// Create two different identities
	identity1, _ := NewIdentityBridge(nil)
	identity2, _ := NewIdentityBridge(nil)

	bridge := NewMessageBridge(identity1)

	msg, _ := bridge.CreateSignedMessage(int(p2p.MessageTypeOrderOpen), "recipient", []byte("test"))

	// Verify with wrong public key should fail
	if bridge.VerifyMessage(msg, identity2.PublicKey()) {
		t.Error("Verification should fail with wrong public key")
	}
}

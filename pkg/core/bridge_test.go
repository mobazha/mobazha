package core

import (
	"testing"

	"github.com/mobazha/mobazha-core/contracts"
	"github.com/mobazha/mobazha-core/identity"
	"github.com/mobazha/mobazha-core/orders"
	"github.com/mobazha/mobazha-core/p2p"
)

func newTestSigner(t *testing.T) *contracts.KeyPairSigner {
	t.Helper()
	kp, err := identity.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	pid, err := identity.PeerIDFromPublicKey(kp.PubKey)
	if err != nil {
		t.Fatalf("PeerIDFromPublicKey: %v", err)
	}
	return contracts.NewKeyPairSigner(kp, pid)
}

func TestOrderStateBridge_ValidTransition(t *testing.T) {
	bridge := NewOrderStateBridge()

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

	_, valid := bridge.ValidateTransition(int(orders.StateCompleted), int(orders.EventPaymentSent))
	if valid {
		t.Error("Expected invalid transition from Completed on PaymentSent")
	}
}

func TestOrderStateBridge_AllowedEvents(t *testing.T) {
	bridge := NewOrderStateBridge()

	allowed := bridge.GetAllowedEvents(int(orders.StatePending))
	if len(allowed) == 0 {
		t.Error("Expected some allowed events for Pending state")
	}

	t.Logf("Allowed events for Pending: %v", allowed)
}

func TestMessageBridge_CreateSignedMessage(t *testing.T) {
	signer := newTestSigner(t)
	bridge := NewMessageBridge(signer)

	recipientID := "12D3KooWTestRecipient"
	payload := []byte(`{"action": "test"}`)

	msg, err := bridge.CreateSignedMessage(int(p2p.MessageTypeOrderOpen), recipientID, payload)
	if err != nil {
		t.Fatalf("CreateSignedMessage failed: %v", err)
	}

	if msg == nil {
		t.Fatal("Expected non-nil message")
	}

	if GetSenderPeerID(msg) != signer.PeerID() {
		t.Errorf("Sender mismatch: expected %s, got %s", signer.PeerID(), GetSenderPeerID(msg))
	}

	if string(GetRecipientPeerID(msg)) != recipientID {
		t.Errorf("Recipient mismatch: expected %s, got %s", recipientID, GetRecipientPeerID(msg))
	}

	if len(msg.Signature) == 0 {
		t.Error("Expected non-empty signature")
	}

	// Verify the signature
	pubKey, err := signer.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey: %v", err)
	}
	if !bridge.VerifyMessage(msg, pubKey) {
		t.Error("Message signature verification failed")
	}
}

func TestMessageBridge_VerifyInvalidSignature(t *testing.T) {
	signer1 := newTestSigner(t)
	signer2 := newTestSigner(t)

	bridge := NewMessageBridge(signer1)

	msg, err := bridge.CreateSignedMessage(int(p2p.MessageTypeOrderOpen), "recipient", []byte("test"))
	if err != nil {
		t.Fatalf("CreateSignedMessage: %v", err)
	}

	// Verify with wrong public key should fail
	wrongPubKey, err := signer2.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey: %v", err)
	}
	if bridge.VerifyMessage(msg, wrongPubKey) {
		t.Error("Verification should fail with wrong public key")
	}
}

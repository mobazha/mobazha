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

func TestOrderStateBridge_HappyPath(t *testing.T) {
	bridge := NewOrderStateBridge()

	// AWAITING_PAYMENT + PaymentSent → PENDING
	newState, valid := bridge.ValidateTransition(
		int(orders.StateAwaitingPayment), int(orders.EventPaymentSent))
	if !valid {
		t.Fatal("Expected valid transition AwaitingPayment + PaymentSent")
	}
	if newState != int(orders.StatePending) {
		t.Errorf("Expected Pending, got %d", newState)
	}

	// PENDING + VendorConfirm → AWAITING_FULFILLMENT
	newState, valid = bridge.ValidateTransition(
		int(orders.StatePending), int(orders.EventVendorConfirm))
	if !valid {
		t.Fatal("Expected valid transition Pending + VendorConfirm")
	}
	if newState != int(orders.StateAwaitingFulfillment) {
		t.Errorf("Expected AwaitingFulfillment, got %d", newState)
	}

	// AWAITING_FULFILLMENT + OrderFulfilled → FULFILLED
	newState, valid = bridge.ValidateTransition(
		int(orders.StateAwaitingFulfillment), int(orders.EventOrderFulfilled))
	if !valid {
		t.Fatal("Expected valid transition AwaitingFulfillment + OrderFulfilled")
	}
	if newState != int(orders.StateFulfilled) {
		t.Errorf("Expected Fulfilled, got %d", newState)
	}

	// FULFILLED + BuyerComplete → COMPLETED
	newState, valid = bridge.ValidateTransition(
		int(orders.StateFulfilled), int(orders.EventBuyerComplete))
	if !valid {
		t.Fatal("Expected valid transition Fulfilled + BuyerComplete")
	}
	if newState != int(orders.StateCompleted) {
		t.Errorf("Expected Completed, got %d", newState)
	}
}

func TestOrderStateBridge_InvalidTransition(t *testing.T) {
	bridge := NewOrderStateBridge()

	tests := []struct {
		name  string
		state orders.OrderState
		event orders.OrderEvent
	}{
		{"completed cannot transition", orders.StateCompleted, orders.EventPaymentSent},
		{"awaiting payment cannot be confirmed", orders.StateAwaitingPayment, orders.EventVendorConfirm},
		{"canceled cannot transition", orders.StateCanceled, orders.EventPaymentSent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, valid := bridge.ValidateTransition(int(tt.state), int(tt.event))
			if valid {
				t.Errorf("Expected invalid transition %s + %s", tt.state, tt.event)
			}
		})
	}
}

func TestOrderStateBridge_AllowedEvents(t *testing.T) {
	bridge := NewOrderStateBridge()

	// AwaitingPayment allows PaymentSent + OrderTimeout + VendorDecline
	allowed := bridge.GetAllowedEvents(int(orders.StateAwaitingPayment))
	if len(allowed) != 3 {
		t.Errorf("Expected 3 allowed events for AwaitingPayment, got %d: %v", len(allowed), allowed)
	}

	// Pending should have 6 allowed events
	allowed = bridge.GetAllowedEvents(int(orders.StatePending))
	if len(allowed) != 6 {
		t.Errorf("Expected 6 allowed events for Pending, got %d: %v", len(allowed), allowed)
	}

	// Completed (final state) should have no allowed events
	allowed = bridge.GetAllowedEvents(int(orders.StateCompleted))
	if len(allowed) != 0 {
		t.Errorf("Expected 0 allowed events for Completed, got %d: %v", len(allowed), allowed)
	}
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

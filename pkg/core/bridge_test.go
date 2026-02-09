package core

import (
	"testing"

	"github.com/mobazha/mobazha-core/contracts"
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
	if !peerID.IsValid() {
		t.Errorf("Expected valid libp2p PeerID, got: %s", peerID)
	}

	pubKey, err := bridge.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey() error: %v", err)
	}
	if len(pubKey) == 0 {
		t.Error("Expected non-empty public key")
	}

	privKey, err := bridge.RawPrivateKey()
	if err != nil {
		t.Fatalf("RawPrivateKey() error: %v", err)
	}
	if len(privKey) == 0 {
		t.Error("Expected non-empty private key")
	}

	// Verify libp2p key accessors work
	if bridge.Libp2pPrivKey() == nil {
		t.Error("Expected non-nil libp2p private key")
	}
	if bridge.Libp2pPubKey() == nil {
		t.Error("Expected non-nil libp2p public key")
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
	privKey, err := bridge1.RawPrivateKey()
	if err != nil {
		t.Fatalf("RawPrivateKey() error: %v", err)
	}
	bridge2, err := NewIdentityBridge(privKey)
	if err != nil {
		t.Fatalf("Second NewIdentityBridge failed: %v", err)
	}

	// Should have same PeerID
	if bridge1.PeerID() != bridge2.PeerID() {
		t.Errorf("PeerIDs should match: %s != %s", bridge1.PeerID(), bridge2.PeerID())
	}
}

func TestIdentityBridge_FromMarshaledKey(t *testing.T) {
	// Create first bridge
	bridge1, err := NewIdentityBridge(nil)
	if err != nil {
		t.Fatalf("NewIdentityBridge failed: %v", err)
	}

	// Marshal the key (simulating database storage)
	marshaledKey, err := bridge1.KeyPair().MarshalPrivateKey()
	if err != nil {
		t.Fatalf("MarshalPrivateKey() error: %v", err)
	}

	// Create second bridge from marshaled key
	bridge2, err := NewIdentityBridgeFromMarshaledKey(marshaledKey)
	if err != nil {
		t.Fatalf("NewIdentityBridgeFromMarshaledKey failed: %v", err)
	}

	// Should have same PeerID
	if bridge1.PeerID() != bridge2.PeerID() {
		t.Errorf("PeerIDs should match: %s != %s", bridge1.PeerID(), bridge2.PeerID())
	}
}

func TestIdentityBridge_SignerInterface(t *testing.T) {
	bridge, err := NewIdentityBridge(nil)
	if err != nil {
		t.Fatalf("NewIdentityBridge failed: %v", err)
	}

	// Verify IdentityBridge implements contracts.Signer
	var signer contracts.Signer = bridge
	_ = signer

	message := []byte("test message to sign")

	signature, err := bridge.Sign(message)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	valid, err := bridge.Verify(message, signature)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if !valid {
		t.Error("Signature verification failed")
	}

	// Verify with wrong message should fail
	wrongMessage := []byte("different message")
	valid, err = bridge.Verify(wrongMessage, signature)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if valid {
		t.Error("Verification should fail for wrong message")
	}
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
	idBridge, err := NewIdentityBridge(nil)
	if err != nil {
		t.Fatalf("NewIdentityBridge failed: %v", err)
	}

	// MessageBridge accepts contracts.Signer
	bridge := NewMessageBridge(idBridge)

	recipientID := "12D3KooWTestRecipient"
	payload := []byte(`{"action": "test"}`)

	msg, err := bridge.CreateSignedMessage(int(p2p.MessageTypeOrderOpen), recipientID, payload)
	if err != nil {
		t.Fatalf("CreateSignedMessage failed: %v", err)
	}

	if msg == nil {
		t.Fatal("Expected non-nil message")
	}

	if GetSenderPeerID(msg) != idBridge.PeerID() {
		t.Errorf("Sender mismatch: expected %s, got %s", idBridge.PeerID(), GetSenderPeerID(msg))
	}

	if string(GetRecipientPeerID(msg)) != recipientID {
		t.Errorf("Recipient mismatch: expected %s, got %s", recipientID, GetRecipientPeerID(msg))
	}

	if len(msg.Signature) == 0 {
		t.Error("Expected non-empty signature")
	}

	// Verify the signature
	pubKey, _ := idBridge.PublicKey()
	if !bridge.VerifyMessage(msg, pubKey) {
		t.Error("Message signature verification failed")
	}
}

func TestMessageBridge_VerifyInvalidSignature(t *testing.T) {
	identity1, _ := NewIdentityBridge(nil)
	identity2, _ := NewIdentityBridge(nil)

	bridge := NewMessageBridge(identity1)

	msg, _ := bridge.CreateSignedMessage(int(p2p.MessageTypeOrderOpen), "recipient", []byte("test"))

	// Verify with wrong public key should fail
	wrongPubKey, _ := identity2.PublicKey()
	if bridge.VerifyMessage(msg, wrongPubKey) {
		t.Error("Verification should fail with wrong public key")
	}
}

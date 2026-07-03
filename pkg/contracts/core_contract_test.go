// Package contracts provides contract test helpers.
// These tests verify that implementations conform to the contract interfaces.
package contracts

import (
	"context"
	"crypto/ed25519"
	"testing"

	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/orders"
	"github.com/mobazha/mobazha/pkg/p2p"
)

// --- Mock Signer ---

// MockSigner is a test implementation of the Signer contract using real libp2p keys.
type MockSigner struct {
	keyPair *identity.KeyPair
	peerID  identity.PeerID
}

// NewMockSigner creates a new mock signer for testing.
func NewMockSigner() (*MockSigner, error) {
	kp, err := identity.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	pid, err := identity.PeerIDFromPublicKey(kp.PubKey)
	if err != nil {
		return nil, err
	}
	return &MockSigner{keyPair: kp, peerID: pid}, nil
}

func (s *MockSigner) Sign(message []byte) ([]byte, error) {
	return s.keyPair.PrivKey.Sign(message)
}

func (s *MockSigner) Verify(message []byte, signature []byte) (bool, error) {
	return s.keyPair.PubKey.Verify(message, signature)
}

func (s *MockSigner) PublicKey() (ed25519.PublicKey, error) {
	return s.keyPair.RawPublicKey()
}

func (s *MockSigner) PeerID() identity.PeerID {
	return s.peerID
}

// --- Mock OrderProcessor ---

type MockOrderProcessor struct {
	states map[string]orders.OrderState
}

func NewMockOrderProcessor() *MockOrderProcessor {
	return &MockOrderProcessor{
		states: make(map[string]orders.OrderState),
	}
}

func (p *MockOrderProcessor) ProcessEvent(_ context.Context, orderID string, event orders.OrderEvent) (orders.OrderState, error) {
	currentState := p.states[orderID]

	result := orders.Transition(currentState, event)
	if !result.Valid {
		return currentState, result.Error
	}

	p.states[orderID] = result.NewState
	return result.NewState, nil
}

func (p *MockOrderProcessor) GetState(_ context.Context, orderID string) (orders.OrderState, error) {
	state, ok := p.states[orderID]
	if !ok {
		return orders.InitialState(), nil
	}
	return state, nil
}

// CreateOrder initializes a new order with the FSM initial state.
func (p *MockOrderProcessor) CreateOrder(orderID string) {
	p.states[orderID] = orders.InitialState()
}

func (p *MockOrderProcessor) ValidateTransition(_ context.Context, orderID string, event orders.OrderEvent) error {
	currentState, _ := p.GetState(context.TODO(), orderID)
	result := orders.Transition(currentState, event)
	return result.Error
}

// --- Mock MessageSender ---

type MockMessageSender struct {
	sentMessages []*p2p.Message
}

func NewMockMessageSender() *MockMessageSender {
	return &MockMessageSender{
		sentMessages: make([]*p2p.Message, 0),
	}
}

func (s *MockMessageSender) SendMessage(_ context.Context, _ identity.PeerID, msg *p2p.Message) error {
	s.sentMessages = append(s.sentMessages, msg)
	return nil
}

func (s *MockMessageSender) SendReliableMessage(ctx context.Context, recipient identity.PeerID, msg *p2p.Message) error {
	return s.SendMessage(ctx, recipient, msg)
}

// --- Contract Tests ---

func TestSignerContract(t *testing.T) {
	signer, err := NewMockSigner()
	if err != nil {
		t.Fatalf("NewMockSigner() error = %v", err)
	}

	// Verify interface compliance
	var _ Signer = signer

	// Test Sign + Verify roundtrip
	message := []byte("test message for signing")
	sig, err := signer.Sign(message)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	valid, err := signer.Verify(message, sig)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !valid {
		t.Error("Verify() should return true for valid signature")
	}

	// Verify with wrong message should fail
	valid, err = signer.Verify([]byte("wrong message"), sig)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if valid {
		t.Error("Verify() should return false for wrong message")
	}

	// Test PublicKey
	pubKey, err := signer.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey() error = %v", err)
	}
	if len(pubKey) == 0 {
		t.Error("PublicKey() should not be empty")
	}

	// Test PeerID is a valid libp2p peer ID
	peerID := signer.PeerID()
	if !peerID.IsValid() {
		t.Errorf("PeerID() returned invalid peer ID: %s", peerID)
	}
}

func TestOrderProcessorContract(t *testing.T) {
	processor := NewMockOrderProcessor()
	ctx := context.Background()

	// Verify interface compliance
	var _ OrderProcessor = processor

	orderID := "test-order-123"
	processor.CreateOrder(orderID)

	// Initial state should be AwaitingPayment
	state, err := processor.GetState(ctx, orderID)
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}
	if state != orders.StateAwaitingPayment {
		t.Errorf("Initial state = %v, want %v", state, orders.StateAwaitingPayment)
	}

	// Process PaymentSent event: AWAITING_PAYMENT → AWAITING_PAYMENT_VERIFICATION
	newState, err := processor.ProcessEvent(ctx, orderID, orders.EventPaymentSent)
	if err != nil {
		t.Fatalf("ProcessEvent(PaymentSent) error = %v", err)
	}
	if newState != orders.StateAwaitingPaymentVerification {
		t.Errorf("State after PaymentSent = %v, want %v", newState, orders.StateAwaitingPaymentVerification)
	}

	// Process PaymentVerified event: AWAITING_PAYMENT_VERIFICATION → PENDING
	newState, err = processor.ProcessEvent(ctx, orderID, orders.EventPaymentVerified)
	if err != nil {
		t.Fatalf("ProcessEvent(PaymentVerified) error = %v", err)
	}
	if newState != orders.StatePending {
		t.Errorf("State after PaymentVerified = %v, want %v", newState, orders.StatePending)
	}

	// Process VendorConfirm: PENDING → AWAITING_SHIPMENT
	newState, err = processor.ProcessEvent(ctx, orderID, orders.EventVendorConfirm)
	if err != nil {
		t.Fatalf("ProcessEvent(VendorConfirm) error = %v", err)
	}
	if newState != orders.StateAwaitingShipment {
		t.Errorf("State after VendorConfirm = %v, want %v", newState, orders.StateAwaitingShipment)
	}

	// Validate an invalid transition: can't send payment from AwaitingFulfillment
	err = processor.ValidateTransition(ctx, orderID, orders.EventPaymentSent)
	if err == nil {
		t.Error("ValidateTransition() should fail for invalid transition")
	}
}

func TestMessageSenderContract(t *testing.T) {
	sender := NewMockMessageSender()
	ctx := context.Background()
	recipient := identity.PeerID("12D3KooWTestRecipient")

	// Verify interface compliance
	var _ MessageSender = sender

	msg := p2p.NewMessage(
		p2p.MessageTypeOrderOpen,
		"sender-peer",
		"recipient-peer",
		[]byte("test payload"),
	)

	// Test SendMessage
	err := sender.SendMessage(ctx, recipient, msg)
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if len(sender.sentMessages) != 1 {
		t.Errorf("sentMessages count = %d, want 1", len(sender.sentMessages))
	}

	// Test SendReliableMessage
	err = sender.SendReliableMessage(ctx, recipient, msg)
	if err != nil {
		t.Fatalf("SendReliableMessage() error = %v", err)
	}
	if len(sender.sentMessages) != 2 {
		t.Errorf("sentMessages count = %d, want 2", len(sender.sentMessages))
	}
}

func TestContractConsistency(t *testing.T) {
	// Create mock implementations
	signer, err := NewMockSigner()
	if err != nil {
		t.Fatalf("NewMockSigner() error = %v", err)
	}
	processor := NewMockOrderProcessor()
	sender := NewMockMessageSender()
	ctx := context.Background()

	// Simulate a complete order flow
	orderID := "order-456"
	vendorPeerID := identity.PeerID("12D3KooWVendor")
	processor.CreateOrder(orderID)

	// 1. Create order message
	msg := p2p.NewMessage(
		p2p.MessageTypeOrderOpen,
		signer.PeerID(),
		vendorPeerID,
		[]byte(`{"orderID": "order-456"}`),
	)

	// 2. Sign the message payload
	sig, err := signer.Sign(msg.SignableBytes())
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	msg.Signature = sig

	// 3. Send the message
	err = sender.SendMessage(ctx, vendorPeerID, msg)
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	// 4. Process PaymentSent: AWAITING_PAYMENT → AWAITING_PAYMENT_VERIFICATION
	_, err = processor.ProcessEvent(ctx, orderID, orders.EventPaymentSent)
	if err != nil {
		t.Fatalf("ProcessEvent(PaymentSent) error = %v", err)
	}

	// 4.1 Process PaymentVerified: AWAITING_PAYMENT_VERIFICATION → PENDING
	_, err = processor.ProcessEvent(ctx, orderID, orders.EventPaymentVerified)
	if err != nil {
		t.Fatalf("ProcessEvent(PaymentVerified) error = %v", err)
	}

	// 5. Verify signature is valid
	valid, err := signer.Verify(msg.SignableBytes(), msg.Signature)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !valid {
		t.Error("Message signature should be valid")
	}

	// 6. Verify order state is now PENDING
	state, _ := processor.GetState(ctx, orderID)
	if state != orders.StatePending {
		t.Errorf("Final state = %v, want %v", state, orders.StatePending)
	}
}

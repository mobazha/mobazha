package orders

import (
	"testing"

	coreorders "github.com/mobazha/mobazha-core/orders"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
)

func TestMessageTypeToEvent_AllMappings(t *testing.T) {
	tests := []struct {
		name         string
		msgType      npb.OrderMessage_MessageType
		senderPeerID string
		wantEvent    coreorders.OrderEvent
		isTransition bool
	}{
		{
			name:         "ORDER_OPEN -> no event (order creation)",
			msgType:      npb.OrderMessage_ORDER_OPEN,
			wantEvent:    coreorders.EventUnknown,
			isTransition: false,
		},
		{
			name:         "PAYMENT_SENT -> EventPaymentSent",
			msgType:      npb.OrderMessage_PAYMENT_SENT,
			wantEvent:    coreorders.EventPaymentSent,
			isTransition: true,
		},
		{
			name:         "ORDER_CONFIRMATION -> EventVendorConfirm",
			msgType:      npb.OrderMessage_ORDER_CONFIRMATION,
			wantEvent:    coreorders.EventVendorConfirm,
			isTransition: true,
		},
		{
			name:         "ORDER_DECLINE -> EventVendorDecline",
			msgType:      npb.OrderMessage_ORDER_DECLINE,
			wantEvent:    coreorders.EventVendorDecline,
			isTransition: true,
		},
		{
			name:         "ORDER_CANCEL (nil order) -> EventUnknown",
			msgType:      npb.OrderMessage_ORDER_CANCEL,
			senderPeerID: "QmAnyone",
			wantEvent:    coreorders.EventUnknown,
			isTransition: true,
		},
		{
			name:         "REFUND -> EventRefundIssued",
			msgType:      npb.OrderMessage_REFUND,
			wantEvent:    coreorders.EventRefundIssued,
			isTransition: true,
		},
		{
			name:         "ORDER_FULFILLMENT -> EventOrderFulfilled",
			msgType:      npb.OrderMessage_ORDER_FULFILLMENT,
			wantEvent:    coreorders.EventOrderFulfilled,
			isTransition: true,
		},
		{
			name:         "ORDER_COMPLETE -> EventBuyerComplete",
			msgType:      npb.OrderMessage_ORDER_COMPLETE,
			wantEvent:    coreorders.EventBuyerComplete,
			isTransition: true,
		},
		{
			name:         "DISPUTE_OPEN -> EventDisputeOpened",
			msgType:      npb.OrderMessage_DISPUTE_OPEN,
			wantEvent:    coreorders.EventDisputeOpened,
			isTransition: true,
		},
		{
			name:         "DISPUTE_CLOSE -> EventModeratorDecide",
			msgType:      npb.OrderMessage_DISPUTE_CLOSE,
			wantEvent:    coreorders.EventModeratorDecide,
			isTransition: true,
		},
		{
			name:         "DISPUTE_ACCEPT -> EventDisputeResolved",
			msgType:      npb.OrderMessage_DISPUTE_ACCEPT,
			wantEvent:    coreorders.EventDisputeResolved,
			isTransition: true,
		},
		{
			name:         "PAYMENT_FINALIZED -> EventPaymentFinalize",
			msgType:      npb.OrderMessage_PAYMENT_FINALIZED,
			wantEvent:    coreorders.EventPaymentFinalize,
			isTransition: true,
		},
		{
			name:         "RATING_SIGNATURES -> no event (utility message)",
			msgType:      npb.OrderMessage_RATING_SIGNATURES,
			wantEvent:    coreorders.EventUnknown,
			isTransition: false,
		},
		{
			name:         "DISPUTE_UPDATE -> no event",
			msgType:      npb.OrderMessage_DISPUTE_UPDATE,
			wantEvent:    coreorders.EventUnknown,
			isTransition: false,
		},
		{
			name:         "PAYMENT_LOCKED -> no event (RWA)",
			msgType:      npb.OrderMessage_PAYMENT_LOCKED,
			wantEvent:    coreorders.EventUnknown,
			isTransition: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// order=nil for most tests; cancel-specific tests are separate.
			got := MessageTypeToEvent(tt.msgType, nil, tt.senderPeerID)
			if got != tt.wantEvent {
				t.Errorf("MessageTypeToEvent(%v) = %v, want %v", tt.msgType, got, tt.wantEvent)
			}

			isTransition := IsStateTransitionMessage(tt.msgType)
			if isTransition != tt.isTransition {
				t.Errorf("IsStateTransitionMessage(%v) = %v, want %v",
					tt.msgType, isTransition, tt.isTransition)
			}
		})
	}
}

// TestMapCancelEvent_NilOrder verifies that nil order returns EventUnknown.
func TestMapCancelEvent_NilOrder(t *testing.T) {
	got := mapCancelEvent(nil, "QmAnyone")
	if got != coreorders.EventUnknown {
		t.Errorf("mapCancelEvent(nil, ...) = %v, want EventUnknown", got)
	}
}

// TestMapCancelEvent_NonVendorSender verifies that a non-vendor sender results in buyer cancel.
func TestMapCancelEvent_NonVendorSender(t *testing.T) {
	// An empty order can't resolve vendor, so it falls through to buyer cancel.
	order := &models.Order{}
	got := mapCancelEvent(order, "QmBuyer")
	if got != coreorders.EventBuyerCancel {
		t.Errorf("mapCancelEvent(emptyOrder, QmBuyer) = %v, want EventBuyerCancel", got)
	}
}

// TestCoreTransitionTable_HappyPath verifies the canonical order flow through the FSM.
func TestCoreTransitionTable_HappyPath(t *testing.T) {
	steps := []struct {
		name  string
		from  coreorders.OrderState
		event coreorders.OrderEvent
		to    coreorders.OrderState
	}{
		{"buyer pays", coreorders.StateAwaitingPayment, coreorders.EventPaymentSent, coreorders.StatePending},
		{"vendor confirms", coreorders.StatePending, coreorders.EventVendorConfirm, coreorders.StateAwaitingFulfillment},
		{"vendor fulfills", coreorders.StateAwaitingFulfillment, coreorders.EventOrderFulfilled, coreorders.StateFulfilled},
		{"buyer completes", coreorders.StateFulfilled, coreorders.EventBuyerComplete, coreorders.StateCompleted},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			result := coreorders.Transition(step.from, step.event)
			if !result.Valid {
				t.Fatalf("expected valid transition %s + %s, got: %v", step.from, step.event, result.Error)
			}
			if result.NewState != step.to {
				t.Errorf("expected %s, got %s", step.to, result.NewState)
			}
		})
	}
}

// TestCoreTransitionTable_DisputeFlow verifies the dispute resolution flow.
func TestCoreTransitionTable_DisputeFlow(t *testing.T) {
	steps := []struct {
		name  string
		from  coreorders.OrderState
		event coreorders.OrderEvent
		to    coreorders.OrderState
	}{
		{"dispute opened from pending", coreorders.StatePending, coreorders.EventDisputeOpened, coreorders.StateDisputed},
		{"moderator decides", coreorders.StateDisputed, coreorders.EventModeratorDecide, coreorders.StateDecided},
		{"dispute resolved", coreorders.StateDecided, coreorders.EventDisputeResolved, coreorders.StateResolved},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			result := coreorders.Transition(step.from, step.event)
			if !result.Valid {
				t.Fatalf("expected valid transition, got: %v", result.Error)
			}
			if result.NewState != step.to {
				t.Errorf("expected %s, got %s", step.to, result.NewState)
			}
		})
	}
}

// TestCoreTransitionTable_InvalidTransitions verifies some impossible paths.
func TestCoreTransitionTable_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name  string
		from  coreorders.OrderState
		event coreorders.OrderEvent
	}{
		{"fulfill from awaiting payment", coreorders.StateAwaitingPayment, coreorders.EventOrderFulfilled},
		{"complete from awaiting payment", coreorders.StateAwaitingPayment, coreorders.EventBuyerComplete},
		{"event on completed", coreorders.StateCompleted, coreorders.EventPaymentSent},
		{"event on canceled", coreorders.StateCanceled, coreorders.EventPaymentSent},
		{"vendor confirm from awaiting payment", coreorders.StateAwaitingPayment, coreorders.EventVendorConfirm},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coreorders.Transition(tt.from, tt.event)
			if result.Valid {
				t.Errorf("expected invalid transition %s + %s", tt.from, tt.event)
			}
		})
	}
}

// TestStateValidator_Interface verifies the StateValidator interface works with the core bridge.
func TestStateValidator_Interface(t *testing.T) {
	bridge := &mockStateBridge{}

	// Happy path: AWAITING_PAYMENT + PaymentSent → PENDING
	newState, valid := bridge.ValidateTransition(
		int(coreorders.StateAwaitingPayment),
		int(coreorders.EventPaymentSent),
	)
	if !valid {
		t.Fatal("expected valid transition")
	}
	if coreorders.OrderState(newState) != coreorders.StatePending {
		t.Errorf("expected StatePending, got %v", coreorders.OrderState(newState))
	}

	// Invalid: AWAITING_PAYMENT + VendorConfirm
	_, valid = bridge.ValidateTransition(
		int(coreorders.StateAwaitingPayment),
		int(coreorders.EventVendorConfirm),
	)
	if valid {
		t.Error("expected invalid transition")
	}
}

// mockStateBridge mirrors the real bridge for testing.
type mockStateBridge struct{}

func (b *mockStateBridge) ValidateTransition(currentState, event int) (int, bool) {
	result := coreorders.Transition(coreorders.OrderState(currentState), coreorders.OrderEvent(event))
	return int(result.NewState), result.Valid
}

func (b *mockStateBridge) GetAllowedEvents(state int) []int {
	allowed := coreorders.AllowedEvents(coreorders.OrderState(state))
	result := make([]int, len(allowed))
	for i, e := range allowed {
		result[i] = int(e)
	}
	return result
}

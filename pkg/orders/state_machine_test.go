package orders

import (
	"testing"
)

func TestInitialState(t *testing.T) {
	if got := InitialState(); got != StateAwaitingPayment {
		t.Errorf("InitialState() = %v, want StateAwaitingPayment", got)
	}
}

func TestTransition_HappyPath(t *testing.T) {
	// Full happy path:
	// AWAITING_PAYMENT → AWAITING_PAYMENT_VERIFICATION → PENDING
	// → AWAITING_SHIPMENT → SHIPPED → COMPLETED
	steps := []struct {
		name      string
		fromState OrderState
		event     OrderEvent
		toState   OrderState
	}{
		{"buyer submits payment", StateAwaitingPayment, EventPaymentSent, StateAwaitingPaymentVerification},
		{"payment verified", StateAwaitingPaymentVerification, EventPaymentVerified, StatePending},
		{"vendor confirms", StatePending, EventVendorConfirm, StateAwaitingShipment},
		{"vendor fulfills", StateAwaitingShipment, EventOrderShipped, StateShipped},
		{"buyer completes", StateShipped, EventBuyerComplete, StateCompleted},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			result := Transition(step.fromState, step.event)
			if !result.Valid {
				t.Fatalf("expected valid transition %s + %s, got error: %v",
					step.fromState, step.event, result.Error)
			}
			if result.NewState != step.toState {
				t.Errorf("expected %s, got %s", step.toState, result.NewState)
			}
		})
	}
}

func TestTransition_DisputeFlow(t *testing.T) {
	// Dispute flow: AWAITING_SHIPMENT → DISPUTED → DECIDED → RESOLVED
	steps := []struct {
		name      string
		fromState OrderState
		event     OrderEvent
		toState   OrderState
	}{
		{"dispute opened", StateAwaitingShipment, EventDisputeOpened, StateDisputed},
		{"moderator decides", StateDisputed, EventModeratorDecide, StateDecided},
		{"dispute resolved", StateDecided, EventDisputeResolved, StateResolved},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			result := Transition(step.fromState, step.event)
			if !result.Valid {
				t.Fatalf("expected valid transition, got error: %v", result.Error)
			}
			if result.NewState != step.toState {
				t.Errorf("expected %s, got %s", step.toState, result.NewState)
			}
		})
	}
}

func TestTransition_ValidTransitions(t *testing.T) {
	tests := []struct {
		name         string
		currentState OrderState
		event        OrderEvent
		wantState    OrderState
	}{
		// AwaitingPayment (initial state)
		{
			name:         "awaiting payment to awaiting verification",
			currentState: StateAwaitingPayment,
			event:        EventPaymentSent,
			wantState:    StateAwaitingPaymentVerification,
		},
		{
			name:         "awaiting payment timeout to canceled",
			currentState: StateAwaitingPayment,
			event:        EventOrderTimeout,
			wantState:    StateCanceled,
		},
		{
			name:         "awaiting payment to declined (vendor declines unfunded)",
			currentState: StateAwaitingPayment,
			event:        EventVendorDecline,
			wantState:    StateDeclined,
		},
		// Awaiting payment verification transitions
		{
			name:         "awaiting verification to pending",
			currentState: StateAwaitingPaymentVerification,
			event:        EventPaymentVerified,
			wantState:    StatePending,
		},
		{
			name:         "awaiting verification to declined",
			currentState: StateAwaitingPaymentVerification,
			event:        EventVendorDecline,
			wantState:    StateDeclined,
		},
		// Pending state transitions
		{
			name:         "pending to awaiting fulfillment (vendor confirms)",
			currentState: StatePending,
			event:        EventVendorConfirm,
			wantState:    StateAwaitingShipment,
		},
		{
			name:         "pending to declined (vendor declines)",
			currentState: StatePending,
			event:        EventVendorDecline,
			wantState:    StateDeclined,
		},
		{
			name:         "pending to canceled (vendor)",
			currentState: StatePending,
			event:        EventVendorCancel,
			wantState:    StateCanceled,
		},
		{
			name:         "pending to canceled (buyer)",
			currentState: StatePending,
			event:        EventBuyerCancel,
			wantState:    StateCanceled,
		},
		{
			name:         "pending to refunded",
			currentState: StatePending,
			event:        EventRefundIssued,
			wantState:    StateRefunded,
		},
		{
			name:         "pending to disputed",
			currentState: StatePending,
			event:        EventDisputeOpened,
			wantState:    StateDisputed,
		},
		// Awaiting fulfillment transitions
		{
			name:         "awaiting fulfillment to fulfilled",
			currentState: StateAwaitingShipment,
			event:        EventOrderShipped,
			wantState:    StateShipped,
		},
		{
			name:         "awaiting fulfillment to partially fulfilled",
			currentState: StateAwaitingShipment,
			event:        EventPartialShip,
			wantState:    StatePartiallyShipped,
		},
		{
			name:         "awaiting fulfillment to refunded",
			currentState: StateAwaitingShipment,
			event:        EventRefundIssued,
			wantState:    StateRefunded,
		},
		{
			name:         "awaiting fulfillment to disputed",
			currentState: StateAwaitingShipment,
			event:        EventDisputeOpened,
			wantState:    StateDisputed,
		},
		// Fulfilled transitions
		{
			name:         "fulfilled to completed",
			currentState: StateShipped,
			event:        EventBuyerComplete,
			wantState:    StateCompleted,
		},
		{
			name:         "fulfilled to disputed",
			currentState: StateShipped,
			event:        EventDisputeOpened,
			wantState:    StateDisputed,
		},
		// Partially fulfilled transitions
		{
			name:         "partially fulfilled to fulfilled",
			currentState: StatePartiallyShipped,
			event:        EventOrderShipped,
			wantState:    StateShipped,
		},
		{
			name:         "partially fulfilled to disputed",
			currentState: StatePartiallyShipped,
			event:        EventDisputeOpened,
			wantState:    StateDisputed,
		},
		// Awaiting pickup transitions
		{
			name:         "awaiting pickup to completed",
			currentState: StateAwaitingPickup,
			event:        EventBuyerComplete,
			wantState:    StateCompleted,
		},
		// Disputed transitions
		{
			name:         "disputed to decided",
			currentState: StateDisputed,
			event:        EventModeratorDecide,
			wantState:    StateDecided,
		},
		{
			name:         "disputed to resolved",
			currentState: StateDisputed,
			event:        EventDisputeResolved,
			wantState:    StateResolved,
		},
		{
			name:         "disputed to payment finalized",
			currentState: StateDisputed,
			event:        EventPaymentFinalize,
			wantState:    StatePaymentFinalized,
		},
		// Decided transitions
		{
			name:         "decided to resolved",
			currentState: StateDecided,
			event:        EventDisputeResolved,
			wantState:    StateResolved,
		},
		{
			name:         "decided to payment finalized",
			currentState: StateDecided,
			event:        EventPaymentFinalize,
			wantState:    StatePaymentFinalized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Transition(tt.currentState, tt.event)
			if !result.Valid {
				t.Errorf("Transition(%s, %s) should be valid, got error: %v",
					tt.currentState, tt.event, result.Error)
			}
			if result.NewState != tt.wantState {
				t.Errorf("Transition(%s, %s).NewState = %s, want %s",
					tt.currentState, tt.event, result.NewState, tt.wantState)
			}
		})
	}
}

func TestTransition_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name         string
		currentState OrderState
		event        OrderEvent
	}{
		// Final states cannot transition
		{"completed cannot transition", StateCompleted, EventBuyerComplete},
		{"canceled cannot transition", StateCanceled, EventPaymentSent},
		{"declined cannot transition", StateDeclined, EventPaymentSent},
		{"refunded cannot transition", StateRefunded, EventPaymentSent},
		{"resolved cannot transition", StateResolved, EventPaymentSent},

		// Invalid events for states
		{"awaiting payment cannot be fulfilled", StateAwaitingPayment, EventOrderShipped},
		{"awaiting payment cannot be confirmed", StateAwaitingPayment, EventVendorConfirm},
		{"awaiting verification cannot be confirmed", StateAwaitingPaymentVerification, EventVendorConfirm},
		{"pending cannot be fulfilled", StatePending, EventOrderShipped},
		{"awaiting fulfillment cannot be completed", StateAwaitingShipment, EventBuyerComplete},
		{"fulfilled cannot be refunded directly", StateShipped, EventRefundIssued},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Transition(tt.currentState, tt.event)
			if result.Valid {
				t.Errorf("Transition(%s, %s) should be invalid", tt.currentState, tt.event)
			}
			if result.Error == nil {
				t.Error("expected error for invalid transition")
			}
		})
	}
}

func TestIsFinalState(t *testing.T) {
	tests := []struct {
		state OrderState
		want  bool
	}{
		{StateCompleted, true},
		{StateCanceled, true},
		{StateDeclined, true},
		{StateRefunded, true},
		{StateResolved, true},
		{StatePaymentFinalized, true},
		{StateProcessingError, true},
		{StatePending, false},
		{StateAwaitingPayment, false},
		{StateAwaitingPaymentVerification, false},
		{StateAwaitingPickup, false},
		{StateAwaitingShipment, false},
		{StatePartiallyShipped, false},
		{StateShipped, false},
		{StateDisputed, false},
		{StateDecided, false},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			if got := IsFinalState(tt.state); got != tt.want {
				t.Errorf("IsFinalState(%v) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestAllowedEvents(t *testing.T) {
	tests := []struct {
		state     OrderState
		wantCount int
	}{
		{StateAwaitingPayment, 3},             // PaymentSent, OrderTimeout, VendorDecline
		{StateAwaitingPaymentVerification, 2}, // PaymentVerified, VendorDecline
		{StatePending, 6},                     // VendorConfirm, VendorDecline, VendorCancel, BuyerCancel, Refund, Dispute
		{StateAwaitingShipment, 5},          // Shipped, PartialShip, FulfillmentStarted, Refund, Dispute
		{StatePartiallyShipped, 3},          // Shipped, Refund, Dispute
		{StateAwaitingFulfillment, 4},       // FulfillmentPartial, FulfillmentComplete, Refund, Dispute
		{StatePartiallyFulfilled, 3},        // FulfillmentComplete, Refund, Dispute
		{StateFulfilled, 3},                 // Shipped, Refund, Dispute
		{StateAwaitingPickup, 3},            // BuyerComplete, Refund, Dispute
		{StateShipped, 2},                   // BuyerComplete, Dispute
		{StateDisputed, 3},                  // ModeratorDecide, DisputeResolved, PaymentFinalize
		{StateDecided, 2},                   // DisputeResolved, PaymentFinalize
		// Final states: no allowed events
		{StateCompleted, 0},
		{StateCanceled, 0},
		{StateDeclined, 0},
		{StateRefunded, 0},
		{StateResolved, 0},
		{StatePaymentFinalized, 0},
		{StateProcessingError, 0},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			events := AllowedEvents(tt.state)
			if len(events) != tt.wantCount {
				t.Errorf("AllowedEvents(%s) returned %d events, want %d",
					tt.state, len(events), tt.wantCount)
			}

			// Every allowed event must produce a valid transition
			for _, event := range events {
				result := Transition(tt.state, event)
				if !result.Valid {
					t.Errorf("AllowedEvents(%s) includes %s, but Transition rejects it",
						tt.state, event)
				}
			}
		})
	}
}

func TestAllowedEvents_ConsistentWithTransition(t *testing.T) {
	nonFinalStates := []OrderState{
		StateAwaitingPayment, StateAwaitingPaymentVerification, StatePending, StateAwaitingShipment,
		StatePartiallyShipped, StateAwaitingFulfillment, StatePartiallyFulfilled, StateFulfilled,
		StateAwaitingPickup, StateShipped,
		StateDisputed, StateDecided,
	}

	allEvents := []OrderEvent{
		EventPaymentSent, EventPaymentVerified, EventVendorConfirm, EventPartialShip,
		EventOrderShipped, EventBuyerComplete, EventVendorCancel,
		EventVendorDecline, EventBuyerCancel, EventRefundIssued,
		EventFulfillmentStarted, EventFulfillmentPartial, EventFulfillmentComplete,
		EventDisputeOpened, EventModeratorDecide, EventDisputeResolved,
		EventPaymentFinalize, EventOrderTimeout,
	}

	for _, state := range nonFinalStates {
		allowed := AllowedEvents(state)
		allowedSet := make(map[OrderEvent]bool, len(allowed))
		for _, e := range allowed {
			allowedSet[e] = true
		}

		for _, event := range allEvents {
			result := Transition(state, event)
			inAllowed := allowedSet[event]

			if result.Valid && !inAllowed {
				t.Errorf("state %s: Transition accepts %s but AllowedEvents omits it",
					state, event)
			}
			if !result.Valid && inAllowed {
				t.Errorf("state %s: AllowedEvents includes %s but Transition rejects it",
					state, event)
			}
		}
	}
}

func TestOrderState_String(t *testing.T) {
	tests := []struct {
		state OrderState
		want  string
	}{
		{StatePending, "PENDING"},
		{StateAwaitingPayment, "AWAITING_PAYMENT"},
		{StateAwaitingPaymentVerification, "AWAITING_PAYMENT_VERIFICATION"},
		{StateAwaitingPickup, "AWAITING_PICKUP"},
		{StateAwaitingShipment, "AWAITING_SHIPMENT"},
		{StatePartiallyShipped, "PARTIALLY_SHIPPED"},
		{StateShipped, "SHIPPED"},
		{StateCompleted, "COMPLETED"},
		{StateCanceled, "CANCELED"},
		{StateDeclined, "DECLINED"},
		{StateRefunded, "REFUNDED"},
		{StateDisputed, "DISPUTED"},
		{StateDecided, "DECIDED"},
		{StateResolved, "RESOLVED"},
		{StatePaymentFinalized, "PAYMENT_FINALIZED"},
		{StateProcessingError, "PROCESSING_ERROR"},
		{OrderState(-1), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("OrderState.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrderEvent_String(t *testing.T) {
	tests := []struct {
		event OrderEvent
		want  string
	}{
		{EventPaymentSent, "PAYMENT_SENT"},
		{EventPaymentVerified, "PAYMENT_VERIFIED"},
		{EventVendorConfirm, "VENDOR_CONFIRM"},
		{EventPartialShip, "PARTIAL_SHIP"},
		{EventOrderShipped, "ORDER_SHIPPED"},
		{EventBuyerComplete, "BUYER_COMPLETE"},
		{EventVendorCancel, "VENDOR_CANCEL"},
		{EventVendorDecline, "VENDOR_DECLINE"},
		{EventBuyerCancel, "BUYER_CANCEL"},
		{EventRefundIssued, "REFUND_ISSUED"},
		{EventDisputeOpened, "DISPUTE_OPENED"},
		{EventModeratorDecide, "MODERATOR_DECIDE"},
		{EventDisputeResolved, "DISPUTE_RESOLVED"},
		{EventPaymentFinalize, "PAYMENT_FINALIZE"},
		{EventOrderTimeout, "ORDER_TIMEOUT"},
		{EventUnknown, "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.event.String(); got != tt.want {
				t.Errorf("OrderEvent.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

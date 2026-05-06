// Package orders implements the order state machine for Mobazha.
//
// The state machine models the lifecycle of an order from creation to completion.
// State transitions are driven by domain events (buyer/vendor/moderator actions).
//
// Canonical order flow:
//
//	AWAITING_PAYMENT → PaymentSent → AWAITING_PAYMENT_VERIFICATION
//	  → PaymentVerified → PENDING → VendorConfirm → AWAITING_SHIPMENT
//	  → OrderShipped → SHIPPED → BuyerComplete → COMPLETED
package orders

import "fmt"

// OrderState represents the current state of an order.
// Values match mobazha3.0/pkg/models/order_state.go for direct conversion.
type OrderState int32

const (
	// StatePending indicates payment has been sent/funded, waiting for vendor to confirm or decline.
	StatePending OrderState = 0
	// StateAwaitingPayment is the initial state — order created, waiting for buyer to send payment.
	StateAwaitingPayment OrderState = 1
	// StateAwaitingPaymentVerification indicates buyer submitted payment and
	// the system is waiting for chain/provider verification.
	StateAwaitingPaymentVerification OrderState = 2
	// StateAwaitingPickup indicates the order is ready for local pickup.
	StateAwaitingPickup OrderState = 3
	// StateAwaitingShipment indicates vendor confirmed, waiting for shipment.
	StateAwaitingShipment OrderState = 4
	// StatePartiallyShipped indicates some items have been shipped.
	StatePartiallyShipped OrderState = 5
	// StateShipped indicates the order has been shipped/delivered.
	StateShipped OrderState = 6
	// StateCompleted indicates the order is complete (buyer confirmed receipt).
	StateCompleted OrderState = 7
	// StateCanceled indicates the order was canceled.
	StateCanceled OrderState = 8
	// StateDeclined indicates the vendor declined (rejected) the order.
	StateDeclined OrderState = 9
	// StateRefunded indicates the order was refunded.
	StateRefunded OrderState = 10
	// StateDisputed indicates the order is in dispute.
	StateDisputed OrderState = 11
	// StateDecided indicates a moderator has decided a dispute.
	StateDecided OrderState = 12
	// StateResolved indicates a dispute has been resolved (accepted by parties).
	StateResolved OrderState = 13
	// StatePaymentFinalized indicates escrow payout finalized after timeout.
	StatePaymentFinalized OrderState = 14
	// StateProcessingError indicates an unrecoverable processing error.
	StateProcessingError OrderState = 15
	// StateAwaitingFulfillment indicates the order is confirmed and waiting
	// for a fulfillment provider (e.g., Printful) to process it.
	StateAwaitingFulfillment OrderState = 16
	// StatePartiallyFulfilled indicates some items have been fulfilled by
	// the supply chain provider but others are still pending.
	StatePartiallyFulfilled OrderState = 17
	// StateFulfilled indicates all items have been fulfilled and shipped
	// by the supply chain provider.
	StateFulfilled OrderState = 18
)

// InitialState returns the state assigned to a newly created order.
func InitialState() OrderState {
	return StateAwaitingPayment
}

// String returns the string representation of an OrderState.
func (s OrderState) String() string {
	switch s {
	case StatePending:
		return "PENDING"
	case StateAwaitingPayment:
		return "AWAITING_PAYMENT"
	case StateAwaitingPaymentVerification:
		return "AWAITING_PAYMENT_VERIFICATION"
	case StateAwaitingPickup:
		return "AWAITING_PICKUP"
	case StateAwaitingShipment:
		return "AWAITING_SHIPMENT"
	case StatePartiallyShipped:
		return "PARTIALLY_SHIPPED"
	case StateShipped:
		return "SHIPPED"
	case StateCompleted:
		return "COMPLETED"
	case StateCanceled:
		return "CANCELED"
	case StateDeclined:
		return "DECLINED"
	case StateRefunded:
		return "REFUNDED"
	case StateDisputed:
		return "DISPUTED"
	case StateDecided:
		return "DECIDED"
	case StateResolved:
		return "RESOLVED"
	case StatePaymentFinalized:
		return "PAYMENT_FINALIZED"
	case StateProcessingError:
		return "PROCESSING_ERROR"
	case StateAwaitingFulfillment:
		return "AWAITING_FULFILLMENT"
	case StatePartiallyFulfilled:
		return "PARTIALLY_FULFILLED"
	case StateFulfilled:
		return "FULFILLED"
	default:
		return "UNKNOWN"
	}
}

// OrderEvent represents an event that triggers a state transition.
type OrderEvent int

const (
	// EventUnknown is an invalid/unmapped event.
	EventUnknown OrderEvent = iota
	// EventPaymentSent indicates the buyer has sent payment (and it is funded on-chain).
	EventPaymentSent
	// EventPaymentVerified indicates the submitted payment has been verified
	// by chain/provider checks and the order can move to pending.
	EventPaymentVerified
	// EventVendorConfirm indicates the vendor has confirmed/accepted the order.
	// Maps to ORDER_CONFIRMATION in the P2P protocol.
	EventVendorConfirm
	// EventPartialShip indicates the vendor partially shipped the order.
	EventPartialShip
	// EventOrderShipped indicates the vendor has shipped the order.
	EventOrderShipped
	// EventBuyerComplete indicates the buyer has marked the order complete.
	EventBuyerComplete
	// EventVendorCancel indicates the vendor has canceled the order.
	EventVendorCancel
	// EventVendorDecline indicates the vendor declined the order.
	// Maps to ORDER_DECLINE in the P2P protocol.
	EventVendorDecline
	// EventBuyerCancel indicates the buyer has canceled the order.
	EventBuyerCancel
	// EventRefundIssued indicates a refund has been issued.
	EventRefundIssued
	// EventDisputeOpened indicates a dispute has been opened.
	EventDisputeOpened
	// EventModeratorDecide indicates a moderator decision in dispute.
	EventModeratorDecide
	// EventDisputeResolved indicates a dispute has been resolved.
	EventDisputeResolved
	// EventPaymentFinalize indicates escrow payout was finalized.
	EventPaymentFinalize
	// EventOrderTimeout indicates the order expired (e.g. AWAITING_PAYMENT timed out).
	EventOrderTimeout
	// EventFulfillmentStarted indicates a supply-chain provider has accepted the order.
	EventFulfillmentStarted
	// EventFulfillmentPartial indicates some items fulfilled, others pending.
	EventFulfillmentPartial
	// EventFulfillmentComplete indicates all items fulfilled by the provider.
	EventFulfillmentComplete
)

// String returns the string representation of an OrderEvent.
func (e OrderEvent) String() string {
	switch e {
	case EventPaymentSent:
		return "PAYMENT_SENT"
	case EventPaymentVerified:
		return "PAYMENT_VERIFIED"
	case EventVendorConfirm:
		return "VENDOR_CONFIRM"
	case EventPartialShip:
		return "PARTIAL_SHIP"
	case EventOrderShipped:
		return "ORDER_SHIPPED"
	case EventBuyerComplete:
		return "BUYER_COMPLETE"
	case EventVendorCancel:
		return "VENDOR_CANCEL"
	case EventVendorDecline:
		return "VENDOR_DECLINE"
	case EventBuyerCancel:
		return "BUYER_CANCEL"
	case EventRefundIssued:
		return "REFUND_ISSUED"
	case EventDisputeOpened:
		return "DISPUTE_OPENED"
	case EventModeratorDecide:
		return "MODERATOR_DECIDE"
	case EventDisputeResolved:
		return "DISPUTE_RESOLVED"
	case EventPaymentFinalize:
		return "PAYMENT_FINALIZE"
	case EventOrderTimeout:
		return "ORDER_TIMEOUT"
	case EventFulfillmentStarted:
		return "FULFILLMENT_STARTED"
	case EventFulfillmentPartial:
		return "FULFILLMENT_PARTIAL"
	case EventFulfillmentComplete:
		return "FULFILLMENT_COMPLETE"
	default:
		return "UNKNOWN"
	}
}

// TransitionResult contains the result of a state transition.
type TransitionResult struct {
	NewState OrderState
	Valid    bool
	Error    error
}

// transitionTable is the single source of truth for all valid state transitions.
// The table is aligned with legacy mobazha3.0 handler behavior:
//   - AWAITING_PAYMENT is the initial state (order created, buyer hasn't paid)
//   - AWAITING_PAYMENT_VERIFICATION means payment submitted, awaiting verification
//   - PENDING means payment verified/funded, waiting for vendor to confirm/decline
//   - Handlers park messages until PAYMENT_SENT, so cancel/decline only happen from PENDING
var transitionTable = map[OrderState]map[OrderEvent]OrderState{
	// Initial state: order created, waiting for buyer to send payment.
	// Vendor can decline before payment (e.g., out of stock, cannot fulfill).
	StateAwaitingPayment: {
		EventPaymentSent:   StateAwaitingPaymentVerification,
		EventOrderTimeout:  StateCanceled,
		EventVendorDecline: StateDeclined,
	},
	// Payment submitted, awaiting chain/provider verification.
	StateAwaitingPaymentVerification: {
		EventPaymentVerified: StatePending,
		EventVendorDecline:   StateDeclined,
	},
	// Payment sent/funded. Vendor can confirm, decline, cancel. Buyer can cancel.
	// Refund and dispute are also possible at this stage.
	StatePending: {
		EventVendorConfirm: StateAwaitingShipment,
		EventVendorDecline: StateDeclined,
		EventVendorCancel:  StateCanceled,
		EventBuyerCancel:   StateCanceled,
		EventRefundIssued:  StateRefunded,
		EventDisputeOpened: StateDisputed,
	},
	// Vendor confirmed, waiting for shipment (or fulfillment provider handoff).
	StateAwaitingShipment: {
		EventOrderShipped:       StateShipped,
		EventPartialShip:        StatePartiallyShipped,
		EventFulfillmentStarted: StateAwaitingFulfillment,
		EventRefundIssued:       StateRefunded,
		EventDisputeOpened:      StateDisputed,
	},
	// Some items shipped, waiting for the rest.
	StatePartiallyShipped: {
		EventOrderShipped: StateShipped,
		EventRefundIssued:   StateRefunded,
		EventDisputeOpened:  StateDisputed,
	},
	// Ready for local pickup (alternative to shipping).
	// TODO: Currently unreachable — no incoming transition targets this state.
	// A future EventReadyForPickup or fulfillment-type distinction is needed
	// to transition here from AWAITING_SHIPMENT for local-pickup orders.
	StateAwaitingPickup: {
		EventBuyerComplete: StateCompleted,
		EventRefundIssued:  StateRefunded,
		EventDisputeOpened: StateDisputed,
	},
	// Fulfillment provider is processing the order.
	StateAwaitingFulfillment: {
		EventFulfillmentPartial:  StatePartiallyFulfilled,
		EventFulfillmentComplete: StateFulfilled,
		EventRefundIssued:        StateRefunded,
		EventDisputeOpened:       StateDisputed,
	},
	// Some items fulfilled, others pending.
	StatePartiallyFulfilled: {
		EventFulfillmentComplete: StateFulfilled,
		EventRefundIssued:        StateRefunded,
		EventDisputeOpened:       StateDisputed,
	},
	// All items fulfilled — provider marks shipped.
	StateFulfilled: {
		EventOrderShipped:  StateShipped,
		EventRefundIssued:  StateRefunded,
		EventDisputeOpened: StateDisputed,
	},
	// Order shipped/delivered, waiting for buyer to confirm receipt.
	StateShipped: {
		EventBuyerComplete: StateCompleted,
		EventDisputeOpened: StateDisputed,
	},
	// Dispute opened, waiting for moderator or resolution.
	StateDisputed: {
		EventModeratorDecide: StateDecided,
		EventDisputeResolved: StateResolved,
		EventPaymentFinalize: StatePaymentFinalized,
	},
	// Moderator has decided, waiting for acceptance or timeout.
	StateDecided: {
		EventDisputeResolved: StateResolved,
		EventPaymentFinalize: StatePaymentFinalized,
	},
}

// Transition performs a state transition based on the current state and event.
// Returns the new state if the transition is valid, or an error if not.
func Transition(currentState OrderState, event OrderEvent) TransitionResult {
	stateTransitions, ok := transitionTable[currentState]
	if !ok {
		return TransitionResult{
			NewState: currentState,
			Valid:    false,
			Error:    fmt.Errorf("no transitions defined for state %s", currentState),
		}
	}

	newState, ok := stateTransitions[event]
	if !ok {
		return TransitionResult{
			NewState: currentState,
			Valid:    false,
			Error:    fmt.Errorf("invalid transition: %s + %s", currentState, event),
		}
	}

	return TransitionResult{
		NewState: newState,
		Valid:    true,
		Error:    nil,
	}
}

// IsFinalState returns true if the state is a terminal state with no outgoing transitions.
func IsFinalState(state OrderState) bool {
	switch state {
	case StateCompleted, StateCanceled, StateDeclined, StateRefunded,
		StateResolved, StatePaymentFinalized, StateProcessingError:
		return true
	default:
		return false
	}
}

// AllowedEvents returns the list of events allowed for a given state.
// For final states or unknown states, returns an empty slice.
func AllowedEvents(state OrderState) []OrderEvent {
	stateTransitions, ok := transitionTable[state]
	if !ok {
		return []OrderEvent{}
	}
	events := make([]OrderEvent, 0, len(stateTransitions))
	for event := range stateTransitions {
		events = append(events, event)
	}
	return events
}

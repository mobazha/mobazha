package orders

import (
	coreorders "github.com/mobazha/mobazha-core/orders"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
)

// MessageTypeToEvent maps a P2P OrderMessage_MessageType to a mobazha-core OrderEvent.
//
// Some messages don't trigger state transitions (e.g., RATING_SIGNATURES, DISPUTE_UPDATE);
// for these, EventUnknown is returned.
//
// The order parameter is used to determine context-dependent mappings:
//   - ORDER_CANCEL: buyer cancel vs vendor cancel (based on sender vs vendor PeerID)
func MessageTypeToEvent(
	msgType npb.OrderMessage_MessageType,
	order *models.Order,
	senderPeerID string,
) coreorders.OrderEvent {
	switch msgType {
	case npb.OrderMessage_ORDER_OPEN:
		// ORDER_OPEN creates the order with InitialState().
		// It is not modeled as an event because there is no "from" state.
		return coreorders.EventUnknown

	case npb.OrderMessage_PAYMENT_SENT:
		// PAYMENT_SENT always means "payment submitted" and moves to
		// AWAITING_PAYMENT_VERIFICATION. Promotion to PENDING happens on
		// verification via EventPaymentVerified in RecordVerifiedPayment.
		return coreorders.EventPaymentSent

	case npb.OrderMessage_ORDER_CONFIRMATION:
		// Vendor confirms/accepts the order.
		return coreorders.EventVendorConfirm

	case npb.OrderMessage_ORDER_DECLINE:
		// Vendor declines the order.
		return coreorders.EventVendorDecline

	case npb.OrderMessage_ORDER_CANCEL:
		return mapCancelEvent(order, senderPeerID)

	case npb.OrderMessage_REFUND:
		return coreorders.EventRefundIssued

	case npb.OrderMessage_ORDER_SHIPMENT:
		return mapShipmentEvent(order)

	case npb.OrderMessage_ORDER_COMPLETE:
		return coreorders.EventBuyerComplete

	case npb.OrderMessage_DISPUTE_OPEN:
		return coreorders.EventDisputeOpened

	case npb.OrderMessage_DISPUTE_CLOSE:
		return coreorders.EventModeratorDecide

	case npb.OrderMessage_DISPUTE_ACCEPT:
		return coreorders.EventDisputeResolved

	case npb.OrderMessage_PAYMENT_FINALIZED:
		return coreorders.EventPaymentFinalize

	case npb.OrderMessage_RATING_SIGNATURES:
		// Utility message; no state change.
		return coreorders.EventUnknown

	case npb.OrderMessage_DISPUTE_UPDATE:
		// Dispute chat update; no state change.
		return coreorders.EventUnknown

	case npb.OrderMessage_PAYMENT_LOCKED:
		// RWA funding lock; not yet modeled in core state machine.
		return coreorders.EventUnknown

	default:
		return coreorders.EventUnknown
	}
}

// mapCancelEvent determines whether a cancel message is from the buyer or vendor.
func mapCancelEvent(order *models.Order, senderPeerID string) coreorders.OrderEvent {
	if order == nil {
		// Without order context we cannot determine the sender's role.
		// Return EventUnknown so the caller can decide how to handle it.
		return coreorders.EventUnknown
	}

	vendor, err := order.Vendor()
	if err == nil && senderPeerID == vendor.String() {
		return coreorders.EventVendorCancel
	}
	return coreorders.EventBuyerCancel
}

// mapShipmentEvent maps an ORDER_SHIPMENT message to the appropriate core event.
//
// The protocol uses a single ORDER_SHIPMENT message type for both partial
// and full shipment. The core state machine distinguishes these via separate events
// (EventPartialShip vs EventOrderShipped). Since the legacy message doesn't carry
// this distinction, we default to EventOrderShipped and let the FSM validate:
//   - From AWAITING_SHIPMENT: EventOrderShipped → SHIPPED
//   - From PARTIALLY_SHIPPED: EventOrderShipped → SHIPPED
func mapShipmentEvent(_ *models.Order) coreorders.OrderEvent {
	return coreorders.EventOrderShipped
}

// IsStateTransitionMessage returns true if the message type triggers a state transition.
// ORDER_OPEN is excluded because it sets InitialState() rather than triggering a transition.
func IsStateTransitionMessage(msgType npb.OrderMessage_MessageType) bool {
	switch msgType {
	case npb.OrderMessage_ORDER_OPEN,
		npb.OrderMessage_RATING_SIGNATURES,
		npb.OrderMessage_DISPUTE_UPDATE,
		npb.OrderMessage_PAYMENT_LOCKED:
		return false
	default:
		return true
	}
}

package models

import "github.com/mobazha/mobazha-core/orders"

// OrderState is a type alias for the canonical definition in mobazha-core.
// This ensures mobazha3.0 and mobazha-cloud share the same OrderState type.
type OrderState = orders.OrderState

// Backward-compatible constants mapping to mobazha-core values.
// Existing code can continue using OrderState_PENDING etc. without changes.
const (
	OrderState_PENDING              = orders.StatePending
	OrderState_AWAITING_PAYMENT     = orders.StateAwaitingPayment
	OrderState_AWAITING_PICKUP      = orders.StateAwaitingPickup
	OrderState_AWAITING_FULFILLMENT = orders.StateAwaitingFulfillment
	OrderState_PARTIALLY_FULFILLED  = orders.StatePartiallyFulfilled
	OrderState_FULFILLED            = orders.StateFulfilled
	OrderState_COMPLETED            = orders.StateCompleted
	OrderState_CANCELED             = orders.StateCanceled
	OrderState_DECLINED             = orders.StateDeclined
	OrderState_REFUNDED             = orders.StateRefunded
	OrderState_DISPUTED             = orders.StateDisputed
	OrderState_DECIDED              = orders.StateDecided
	OrderState_RESOLVED             = orders.StateResolved
	OrderState_PAYMENT_FINALIZED    = orders.StatePaymentFinalized
	OrderState_PROCESSING_ERROR     = orders.StateProcessingError
)

// OrderState_name maps int32 values to string names (kept for protobuf compatibility).
var OrderState_name = map[int32]string{
	0:  "PENDING",
	1:  "AWAITING_PAYMENT",
	2:  "AWAITING_PICKUP",
	3:  "AWAITING_FULFILLMENT",
	4:  "PARTIALLY_FULFILLED",
	5:  "FULFILLED",
	6:  "COMPLETED",
	7:  "CANCELED",
	8:  "DECLINED",
	9:  "REFUNDED",
	10: "DISPUTED",
	11: "DECIDED",
	12: "RESOLVED",
	13: "PAYMENT_FINALIZED",
	14: "PROCESSING_ERROR",
}

// OrderState_value maps string names to int32 values (kept for protobuf compatibility).
var OrderState_value = map[string]int32{
	"PENDING":              0,
	"AWAITING_PAYMENT":     1,
	"AWAITING_PICKUP":      2,
	"AWAITING_FULFILLMENT": 3,
	"PARTIALLY_FULFILLED":  4,
	"FULFILLED":            5,
	"COMPLETED":            6,
	"CANCELED":             7,
	"DECLINED":             8,
	"REFUNDED":             9,
	"DISPUTED":             10,
	"DECIDED":              11,
	"RESOLVED":             12,
	"PAYMENT_FINALIZED":    13,
	"PROCESSING_ERROR":     14,
}

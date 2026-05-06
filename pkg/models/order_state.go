package models

import "github.com/mobazha/mobazha3.0/pkg/orders"

// OrderState is a type alias for the canonical definition in mobazha-core.
// This ensures mobazha3.0 and mobazha-cloud share the same OrderState type.
type OrderState = orders.OrderState

// Backward-compatible constants mapping to mobazha-core values.
// Existing code can continue using OrderState_PENDING etc. without changes.
const (
	OrderState_PENDING                       = orders.StatePending
	OrderState_AWAITING_PAYMENT              = orders.StateAwaitingPayment
	OrderState_AWAITING_PAYMENT_VERIFICATION = orders.StateAwaitingPaymentVerification
	OrderState_AWAITING_PICKUP               = orders.StateAwaitingPickup
	OrderState_AWAITING_SHIPMENT             = orders.StateAwaitingShipment
	OrderState_PARTIALLY_SHIPPED             = orders.StatePartiallyShipped
	OrderState_SHIPPED                       = orders.StateShipped
	OrderState_COMPLETED                     = orders.StateCompleted
	OrderState_CANCELED                      = orders.StateCanceled
	OrderState_DECLINED                      = orders.StateDeclined
	OrderState_REFUNDED                      = orders.StateRefunded
	OrderState_DISPUTED                      = orders.StateDisputed
	OrderState_DECIDED                       = orders.StateDecided
	OrderState_RESOLVED                      = orders.StateResolved
	OrderState_PAYMENT_FINALIZED             = orders.StatePaymentFinalized
	OrderState_PROCESSING_ERROR              = orders.StateProcessingError
)

// OrderState_name maps int32 values to string names (kept for protobuf compatibility).
var OrderState_name = map[int32]string{
	int32(OrderState_PENDING):                       "PENDING",
	int32(OrderState_AWAITING_PAYMENT):              "AWAITING_PAYMENT",
	int32(OrderState_AWAITING_PAYMENT_VERIFICATION): "AWAITING_PAYMENT_VERIFICATION",
	int32(OrderState_AWAITING_PICKUP):               "AWAITING_PICKUP",
	int32(OrderState_AWAITING_SHIPMENT):             "AWAITING_SHIPMENT",
	int32(OrderState_PARTIALLY_SHIPPED):             "PARTIALLY_SHIPPED",
	int32(OrderState_SHIPPED):                       "SHIPPED",
	int32(OrderState_COMPLETED):                     "COMPLETED",
	int32(OrderState_CANCELED):                      "CANCELED",
	int32(OrderState_DECLINED):                      "DECLINED",
	int32(OrderState_REFUNDED):                      "REFUNDED",
	int32(OrderState_DISPUTED):                      "DISPUTED",
	int32(OrderState_DECIDED):                       "DECIDED",
	int32(OrderState_RESOLVED):                      "RESOLVED",
	int32(OrderState_PAYMENT_FINALIZED):             "PAYMENT_FINALIZED",
	int32(OrderState_PROCESSING_ERROR):              "PROCESSING_ERROR",
}

// OrderState_value maps string names to int32 values (kept for protobuf compatibility).
var OrderState_value = map[string]int32{
	"PENDING":                       int32(OrderState_PENDING),
	"AWAITING_PAYMENT":              int32(OrderState_AWAITING_PAYMENT),
	"AWAITING_PAYMENT_VERIFICATION": int32(OrderState_AWAITING_PAYMENT_VERIFICATION),
	"AWAITING_PICKUP":               int32(OrderState_AWAITING_PICKUP),
	"AWAITING_SHIPMENT":             int32(OrderState_AWAITING_SHIPMENT),
	"PARTIALLY_SHIPPED":             int32(OrderState_PARTIALLY_SHIPPED),
	"SHIPPED":                       int32(OrderState_SHIPPED),
	"COMPLETED":                     int32(OrderState_COMPLETED),
	"CANCELED":                      int32(OrderState_CANCELED),
	"DECLINED":                      int32(OrderState_DECLINED),
	"REFUNDED":                      int32(OrderState_REFUNDED),
	"DISPUTED":                      int32(OrderState_DISPUTED),
	"DECIDED":                       int32(OrderState_DECIDED),
	"RESOLVED":                      int32(OrderState_RESOLVED),
	"PAYMENT_FINALIZED":             int32(OrderState_PAYMENT_FINALIZED),
	"PROCESSING_ERROR":              int32(OrderState_PROCESSING_ERROR),
}

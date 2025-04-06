package models

import "github.com/gogo/protobuf/proto"

type OrderState int32

const (
	// Order has been funded and sent to the vendor but vendor has not yet responded
	OrderState_PENDING OrderState = 0
	// Waiting for the buyer to fund the payment address
	OrderState_AWAITING_PAYMENT OrderState = 1
	// Waiting for the customer to pick up the order (customer pickup option only)
	OrderState_AWAITING_PICKUP OrderState = 2
	// Order has been fully funded and we're waiting for the vendor to fulfill
	OrderState_AWAITING_FULFILLMENT OrderState = 3
	// Vendor has fulfilled part of the order
	OrderState_PARTIALLY_FULFILLED OrderState = 4
	// Vendor has fulfilled the order
	OrderState_FULFILLED OrderState = 5
	// Buyer has completed the order and left a review
	OrderState_COMPLETED OrderState = 6
	// Buyer canceled the order (offline order only)
	OrderState_CANCELED OrderState = 7
	// Vendor declined to confirm the order (offline order only)
	OrderState_DECLINED OrderState = 8
	// Vendor refunded the order
	OrderState_REFUNDED OrderState = 9
	// Contract is under active dispute
	OrderState_DISPUTED OrderState = 10
	// The moderator has resolved the dispute and we are waiting for the winning party to
	// accept the payout.
	OrderState_DECIDED OrderState = 11
	// The winning party has accepted the dispute and it is now complete. After the buyer
	// leaves a review the state should be set to COMPLETE.
	OrderState_RESOLVED OrderState = 12
	// Escrow has been released after waiting the timeout period. After the buyer
	// leaves a review the state should be set to COMPLETE.
	OrderState_PAYMENT_FINALIZED OrderState = 13
	// We screwed up and produced a order which didn't validate. This state is only used for offline orders. If a processing
	// error occurred with an open connection between buyer and vendor the vendor just rejects the order on the spot neither party
	// commits the order to the database.
	OrderState_PROCESSING_ERROR OrderState = 14
)

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

func (x OrderState) String() string {
	return proto.EnumName(OrderState_name, int32(x))
}

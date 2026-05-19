package models

// PendingSettlementSpec is the JSON-serializable settlement triple stored on
// Order.PendingPaymentInfo. It mirrors pkg/payment.SettlementSpec without
// importing pkg/payment (avoids models ↔ payment import cycle).
type PendingSettlementSpec struct {
	Method     string `json:"method,omitempty"`
	PayMode    string `json:"payMode,omitempty"`
	EscrowType string `json:"escrowType,omitempty"`
}

package models

// IsPaymentReady reports whether payment instructions may be exposed for the
// buyer-side order. During migration, legacy OrderOpenAcked rows remain valid
// until payment_ready_at is backfilled.
func IsPaymentReady(o *Order) bool {
	if o == nil {
		return false
	}
	return o.PaymentReadyAt != nil || o.OrderOpenAcked
}

// BuyerAwaitingPaymentReadiness reports whether a buyer-side order must wait
// for seller ORDER_OPEN processing before payment instructions may be exposed.
func BuyerAwaitingPaymentReadiness(o *Order) bool {
	return o != nil && o.Role() == RoleBuyer && !IsPaymentReady(o)
}

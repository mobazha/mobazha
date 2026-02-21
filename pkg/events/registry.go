package events

import "reflect"

// EventMeta holds unified metadata for a business event.
// Legacy is the notification type string used by the existing frontend WebSocket protocol.
// Empty Legacy means the event is WebSocket-only (no DB persistence in NotificationRecord).
type EventMeta struct {
	Category string
	Name     string
	Legacy   string
	Sample   interface{}
}

var registry []EventMeta

// typeIndex maps reflect.Type → *EventMeta for O(1) lookup.
// Built once in init() from the registry slice.
var typeIndex map[reflect.Type]*EventMeta

func init() {
	registry = []EventMeta{
		// ── Order events (persistent + WebSocket) ──
		{Category: "order", Name: "order.created", Legacy: "newOrder", Sample: new(NewOrder)},
		{Category: "order", Name: "order.funded", Legacy: "orderFunded", Sample: new(OrderFunded)},
		{Category: "order", Name: "order.payment_received", Legacy: "orderPaymentReceived", Sample: new(OrderPaymentReceived)},
		{Category: "order", Name: "order.confirmed", Legacy: "orderConfirmation", Sample: new(OrderConfirmation)},
		{Category: "order", Name: "order.fulfilled", Legacy: "orderFulfillment", Sample: new(OrderFulfillment)},
		{Category: "order", Name: "order.completed", Legacy: "orderCompletion", Sample: new(OrderCompletion)},
		{Category: "order", Name: "order.cancelled", Legacy: "orderCancel", Sample: new(OrderCancel)},
		{Category: "order", Name: "order.declined", Legacy: "orderDeclined", Sample: new(OrderDeclined)},
		{Category: "order", Name: "order.refunded", Legacy: "refund", Sample: new(Refund)},
		{Category: "order", Name: "order.vendor_finalized", Legacy: "vendorFinalizedPayment", Sample: new(VendorFinalizedPayment)},

		// ── Dispute events (persistent + WebSocket) ──
		{Category: "dispute", Name: "dispute.opened", Legacy: "disputeOpen", Sample: new(DisputeOpen)},
		{Category: "dispute", Name: "dispute.closed", Legacy: "disputeClose", Sample: new(DisputeClose)},
		{Category: "dispute", Name: "dispute.accepted", Legacy: "disputeAccepted", Sample: new(DisputeAccepted)},
		{Category: "dispute", Name: "dispute.case_open", Legacy: "caseOpen", Sample: new(CaseOpen)},
		{Category: "dispute", Name: "dispute.case_update", Legacy: "caseUpdate", Sample: new(CaseUpdate)},

		// ── Social events (persistent + WebSocket) ──
		{Category: "social", Name: "social.follow", Legacy: "follow", Sample: new(Follow)},
		{Category: "social", Name: "social.unfollow", Legacy: "unfollow", Sample: new(Unfollow)},

		// ── Chat events (WebSocket only) ──
		{Category: "chat", Name: "chat.message", Legacy: "", Sample: new(ChatMessage)},
		{Category: "chat", Name: "chat.read", Legacy: "", Sample: new(ChatRead)},
		{Category: "chat", Name: "chat.typing", Legacy: "", Sample: new(ChatTyping)},
		{Category: "chat", Name: "chat.channel", Legacy: "", Sample: new(ChannelMessage)},

		// ── Chat group events (WebSocket only) ──
		{Category: "chatgroup", Name: "chatgroup.created", Legacy: "", Sample: new(ChatGroupCreate)},
		{Category: "chatgroup", Name: "chatgroup.updated", Legacy: "", Sample: new(ChatGroupUpdate)},
		{Category: "chatgroup", Name: "chatgroup.deleted", Legacy: "", Sample: new(ChatGroupDelete)},

		// ── Wallet events (WebSocket only) ──
		{Category: "wallet", Name: "wallet.block_received", Legacy: "", Sample: new(BlockReceived)},
		{Category: "wallet", Name: "wallet.tx_received", Legacy: "", Sample: new(TransactionReceived)},
		{Category: "wallet", Name: "wallet.spend_from_payment", Legacy: "", Sample: new(SpendFromPaymentAddress)},
		{Category: "wallet", Name: "wallet.update", Legacy: "", Sample: new(WalletUpdate)},

		// ── Publish events (WebSocket only) ──
		{Category: "publish", Name: "publish.started", Legacy: "", Sample: new(PublishStarted)},
		{Category: "publish", Name: "publish.finished", Legacy: "", Sample: new(PublishFinished)},
		{Category: "publish", Name: "publish.error", Legacy: "", Sample: new(PublishingError)},

		// ── Shopping cart (WebSocket only) ──
		{Category: "cart", Name: "cart.updated", Legacy: "", Sample: new(ShoppingCartUpdate)},

		// ── Payment events (persistent + WebSocket) ──
		{Category: "payment", Name: "payment.locked", Legacy: "paymentLocked", Sample: new(PaymentLockedReceived)},
		{Category: "payment", Name: "payment.expired", Legacy: "paymentExpired", Sample: new(PaymentExpiredNotification)},
		{Category: "payment", Name: "payment.cancelled", Legacy: "paymentCancelled", Sample: new(PaymentCancelledByBuyer)},
		{Category: "payment", Name: "payment.partial", Legacy: "", Sample: new(PartialPaymentReceived)},
	}

	typeIndex = make(map[reflect.Type]*EventMeta, len(registry)*2)
	for i := range registry {
		m := &registry[i]
		t := reflect.TypeOf(m.Sample)
		typeIndex[t] = m
		if t.Kind() == reflect.Ptr {
			typeIndex[t.Elem()] = m
		}
	}
}

// LookupEvent finds the EventMeta for a Go event value.
// Accepts both value and pointer types. Returns nil for unregistered events.
func LookupEvent(evt interface{}) *EventMeta {
	if evt == nil {
		return nil
	}
	t := reflect.TypeOf(evt)
	if m, ok := typeIndex[t]; ok {
		return m
	}
	if t.Kind() == reflect.Ptr {
		if m, ok := typeIndex[t.Elem()]; ok {
			return m
		}
	}
	return nil
}

// EventsByCategory returns Sample pointers for the given categories,
// suitable for passing to Bus.Subscribe.
func EventsByCategory(cats ...string) []interface{} {
	catSet := make(map[string]bool, len(cats))
	for _, c := range cats {
		catSet[c] = true
	}
	var result []interface{}
	for _, m := range registry {
		if catSet[m.Category] {
			result = append(result, m.Sample)
		}
	}
	return result
}

// AllSamples returns all registered event Sample pointers,
// suitable for subscribing to every business event on the Bus.
func AllSamples() []interface{} {
	result := make([]interface{}, len(registry))
	for i, m := range registry {
		result[i] = m.Sample
	}
	return result
}

// AllEventNames returns the dot-separated name of every registered event.
func AllEventNames() []string {
	result := make([]string, len(registry))
	for i, m := range registry {
		result[i] = m.Name
	}
	return result
}

// AllMeta returns a copy of the full registry slice.
func AllMeta() []EventMeta {
	cp := make([]EventMeta, len(registry))
	copy(cp, registry)
	return cp
}

package events

import "reflect"

// EventMeta holds unified metadata for a business event.
// Persistent events are saved to DB and pushed via WebSocket.
// Non-persistent events are pushed via WebSocket only.
type EventMeta struct {
	Category   string
	Name       string
	Persistent bool
	Sample     interface{}
}

var registry []EventMeta

// typeIndex maps reflect.Type → *EventMeta for O(1) lookup.
// Built once in init() from the registry slice.
var typeIndex map[reflect.Type]*EventMeta

func init() {
	registry = []EventMeta{
		// ── Order events (persistent + WebSocket) ──
		{Category: "order", Name: "order.created", Persistent: true, Sample: new(NewOrder)},
		{Category: "order", Name: "order.funded", Persistent: true, Sample: new(OrderFunded)},
		{Category: "order", Name: "order.payment_received", Persistent: true, Sample: new(OrderPaymentReceived)},
		{Category: "order", Name: "order.confirmed", Persistent: true, Sample: new(OrderConfirmation)},
		{Category: "order", Name: "order.fulfilled", Persistent: true, Sample: new(OrderFulfillment)},
		{Category: "order", Name: "order.completed", Persistent: true, Sample: new(OrderCompletion)},
		{Category: "order", Name: "order.cancelled", Persistent: true, Sample: new(OrderCancel)},
		{Category: "order", Name: "order.expired", Persistent: true, Sample: new(OrderExpired)},
		{Category: "order", Name: "order.stale_warning", Persistent: true, Sample: new(OrderStaleWarning)},
		{Category: "order", Name: "order.declined", Persistent: true, Sample: new(OrderDeclined)},
		{Category: "order", Name: "order.refunded", Persistent: true, Sample: new(Refund)},
		{Category: "order", Name: "order.vendor_finalized", Persistent: true, Sample: new(VendorFinalizedPayment)},

		// ── Dispute events (persistent + WebSocket) ──
		{Category: "dispute", Name: "dispute.opened", Persistent: true, Sample: new(DisputeOpen)},
		{Category: "dispute", Name: "dispute.closed", Persistent: true, Sample: new(DisputeClose)},
		{Category: "dispute", Name: "dispute.accepted", Persistent: true, Sample: new(DisputeAccepted)},
		{Category: "dispute", Name: "dispute.case_open", Persistent: true, Sample: new(CaseOpen)},
		{Category: "dispute", Name: "dispute.case_update", Persistent: true, Sample: new(CaseUpdate)},

		// ── Social events (persistent + WebSocket) ──
		{Category: "social", Name: "social.follow", Persistent: true, Sample: new(Follow)},
		{Category: "social", Name: "social.unfollow", Persistent: true, Sample: new(Unfollow)},
		{Category: "social", Name: "social.moderator_add", Persistent: true, Sample: new(ModeratorAdd)},
		{Category: "social", Name: "social.moderator_remove", Persistent: true, Sample: new(ModeratorRemove)},

		// ── Chat events (WebSocket only) ──
		{Category: "chat", Name: "chat.message", Sample: new(ChatMessage)},
		{Category: "chat", Name: "chat.read", Sample: new(ChatRead)},
		{Category: "chat", Name: "chat.typing", Sample: new(ChatTyping)},
		{Category: "chat", Name: "chat.channel", Sample: new(ChannelMessage)},

		// ── Chat group events (WebSocket only) ──
		{Category: "chatgroup", Name: "chatgroup.created", Sample: new(ChatGroupCreate)},
		{Category: "chatgroup", Name: "chatgroup.updated", Sample: new(ChatGroupUpdate)},
		{Category: "chatgroup", Name: "chatgroup.deleted", Sample: new(ChatGroupDelete)},

		// ── Wallet events (WebSocket only) ──
		{Category: "wallet", Name: "wallet.block_received", Sample: new(BlockReceived)},
		{Category: "wallet", Name: "wallet.tx_received", Sample: new(TransactionReceived)},
		{Category: "wallet", Name: "wallet.spend_from_payment", Sample: new(SpendFromPaymentAddress)},
		{Category: "wallet", Name: "wallet.update", Sample: new(WalletUpdate)},

		// ── Publish events (WebSocket only) ──
		{Category: "publish", Name: "publish.started", Sample: new(PublishStarted)},
		{Category: "publish", Name: "publish.finished", Sample: new(PublishFinished)},
		{Category: "publish", Name: "publish.error", Sample: new(PublishingError)},

		// ── Shopping cart (WebSocket only) ──
		{Category: "cart", Name: "cart.updated", Sample: new(ShoppingCartUpdate)},

		// ── Collection events (persistent + WebSocket) ──
		{Category: "collection", Name: "collection.created", Persistent: true, Sample: new(CollectionCreated)},
		{Category: "collection", Name: "collection.updated", Persistent: true, Sample: new(CollectionUpdated)},
		{Category: "collection", Name: "collection.deleted", Persistent: true, Sample: new(CollectionDeleted)},
		{Category: "collection", Name: "collection.products_changed", Persistent: true, Sample: new(CollectionProductsChanged)},

		// ── Shipping events (WebSocket only) ──
		{Category: "shipping", Name: "shipping.profile_updated", Sample: new(ShippingProfileUpdated)},
		{Category: "shipping", Name: "shipping.profile_deleted", Sample: new(ShippingProfileDeleted)},
		{Category: "shipping", Name: "shipping.snapshots_refreshed", Sample: new(ShippingSnapshotsRefreshed)},

		// ── Payment events (persistent + WebSocket) ──
		{Category: "payment", Name: "payment.locked", Persistent: true, Sample: new(PaymentLockedReceived)},
		{Category: "payment", Name: "payment.expired", Persistent: true, Sample: new(PaymentExpiredNotification)},
		{Category: "payment", Name: "payment.cancelled", Persistent: true, Sample: new(PaymentCancelledByBuyer)},
		{Category: "payment", Name: "payment.partial", Sample: new(PartialPaymentReceived)},

		// ── Internal domain events (non-persistent, no WebSocket push) ──
		{Category: "internal", Name: "internal.order_auto_confirm", Sample: new(OrderAutoConfirmRequest)},
		{Category: "internal", Name: "internal.utxo_payment_detected", Sample: new(UTXOPaymentDetected)},
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

// LookupByName finds the EventMeta for a dot-separated event name
// (e.g. "order.expired"). Returns nil for unregistered names.
func LookupByName(name string) *EventMeta {
	for i := range registry {
		if registry[i].Name == name {
			return &registry[i]
		}
	}
	return nil
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

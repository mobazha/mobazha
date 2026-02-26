package events

import (
	"testing"
)

func TestLookupEvent_PointerType(t *testing.T) {
	meta := LookupEvent(&NewOrder{})
	if meta == nil {
		t.Fatal("expected non-nil meta for *NewOrder")
	}
	if meta.Name != "order.created" {
		t.Errorf("expected order.created, got %s", meta.Name)
	}
	if !meta.Persistent {
		t.Error("expected order.created to be persistent")
	}
}

func TestLookupEvent_ValueType(t *testing.T) {
	meta := LookupEvent(NewOrder{})
	if meta == nil {
		t.Fatal("expected non-nil meta for NewOrder value")
	}
	if meta.Name != "order.created" {
		t.Errorf("expected order.created, got %s", meta.Name)
	}
}

func TestLookupEvent_Nil(t *testing.T) {
	if LookupEvent(nil) != nil {
		t.Error("expected nil for nil input")
	}
}

func TestLookupEvent_Unregistered(t *testing.T) {
	if LookupEvent("random string") != nil {
		t.Error("expected nil for unregistered type")
	}
}

func TestLookupEvent_AllRegistered(t *testing.T) {
	for _, m := range registry {
		got := LookupEvent(m.Sample)
		if got == nil {
			t.Errorf("LookupEvent returned nil for %s (sample type %T)", m.Name, m.Sample)
			continue
		}
		if got.Name != m.Name {
			t.Errorf("for sample %T: expected %s, got %s", m.Sample, m.Name, got.Name)
		}
	}
}

func TestEventsByCategory_Order(t *testing.T) {
	samples := EventsByCategory("order")
	if len(samples) != 10 {
		t.Errorf("expected 10 order events, got %d", len(samples))
	}
}

func TestEventsByCategory_Multiple(t *testing.T) {
	samples := EventsByCategory("order", "dispute")
	if len(samples) != 15 {
		t.Errorf("expected 15 events for order+dispute, got %d", len(samples))
	}
}

func TestEventsByCategory_Unknown(t *testing.T) {
	samples := EventsByCategory("nonexistent")
	if len(samples) != 0 {
		t.Errorf("expected 0 events for unknown category, got %d", len(samples))
	}
}

func TestAllSamples_Count(t *testing.T) {
	all := AllSamples()
	if len(all) != len(registry) {
		t.Errorf("expected %d samples, got %d", len(registry), len(all))
	}
}

func TestAllEventNames_Contains(t *testing.T) {
	names := AllEventNames()
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []string{"order.created", "dispute.opened", "chat.message", "wallet.update"} {
		if !nameSet[expected] {
			t.Errorf("AllEventNames missing %s", expected)
		}
	}
}

func TestAllMeta_IsCopy(t *testing.T) {
	cp := AllMeta()
	if len(cp) != len(registry) {
		t.Fatalf("expected %d, got %d", len(registry), len(cp))
	}
	cp[0].Name = "mutated"
	if registry[0].Name == "mutated" {
		t.Error("AllMeta returned slice that mutates the original registry")
	}
}

func TestPersistentConsistency(t *testing.T) {
	persistentNames := map[string]bool{
		"order.created":          true,
		"order.funded":           true,
		"order.payment_received": true,
		"order.confirmed":        true,
		"order.fulfilled":        true,
		"order.completed":        true,
		"order.cancelled":        true,
		"order.declined":         true,
		"order.refunded":         true,
		"order.vendor_finalized": true,
		"dispute.opened":         true,
		"dispute.closed":         true,
		"dispute.accepted":       true,
		"dispute.case_open":      true,
		"dispute.case_update":    true,
		"social.follow":          true,
		"social.unfollow":        true,
		"social.moderator_add":   true,
		"social.moderator_remove": true,
		"payment.locked":         true,
		"payment.expired":        true,
		"payment.cancelled":      true,
	}
	for _, m := range registry {
		if want, ok := persistentNames[m.Name]; ok {
			if m.Persistent != want {
				t.Errorf("%s: expected persistent=%v, got %v", m.Name, want, m.Persistent)
			}
		} else if m.Persistent {
			t.Errorf("%s: unexpectedly persistent", m.Name)
		}
	}
}

func TestNoDuplicateNames(t *testing.T) {
	seen := make(map[string]bool, len(registry))
	for _, m := range registry {
		if seen[m.Name] {
			t.Errorf("duplicate event name: %s", m.Name)
		}
		seen[m.Name] = true
	}
}

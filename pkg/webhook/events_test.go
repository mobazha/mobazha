package webhook

import (
	"encoding/json"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/events"
)

func TestClassifyEvent_AllTypes(t *testing.T) {
	cases := []struct {
		name     string
		event    interface{}
		expected string
	}{
		{"NewOrder", events.NewOrder{}, EventOrderCreated},
		{"NewOrder ptr", &events.NewOrder{}, EventOrderCreated},
		{"OrderFunded", events.OrderFunded{}, EventOrderFunded},
		{"OrderPaymentReceived", events.OrderPaymentReceived{}, EventOrderPaymentReceived},
		{"OrderConfirmation", events.OrderConfirmation{}, EventOrderConfirmed},
		{"OrderFulfillment", events.OrderFulfillment{}, EventOrderFulfilled},
		{"OrderCompletion", events.OrderCompletion{}, EventOrderCompleted},
		{"OrderCancel", events.OrderCancel{}, EventOrderCancelled},
		{"OrderDeclined", events.OrderDeclined{}, EventOrderDeclined},
		{"Refund", events.Refund{}, EventOrderRefunded},
		{"DisputeOpen", events.DisputeOpen{}, EventDisputeOpened},
		{"DisputeClose", events.DisputeClose{}, EventDisputeClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyEvent(tc.event)
			if got != tc.expected {
				t.Errorf("ClassifyEvent(%T) = %q, want %q", tc.event, got, tc.expected)
			}
		})
	}
}

func TestClassifyEvent_UnknownEvent(t *testing.T) {
	got := ClassifyEvent("not-an-event")
	if got != "" {
		t.Errorf("ClassifyEvent(string) = %q, want empty", got)
	}
}

func TestClassifyEvent_NilEvent(t *testing.T) {
	got := ClassifyEvent(nil)
	if got != "" {
		t.Errorf("ClassifyEvent(nil) = %q, want empty", got)
	}
}

func TestBuildCloudEvent_Format(t *testing.T) {
	data := map[string]string{"orderID": "ord-1"}
	payload, err := BuildCloudEvent("tenant-1", EventOrderCreated, data)
	if err != nil {
		t.Fatalf("BuildCloudEvent error: %v", err)
	}

	var ce CloudEvent
	if err := json.Unmarshal(payload, &ce); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if ce.SpecVersion != "1.0" {
		t.Errorf("specversion = %q, want %q", ce.SpecVersion, "1.0")
	}
	if ce.Type != "com.mobazha.order.created" {
		t.Errorf("type = %q, want %q", ce.Type, "com.mobazha.order.created")
	}
	if ce.Source != "/tenants/tenant-1" {
		t.Errorf("source = %q, want %q", ce.Source, "/tenants/tenant-1")
	}
	if ce.DataContentType != "application/json" {
		t.Errorf("datacontenttype = %q, want %q", ce.DataContentType, "application/json")
	}
	if ce.ID == "" {
		t.Error("expected non-empty ID")
	}
	if ce.Time == "" {
		t.Error("expected non-empty time")
	}
}

func TestBuildCloudEvent_DataPayload(t *testing.T) {
	data := events.NewOrder{
		Notification: events.Notification{ID: "n-1", Typ: "order"},
		OrderID:      "ord-abc",
		Title:        "Test Product",
	}
	payload, err := BuildCloudEvent("t-1", EventOrderCreated, data)
	if err != nil {
		t.Fatalf("BuildCloudEvent error: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(payload, &raw)

	d, ok := raw["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}
	if d["orderID"] != "ord-abc" {
		t.Errorf("data.orderID = %v, want %q", d["orderID"], "ord-abc")
	}
}

func TestMatchEventFilter_Exact(t *testing.T) {
	if !MatchEventFilter("order.created", "order.created") {
		t.Error("expected exact match to pass")
	}
}

func TestMatchEventFilter_Wildcard(t *testing.T) {
	if !MatchEventFilter("order.*", "order.created") {
		t.Error("expected wildcard match to pass")
	}
	if !MatchEventFilter("order.*", "order.funded") {
		t.Error("expected wildcard match to pass for order.funded")
	}
}

func TestMatchEventFilter_NoMatch(t *testing.T) {
	if MatchEventFilter("order.created", "dispute.opened") {
		t.Error("expected no match")
	}
}

func TestMatchEventFilter_MultipleTypes(t *testing.T) {
	filter := "order.created, dispute.opened, chat.message"
	if !MatchEventFilter(filter, "order.created") {
		t.Error("expected multi-type match for order.created")
	}
	if !MatchEventFilter(filter, "dispute.opened") {
		t.Error("expected multi-type match for dispute.opened")
	}
	if MatchEventFilter(filter, "order.funded") {
		t.Error("expected no match for order.funded in multi-type filter")
	}
}

func TestMatchEventFilter_WildcardMulti(t *testing.T) {
	filter := "order.*,dispute.*"
	if !MatchEventFilter(filter, "order.completed") {
		t.Error("expected wildcard multi match")
	}
	if !MatchEventFilter(filter, "dispute.closed") {
		t.Error("expected wildcard multi match for dispute.closed")
	}
	if MatchEventFilter(filter, "chat.message") {
		t.Error("expected no match for chat.message")
	}
}

func TestMatchEventFilter_EmptyFilter(t *testing.T) {
	if MatchEventFilter("", "order.created") {
		t.Error("expected empty filter to not match")
	}
}

func TestAllWebhookEventTypes_ReturnsAll(t *testing.T) {
	types := AllWebhookEventTypes()
	allNames := events.AllEventNames()
	if len(types) != len(allNames) {
		t.Errorf("expected %d event types (all registry events), got %d", len(allNames), len(types))
	}
}

func TestClassifyEvent_UsesRegistry(t *testing.T) {
	for _, m := range events.AllMeta() {
		got := ClassifyEvent(m.Sample)
		if got != m.Name {
			t.Errorf("ClassifyEvent(%T) = %q, want %q", m.Sample, got, m.Name)
		}
	}
}

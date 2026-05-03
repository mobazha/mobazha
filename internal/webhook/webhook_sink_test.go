package webhook

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/events"
	wh "github.com/mobazha/mobazha3.0/pkg/webhook"
)

type recordingStore struct {
	wh.EndpointStore
	mu          sync.Mutex
	deliveries  []wh.Delivery
	endpoints   []wh.Endpoint
}

func (s *recordingStore) ListActive() ([]wh.Endpoint, error) {
	return s.endpoints, nil
}

func (s *recordingStore) CreateDeliveries(d []wh.Delivery) error {
	s.mu.Lock()
	s.deliveries = append(s.deliveries, d...)
	s.mu.Unlock()
	return nil
}

func TestWebhookSink_Name(t *testing.T) {
	sink := NewWebhookSink(nil, "node-1")
	if sink.Name() != "webhook" {
		t.Errorf("expected name 'webhook', got %q", sink.Name())
	}
}

func TestWebhookSink_AcceptAll(t *testing.T) {
	sink := NewWebhookSink(nil, "node-1")
	meta := events.EventMeta{Category: "order", Name: "order.created"}
	if !sink.Accept(meta) {
		t.Error("expected Accept to return true for all events")
	}
}

func TestWebhookSink_Handle_EnqueuesEvent(t *testing.T) {
	store := &recordingStore{
		endpoints: []wh.Endpoint{
			{ID: "ep-1", URL: "https://example.com/hook", EventTypes: "order.*", Active: true, Secret: "s"},
		},
	}
	cfg := wh.DefaultConfig()
	engine := wh.NewEngine(store, cfg)

	sink := NewWebhookSink(engine, "node-test")
	meta := events.EventMeta{Category: "order", Name: "order.created"}
	evt := &events.NewOrder{OrderID: "ord-123"}

	err := sink.Handle(context.Background(), meta, evt)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(store.deliveries))
	}

	d := store.deliveries[0]
	if d.EndpointID != "ep-1" {
		t.Errorf("expected endpoint ep-1, got %s", d.EndpointID)
	}
	if d.EventType != "order.created" {
		t.Errorf("expected event type order.created, got %s", d.EventType)
	}

	var ce wh.CloudEvent
	if err := json.Unmarshal([]byte(d.Payload), &ce); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if ce.Source != "/tenants/node-test" {
		t.Errorf("expected source /tenants/node-test, got %s", ce.Source)
	}
}

func TestWebhookSink_Handle_FiltersByEventType(t *testing.T) {
	store := &recordingStore{
		endpoints: []wh.Endpoint{
			{ID: "ep-1", URL: "https://example.com/hook", EventTypes: "dispute.*", Active: true, Secret: "s"},
		},
	}
	cfg := wh.DefaultConfig()
	engine := wh.NewEngine(store, cfg)

	sink := NewWebhookSink(engine, "node-test")
	meta := events.EventMeta{Category: "order", Name: "order.created"}
	evt := &events.NewOrder{OrderID: "ord-123"}

	_ = sink.Handle(context.Background(), meta, evt)

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.deliveries) != 0 {
		t.Errorf("expected 0 deliveries (event doesn't match filter), got %d", len(store.deliveries))
	}
}

package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/mobazha/mobazha/pkg/events"
)

func newTestTGChannel(id string, serverURL string, filter string) ChannelConfig {
	return ChannelConfig{
		ID:          id,
		Type:        ChannelTelegram,
		Name:        "Test TG " + id,
		Enabled:     true,
		EventFilter: filter,
		Settings: map[string]string{
			"bot_token": "test-token",
			"chat_id":   "12345",
			"base_url":  serverURL,
		},
	}
}

func TestChannelNotificationSink_Accept(t *testing.T) {
	sink := NewChannelNotificationSink([]ChannelConfig{
		{
			ID: "ch1", Type: ChannelTelegram, Enabled: true,
			EventFilter: "order.*",
			Settings:    map[string]string{"bot_token": "t", "chat_id": "1"},
		},
	}, "test-node")

	if !sink.Accept(events.EventMeta{Name: "order.created", Category: "order"}) {
		t.Error("expected Accept=true for order.created")
	}
	if sink.Accept(events.EventMeta{Name: "chat.message", Category: "chat"}) {
		t.Error("expected Accept=false for chat.message (not in filter)")
	}
}

func TestChannelNotificationSink_Accept_NoEnabledChannels(t *testing.T) {
	sink := NewChannelNotificationSink([]ChannelConfig{
		{ID: "ch1", Type: ChannelTelegram, Enabled: false, Settings: map[string]string{}},
	}, "test-node")

	if sink.Accept(events.EventMeta{Name: "order.created"}) {
		t.Error("expected Accept=false when all channels disabled")
	}
}

func TestChannelNotificationSink_Accept_EmptyFilter_AcceptsAll(t *testing.T) {
	sink := NewChannelNotificationSink([]ChannelConfig{
		{ID: "ch1", Type: ChannelTelegram, Enabled: true, EventFilter: "", Settings: map[string]string{}},
	}, "test-node")

	if !sink.Accept(events.EventMeta{Name: "anything.here"}) {
		t.Error("expected Accept=true for empty filter (accept all)")
	}
}

func TestChannelNotificationSink_Handle_MultiChannel(t *testing.T) {
	var mu sync.Mutex
	var received []map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		received = append(received, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer server.Close()

	sink := NewChannelNotificationSink([]ChannelConfig{
		newTestTGChannel("ch1", server.URL, "order.*"),
		newTestTGChannel("ch2", server.URL, ""), // accepts all
	}, "test-node")

	meta := events.EventMeta{Name: "order.created", Category: "order"}
	event := &events.NewOrder{OrderID: "QmTest123", BuyerName: "alice", Title: "Widget"}

	err := sink.Handle(context.Background(), meta, event)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("expected 2 messages (one per channel), got %d", len(received))
	}
}

func TestChannelNotificationSink_Handle_FilteredOut(t *testing.T) {
	var mu sync.Mutex
	var count int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer server.Close()

	sink := NewChannelNotificationSink([]ChannelConfig{
		newTestTGChannel("ch1", server.URL, "order.*"),
	}, "test-node")

	meta := events.EventMeta{Name: "cart.updated", Category: "cart"}
	event := &events.ShoppingCartUpdate{ItemsCount: 1}

	_ = sink.Handle(context.Background(), meta, event)

	mu.Lock()
	defer mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 messages for filtered-out event, got %d", count)
	}
}

func TestChannelNotificationSink_AddChannel(t *testing.T) {
	sink := NewChannelNotificationSink(nil, "test-node")

	var persisted []ChannelConfig
	sink.SetOnChanged(func(channels []ChannelConfig) {
		persisted = channels
	})

	cfg, err := sink.AddChannel(ChannelConfig{
		Type:     ChannelTelegram,
		Name:     "My TG",
		Enabled:  true,
		Settings: map[string]string{"bot_token": "tok", "chat_id": "123"},
	})
	if err != nil {
		t.Fatalf("AddChannel error: %v", err)
	}
	if cfg.ID == "" {
		t.Error("expected auto-generated ID")
	}

	channels := sink.ListChannels()
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if len(persisted) != 1 {
		t.Fatal("onChanged not called")
	}
}

func TestChannelNotificationSink_AddChannel_UnsupportedType(t *testing.T) {
	sink := NewChannelNotificationSink(nil, "test-node")
	_, err := sink.AddChannel(ChannelConfig{Type: "unknown"})
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

func TestChannelNotificationSink_AddChannel_MaxLimit(t *testing.T) {
	existing := make([]ChannelConfig, maxChannelsPerNode)
	for i := range existing {
		existing[i] = ChannelConfig{
			ID: fmt.Sprintf("ch%d", i), Type: ChannelTelegram, Enabled: true,
			Settings: map[string]string{},
		}
	}
	sink := NewChannelNotificationSink(existing, "test-node")

	_, err := sink.AddChannel(ChannelConfig{
		Type: ChannelTelegram, Name: "One too many",
		Settings: map[string]string{"bot_token": "t", "chat_id": "1"},
	})
	if err == nil {
		t.Error("expected error when exceeding max channels")
	}
}

func TestChannelNotificationSink_UpdateChannel(t *testing.T) {
	sink := NewChannelNotificationSink([]ChannelConfig{
		{ID: "ch1", Type: ChannelTelegram, Name: "Old", Enabled: false,
			Settings: map[string]string{"bot_token": "tok"}},
	}, "test-node")

	err := sink.UpdateChannel("ch1", ChannelConfig{
		Type: ChannelTelegram, Name: "New", Enabled: true,
	})
	if err != nil {
		t.Fatalf("UpdateChannel error: %v", err)
	}

	channels := sink.ListChannels()
	if channels[0].Name != "New" || !channels[0].Enabled {
		t.Errorf("channel not updated: %+v", channels[0])
	}
	if channels[0].Settings["bot_token"] != "tok" {
		t.Error("settings should be preserved when update.Settings is nil")
	}
}

func TestChannelNotificationSink_UpdateChannel_MergeSettings(t *testing.T) {
	sink := NewChannelNotificationSink([]ChannelConfig{
		{ID: "ch1", Type: ChannelTelegram, Name: "TG", Enabled: true,
			Settings: map[string]string{"bot_token": "secret", "chat_id": "old_id"}},
	}, "test-node")

	err := sink.UpdateChannel("ch1", ChannelConfig{
		Type: ChannelTelegram, Name: "TG Updated", Enabled: true,
		Settings: map[string]string{"chat_id": "new_id"},
	})
	if err != nil {
		t.Fatalf("UpdateChannel error: %v", err)
	}

	ch := sink.ListChannels()[0]
	if ch.Settings["bot_token"] != "secret" {
		t.Error("bot_token should be preserved via merge when not in update")
	}
	if ch.Settings["chat_id"] != "new_id" {
		t.Errorf("chat_id should be updated to new_id, got %q", ch.Settings["chat_id"])
	}
}

func TestChannelNotificationSink_UpdateChannel_NotFound(t *testing.T) {
	sink := NewChannelNotificationSink(nil, "test-node")
	err := sink.UpdateChannel("nonexistent", ChannelConfig{})
	if err == nil {
		t.Error("expected error for missing channel")
	}
}

func TestChannelNotificationSink_RemoveChannel(t *testing.T) {
	sink := NewChannelNotificationSink([]ChannelConfig{
		{ID: "ch1", Type: ChannelTelegram, Settings: map[string]string{}},
		{ID: "ch2", Type: ChannelTelegram, Settings: map[string]string{}},
	}, "test-node")

	err := sink.RemoveChannel("ch1")
	if err != nil {
		t.Fatalf("RemoveChannel error: %v", err)
	}

	channels := sink.ListChannels()
	if len(channels) != 1 || channels[0].ID != "ch2" {
		t.Errorf("unexpected channels after removal: %+v", channels)
	}
}

func TestChannelNotificationSink_RemoveChannel_NotFound(t *testing.T) {
	sink := NewChannelNotificationSink(nil, "test-node")
	err := sink.RemoveChannel("nonexistent")
	if err == nil {
		t.Error("expected error for missing channel")
	}
}

func TestChannelNotificationSink_TestChannel(t *testing.T) {
	var received bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer server.Close()

	sink := NewChannelNotificationSink([]ChannelConfig{
		newTestTGChannel("ch1", server.URL, ""),
	}, "test-node")

	err := sink.TestChannel("ch1")
	if err != nil {
		t.Fatalf("TestChannel error: %v", err)
	}
	if !received {
		t.Error("expected test message to be sent")
	}
}

func TestChannelNotificationSink_TestChannel_NotFound(t *testing.T) {
	sink := NewChannelNotificationSink(nil, "test-node")
	err := sink.TestChannel("nonexistent")
	if err == nil {
		t.Error("expected error for missing channel")
	}
}

func TestChannelNotificationSink_SupportedTypes(t *testing.T) {
	sink := NewChannelNotificationSink(nil, "test-node")
	types := sink.SupportedTypes()
	if len(types) == 0 {
		t.Fatal("expected at least one supported type")
	}
	if types[0].Type != ChannelTelegram {
		t.Errorf("first type = %s, want telegram", types[0].Type)
	}
	if len(types[0].Fields) < 2 {
		t.Error("telegram should have at least bot_token and chat_id fields")
	}
}

package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/internal/notifier"
)

var sensitiveSettingsKeys = map[string]bool{
	"bot_token": true,
	"api_key":   true,
}

func sanitizeChannelForResponse(ch notifier.ChannelConfig) map[string]interface{} {
	settings := make(map[string]interface{}, len(ch.Settings))
	for k, v := range ch.Settings {
		if sensitiveSettingsKeys[k] {
			settings[k] = v != ""
		} else {
			settings[k] = v
		}
	}
	return map[string]interface{}{
		"id":           ch.ID,
		"type":         ch.Type,
		"name":         ch.Name,
		"enabled":      ch.Enabled,
		"event_filter": ch.EventFilter,
		"settings":     settings,
	}
}

func (g *Gateway) handleGETNotificationChannels(w http.ResponseWriter, r *http.Request) {
	sink := getNotifierSink(r)
	if sink == nil {
		http.Error(w, "Notification channels not available", http.StatusNotImplemented)
		return
	}

	channels := sink.ListChannels()
	sanitized := make([]map[string]interface{}, len(channels))
	for i, ch := range channels {
		sanitized[i] = sanitizeChannelForResponse(ch)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sanitized)
}

func (g *Gateway) handlePOSTNotificationChannel(w http.ResponseWriter, r *http.Request) {
	sink := getNotifierSink(r)
	if sink == nil {
		http.Error(w, "Notification channels not available", http.StatusNotImplemented)
		return
	}

	var cfg notifier.ChannelConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	created, err := sink.AddChannel(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sanitizeChannelForResponse(created))
}

func (g *Gateway) handlePUTNotificationChannel(w http.ResponseWriter, r *http.Request) {
	sink := getNotifierSink(r)
	if sink == nil {
		http.Error(w, "Notification channels not available", http.StatusNotImplemented)
		return
	}

	id := mux.Vars(r)["id"]

	var cfg notifier.ChannelConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := sink.UpdateChannel(id, cfg); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (g *Gateway) handleDELETENotificationChannel(w http.ResponseWriter, r *http.Request) {
	sink := getNotifierSink(r)
	if sink == nil {
		http.Error(w, "Notification channels not available", http.StatusNotImplemented)
		return
	}

	id := mux.Vars(r)["id"]
	if err := sink.RemoveChannel(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (g *Gateway) handlePOSTNotificationChannelTest(w http.ResponseWriter, r *http.Request) {
	sink := getNotifierSink(r)
	if sink == nil {
		http.Error(w, "Notification channels not available", http.StatusNotImplemented)
		return
	}

	id := mux.Vars(r)["id"]
	if err := sink.TestChannel(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "test message sent"})
}

func (g *Gateway) handleGETNotificationChannelTypes(w http.ResponseWriter, r *http.Request) {
	sink := getNotifierSink(r)
	if sink == nil {
		http.Error(w, "Notification channels not available", http.StatusNotImplemented)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sink.SupportedTypes())
}

func getNotifierSink(r *http.Request) *notifier.ChannelNotificationSink {
	node := getNodeService(r)
	if provider, ok := node.(interface {
		NotifierSink() *notifier.ChannelNotificationSink
	}); ok {
		return provider.NotifierSink()
	}
	return nil
}

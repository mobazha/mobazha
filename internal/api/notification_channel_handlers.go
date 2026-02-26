package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/internal/notifier"
	"github.com/mobazha/mobazha3.0/pkg/events"
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

	cats := extractEventCategories()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"channel_types":    sink.SupportedTypes(),
		"event_categories": cats,
	})
}

func extractEventCategories() []string {
	seen := map[string]bool{}
	var cats []string
	for _, m := range events.AllMeta() {
		if !seen[m.Category] {
			seen[m.Category] = true
			cats = append(cats, m.Category)
		}
	}
	return cats
}

func (g *Gateway) handlePOSTDetectTelegramChat(w http.ResponseWriter, r *http.Request) {
	sink := getNotifierSink(r)
	if sink == nil {
		http.Error(w, "Notification channels not available", http.StatusNotImplemented)
		return
	}

	var req struct {
		BotToken string `json:"bot_token"`
		BaseURL  string `json:"base_url,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.BotToken == "" {
		http.Error(w, "bot_token is required", http.StatusBadRequest)
		return
	}
	if req.BaseURL != "" && !isAllowedTelegramBaseURL(req.BaseURL) {
		http.Error(w, "base_url must be empty or a valid Telegram API URL", http.StatusBadRequest)
		return
	}

	sender := sink.TelegramSender()
	if sender == nil {
		http.Error(w, "Telegram sender not available", http.StatusNotImplemented)
		return
	}

	chats, err := sender.DetectChats(req.BotToken, req.BaseURL)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "telegram API error") {
			http.Error(w, errMsg, http.StatusBadGateway)
		} else {
			http.Error(w, "Failed to communicate with Telegram API", http.StatusBadGateway)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"chats": chats})
}

func isAllowedTelegramBaseURL(u string) bool {
	allowed := []string{
		"https://api.telegram.org",
		"http://127.0.0.1",
		"http://localhost",
	}
	for _, base := range allowed {
		if u == base || strings.HasPrefix(u, base+"/") || strings.HasPrefix(u, base+":") {
			return true
		}
	}
	return false
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

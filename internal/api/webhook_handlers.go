package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	wh "github.com/mobazha/mobazha3.0/pkg/webhook"
)

func getWebhookProvider(r *http.Request) (contracts.WebhookProvider, bool) {
	wp, ok := getNodeService(r).(contracts.WebhookProvider)
	return wp, ok
}

func (g *Gateway) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	wp, ok := getWebhookProvider(r)
	if !ok {
		http.Error(w, "Webhooks not available", http.StatusNotImplemented)
		return
	}
	store := wp.WebhookStore()
	engine := wp.WebhookEngine()

	var req struct {
		URL        string `json:"url"`
		EventTypes string `json:"event_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	if _, err := url.ParseRequestURI(req.URL); err != nil {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}
	if req.EventTypes == "" {
		req.EventTypes = "order.*,dispute.*,chat.message"
	}

	if err := engine.CheckEndpointQuota(); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	ep := &wh.Endpoint{
		URL:        req.URL,
		EventTypes: req.EventTypes,
		Active:     true,
	}
	if err := store.CreateEndpoint(ep); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":          ep.ID,
		"url":         ep.URL,
		"secret":      ep.Secret,
		"event_types": ep.EventTypes,
		"active":      ep.Active,
		"created_at":  ep.CreatedAt,
	})
}

func (g *Gateway) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	wp, ok := getWebhookProvider(r)
	if !ok {
		http.Error(w, "Webhooks not available", http.StatusNotImplemented)
		return
	}

	endpoints, err := wp.WebhookStore().ListEndpoints()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoints)
}

func (g *Gateway) handleGetWebhook(w http.ResponseWriter, r *http.Request) {
	wp, ok := getWebhookProvider(r)
	if !ok {
		http.Error(w, "Webhooks not available", http.StatusNotImplemented)
		return
	}

	id := mux.Vars(r)["id"]
	ep, err := wp.WebhookStore().GetEndpoint(id)
	if err != nil {
		if errors.Is(err, wh.ErrEndpointNotFound) {
			http.Error(w, "Not found", http.StatusNotFound)
		} else {
			log.Errorf("GetEndpoint %s: %v", id, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ep)
}

func (g *Gateway) handleUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	wp, ok := getWebhookProvider(r)
	if !ok {
		http.Error(w, "Webhooks not available", http.StatusNotImplemented)
		return
	}
	store := wp.WebhookStore()

	id := mux.Vars(r)["id"]
	if _, err := store.GetEndpoint(id); err != nil {
		if errors.Is(err, wh.ErrEndpointNotFound) {
			http.Error(w, "Not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	updates := make(map[string]interface{})
	if v, ok := req["url"]; ok {
		if u, ok := v.(string); ok {
			if _, err := url.ParseRequestURI(u); err != nil {
				http.Error(w, "Invalid URL format", http.StatusBadRequest)
				return
			}
			updates["url"] = u
		}
	}
	if v, ok := req["event_types"]; ok {
		if s, ok := v.(string); ok {
			updates["event_types"] = s
		}
	}
	if v, ok := req["active"]; ok {
		if b, ok := v.(bool); ok {
			updates["active"] = b
		}
	}

	if len(updates) == 0 {
		http.Error(w, "No valid fields to update", http.StatusBadRequest)
		return
	}

	if err := store.UpdateEndpoint(id, updates); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (g *Gateway) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	wp, ok := getWebhookProvider(r)
	if !ok {
		http.Error(w, "Webhooks not available", http.StatusNotImplemented)
		return
	}
	store := wp.WebhookStore()

	id := mux.Vars(r)["id"]
	if _, err := store.GetEndpoint(id); err != nil {
		if errors.Is(err, wh.ErrEndpointNotFound) {
			http.Error(w, "Not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	if err := store.DeleteEndpoint(id); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (g *Gateway) handleListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	wp, ok := getWebhookProvider(r)
	if !ok {
		http.Error(w, "Webhooks not available", http.StatusNotImplemented)
		return
	}
	store := wp.WebhookStore()

	endpointID := mux.Vars(r)["id"]
	if _, err := store.GetEndpoint(endpointID); err != nil {
		if errors.Is(err, wh.ErrEndpointNotFound) {
			http.Error(w, "Not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	status := r.URL.Query().Get("status")

	deliveries, total, err := store.ListDeliveries(endpointID, status, limit, offset)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deliveries": deliveries,
		"total":      total,
	})
}

func (g *Gateway) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	wp, ok := getWebhookProvider(r)
	if !ok {
		http.Error(w, "Webhooks not available", http.StatusNotImplemented)
		return
	}
	store := wp.WebhookStore()
	engine := wp.WebhookEngine()

	endpointID := mux.Vars(r)["id"]
	if _, err := store.GetEndpoint(endpointID); err != nil {
		if errors.Is(err, wh.ErrEndpointNotFound) {
			http.Error(w, "Not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	nodeID := getIdentityService(r).GetNodeID()
	testData := map[string]string{
		"message": "This is a test webhook event from Mobazha",
	}
	payload, err := wh.BuildCloudEvent(nodeID, "test.ping", testData)
	if err != nil {
		log.Errorf("Failed to build test CloudEvent: %v", err)
		http.Error(w, "Failed to build test event", http.StatusInternalServerError)
		return
	}
	engine.Enqueue("test.ping", payload)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "test event enqueued"})
}

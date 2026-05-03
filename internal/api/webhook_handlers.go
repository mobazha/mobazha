//go:build !private_distribution

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/response"
	wh "github.com/mobazha/mobazha3.0/pkg/webhook"
)

func getWebhookProvider(r *http.Request) (contracts.WebhookProvider, bool) {
	wp, ok := getNodeService(r).(contracts.WebhookProvider)
	return wp, ok
}

func (g *Gateway) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	wp, ok := getWebhookProvider(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Webhooks not available")
		return
	}
	store := wp.WebhookStore()
	engine := wp.WebhookEngine()

	var req struct {
		URL        string `json:"url"`
		EventTypes string `json:"event_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}
	if req.URL == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "url is required")
		return
	}
	if _, err := url.ParseRequestURI(req.URL); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid URL format")
		return
	}
	if req.EventTypes == "" {
		req.EventTypes = "order.*,dispute.*,chat.message"
	}

	if err := engine.CheckEndpointQuota(); err != nil {
		response.Error(w, http.StatusConflict, response.CodeConflict, "Maximum number of webhook endpoints reached")
		return
	}

	ep := &wh.Endpoint{
		URL:        req.URL,
		EventTypes: req.EventTypes,
		Active:     true,
	}
	if err := store.CreateEndpoint(ep); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
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
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Webhooks not available")
		return
	}

	endpoints, err := wp.WebhookStore().ListEndpoints()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoints)
}

func (g *Gateway) handleGetWebhook(w http.ResponseWriter, r *http.Request) {
	wp, ok := getWebhookProvider(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Webhooks not available")
		return
	}

	id := chi.URLParam(r, "id")
	ep, err := wp.WebhookStore().GetEndpoint(id)
	if err != nil {
		if errors.Is(err, wh.ErrEndpointNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "Not found")
		} else {
			log.Errorf("GetEndpoint %s: %v", id, err)
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ep)
}

func (g *Gateway) handleUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	wp, ok := getWebhookProvider(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Webhooks not available")
		return
	}
	store := wp.WebhookStore()

	id := chi.URLParam(r, "id")
	if _, err := store.GetEndpoint(id); err != nil {
		if errors.Is(err, wh.ErrEndpointNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "Not found")
		} else {
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
		}
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	updates := make(map[string]interface{})
	if v, ok := req["url"]; ok {
		if u, ok := v.(string); ok {
			if _, err := url.ParseRequestURI(u); err != nil {
				response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid URL format")
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
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "No valid fields to update")
		return
	}

	if err := store.UpdateEndpoint(id, updates); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (g *Gateway) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	wp, ok := getWebhookProvider(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Webhooks not available")
		return
	}
	store := wp.WebhookStore()

	id := chi.URLParam(r, "id")
	if _, err := store.GetEndpoint(id); err != nil {
		if errors.Is(err, wh.ErrEndpointNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "Not found")
		} else {
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
		}
		return
	}

	if err := store.DeleteEndpoint(id); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (g *Gateway) handleListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	wp, ok := getWebhookProvider(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Webhooks not available")
		return
	}
	store := wp.WebhookStore()

	endpointID := chi.URLParam(r, "id")
	if _, err := store.GetEndpoint(endpointID); err != nil {
		if errors.Is(err, wh.ErrEndpointNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "Not found")
		} else {
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
		}
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	status := r.URL.Query().Get("status")

	deliveries, total, err := store.ListDeliveries(endpointID, status, limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
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
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Webhooks not available")
		return
	}
	store := wp.WebhookStore()
	engine := wp.WebhookEngine()

	endpointID := chi.URLParam(r, "id")
	if _, err := store.GetEndpoint(endpointID); err != nil {
		if errors.Is(err, wh.ErrEndpointNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "Not found")
		} else {
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
		}
		return
	}

	idSvc := getIdentityService(r)
	if idSvc == nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Identity service not available")
		return
	}
	nodeID := idSvc.GetNodeID()
	testData := map[string]string{
		"message": "This is a test webhook event from Mobazha",
	}
	payload, err := wh.BuildCloudEvent(nodeID, "test.ping", testData)
	if err != nil {
		log.Errorf("Failed to build test CloudEvent: %v", err)
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Failed to build test event")
		return
	}
	engine.Enqueue("test.ping", payload)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "test event enqueued"})
}

package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// getGuestOrderService extracts the GuestOrderService from the request's NodeService.
// Returns nil if Guest Checkout is not enabled.
func getGuestOrderService(r *http.Request) contracts.GuestOrderService {
	ns := getNodeService(r)
	return ns.GuestOrder()
}

// handlePOSTGuestOrder creates a new guest order (public — anonymous buyer).
// POST /v1/guest/orders
func (g *Gateway) handlePOSTGuestOrder(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	var req contracts.CreateGuestOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	if len(req.Items) == 0 {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "At least one item is required")
		return
	}
	if req.PaymentCoin == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "paymentCoin is required")
		return
	}

	resp, err := svc.CreateGuestOrder(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	response.Created(w, resp)
}

// handleGETGuestOrder returns the public status of a guest order by token (anonymous buyer).
// GET /v1/guest/orders/{token}
func (g *Gateway) handleGETGuestOrder(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	token := mux.Vars(r)["token"]
	if token == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Order token is required")
		return
	}

	resp, err := svc.GetGuestOrderStatus(r.Context(), token)
	if err != nil {
		response.Error(w, http.StatusNotFound, response.CodeNotFound, "Order not found")
		return
	}

	response.Success(w, resp)
}

// handleGETGuestOrders lists guest orders for the seller (requires auth).
// GET /v1/guest/orders?state=funded&page=0&pageSize=20
func (g *Gateway) handleGETGuestOrders(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	filter := contracts.GuestOrderFilter{
		Page:     parseIntQuery(r, "page", 0),
		PageSize: parseIntQuery(r, "pageSize", 20),
	}
	if stateStr := r.URL.Query().Get("state"); stateStr != "" {
		if s, ok := models.ParseGuestOrderState(stateStr); ok {
			filter.State = &s
		}
	}

	orders, total, err := svc.ListGuestOrders(r.Context(), filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Failed to list guest orders")
		return
	}

	response.List(w, orders, response.Meta{
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
	})
}

// handleShipGuestOrder marks a guest order as shipped with tracking info.
// PUT /v1/guest/orders/{token}/ship
func (g *Gateway) handleShipGuestOrder(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	token := mux.Vars(r)["token"]
	if token == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Order token is required")
		return
	}

	var body struct {
		TrackingNumber string `json:"trackingNumber"`
		Carrier        string `json:"carrier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	if err := svc.ShipGuestOrder(r.Context(), token, body.TrackingNumber, body.Carrier); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.NoContent(w)
}

// handleCompleteGuestOrder manually completes a funded/shipped guest order.
// PUT /v1/guest/orders/{token}/complete
func (g *Gateway) handleCompleteGuestOrder(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	token := mux.Vars(r)["token"]
	if token == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Order token is required")
		return
	}

	if err := svc.CompleteGuestOrder(r.Context(), token); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.NoContent(w)
}

// handleGETGuestCheckoutSettings returns the guest checkout configuration.
// GET /v1/settings/guest-checkout
func (g *Gateway) handleGETGuestCheckoutSettings(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	cfg, err := svc.GetGuestCheckoutConfig(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	response.Success(w, cfg)
}

// handlePUTGuestCheckoutSettings updates the guest checkout configuration.
// PUT /v1/settings/guest-checkout
func (g *Gateway) handlePUTGuestCheckoutSettings(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	var req models.GuestCheckoutConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	if err := svc.SaveGuestCheckoutConfig(r.Context(), &req); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	response.Success(w, req)
}

func parseIntQuery(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

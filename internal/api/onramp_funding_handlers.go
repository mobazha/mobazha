// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	corePmt "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/pkg/contracts"
	responsePkg "github.com/mobazha/mobazha/pkg/response"
)

// onrampInitiateBody is the client body for initiating (or resuming) an onramp
// purchase against the order's current payable attempt. The buyer identity
// comes from the authenticated request context, never from this body.
type onrampInitiateBody struct {
	ProviderID           string `json:"providerID"`
	FiatCurrency         string `json:"fiatCurrency"`
	IdempotencyKey       string `json:"idempotencyKey,omitempty"`
	DeliverToBuyerWallet bool   `json:"deliverToBuyerWallet,omitempty"`
	BuyerWalletAddress   string `json:"buyerWalletAddress,omitempty"`
}

// handlePOSTOrderPaymentSessionOnramp initiates or resumes an onramp funding
// source for the order's current payable attempt (ADR-019).
//
// POST /v1/orders/{orderID}/payment-session/onramp
//
// Idempotent: repeating the call with the same idempotency key (default
// "primary") returns the existing purchase — a buyer who closes the page and
// returns does not create a second onramp order. The response is the same
// onrampFunding view the payment session projection exposes; the session's
// funded/verified status continues to come only from the on-chain observation.
func (g *Gateway) handlePOSTOrderPaymentSessionOnramp(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "orderID is required")
		return
	}
	svc := getNodeService(r).OnrampFunding()
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail,
			"onramp funding subsystem not available")
		return
	}

	identity := GetAuthIdentity(r.Context())
	if identity == nil || strings.TrimSpace(identity.UserID) == "" {
		responsePkg.Error(w, http.StatusUnauthorized, responsePkg.CodeUnauthorized,
			"onramp funding requires an authenticated buyer")
		return
	}

	var body onrampInitiateBody
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "invalid onramp funding request")
		return
	}
	if strings.TrimSpace(body.ProviderID) == "" || strings.TrimSpace(body.FiatCurrency) == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation,
			"providerID and fiatCurrency are required")
		return
	}

	view, err := svc.InitiateOrResumeForOrder(r.Context(), orderID, contracts.OnrampFundingInitiation{
		Buyer:                contracts.BuyerRef{Subject: identity.UserID},
		ProviderID:           body.ProviderID,
		FiatCurrency:         body.FiatCurrency,
		ClientIP:             remoteIP(r),
		IdempotencyKey:       body.IdempotencyKey,
		DeliverToBuyerWallet: body.DeliverToBuyerWallet,
		BuyerWalletAddress:   body.BuyerWalletAddress,
	})
	if err != nil {
		onrampFundingErrorResponse(w, err)
		return
	}
	responsePkg.Success(w, view)
}

// handlePOSTOrderPaymentSessionOnrampRefresh polls the provider for the
// order's in-flight onramp purchase and persists the transition.
//
// POST /v1/orders/{orderID}/payment-session/onramp/refresh
//
// Returns the latest durable lifecycle record (including a direct-delivery
// terminal outcome), or data:null when no purchase has ever existed.
func (g *Gateway) handlePOSTOrderPaymentSessionOnrampRefresh(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "orderID is required")
		return
	}
	svc := getNodeService(r).OnrampFunding()
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail,
			"onramp funding subsystem not available")
		return
	}
	view, err := svc.RefreshForOrder(r.Context(), orderID)
	if err != nil {
		onrampFundingErrorResponse(w, err)
		return
	}
	responsePkg.Success(w, view)
}

// handleGETOrderPaymentSessionOnrampProviders lists the onramp providers
// whose capability gate is open for the order's frozen settlement rail.
//
// GET /v1/orders/{orderID}/payment-session/onramp/providers
//
// An empty list means the affordance must not render; clients must never
// assume a specific vendor (RFC-0012 Proposal 4).
func (g *Gateway) handleGETOrderPaymentSessionOnrampProviders(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "orderID is required")
		return
	}
	svc := getNodeService(r).OnrampFunding()
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail,
			"onramp funding subsystem not available")
		return
	}
	options, err := svc.ListProvidersForOrder(r.Context(), orderID)
	if err != nil {
		onrampFundingErrorResponse(w, err)
		return
	}
	responsePkg.Success(w, options)
}

// orderIDPathParam is the standard order path parameter ({descriptiveID} per
// docs/API_DESIGN_STANDARD.md), kept as a named constant so route templates
// below compose from it.
const orderIDPathParam = "{orderID}"

// registerOrderPaymentSessionOnrampPost registers
// POST /v1/orders/{orderID}/payment-session/onramp.
func (g *Gateway) registerOrderPaymentSessionOnrampPost(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-payment-session-onramp",
		Method:      http.MethodPost,
		Path:        "/v1/orders/" + orderIDPathParam + "/payment-session/onramp",
		Summary:     "Initiate or resume onramp funding",
		Description: "Initiates (or idempotently resumes) an onramp purchase funding the order's current " +
			"payable attempt. The settlement asset, network, and amount are fixed by the frozen attempt " +
			"terms; the purchase is a funding source, not a settlement mode, and funded/verified still " +
			"come only from the on-chain funding observation.",
		Tags:     []string{"orders", "payments"},
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/payment-session/onramp"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOrderPaymentSessionOnramp(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerOrderPaymentSessionOnrampRefreshPost registers
// POST /v1/orders/{orderID}/payment-session/onramp/refresh.
func (g *Gateway) registerOrderPaymentSessionOnrampRefreshPost(api huma.API) {
	type in struct {
		OrderID string `path:"orderID" doc:"Order ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-payment-session-onramp-refresh",
		Method:      http.MethodPost,
		Path:        "/v1/orders/" + orderIDPathParam + "/payment-session/onramp/refresh",
		Summary:     "Refresh onramp funding status",
		Description: "Polls the onramp provider for the order's in-flight purchase, persists the " +
			"transition, and returns its latest durable lifecycle view (null only when no purchase exists).",
		Tags:     []string{"orders", "payments"},
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/payment-session/onramp/refresh"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, nil, map[string]string{"orderID": hi.OrderID})
		rr := httptest.NewRecorder()
		g.handlePOSTOrderPaymentSessionOnrampRefresh(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerOrderPaymentSessionOnrampProvidersGet registers
// GET /v1/orders/{orderID}/payment-session/onramp/providers.
func (g *Gateway) registerOrderPaymentSessionOnrampProvidersGet(api huma.API) {
	type in struct {
		OrderID string `path:"orderID" doc:"Order ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-get-payment-session-onramp-providers",
		Method:      http.MethodGet,
		Path:        "/v1/orders/" + orderIDPathParam + "/payment-session/onramp/providers",
		Summary:     "List onramp providers for this order",
		Description: "Enumerates the reviewed onramp providers whose capability gate is open for the " +
			"order's frozen settlement rail. An empty list means onramp funding must not be offered; " +
			"clients must never assume a specific vendor.",
		Tags:     []string{"orders", "payments"},
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/payment-session/onramp/providers"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"orderID": hi.OrderID})
		rr := httptest.NewRecorder()
		g.handleGETOrderPaymentSessionOnrampProviders(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// onrampFundingErrorResponse maps orchestration errors to structured responses.
func onrampFundingErrorResponse(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, corePmt.ErrOnrampAttemptNotFound):
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound,
			"order or payable payment attempt not found")
	case errors.Is(err, corePmt.ErrOnrampAttemptNotReady):
		responsePkg.Error(w, http.StatusConflict, responsePkg.CodeConflict,
			"the order has no frozen, payable funding target yet")
	case errors.Is(err, contracts.ErrOnrampProviderNotFound):
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation,
			"unknown onramp provider")
	case errors.Is(err, contracts.ErrOnrampCapabilityClosed):
		responsePkg.Error(w, http.StatusConflict, responsePkg.CodeConflict,
			"onramp funding is not available for this order's settlement rail")
	case errors.Is(err, contracts.ErrOnrampTermsNotFrozen),
		errors.Is(err, contracts.ErrOnrampDeliveryUnbound),
		errors.Is(err, contracts.ErrOnrampMissingIdemponent):
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
	default:
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
	}
}

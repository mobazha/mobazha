package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/response"
	"github.com/stretchr/testify/require"
)

func orderSupplyQuoteTestServer(t *testing.T, node *mockNode) *httptest.Server {
	t.Helper()
	gateway := &Gateway{
		config:            &GatewayConfig{},
		guestOrderLimiter: newRateLimiter(1000, time.Hour),
	}
	outer := chi.NewMux()
	outer.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
			ctx = WithAuthIdentity(ctx, &AuthIdentity{
				UserID:  "test-admin",
				IsAdmin: true,
			})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	outer.Mount("/", gateway.newV1Router(false, false))

	ts := httptest.NewServer(outer)
	t.Cleanup(ts.Close)
	return ts
}

func orderSupplyQuoteDoReq(t *testing.T, ts *httptest.Server, body []byte) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/orders/supply-quote", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, respBody
}

func TestPOSTOrderSupplyQuote_Valid(t *testing.T) {
	var captured contracts.QuoteCheckoutSupplyRequest
	node := &mockNode{
		quoteCheckoutSupplyFunc: func(_ context.Context, req contracts.QuoteCheckoutSupplyRequest) (*contracts.CheckoutSupplyQuoteResponse, error) {
			captured = req
			return &contracts.CheckoutSupplyQuoteResponse{
				CanSell: true,
				Items: []contracts.CheckoutSupplyQuoteItem{{
					ListingSlug: "test-item",
					Quantity:    1,
					Status:      contracts.SupplyAvailabilityAvailable,
					Available:   true,
				}},
			}, nil
		},
	}
	ts := orderSupplyQuoteTestServer(t, node)

	body, err := json.Marshal(contracts.QuoteCheckoutSupplyRequest{
		Items: []contracts.CheckoutSupplyItemRequest{{
			ListingSlug: "test-item",
			ListingHash: "QmHash",
			Quantity:    1,
		}},
	})
	require.NoError(t, err)

	resp, respBody := orderSupplyQuoteDoReq(t, ts, body)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, captured.Items, 1)
	require.Equal(t, "test-item", captured.Items[0].ListingSlug)

	var envelope struct {
		Data contracts.CheckoutSupplyQuoteResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(respBody, &envelope))
	require.True(t, envelope.Data.CanSell)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, contracts.SupplyAvailabilityAvailable, envelope.Data.Items[0].Status)
	require.NotContains(t, string(respBody), "supplyKind")
}

func TestPOSTOrderSupplyQuote_EmptyItems(t *testing.T) {
	node := &mockNode{}
	ts := orderSupplyQuoteTestServer(t, node)

	body, err := json.Marshal(contracts.QuoteCheckoutSupplyRequest{})
	require.NoError(t, err)

	resp, respBody := orderSupplyQuoteDoReq(t, ts, body)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	guestAssertErrorCode(t, respBody, response.CodeBadRequest)
}

func TestPOSTOrderSupplyQuote_ServiceNotConfigured(t *testing.T) {
	node := &mockNode{
		quoteCheckoutSupplyFunc: func(context.Context, contracts.QuoteCheckoutSupplyRequest) (*contracts.CheckoutSupplyQuoteResponse, error) {
			return nil, errors.New("checkout supply quote service not configured")
		},
	}
	ts := orderSupplyQuoteTestServer(t, node)

	body, err := json.Marshal(contracts.QuoteCheckoutSupplyRequest{
		Items: []contracts.CheckoutSupplyItemRequest{{ListingSlug: "item", Quantity: 1}},
	})
	require.NoError(t, err)

	resp, respBody := orderSupplyQuoteDoReq(t, ts, body)
	require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	guestAssertErrorCode(t, respBody, response.CodeNotImplemented)
}

func TestPOSTOrderSupplyQuote_InvalidRequest(t *testing.T) {
	node := &mockNode{
		quoteCheckoutSupplyFunc: func(context.Context, contracts.QuoteCheckoutSupplyRequest) (*contracts.CheckoutSupplyQuoteResponse, error) {
			return nil, contracts.ErrInvalidGuestRequest
		},
	}
	ts := orderSupplyQuoteTestServer(t, node)

	body, err := json.Marshal(contracts.QuoteCheckoutSupplyRequest{
		Items: []contracts.CheckoutSupplyItemRequest{{ListingSlug: "item", Quantity: 1}},
	})
	require.NoError(t, err)

	resp, respBody := orderSupplyQuoteDoReq(t, ts, body)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	guestAssertErrorCode(t, respBody, response.CodeBadRequest)
}

func TestClassifyCheckoutSupplyQuoteError(t *testing.T) {
	status, code, msg := classifyCheckoutSupplyQuoteError(errors.New("checkout supply quote service not configured"))
	require.Equal(t, http.StatusNotImplemented, status)
	require.Equal(t, response.CodeNotImplemented, code)
	require.Equal(t, "Supply quote is not available", msg)

	status, code, msg = classifyCheckoutSupplyQuoteError(contracts.ErrInvalidGuestRequest)
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, response.CodeBadRequest, code)
	require.Equal(t, "Invalid checkout request", msg)

	status, code, msg = classifyCheckoutSupplyQuoteError(errors.New("resolve item for \"missing\": listing not found"))
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, response.CodeBadRequest, code)
	require.Equal(t, "One or more items could not be found", msg)

	status, code, msg = classifyCheckoutSupplyQuoteError(errors.New("database connection refused"))
	require.Equal(t, http.StatusInternalServerError, status)
	require.Equal(t, response.CodeInternalError, code)
	require.Equal(t, "Unable to check availability", msg)

	status, code, msg = classifyCheckoutSupplyQuoteError(errors.New("invalid database connection"))
	require.Equal(t, http.StatusInternalServerError, status)
	require.Equal(t, response.CodeInternalError, code)
	require.Equal(t, "Unable to check availability", msg)

	status, code, msg = classifyGuestSupplyQuoteError(errors.New("checkout supply quote service not configured"))
	require.Equal(t, http.StatusNotImplemented, status)
	require.Equal(t, response.CodeNotImplemented, code)
	require.Equal(t, "Supply quote is not available", msg)

	status, code, msg = classifyGuestSupplyQuoteError(contracts.ErrGuestCheckoutDisabled)
	require.Equal(t, http.StatusForbidden, status)
	require.Equal(t, response.CodeForbidden, code)
	require.Equal(t, "Guest Checkout is not available", msg)
}

func TestPOSTOrderSupplyQuote_InternalErrorDoesNotLeak(t *testing.T) {
	node := &mockNode{
		quoteCheckoutSupplyFunc: func(context.Context, contracts.QuoteCheckoutSupplyRequest) (*contracts.CheckoutSupplyQuoteResponse, error) {
			return nil, errors.New("database connection refused: host=127.0.0.1")
		},
	}
	ts := orderSupplyQuoteTestServer(t, node)

	body, err := json.Marshal(contracts.QuoteCheckoutSupplyRequest{
		Items: []contracts.CheckoutSupplyItemRequest{{ListingSlug: "item", Quantity: 1}},
	})
	require.NoError(t, err)

	resp, respBody := orderSupplyQuoteDoReq(t, ts, body)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	guestAssertErrorCode(t, respBody, response.CodeInternalError)
	require.NotContains(t, string(respBody), "127.0.0.1")
	require.Contains(t, string(respBody), "Unable to check availability")
}

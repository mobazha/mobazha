//go:build !private_distribution

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/response"
	"github.com/stretchr/testify/require"
)

func listingSupplySummaryTestServer(t *testing.T, node *mockNode) *httptest.Server {
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

func listingSupplySummaryDoReq(t *testing.T, ts *httptest.Server, body []byte) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/listings/supply-summary", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, respBody
}

func TestPOSTListingSupplySummary_Valid(t *testing.T) {
	var captured contracts.ListingSupplySummaryRequest
	onHand := int64(15)
	held := int64(3)
	node := &mockNode{
		summarizeListingSupplyFunc: func(_ context.Context, req contracts.ListingSupplySummaryRequest) (*contracts.ListingSupplySummaryResponse, error) {
			captured = req
			q := int64(7)
			return &contracts.ListingSupplySummaryResponse{
				Limit:  50,
				Offset: 0,
				Total:  1,
				Items: []contracts.ListingSupplySummaryItem{{
					ListingSlug:       "test-item",
					SupplyMode:        contracts.ListingSupplyModeLicenseCodes,
					Status:            contracts.SupplyAvailabilityAvailable,
					AvailableQuantity: &q,
					OnHandQuantity:    &onHand,
					HeldQuantity:      &held,
				}},
			}, nil
		},
	}
	ts := listingSupplySummaryTestServer(t, node)

	body, err := json.Marshal(contracts.ListingSupplySummaryRequest{
		Slugs: []string{"test-item"},
	})
	require.NoError(t, err)

	resp, respBody := listingSupplySummaryDoReq(t, ts, body)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, []string{"test-item"}, captured.Slugs)

	var envelope struct {
		Data contracts.ListingSupplySummaryResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(respBody, &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, contracts.ListingSupplyModeLicenseCodes, envelope.Data.Items[0].SupplyMode)
	require.EqualValues(t, 7, *envelope.Data.Items[0].AvailableQuantity)
	require.NotNil(t, envelope.Data.Items[0].OnHandQuantity)
	require.EqualValues(t, 15, *envelope.Data.Items[0].OnHandQuantity)
	require.NotNil(t, envelope.Data.Items[0].HeldQuantity)
	require.EqualValues(t, 3, *envelope.Data.Items[0].HeldQuantity)
	require.NotContains(t, string(respBody), "supplyKind")
	require.NotContains(t, string(respBody), "providerID")
	require.NotContains(t, string(respBody), "providerRef")
}

func TestPOSTListingSupplySummary_PaginatesAllListings(t *testing.T) {
	var captured contracts.ListingSupplySummaryRequest
	node := &mockNode{
		summarizeListingSupplyFunc: func(_ context.Context, req contracts.ListingSupplySummaryRequest) (*contracts.ListingSupplySummaryResponse, error) {
			captured = req
			return &contracts.ListingSupplySummaryResponse{
				Limit:  req.Limit,
				Offset: req.Offset,
				Total:  0,
				Items:  []contracts.ListingSupplySummaryItem{},
			}, nil
		},
	}
	ts := listingSupplySummaryTestServer(t, node)

	body, err := json.Marshal(contracts.ListingSupplySummaryRequest{
		Limit:  25,
		Offset: 50,
	})
	require.NoError(t, err)

	resp, _ := listingSupplySummaryDoReq(t, ts, body)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, captured.Slugs)
	require.Equal(t, 25, captured.Limit)
	require.Equal(t, 50, captured.Offset)
}

func TestPOSTListingSupplySummary_InvalidJSON(t *testing.T) {
	ts := listingSupplySummaryTestServer(t, &mockNode{})

	resp, respBody := listingSupplySummaryDoReq(t, ts, []byte(`{`))
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	guestAssertErrorCode(t, respBody, response.CodeValidation)
}

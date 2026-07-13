// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	corePmt "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/payment"
	"github.com/stretchr/testify/require"
)

type mockNodeWithOnramp struct {
	*mockNode
	svc contracts.OnrampFundingService
}

func (m *mockNodeWithOnramp) OnrampFunding() contracts.OnrampFundingService { return m.svc }

type fakeOnrampFundingService struct {
	initiateErr error
	refreshErr  error
	lastReq     contracts.OnrampFundingInitiation
	view        *payment.OnrampFundingSourceView
}

func (f *fakeOnrampFundingService) InitiateOrResumeForOrder(_ context.Context, _ string, req contracts.OnrampFundingInitiation) (*payment.OnrampFundingSourceView, error) {
	f.lastReq = req
	if f.initiateErr != nil {
		return nil, f.initiateErr
	}
	return f.view, nil
}

func (f *fakeOnrampFundingService) RefreshForOrder(context.Context, string) (*payment.OnrampFundingSourceView, error) {
	if f.refreshErr != nil {
		return nil, f.refreshErr
	}
	return f.view, nil
}

func onrampHandlerRequest(t *testing.T, svc contracts.OnrampFundingService, authed bool, body interface{}) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/orders/order-1/payment-session/onramp", &buf)
	req.Header.Set("Content-Type", "application/json")

	var node contracts.NodeService = &mockNodeWithOnramp{mockNode: &mockNode{}, svc: svc}
	ctx := context.WithValue(req.Context(), nodeContextKey, node)
	if authed {
		ctx = WithAuthIdentity(ctx, &AuthIdentity{UserID: "buyer-user"})
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orderID", "order-1")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)
	return req, httptest.NewRecorder()
}

func TestOnrampInitiateRequiresAuth(t *testing.T) {
	svc := &fakeOnrampFundingService{view: &payment.OnrampFundingSourceView{OnrampOrderID: "o-1"}}
	req, rr := onrampHandlerRequest(t, svc, false, map[string]string{"providerID": "mock-onramp", "fiatCurrency": "USD"})
	(&Gateway{}).handlePOSTOrderPaymentSessionOnramp(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOnrampInitiateSuccessCarriesBuyerFromAuth(t *testing.T) {
	svc := &fakeOnrampFundingService{view: &payment.OnrampFundingSourceView{OnrampOrderID: "o-1", Status: "awaiting_payment"}}
	req, rr := onrampHandlerRequest(t, svc, true, map[string]string{"providerID": "mock-onramp", "fiatCurrency": "USD"})
	(&Gateway{}).handlePOSTOrderPaymentSessionOnramp(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, "buyer-user", svc.lastReq.Buyer.Subject, "buyer identity must come from auth context")

	var envelope struct {
		Data payment.OnrampFundingSourceView `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&envelope))
	require.Equal(t, "o-1", envelope.Data.OnrampOrderID)
}

func TestOnrampInitiateValidatesBody(t *testing.T) {
	svc := &fakeOnrampFundingService{}
	req, rr := onrampHandlerRequest(t, svc, true, map[string]string{"providerID": "mock-onramp"})
	(&Gateway{}).handlePOSTOrderPaymentSessionOnramp(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOnrampInitiateErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{corePmt.ErrOnrampAttemptNotFound, http.StatusNotFound},
		{corePmt.ErrOnrampAttemptNotReady, http.StatusConflict},
		{contracts.ErrOnrampProviderNotFound, http.StatusBadRequest},
		{contracts.ErrOnrampCapabilityClosed, http.StatusConflict},
	}
	for _, tc := range cases {
		svc := &fakeOnrampFundingService{initiateErr: tc.err}
		req, rr := onrampHandlerRequest(t, svc, true, map[string]string{"providerID": "mock-onramp", "fiatCurrency": "USD"})
		(&Gateway{}).handlePOSTOrderPaymentSessionOnramp(rr, req)
		require.Equal(t, tc.code, rr.Code, "error %v", tc.err)
	}
}

func TestOnrampSubsystemUnavailable(t *testing.T) {
	req, rr := onrampHandlerRequest(t, nil, true, map[string]string{"providerID": "mock-onramp", "fiatCurrency": "USD"})
	(&Gateway{}).handlePOSTOrderPaymentSessionOnramp(rr, req)
	require.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

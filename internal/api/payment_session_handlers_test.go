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

	"github.com/go-chi/chi/v5"
	corePmt "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/pkg/contracts"
	paypb "github.com/mobazha/mobazha/pkg/payment"
	responsePkg "github.com/mobazha/mobazha/pkg/response"
	"github.com/stretchr/testify/require"
)

type mockPaymentSessionService struct {
	createFunc func(ctx context.Context, req contracts.CreatePaymentSessionRequest) (*paypb.PaymentSession, error)
	quoteFunc  func(ctx context.Context, req contracts.CreatePaymentSelectionQuoteRequest) (*paypb.PaymentSelectionQuote, error)
	getFunc    func(ctx context.Context, orderID string) (*paypb.PaymentSession, error)
}

func (m *mockPaymentSessionService) CreateSelectionQuote(ctx context.Context, req contracts.CreatePaymentSelectionQuoteRequest) (*paypb.PaymentSelectionQuote, error) {
	if m.quoteFunc != nil {
		return m.quoteFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockPaymentSessionService) CreateSession(ctx context.Context, req contracts.CreatePaymentSessionRequest) (*paypb.PaymentSession, error) {
	return m.createFunc(ctx, req)
}

func (m *mockPaymentSessionService) GetSession(ctx context.Context, orderID string) (*paypb.PaymentSession, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, orderID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockPaymentSessionService) RefreshSession(ctx context.Context, orderID string) (*paypb.PaymentSession, error) {
	return m.GetSession(ctx, orderID)
}

type mockNodeWithPaymentSession struct {
	*mockNode
	paymentSessionSvc contracts.PaymentSessionService
}

func (m *mockNodeWithPaymentSession) PaymentSession() contracts.PaymentSessionService {
	return m.paymentSessionSvc
}

func newPaymentSessionHandlerRequest(t *testing.T, method, path string, body interface{}, vars map[string]string, svc contracts.PaymentSessionService) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")

	node := &mockNodeWithPaymentSession{
		mockNode:          &mockNode{},
		paymentSessionSvc: svc,
	}
	ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
	req = req.WithContext(ctx)

	if len(vars) > 0 {
		rctx := chi.NewRouteContext()
		for k, v := range vars {
			rctx.URLParams.Add(k, v)
		}
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}
	return req
}

func TestHandlePOSTOrderPaymentSession_PaymentCoinMismatch(t *testing.T) {
	svc := &mockPaymentSessionService{
		createFunc: func(_ context.Context, _ contracts.CreatePaymentSessionRequest) (*paypb.PaymentSession, error) {
			return nil, corePmt.ErrPaymentCoinMismatch
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newPaymentSessionHandlerRequest(t,
		http.MethodPost,
		"/v1/orders/o1/payment-session",
		map[string]interface{}{
			"paymentCoin":   "crypto:eip155:1:native",
			"refundAddress": "0xrefund",
		},
		map[string]string{"orderID": "o1"},
		svc,
	)

	g.handlePOSTOrderPaymentSession(w, r)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandlePOSTOrderPaymentSession_DealIntegrityConflict(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "missing fee quote", err: corePmt.ErrDealPaymentQuoteRequired},
		{name: "conversion quote required", err: corePmt.ErrDealPaymentConversionQuoteRequired},
		{name: "selection quote invalid", err: corePmt.ErrDealPaymentSelectionQuoteInvalid},
		{name: "amount mismatch", err: corePmt.ErrDealPaymentAmountIntegrity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockPaymentSessionService{
				createFunc: func(_ context.Context, _ contracts.CreatePaymentSessionRequest) (*paypb.PaymentSession, error) {
					return nil, tt.err
				},
			}
			g := &Gateway{}
			w := httptest.NewRecorder()
			r := newPaymentSessionHandlerRequest(t,
				http.MethodPost,
				"/v1/orders/o1/payment-session",
				map[string]interface{}{
					"paymentCoin":     "fiat:stripe:USD",
					"fiatAmountCents": 999,
				},
				map[string]string{"orderID": "o1"},
				svc,
			)

			g.handlePOSTOrderPaymentSession(w, r)
			if w.Code != http.StatusConflict {
				t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandlePOSTOrderPaymentSelectionQuote_ReturnsImmutableQuote(t *testing.T) {
	called := false
	svc := &mockPaymentSessionService{
		quoteFunc: func(_ context.Context, req contracts.CreatePaymentSelectionQuoteRequest) (*paypb.PaymentSelectionQuote, error) {
			called = true
			if req.OrderID != "o1" || req.PaymentCoin != "crypto:eip155:1:native" {
				t.Fatalf("request = %+v", req)
			}
			return &paypb.PaymentSelectionQuote{
				ID: "psq-1", OrderID: req.OrderID, PaymentCoin: req.PaymentCoin,
				PricingCurrency: "USD", PricingAmount: "4900", BuyerPaymentTotal: "19600000000000000",
			}, nil
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newPaymentSessionHandlerRequest(t,
		http.MethodPost,
		"/v1/orders/o1/payment-selection-quotes",
		map[string]interface{}{"paymentCoin": "crypto:eip155:1:native"},
		map[string]string{"orderID": "o1"},
		svc,
	)

	g.handlePOSTOrderPaymentSelectionQuote(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !called || !bytes.Contains(w.Body.Bytes(), []byte(`"id":"psq-1"`)) {
		t.Fatalf("quote handler result = %s", w.Body.String())
	}
}

func TestHandlePOSTOrderPaymentSession_SelectionQuoteSuppliesFiatAmount(t *testing.T) {
	svc := &mockPaymentSessionService{
		createFunc: func(_ context.Context, req contracts.CreatePaymentSessionRequest) (*paypb.PaymentSession, error) {
			if req.PaymentSelectionQuoteID != "psq-1" {
				t.Fatalf("paymentSelectionQuoteID = %q", req.PaymentSelectionQuoteID)
			}
			if req.FiatAmountCents != 0 {
				t.Fatalf("handler must not invent quoted amount, got %d", req.FiatAmountCents)
			}
			return &paypb.PaymentSession{SessionID: "ps_o1", OrderID: "o1", PaymentCoin: req.PaymentCoin}, nil
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newPaymentSessionHandlerRequest(t,
		http.MethodPost,
		"/v1/orders/o1/payment-session",
		map[string]interface{}{
			"paymentCoin": "fiat:stripe:EUR", "paymentSelectionQuoteID": "psq-1",
		},
		map[string]string{"orderID": "o1"},
		svc,
	)

	g.handlePOSTOrderPaymentSession(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandlePOSTOrderPaymentSession_OrderExtensionPolicyConflict(t *testing.T) {
	for _, policyErr := range []error{
		corePmt.ErrOrderExtensionReservation,
		corePmt.ErrOrderExtensionSettlement,
		corePmt.ErrOrderExtensionCollateral,
	} {
		t.Run(policyErr.Error(), func(t *testing.T) {
			svc := &mockPaymentSessionService{createFunc: func(_ context.Context, _ contracts.CreatePaymentSessionRequest) (*paypb.PaymentSession, error) {
				return nil, policyErr
			}}
			w := httptest.NewRecorder()
			r := newPaymentSessionHandlerRequest(t, http.MethodPost, "/v1/orders/o1/payment-session", map[string]interface{}{
				"paymentCoin": "crypto:eip155:56:native",
			}, map[string]string{"orderID": "o1"}, svc)
			(&Gateway{}).handlePOSTOrderPaymentSession(w, r)
			require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		})
	}
}

func TestHandlePOSTOrderPaymentSession_PaymentCoinDisabled(t *testing.T) {
	svc := &mockPaymentSessionService{
		createFunc: func(_ context.Context, _ contracts.CreatePaymentSessionRequest) (*paypb.PaymentSession, error) {
			return nil, corePmt.ErrPaymentCoinDisabled
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newPaymentSessionHandlerRequest(t,
		http.MethodPost,
		"/v1/orders/o1/payment-session",
		map[string]interface{}{
			"paymentCoin": "crypto:zcash:mainnet:native",
		},
		map[string]string{"orderID": "o1"},
		svc,
	)

	g.handlePOSTOrderPaymentSession(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandlePOSTOrderPaymentSession_TRONPaymentRetired(t *testing.T) {
	svc := &mockPaymentSessionService{
		createFunc: func(_ context.Context, req contracts.CreatePaymentSessionRequest) (*paypb.PaymentSession, error) {
			if req.PaymentCoin != "crypto:tron:mainnet:native" {
				t.Fatalf("paymentCoin = %q", req.PaymentCoin)
			}
			return nil, corePmt.ErrTRONPaymentRetired
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newPaymentSessionHandlerRequest(t,
		http.MethodPost,
		"/v1/orders/o1/payment-session",
		map[string]interface{}{
			"paymentCoin": "TRX",
		},
		map[string]string{"orderID": "o1"},
		svc,
	)

	g.handlePOSTOrderPaymentSession(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}

	var env responsePkg.ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != responsePkg.CodeTRONPaymentRetired {
		t.Fatalf("code = %q body=%s", env.Error.Code, w.Body.String())
	}
}

func TestHandlePOSTOrderPaymentSession_CryptoAllowsEmptyRefundAddress(t *testing.T) {
	called := false
	svc := &mockPaymentSessionService{
		createFunc: func(_ context.Context, req contracts.CreatePaymentSessionRequest) (*paypb.PaymentSession, error) {
			called = true
			if req.PaymentCoin != "crypto:eip155:1:native" {
				t.Fatalf("paymentCoin = %q", req.PaymentCoin)
			}
			if req.RefundAddress != "" {
				t.Fatalf("refundAddress = %q, want empty", req.RefundAddress)
			}
			return &paypb.PaymentSession{
				SessionID:      "ps_o1",
				OrderID:        "o1",
				PaymentCoin:    req.PaymentCoin,
				SettlementMode: paypb.SettlementModeAddressMonitored,
				Status:         paypb.SessionStatusAwaitingFunds,
			}, nil
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newPaymentSessionHandlerRequest(t,
		http.MethodPost,
		"/v1/orders/o1/payment-session",
		map[string]interface{}{
			"paymentCoin": "crypto:eip155:1:native",
		},
		map[string]string{"orderID": "o1"},
		svc,
	)

	g.handlePOSTOrderPaymentSession(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !called {
		t.Fatal("CreateSession was not called")
	}
}

func TestHandlePOSTOrderPaymentSession_FiatReturnsUnifiedSession(t *testing.T) {
	svc := &mockPaymentSessionService{
		createFunc: func(_ context.Context, req contracts.CreatePaymentSessionRequest) (*paypb.PaymentSession, error) {
			if req.PaymentCoin != "fiat:stripe:USD" {
				t.Fatalf("paymentCoin = %q", req.PaymentCoin)
			}
			return &paypb.PaymentSession{
				SessionID:      "ps_o1",
				OrderID:        "o1",
				PaymentCoin:    req.PaymentCoin,
				SettlementMode: paypb.SettlementModeProviderCheckout,
				Status:         paypb.SessionStatusAwaitingFunds,
				FundingTarget: paypb.FundingTargetView{
					Type: paypb.FundingTargetTypeProviderSession,
					ProviderData: map[string]interface{}{
						"providerID":     "stripe",
						"sessionID":      "sess_123",
						"clientSecret":   "cs_test",
						"publishableKey": "pk_test",
					},
				},
			}, nil
		},
	}
	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newPaymentSessionHandlerRequest(t,
		http.MethodPost,
		"/v1/orders/o1/payment-session",
		map[string]interface{}{
			"paymentCoin":     "FIAT:Stripe:usd",
			"fiatAmountCents": 1250,
		},
		map[string]string{"orderID": "o1"},
		svc,
	)

	g.handlePOSTOrderPaymentSession(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data paypb.PaymentSession `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.FundingTarget.ProviderData["clientSecret"] != "cs_test" {
		t.Fatalf("providerData = %+v", resp.Data.FundingTarget.ProviderData)
	}
}

func TestHandlePOSTOrderPaymentSession_RejectsTrailingJSON(t *testing.T) {
	svc := &mockPaymentSessionService{
		createFunc: func(_ context.Context, _ contracts.CreatePaymentSessionRequest) (*paypb.PaymentSession, error) {
			t.Fatal("CreateSession should not be called")
			return nil, nil
		},
	}
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/orders/o1/payment-session",
		bytes.NewBufferString(`{"paymentCoin":"fiat:stripe:USD","fiatAmountCents":1250}{"extra":true}`),
	)
	req.Header.Set("Content-Type", "application/json")
	node := &mockNodeWithPaymentSession{
		mockNode:          &mockNode{},
		paymentSessionSvc: svc,
	}
	ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orderID", "o1")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	g := &Gateway{}
	w := httptest.NewRecorder()
	g.handlePOSTOrderPaymentSession(w, req)

	if w.Code != http.StatusBadRequest {
		body, _ := io.ReadAll(w.Result().Body)
		t.Fatalf("status = %d body=%s", w.Code, string(body))
	}
}

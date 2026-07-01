package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

func newRefundAddressHandlerRequest(
	t *testing.T,
	body map[string]interface{},
	orderID string,
	node contracts.NodeService,
) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/orders/"+orderID+"/refund-address", &buf)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orderID", orderID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestHandlePOSTOrderRefundAddress_BuyerSuccess(t *testing.T) {
	const (
		orderID = "order-refund-1"
		addr    = "0x1111111111111111111111111111111111111111"
		coin    = "crypto:eip155:1:native"
	)
	var gotOrderID, gotAddr string
	var gotCoin iwallet.CoinType

	node := &mockNode{
		getOrderFunc: func(id string) (*models.Order, error) {
			if id != orderID {
				t.Fatalf("orderID = %q", id)
			}
			order := &models.Order{ID: models.OrderID(orderID)}
			order.SetRole(models.RoleBuyer)
			return order, nil
		},
		setOrderRefundAddressFunc: func(_ context.Context, id string, c iwallet.CoinType, refundAddr string) error {
			gotOrderID = id
			gotCoin = c
			gotAddr = refundAddr
			return nil
		},
	}

	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newRefundAddressHandlerRequest(t, map[string]interface{}{
		"refundAddress": addr,
		"paymentCoin":   coin,
	}, orderID, node)

	g.handlePOSTOrderRefundAddress(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if gotOrderID != orderID || gotAddr != addr || string(gotCoin) != coin {
		t.Fatalf("saved orderID=%q addr=%q coin=%q", gotOrderID, gotAddr, gotCoin)
	}

	var resp struct {
		Data struct {
			OrderID       string `json:"orderID"`
			RefundAddress string `json:"refundAddress"`
			PaymentCoin   string `json:"paymentCoin"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.RefundAddress != addr {
		t.Fatalf("response refundAddress = %q", resp.Data.RefundAddress)
	}
}

func TestHandlePOSTOrderRefundAddress_SellerForbidden(t *testing.T) {
	node := &mockNode{
		getOrderFunc: func(orderID string) (*models.Order, error) {
			order := &models.Order{ID: models.OrderID(orderID)}
			order.SetRole(models.RoleVendor)
			return order, nil
		},
		setOrderRefundAddressFunc: func(context.Context, string, iwallet.CoinType, string) error {
			t.Fatal("SetOrderRefundAddressForPayment should not be called for seller")
			return nil
		},
	}

	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newRefundAddressHandlerRequest(t, map[string]interface{}{
		"refundAddress": "0x1111111111111111111111111111111111111111",
		"paymentCoin":   "crypto:eip155:1:native",
	}, "order-refund-2", node)

	g.handlePOSTOrderRefundAddress(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandlePOSTOrderRefundAddress_FiatNotApplicable(t *testing.T) {
	node := &mockNode{
		getOrderFunc: func(orderID string) (*models.Order, error) {
			order := &models.Order{ID: models.OrderID(orderID)}
			order.SetRole(models.RoleBuyer)
			return order, nil
		},
		setOrderRefundAddressFunc: func(context.Context, string, iwallet.CoinType, string) error {
			t.Fatal("SetOrderRefundAddressForPayment should not be called for fiat")
			return nil
		},
	}

	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newRefundAddressHandlerRequest(t, map[string]interface{}{
		"refundAddress": "0x1111111111111111111111111111111111111111",
		"paymentCoin":   "fiat:stripe:USD",
	}, "order-refund-3", node)

	g.handlePOSTOrderRefundAddress(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandlePOSTOrderRefundAddress_OrderNotFound(t *testing.T) {
	node := &mockNode{
		getOrderFunc: func(string) (*models.Order, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}

	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newRefundAddressHandlerRequest(t, map[string]interface{}{
		"refundAddress": "0x1111111111111111111111111111111111111111",
		"paymentCoin":   "crypto:eip155:1:native",
	}, "missing-order", node)

	g.handlePOSTOrderRefundAddress(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandlePOSTOrderRefundAddress_ValidationError(t *testing.T) {
	node := &mockNode{
		getOrderFunc: func(orderID string) (*models.Order, error) {
			order := &models.Order{ID: models.OrderID(orderID)}
			order.SetRole(models.RoleBuyer)
			return order, nil
		},
		setOrderRefundAddressFunc: func(context.Context, string, iwallet.CoinType, string) error {
			return models.ErrRefundAddressInvalid
		},
	}

	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newRefundAddressHandlerRequest(t, map[string]interface{}{
		"refundAddress": "not-an-address",
		"paymentCoin":   "crypto:eip155:1:native",
	}, "order-refund-4", node)

	g.handlePOSTOrderRefundAddress(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var env responsePkg.ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != responsePkg.CodeBadRequest {
		t.Fatalf("code = %q", env.Error.Code)
	}
}

func TestHandlePOSTOrderRefundAddress_RequiredError(t *testing.T) {
	node := &mockNode{
		getOrderFunc: func(orderID string) (*models.Order, error) {
			order := &models.Order{ID: models.OrderID(orderID)}
			order.SetRole(models.RoleBuyer)
			return order, nil
		},
		setOrderRefundAddressFunc: func(context.Context, string, iwallet.CoinType, string) error {
			return models.ErrRefundAddressRequired
		},
	}

	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newRefundAddressHandlerRequest(t, map[string]interface{}{
		"refundAddress": "",
		"paymentCoin":   "crypto:eip155:1:native",
	}, "order-refund-5", node)

	g.handlePOSTOrderRefundAddress(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var env responsePkg.ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != responsePkg.CodeRefundAddressRequired {
		t.Fatalf("code = %q", env.Error.Code)
	}
}

func TestHandlePOSTOrderRefundAddress_BadRequestWrapped(t *testing.T) {
	node := &mockNode{
		getOrderFunc: func(orderID string) (*models.Order, error) {
			order := &models.Order{ID: models.OrderID(orderID)}
			order.SetRole(models.RoleBuyer)
			return order, nil
		},
		setOrderRefundAddressFunc: func(context.Context, string, iwallet.CoinType, string) error {
			return coreiface.ErrBadRequest
		},
	}

	g := &Gateway{}
	w := httptest.NewRecorder()
	r := newRefundAddressHandlerRequest(t, map[string]interface{}{
		"refundAddress": "0x1111111111111111111111111111111111111111",
		"paymentCoin":   "crypto:eip155:1:native",
	}, "order-refund-6", node)

	g.handlePOSTOrderRefundAddress(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}

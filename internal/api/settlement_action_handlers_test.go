//go:build !private_distribution

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestHandlePOSTOrderSettlementAction_Success(t *testing.T) {
	node := &mockNode{
		executeSettlementActionFunc: func(ctx context.Context, action string, orderID models.OrderID, payoutAddr string) (*payment.ActionResult, iwallet.CoinType, error) {
			if action != "confirm" {
				t.Fatalf("action = %s, want confirm", action)
			}
			if orderID != models.OrderID("QmOrder123") {
				t.Fatalf("orderID = %s, want QmOrder123", orderID)
			}
			if payoutAddr != "0x1111111111111111111111111111111111111111" {
				t.Fatalf("payoutAddr = %s", payoutAddr)
			}
			return &payment.ActionResult{
				Mode:       payment.ActionModeSubmitted,
				ActionID:   "act-123",
				EscrowAddr: "0x2222222222222222222222222222222222222222",
			}, iwallet.CoinType("crypto:eip155:56:native"), nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/orders/QmOrder123/settlement-actions/confirm",
		bytes.NewBufferString(`{"payoutAddress":"0x1111111111111111111111111111111111111111"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withNodeAndRouteParams(req, node, map[string]string{
		"orderID": "QmOrder123",
		"action":  "confirm",
	})
	rec := httptest.NewRecorder()

	g := &Gateway{}
	g.handlePOSTOrderSettlementAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var env responsePkg.SuccessEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode success envelope: %v", err)
	}
	body, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	if got := body["mode"]; got != string(payment.ActionModeSubmitted) {
		t.Fatalf("mode = %v", got)
	}
	if _, exists := body["instructions"]; exists {
		t.Fatalf("response leaked legacy instructions field: %#v", body["instructions"])
	}
}

func TestHandleGETOrderSettlementActionStatus_Success(t *testing.T) {
	node := &mockNode{
		getSettlementActionStatusFunc: func(ctx context.Context, action string, orderID models.OrderID, actionID string) (*payment.ActionStatus, iwallet.CoinType, error) {
			if action != "confirm" {
				t.Fatalf("action = %s, want confirm", action)
			}
			if orderID != models.OrderID("QmOrder123") {
				t.Fatalf("orderID = %s, want QmOrder123", orderID)
			}
			if actionID != "act-123" {
				t.Fatalf("actionID = %s, want act-123", actionID)
			}
			return &payment.ActionStatus{
				State:            "submitted",
				TxHash:           "0xabc",
				Confirmations:    2,
				RelayTaskID:      "task-1",
				OrderID:          "QmOrder123",
				SettlementAction: "confirm",
			}, iwallet.CoinType("crypto:eip155:56:native"), nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/orders/QmOrder123/settlement-actions/confirm/status?actionId=act-123", nil)
	req = withNodeAndRouteParams(req, node, map[string]string{
		"orderID": "QmOrder123",
		"action":  "confirm",
	})
	rec := httptest.NewRecorder()

	g := &Gateway{}
	g.handleGETOrderSettlementActionStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var env responsePkg.SuccessEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode success envelope: %v", err)
	}
	body, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	if got := body["state"]; got != "submitted" {
		t.Fatalf("state = %v, want submitted", got)
	}
	if got := body["paymentChain"]; got != "BSC" {
		t.Fatalf("paymentChain = %v, want BSC", got)
	}
	if got := body["relayTaskId"]; got != "task-1" {
		t.Fatalf("relayTaskId = %v, want task-1", got)
	}
}

func TestHandleGETOrderSettlementActionStatus_AllowsComplete(t *testing.T) {
	node := &mockNode{
		getSettlementActionStatusFunc: func(ctx context.Context, action string, orderID models.OrderID, actionID string) (*payment.ActionStatus, iwallet.CoinType, error) {
			if action != "complete" {
				t.Fatalf("action = %s, want complete", action)
			}
			return &payment.ActionStatus{State: "submitted"}, iwallet.CoinType("crypto:eip155:1:native"), nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/orders/QmOrder123/settlement-actions/complete/status?actionId=act-123", nil)
	req = withNodeAndRouteParams(req, node, map[string]string{
		"orderID": "QmOrder123",
		"action":  "complete",
	})
	rec := httptest.NewRecorder()

	g := &Gateway{}
	g.handleGETOrderSettlementActionStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleGETOrderSettlementActionStatus_RequiresActionID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/orders/QmOrder123/settlement-actions/confirm/status", nil)
	req = withNodeAndRouteParams(req, &mockNode{}, map[string]string{
		"orderID": "QmOrder123",
		"action":  "confirm",
	})
	rec := httptest.NewRecorder()

	g := &Gateway{}
	g.handleGETOrderSettlementActionStatus(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func withNodeAndRouteParams(req *http.Request, node *mockNode, params map[string]string) *http.Request {
	ctx := context.WithValue(req.Context(), nodeContextKey, node)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

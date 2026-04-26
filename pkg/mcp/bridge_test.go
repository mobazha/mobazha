package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestHTTPBridge_Call_Success(t *testing.T) {
	expectedBody := `{"data":{"name":"Test Store"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", got)
		}
		if got := r.Header.Get("X-Store-PeerID"); got != "QmTestPeer" {
			t.Errorf("expected X-Store-PeerID=QmTestPeer, got %s", got)
		}
		if r.URL.Path != "/v1/profiles" {
			t.Errorf("expected path /v1/profiles, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	}))
	defer srv.Close()

	bridge := NewHTTPBridge(srv.URL, "test-token", "QmTestPeer", nil)
	code, body, err := bridge.Call(context.Background(), "GET", "/v1/profiles", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 200 {
		t.Errorf("expected status 200, got %d", code)
	}
	if string(body) != expectedBody {
		t.Errorf("expected body %s, got %s", expectedBody, string(body))
	}
}

func TestHTTPBridge_Call_WithQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "10" {
			t.Errorf("expected limit=10, got %s", got)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	bridge := NewHTTPBridge(srv.URL, "test-token", "", nil)
	query := url.Values{"limit": {"10"}}
	code, _, err := bridge.Call(context.Background(), "GET", "/v1/sales", query, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 200 {
		t.Errorf("expected 200, got %d", code)
	}
}

func TestHTTPBridge_Call_WithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type application/json")
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["name"] != "test" {
			t.Errorf("expected name=test, got %v", body["name"])
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"data":{"id":"123"}}`))
	}))
	defer srv.Close()

	bridge := NewHTTPBridge(srv.URL, "test-token", "", nil)
	code, _, err := bridge.Call(context.Background(), "POST", "/v1/listings", nil, map[string]string{"name": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 201 {
		t.Errorf("expected 201, got %d", code)
	}
}

func TestHTTPBridge_Call_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"db connection failed"}}`))
	}))
	defer srv.Close()

	bridge := NewHTTPBridge(srv.URL, "test-token", "", nil)
	code, body, err := bridge.Call(context.Background(), "GET", "/v1/sales", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 500 {
		t.Errorf("expected 500, got %d", code)
	}
	if len(body) == 0 {
		t.Error("expected non-empty error body")
	}
}

func TestHandleBridgeResult_Success(t *testing.T) {
	result, err := HandleBridgeResult(200, []byte(`{"data":{"count":5}}`), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("expected non-error result")
	}
}

func TestHandleBridgeResult_NotFound(t *testing.T) {
	body := []byte(`{"error":{"code":"NOT_FOUND","message":"order not found"}}`)
	result, err := HandleBridgeResult(404, body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for 404")
	}
}

func TestHandleBridgeResult_Forbidden(t *testing.T) {
	body := []byte(`{"error":{"code":"FORBIDDEN","message":"missing scope"}}`)
	result, err := HandleBridgeResult(403, body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for 403")
	}
}

func TestHandleBridgeResult_TransportError(t *testing.T) {
	_, err := HandleBridgeResult(0, nil, http.ErrServerClosed)
	if err == nil {
		t.Error("expected error for transport failure")
	}
}

func TestHandleBridgeResult_EmptySuccess(t *testing.T) {
	result, err := HandleBridgeResult(204, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("expected non-error result for 204")
	}
}

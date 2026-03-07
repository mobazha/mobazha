package net

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestStoreHeartbeatSender_SendsHeartbeat(t *testing.T) {
	var received atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/platform/v1/stores/heartbeat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			return
		}
		if r.Header.Get("X-Standalone-Store-Key") != "test-api-key" {
			t.Errorf("missing or wrong API key header")
			return
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["peer_id"] != "12D3KooWTest" {
			t.Errorf("unexpected peer_id: %s", body["peer_id"])
		}

		received.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sender := NewStoreHeartbeatSender(StoreHeartbeatConfig{
		SaaSURL:     server.URL,
		PeerID:      "12D3KooWTest",
		EndpointURL: "http://localhost:5102",
		APIKey:      "test-api-key",
		Interval:    100 * time.Millisecond,
	})
	sender.Start(ctx)

	time.Sleep(350 * time.Millisecond)
	cancel()

	count := received.Load()
	if count < 2 {
		t.Errorf("expected at least 2 heartbeats (immediate + ticker), got %d", count)
	}
}

func TestStoreHeartbeatSender_IncludesOwnerUserID(t *testing.T) {
	var receivedOwnerID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		receivedOwnerID = body["owner_user_id"]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	sender := NewStoreHeartbeatSender(StoreHeartbeatConfig{
		SaaSURL:       server.URL,
		PeerID:        "12D3KooWTest",
		EndpointURL:   "http://localhost:5102",
		APIKey:        "test-key",
		Version:       "1.0.0",
		OwnerUserIDFn: func() string { return "casdoor-user-123" },
		Interval:      50 * time.Millisecond,
	})
	sender.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()

	if receivedOwnerID != "casdoor-user-123" {
		t.Errorf("expected owner_user_id=casdoor-user-123, got=%s", receivedOwnerID)
	}
}

func TestStoreHeartbeatSender_IncludesVersion(t *testing.T) {
	var receivedVersion string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		receivedVersion = body["version"]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	sender := NewStoreHeartbeatSender(StoreHeartbeatConfig{
		SaaSURL:  server.URL,
		PeerID:   "12D3KooWTest",
		APIKey:   "test-key",
		Version:  "2.5.0",
		Interval: 50 * time.Millisecond,
	})
	sender.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()

	if receivedVersion != "2.5.0" {
		t.Errorf("expected version=2.5.0, got=%s", receivedVersion)
	}
}

func TestStoreHeartbeatSender_NATOnly_NoEndpointURL(t *testing.T) {
	var receivedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		receivedURL = body["endpoint_url"]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	sender := NewStoreHeartbeatSender(StoreHeartbeatConfig{
		SaaSURL:  server.URL,
		PeerID:   "12D3KooWNAT",
		APIKey:   "nat-key",
		Interval: 50 * time.Millisecond,
	})
	sender.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()

	if receivedURL != "" {
		t.Errorf("NAT-only store should not send endpoint_url, got=%s", receivedURL)
	}
}

func TestStoreHeartbeatSender_StopsOnCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	sender := NewStoreHeartbeatSender(StoreHeartbeatConfig{
		SaaSURL:     server.URL,
		PeerID:      "12D3KooWTest",
		EndpointURL: "http://localhost:5102",
		APIKey:      "test-key",
		Interval:    50 * time.Millisecond,
	})
	sender.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestRegisterWithSaaS_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/platform/v1/stores/register" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			return
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["peer_id"] != "12D3KooWTestReg" {
			t.Errorf("unexpected peer_id: %s", body["peer_id"])
		}
		if body["connectivity"] != "public" {
			t.Errorf("expected connectivity=public, got=%s", body["connectivity"])
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{
				"peer_id": "12D3KooWTestReg",
				"api_key": "generated-key-abc",
			},
		})
	}))
	defer server.Close()

	apiKey, err := RegisterWithSaaS(context.Background(), server.URL, "12D3KooWTestReg", "http://my-store:5102", "public")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if apiKey != "generated-key-abc" {
		t.Errorf("expected api_key=generated-key-abc, got=%s", apiKey)
	}
}

func TestRegisterWithSaaS_NATOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["endpoint_url"] != "" {
			t.Errorf("NAT-only should not send endpoint_url, got=%s", body["endpoint_url"])
		}
		if body["connectivity"] != "nat" {
			t.Errorf("expected connectivity=nat, got=%s", body["connectivity"])
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{"peer_id": "peer-nat", "api_key": "nat-key"},
		})
	}))
	defer server.Close()

	apiKey, err := RegisterWithSaaS(context.Background(), server.URL, "peer-nat", "", "nat")
	if err != nil {
		t.Fatalf("register NAT: %v", err)
	}
	if apiKey != "nat-key" {
		t.Errorf("expected api_key=nat-key, got=%s", apiKey)
	}
}

func TestRegisterWithSaaS_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "internal error")
	}))
	defer server.Close()

	_, err := RegisterWithSaaS(context.Background(), server.URL, "peer1", "http://store:5102", "public")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

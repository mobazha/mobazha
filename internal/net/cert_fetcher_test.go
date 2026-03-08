package net

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchCasdoorCertificate_Success(t *testing.T) {
	cert := "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJAK..."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/platform/v1/auth/certificate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		body, _ := json.Marshal(map[string]interface{}{
			"data": map[string]string{"certificate": cert},
		})
		w.Write(body)
	}))
	defer server.Close()

	got, err := FetchCasdoorCertificate(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("FetchCasdoorCertificate: %v", err)
	}
	if got != cert {
		t.Errorf("certificate = %q, want %q", got, cert)
	}
}

func TestFetchCasdoorCertificate_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	_, err := FetchCasdoorCertificate(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

func TestFetchCasdoorCertificate_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	_, err := FetchCasdoorCertificate(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestFetchCasdoorCertificate_EmptyCertificate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"certificate":""}}`))
	}))
	defer server.Close()

	_, err := FetchCasdoorCertificate(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for empty certificate, got nil")
	}
}

func TestFetchCasdoorCertificate_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block forever so the request times out
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := FetchCasdoorCertificate(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

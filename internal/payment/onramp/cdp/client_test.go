// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package cdp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type noopAuthenticator struct{}

func (noopAuthenticator) Authorize(*http.Request) error { return nil }

func TestHTTPClientSessionTokenIncludesClientIP(t *testing.T) {
	var got sessionTokenBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(sessionTokenResponse{Token: "session-token"})
	}))
	defer server.Close()

	client := NewHTTPClient(noopAuthenticator{}, server.URL)
	token, err := client.CreateSessionToken(context.Background(), SessionTokenRequest{
		Address:  "0x1111111111111111111111111111111111111111",
		Networks: []string{"base"},
		Assets:   []string{"USDC"},
		ClientIP: "192.0.2.10",
	})
	if err != nil {
		t.Fatalf("CreateSessionToken: %v", err)
	}
	if token != "session-token" {
		t.Fatalf("token = %q", token)
	}
	if got.ClientIP != "192.0.2.10" {
		t.Fatalf("clientIp = %q", got.ClientIP)
	}
}

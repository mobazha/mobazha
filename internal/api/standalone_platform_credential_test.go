// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mobazha/mobazha/pkg/response"
)

func TestHandlePOSTRefreshPlatformCredential_RefreshesRuntimeKeyWithoutExposingIt(t *testing.T) {
	gateway := &Gateway{config: &GatewayConfig{
		SaaSAPIURL:  "https://platform.example",
		LocalPeerID: "12D3KooWStore",
	}}
	gateway.SetStoreCredentialRefresher(func(context.Context) (string, error) {
		return "mbz_rotated_secret", nil
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/system/refresh-platform-credential", nil)
	gateway.handlePOSTRefreshPlatformCredential(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := gateway.StandaloneAPIKey(); got != "mbz_rotated_secret" {
		t.Fatalf("expected hot-reloaded key, got %q", got)
	}
	if strings.Contains(recorder.Body.String(), "mbz_rotated_secret") {
		t.Fatal("credential must not be exposed in the response")
	}
	var envelope struct {
		Data refreshPlatformCredentialResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Data.Refreshed {
		t.Fatal("expected refreshed=true")
	}
}

func TestHandlePOSTRefreshPlatformCredential_UnavailableFailsClosed(t *testing.T) {
	gateway := &Gateway{config: &GatewayConfig{}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/system/refresh-platform-credential", nil)

	gateway.handlePOSTRefreshPlatformCredential(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlePOSTRefreshPlatformCredential_RegistrationFailureKeepsCurrentKey(t *testing.T) {
	gateway := &Gateway{config: &GatewayConfig{
		SaaSAPIURL:       "https://platform.example",
		LocalPeerID:      "12D3KooWStore",
		StandaloneAPIKey: "mbz_current_secret",
	}}
	gateway.SetStoreCredentialRefresher(func(context.Context) (string, error) {
		return "", errors.New("registration rejected")
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/system/refresh-platform-credential", nil)

	gateway.handlePOSTRefreshPlatformCredential(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := gateway.StandaloneAPIKey(); got != "mbz_current_secret" {
		t.Fatalf("failed refresh changed current key to %q", got)
	}
}

func TestHandleDELETEConnectPlatform_RemoteFirstThenClearsLocalOwner(t *testing.T) {
	dataDir := t.TempDir()
	ownerPath := filepath.Join(dataDir, ownerUserIDFile)
	if err := os.WriteFile(ownerPath, []byte("casdoor-owner"), 0600); err != nil {
		t.Fatalf("seed owner binding: %v", err)
	}
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/platform/v1/stores/12D3KooWStore/owner" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Standalone-Store-Key"); got != "mbz_store_secret" {
			t.Errorf("store credential = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer platform.Close()

	gateway := &Gateway{config: &GatewayConfig{
		SaaSAPIURL:       platform.URL,
		LocalPeerID:      "12D3KooWStore",
		StandaloneAPIKey: "mbz_store_secret",
		DataDir:          dataDir,
	}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/v1/system/connect-platform", nil)
	gateway.handleDELETEConnectPlatform(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if _, err := os.Stat(ownerPath); !os.IsNotExist(err) {
		t.Fatalf("owner binding still exists after disconnect: %v", err)
	}
	if gateway.StandaloneAPIKey() != "mbz_store_secret" {
		t.Fatal("disconnect must preserve the store credential")
	}
}

func TestHandleDELETEConnectPlatform_RemoteFailurePreservesLocalOwner(t *testing.T) {
	dataDir := t.TempDir()
	ownerPath := filepath.Join(dataDir, ownerUserIDFile)
	if err := os.WriteFile(ownerPath, []byte("casdoor-owner"), 0600); err != nil {
		t.Fatalf("seed owner binding: %v", err)
	}
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer platform.Close()

	gateway := &Gateway{config: &GatewayConfig{
		SaaSAPIURL:       platform.URL,
		LocalPeerID:      "12D3KooWStore",
		StandaloneAPIKey: "mbz_store_secret",
		DataDir:          dataDir,
	}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/v1/system/connect-platform", nil)
	gateway.handleDELETEConnectPlatform(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got, err := os.ReadFile(ownerPath); err != nil || string(got) != "casdoor-owner" {
		t.Fatalf("remote failure changed local owner: %q, %v", got, err)
	}
}

func TestHandleDELETEConnectPlatform_InvalidStoreCredentialReturnsStableCode(t *testing.T) {
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer platform.Close()
	gateway := &Gateway{config: &GatewayConfig{
		SaaSAPIURL:       platform.URL,
		LocalPeerID:      "12D3KooWStore",
		StandaloneAPIKey: "mbz_revoked_secret",
		DataDir:          t.TempDir(),
	}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/v1/system/connect-platform", nil)
	gateway.handleDELETEConnectPlatform(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode denial: %v", err)
	}
	if envelope.Error.Code != response.CodeStoreCredentialInvalid {
		t.Fatalf("code = %q, want %q", envelope.Error.Code, response.CodeStoreCredentialInvalid)
	}
}

// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestPOSTClaimStorePersistsOwnerUserID(t *testing.T) {
	const ownerID = "casdoor-owner-123"

	saas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/platform/v1/stores/12D3KooWClaimTest/claim" {
			t.Fatalf("unexpected SaaS claim path: %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Standalone-Store-Key"); got != "store-key" {
			t.Fatalf("standalone key = %q, want store-key", got)
		}
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			t.Fatalf("account proof was not forwarded: %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer saas.Close()

	certPEM, privateKey := generateTestRSACert()
	validator, err := NewJWTValidator(certPEM, "12D3KooWClaimTest", "")
	if err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	g := testGateway(t, "admin-password")
	g.config = &GatewayConfig{
		DataDir:          dataDir,
		LocalPeerID:      "12D3KooWClaimTest",
		SaaSAPIURL:       saas.URL,
		StandaloneAPIKey: "store-key",
	}
	g.auth.hashFile = filepath.Join(dataDir, adminHashFile)
	g.jwtValidator = validator

	token := signToken(&JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Id: ownerID,
	}, privateKey)
	body, err := json.Marshal(claimStoreRequest{AdminPassword: "admin-password"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/system/claim-store", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	g.handlePOSTClaimStore(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("claim status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	persisted, err := os.ReadFile(OwnerUserIDFilePath(dataDir))
	if err != nil {
		t.Fatalf("read persisted owner: %v", err)
	}
	if got := string(persisted); got != ownerID {
		t.Fatalf("persisted owner = %q, want %q", got, ownerID)
	}
	if got := validator.OwnerUserID(); got != ownerID {
		t.Fatalf("runtime owner = %q, want %q", got, ownerID)
	}
}

// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

//go:build cdplive

package cdp

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLiveSessionTokenProbe exercises the REAL CDP Session Token API with the
// key file named by MOBAZHA_CDP_ONRAMP_KEY_FILE. It never prints the token or
// any key material — only outcome classes. Run explicitly:
//
//	MOBAZHA_CDP_ONRAMP_KEY_FILE=... go test -tags cdplive -run LiveSessionTokenProbe -v ./internal/payment/onramp/cdp/
//
// Excluded from normal builds by the cdplive tag: it is a sandbox
// verification instrument, not CI material.
func TestLiveSessionTokenProbe(t *testing.T) {
	keyFile := os.Getenv("MOBAZHA_CDP_ONRAMP_KEY_FILE")
	if keyFile == "" {
		t.Skip("MOBAZHA_CDP_ONRAMP_KEY_FILE not set")
	}
	clientIP := os.Getenv("MOBAZHA_CDP_ONRAMP_CLIENT_IP")
	if clientIP == "" {
		t.Skip("MOBAZHA_CDP_ONRAMP_CLIENT_IP not set")
	}
	auth, err := LoadKeyAuthenticator(keyFile)
	if err != nil {
		t.Fatalf("load key: %v", err)
	}
	client := NewHTTPClient(auth, "")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	token, err := client.CreateSessionToken(ctx, SessionTokenRequest{
		// Anvil dev address #0 — a harmless, well-known placeholder.
		Address:       "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		Networks:      []string{"base"},
		Assets:        []string{"USDC"},
		ClientIP:      clientIP,
		PartnerUserID: "cdp-live-probe",
	})
	if err != nil {
		t.Fatalf("create session token failed: %v", err)
	}
	t.Logf("session token OK: len=%d", len(token))
}

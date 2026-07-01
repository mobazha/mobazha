package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestSovereignOpenAPI_OperationIDSnapshot uses the real registerHumaAPI chain to
// verify the sovereign API surface matches an explicit allowlist. This catches
// both missing routes AND unexpected route leaks.
func TestSovereignOpenAPI_OperationIDSnapshot(t *testing.T) {
	r := chi.NewMux()
	g := &Gateway{config: &GatewayConfig{ProductSurfacePolicy: restrictedProductSurfacePolicy{}}}
	g.registerHumaAPI(r)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /v1/openapi.json returned %d, want 200", rr.Code)
	}

	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil {
		t.Fatalf("Failed to unmarshal OpenAPI spec: %v", err)
	}

	var got []string
	for _, methods := range spec.Paths {
		for _, op := range methods {
			if op.OperationID != "" {
				got = append(got, op.OperationID)
			}
		}
	}
	sort.Strings(got)

	expectedOps := []string{
		"node-huma-ping",
		"runtime-config-get",
		// listings
		"listings-get-mine-slug-or-cid",
		"listings-create",
		"listings-update",
		"listings-delete",
		"listings-import-json",
		"listing-import-multipart",
		"listings-index-by-peer-id",
		"listings-index",
		"listings-template",
		"listings-get-by-peer-slug",
		"listings-get-by-listing-id",
		// media
		"media-post-avatar",
		"media-post-header",
		"media-post-images",
		"media-post-product-images",
		"media-post-file",
		"media-get-image",
		"media-get-file",
		"profiles-get-avatar",
		"profiles-get-header",
		// profiles
		"profiles-batch-fetch-get",
		"profiles-batch-fetch-post",
		"profiles-create",
		"profiles-create-scoped",
		"profiles-update",
		"profiles-update-scoped",
		"profiles-get-by-peer-id",
		"profiles-get-self",
		// discounts
		"discounts-applicable",
		"discounts-calculate",
		"discounts-get",
		"discounts-id-codes-code-delete",
		"discounts-id-codes-get",
		"discounts-id-codes-post",
		"discounts-id-delete",
		"discounts-id-get",
		"discounts-id-put",
		"discounts-id-redemptions-get",
		"discounts-post",
		"discounts-validate",
		// collections
		"collections-get",
		"collections-id-delete",
		"collections-id-get",
		"collections-id-products-post",
		"collections-id-products-reorder-put",
		"collections-id-products-slug-delete",
		"collections-id-put",
		"collections-peer-published-get",
		"collections-peer-published-id-get",
		"collections-post",
		"store-policy-get",
		"store-policy-moderators-get",
		"store-policy-moderators-peer-delete",
		"store-policy-moderators-post",
		"store-policy-moderators-put",
		"store-policy-peer-published-get",
		// system (common)
		"config-get",
		"system-info-get",
		"system-shutdown-post",
		"system-logs-get",
		"system-update-trigger-post",
		"system-update-config-get",
		"system-update-config-put",
		"system-diagnostics-get",
		"system-health-get",
		"system-doctor-get",
		// system (sovereign setup)
		"system-setup-get",
		"system-setup-post",
		// MCP
		"system-mcp-capability-get",
		"system-mcp-connect-post",
		"system-mcp-connect-client-post",
		"system-mcp-clients-get",
		"system-mcp-disconnect-post",
		"system-mcp-disconnect-client-post",
		// settings
		"settings-storefront-get",
		"settings-storefront-public-get",
		"settings-storefront-put",
		"settings-feature-put",
		"settings-guest-checkout-get",
		"settings-guest-checkout-put",
		"settings-payment-policy-get",
		"settings-payment-policy-put",
		"features-get",
		"preferences-currency-post",
		"preferences-get",
		"preferences-put",
		// wishlists
		"wishlists-get",
		"wishlists-peer-slug-delete",
		"wishlists-post",
		// auth
		"admin-version-get",
		"admin-password-post",
		"auth-identity-get",
		"auth-scopes-get",
		"auth-tokens-get",
		"auth-tokens-post",
		"auth-tokens-token-id-delete",
		// shipping
		"shipping-locations-get",
		"shipping-locations-id-delete",
		"shipping-locations-id-get",
		"shipping-locations-id-put",
		"shipping-locations-post",
		"shipping-profiles-default-post",
		"shipping-profiles-get",
		"shipping-profiles-id-delete",
		"shipping-profiles-id-get",
		"shipping-profiles-id-patch",
		"shipping-profiles-id-put",
		"shipping-profiles-listings-get",
		"shipping-profiles-post",
		"shipping-refresh-snapshots-post",
		"shipping-stale-listings-get",
		// carts
		"carts-delete-all",
		"carts-delete-peer-items",
		"carts-get",
		"carts-get-items-count",
		"carts-post-peer-items",
		"carts-put-peer-items",
		// guest checkout
		"guest-orders-post-public",
		"guest-orders-quote-public",
		"guest-orders-get-public",
		"guest-orders-list-auth",
		"guest-orders-ship-token",
		"guest-orders-complete-token",
		"guest-orders-admin-detail",
		"settings-guest-checkout-readiness-get",
		"settings-pgp-key-get",
		"settings-pgp-key-put",
		"settings-pgp-key-delete",
		// payment methods
		"payment-methods-get-by-peer-id",
		// receiving accounts
		"wallet-list-receiving-accounts",
		"wallet-create-receiving-account",
		"wallet-update-receiving-account",
		"wallet-delete-receiving-account",
		// local notification feed (channel outbound — Telegram/Discord — stays
		// non-sovereign). Product boundary: sovereign notifications are SQLite-only
		// and never leave the node, satisfying the zero-outbound guarantee.
		"notifications-get-count",
		"notifications-get-list",
		"notifications-post-batch",
		"notifications-post-read-all",
		"notifications-post-notif-read",
		// AI (local LLM only)
		"settings-ai-get",
		"settings-ai-put",
		"settings-ai-providers-get",
		"settings-ai-test-post",
		"ai-status-get",
		"ai-generate-post",
		// exchange rates
		"exchange-rates-get",
		"exchange-rates-currency-code-get",
		// digital assets (seller)
		"digital-asset-create-license-key",
		"digital-asset-create-link",
		"digital-asset-delete",
		"digital-asset-get",
		"digital-asset-import-license-keys",
		"digital-asset-license-key-stats",
		"digital-asset-list",
		"digital-asset-list-license-keys",
		"digital-asset-revoke-license-key",
		"digital-asset-update",
		// digital-asset-upload-file removed: now a raw chi streaming route
		// (/v1/digital-assets/upload-stream) that bypasses Huma body binding.
		// digital assets (buyer/public)
		"digital-assets-buyer-get",
		"digital-delivery-status-get",
		"digital-delivery-retry",
		"digital-download",
		"license-activate",
		"license-deactivate",
		"license-validate",
	}
	sort.Strings(expectedOps)

	// Check missing: expected but not present
	missing := diffSlices(expectedOps, got)
	if len(missing) > 0 {
		t.Errorf("Missing operation IDs in sovereign OpenAPI spec: %v\nGot: %v", missing, got)
	}

	// Check unexpected: present but not in allowlist (catches leaked routes)
	unexpected := diffSlices(got, expectedOps)
	if len(unexpected) > 0 {
		t.Errorf("Unexpected operation IDs leaked into sovereign OpenAPI spec: %v", unexpected)
	}

	// Denied prefix check (defense-in-depth).
	// NOTE: "notifications-" intentionally NOT denied — sovereign ships a
	// local SQLite notification feed (list/count/read/batch). Outbound
	// channel delivery (Telegram/Discord) lives in
	// huma_notification_handlers.go which is //go:build !sovereign, so the
	// sovereign surface only exposes local-feed operations.
	deniedPrefixes := []string{
		"chat-",
		"fiat-",
		"fulfillment-",
		"orders-",
		"disputes-",
		"moderators-",
		"webhooks-",
		"wallet-spend",
		"wallet-get-mnemonic",
	}
	for _, opID := range got {
		for _, prefix := range deniedPrefixes {
			if strings.HasPrefix(opID, prefix) {
				t.Errorf("Denied operation %q registered in sovereign (prefix %q)", opID, prefix)
			}
		}
	}
}

// TestSovereignOpenAPI_SensitiveOpsRefuseAPIToken pins the security
// declaration for endpoints whose payload (EXTERNAL_PAYMENT seed, private view key,
// full transfer history, wallet setup) must never be reachable via a
// scoped mbz_ API token. The runtime gate in nodeHumaAuthMiddleware
// honours this declaration; this test is the contract that prevents a
// future edit from accidentally re-adding apiToken to the OR-list and
// silently re-opening seed export to wallet:read tokens (the bug that
// existed before the OP-MP-6 P0 fix).
//
// If you need to expose one of these endpoints to API tokens, you must
// also tighten routeScopeMap so prefix matching does not let a broad
// wallet:read scope cover the new path.
func TestSovereignOpenAPI_SensitiveOpsRefuseAPIToken(t *testing.T) {
	r := chi.NewMux()
	g := &Gateway{config: &GatewayConfig{ProductSurfacePolicy: restrictedProductSurfacePolicy{}}}
	g.registerHumaAPI(r)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /v1/openapi.json returned %d, want 200", rr.Code)
	}

	type opMeta struct {
		OperationID string                `json:"operationId"`
		Security    []map[string][]string `json:"security"`
	}
	var spec struct {
		Paths map[string]map[string]opMeta `json:"paths"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil {
		t.Fatalf("Failed to unmarshal OpenAPI spec: %v", err)
	}

	// IDs that must declare basic+jwt only — NEVER apiToken.
	//
	// Private distribution modules pin their own sensitive route security in
	// their OpenAPI contract tests. After TD-117 the Core-owned admin-only
	// write surface (admin password, auth tokens,
	// system shutdown/update/network/domain, MCP connect/disconnect) was
	// also tightened to adminOnlyAuthSecurity so the OpenAPI declaration
	// matches the runtime deny-by-default in matchRouteScope. The
	// TestHumaSecurity_RouteScopeConsistency_Sovereign guard catches new
	// drift; this list pins the highest-risk endpoints by name so
	// reviewers see the contract explicitly.
	sensitive := map[string]bool{
		"admin-password-post":         true,
		"auth-tokens-get":             true,
		"auth-tokens-post":            true,
		"auth-tokens-token-id-delete": true,
		"system-shutdown-post":        true,
		"system-update-trigger-post":  true,
		"system-update-config-put":    true,
	}

	found := make(map[string]bool, len(sensitive))
	for _, methods := range spec.Paths {
		for _, op := range methods {
			if !sensitive[op.OperationID] {
				continue
			}
			found[op.OperationID] = true
			if len(op.Security) == 0 {
				t.Errorf("%s: must declare Security (basic+jwt only)", op.OperationID)
				continue
			}
			for _, req := range op.Security {
				if _, ok := req[SecuritySchemeAPIToken]; ok {
					t.Errorf("%s: Security MUST NOT include apiToken — "+
						"this endpoint exposes secrets/admin-only state. "+
						"If you need API token access here, add an explicit "+
						"deny entry to routeScopeMap so the wallet:read prefix "+
						"match cannot cover it.", op.OperationID)
				}
			}
		}
	}

	for id := range sensitive {
		if !found[id] {
			t.Errorf("expected sensitive operation %q in OpenAPI spec but it was not registered", id)
		}
	}
}

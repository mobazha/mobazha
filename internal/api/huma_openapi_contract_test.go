package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

// TestNodeOpenAPI_OperationIDSnapshot asserts that every huma-registered
// operation appears in the generated OpenAPI spec with the expected ID.
// Add new IDs to expectedOps when migrating handlers.
func TestNodeOpenAPI_OperationIDSnapshot(t *testing.T) {
	r := chi.NewMux()

	cfg := huma.DefaultConfig(nodeHumaAPITitle, nodeHumaAPIVersion)
	cfg.OpenAPIPath = "/v1/openapi"
	cfg.DocsPath = "/v1/docs"
	cfg.SchemasPath = "/v1/schemas"
	installNodeHumaEnvelope(&cfg)
	api := humachi.New(r, cfg)

	g := &Gateway{}
	// Public operations.
	g.registerNodeHumaSmokeRoutes(api)
	g.registerNodeHumaListingPublicOperations(api)
	g.registerNodeHumaMediaPublicOperations(api)
	g.registerNodeHumaProfilePublicOperations(api)
	g.registerNodeHumaDiscountPublicOperations(api)
	g.registerNodeHumaCollectionPublicOperations(api)
	g.registerNodeHumaStorePolicyPublicOperations(api)
	g.registerNodeHumaSystemPublicOperations(api)
	g.registerNodeHumaMiscPublicOperations(api)
	g.registerNodeHumaSocialPublicOperations(api)
	g.registerNodeHumaOrderPublicOperations(api)
	g.registerNodeHumaFiatPublicOperations(api)
	g.registerNodeHumaFulfillmentPublicOperations(api)
	g.registerNodeHumaSettingsPublicOperations(api)
	g.registerNodeHumaAuthPublicOperations(api)
	// Admin operations.
	g.registerNodeHumaListingAdminOperations(api)
	g.registerNodeHumaMediaAdminOperations(api)
	g.registerNodeHumaProfileAdminOperations(api)
	g.registerNodeHumaDiscountAdminOperations(api)
	g.registerNodeHumaCollectionAdminOperations(api)
	g.registerNodeHumaStorePolicyAdminOperations(api)
	g.registerNodeHumaSystemAdminOperations(api)
	g.registerNodeHumaMiscAdminOperations(api)
	g.registerNodeHumaSocialAdminOperations(api)
	g.registerNodeHumaOrderAdminOperations(api)
	g.registerNodeHumaFiatAdminOperations(api)
	g.registerNodeHumaFulfillmentAdminOperations(api)
	g.registerNodeHumaSettingsAdminOperations(api)
	g.registerNodeHumaAuthAdminOperations(api)
	g.registerNodeHumaWalletOperations(api)
	g.registerNodeHumaChatOperations(api)
	g.registerNodeHumaDisputeOperations(api)
	g.registerNodeHumaCartOperations(api)
	g.registerNodeHumaNotificationOperations(api)
	g.registerNodeHumaWebhookOperations(api)
	g.registerAIHTTPCapabilities(api)
	g.registerNodeHumaShippingOperations(api)

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
		// wallet
		"wallet-spend",
		"wallet-get-mnemonic",
		"wallet-get-currencies",
		"wallet-list-receiving-accounts",
		"wallet-create-receiving-account",
		"wallet-update-receiving-account",
		"wallet-delete-receiving-account",
		// chat
		"chat-list-rooms",
		"chat-list-invites",
		"chat-create-room",
		"chat-join-room",
		"chat-leave-room",
		"chat-get-messages",
		"chat-send-message",
		"chat-edit-message",
		"chat-delete-message",
		"chat-react-message",
		"chat-typing",
		"chat-mark-read",
		"chat-get-members",
		"chat-invite-member",
		"chat-kick-member",
		"chat-get-room-settings",
		"chat-put-room-settings",
		"chat-media-download",
		"chat-block-user",
		"chat-unblock-user",
		"chat-list-blocked-users",
		"chat-get-presence",
		"chat-set-presence",
		"chat-get-settings",
		"chat-put-settings",
		"chat-verification-request",
		"chat-verification-accept",
		"chat-verification-start-sas",
		"chat-verification-confirm",
		"chat-verification-cancel",
		"chat-get-status",
		// listings
		"listings-get-mine-slug-or-cid",
		"listings-post-supply-summary",
		"listings-create",
		"listings-update",
		"listings-delete",
		"listings-import-json",
		"listings-import-gumroad",
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
		"media-get-image",
		"profiles-get-avatar",
		"profiles-get-header",
		"media-get-file",
		// profiles
		"profiles-batch-fetch-get",
		"profiles-batch-fetch-post",
		"profiles-create",
		"profiles-create-scoped",
		"profiles-update",
		"profiles-update-scoped",
		"profiles-get-by-peer-id",
		"profiles-get-self",
		// posts
		"posts-create",
		"posts-delete",
		"posts-get-own-by-slug",
		"posts-get-public-by-peer-slug",
		// followers
		"followers-check-follows-me",
		"followers-list-by-peer-id",
		"followers-list-self",
		// following
		"following-follow-peer",
		"following-unfollow-peer",
		"following-list-by-peer-id",
		"following-list-self",
		// ratings
		"ratings-index-by-peer-or-slug",
		"ratings-index-self",
		"ratings-index-by-peer-and-slug",
		"ratings-get-by-id",
		"ratings-batch-fetch",
		// AH-1.4 batch 3: orders, disputes, fiat, fulfillment, carts, analytics / guest / payment-methods
		"analytics-get-stats",
		"analytics-shop-post-events-public",
		"carts-delete-all",
		"carts-delete-peer-items",
		"carts-get",
		"carts-get-items-count",
		"carts-post-peer-items",
		"carts-put-peer-items",
		"cases-get-detail",
		"cases-get-query",
		"cases-post-query",
		"disputes-post-after-sale",
		"disputes-post-close",
		"disputes-post-instructions-release",
		"disputes-post-open",
		"disputes-post-release",
		"disputes-post-release-after-timeout",
		"fiat-capture-payment",
		"fiat-create-payment-session",
		"fiat-disconnect-provider",
		"fiat-get-payment",
		"fiat-get-provider-config-view",
		"fiat-get-provider-connection-status",
		"fiat-list-enabled-providers",
		"fiat-list-provider-actions",
		"fiat-public-capture-checkout-session",
		"fiat-public-create-checkout-session",
		"fiat-public-ingest-provider-webhook",
		"fiat-public-list-providers-by-peer",
		"fiat-refund-payment",
		"fiat-register-provider-webhook",
		"fiat-retry-provider-action",
		"fiat-save-provider-config",
		"fiat-verify-provider-credentials",
		"fulfillment-delete-disconnect",
		"fulfillment-get-catalog",
		"fulfillment-get-catalog-product",
		"fulfillment-get-order-status",
		"fulfillment-get-provider-status",
		"fulfillment-get-store-sync-product",
		"fulfillment-get-store-sync-products",
		"fulfillment-get-synced-products",
		"fulfillment-delete-synced-product",
		"fulfillment-list-providers",
		"fulfillment-post-connect",
		"fulfillment-post-import-product",
		"fulfillment-post-shipping-estimates",
		"fulfillment-post-sync-product-by-slug",
		"fulfillment-public-post-provider-webhook",
		"guest-orders-complete-token",
		"guest-orders-get-public",
		"guest-orders-list-auth",
		"guest-orders-post-public",
		"guest-orders-quote-public",
		"guest-orders-ship-token",
		"orders-delete-payment-watch",
		"orders-get-detail",
		"orders-get-payment-remaining",
		"orders-get-payment-session",
		"orders-post-cancel",
		"orders-post-checkout-breakdown",
		"orders-post-supply-quote",
		"orders-post-complete",
		"orders-post-confirm",
		"orders-post-create-purchase",
		"orders-post-estimate-total",
		"orders-post-extend-protection",
		"orders-post-instructions-cancel",
		"orders-post-instructions-complete",
		"orders-post-instructions-confirm",
		"orders-post-instructions-decline",
		"orders-post-instructions-refund",
		"orders-post-payment-cancel-partial",
		"orders-post-payment-session",
		"orders-post-payment-selection-quote",
		"orders-post-refund-address",
		"orders-post-rwa-token-payment-info",
		"orders-get-settlement-action-status",
		"orders-post-settlement-action",
		"orders-post-payment",
		"orders-post-rate",
		"orders-post-refund",
		"orders-post-ship",
		"orders-post-spend-for-order",
		"payment-methods-get-by-peer-id",
		"purchases-get-query",
		"purchases-post-query",
		"sales-get-query",
		"sales-post-query",
		// AH-1.4 Batch 4: notifications, webhooks, AI, settings, shipping, discounts, collections
		"agent-chat-session-delete",
		"agent-chat-session-get",
		"agent-chat-sessions-get",
		"agent-chat-stream-post",
		"agent-artifact-approval-post",
		"agent-memories-get",
		"agent-memories-post",
		"agent-memory-delete",
		"agent-memory-patch",
		"agent-product-import-ingest-post",
		"agent-attachments-analyze-post",
		"agent-product-import-run-advance-post",
		"agent-product-import-run-approval-applications-post",
		"agent-product-import-run-approval-decisions-post",
		"agent-product-import-run-approvals-post",
		"agent-product-import-workbench-get",
		"ai-generate-post",
		"ai-status-get",
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
		"features-get",
		"notifications-channel-types-get",
		"notifications-channels-detect-chat-post",
		"notifications-channels-get",
		"notifications-channels-id-delete",
		"notifications-channels-id-put",
		"notifications-channels-id-test-post",
		"notifications-channels-post",
		"notifications-get-count",
		"notifications-get-list",
		"notifications-post-batch",
		"notifications-post-notif-read",
		"notifications-post-read-all",
		"preferences-currency-post",
		"preferences-get",
		"preferences-put",
		"settings-ai-get",
		"settings-ai-providers-get",
		"settings-ai-put",
		"settings-ai-test-post",
		"settings-feature-put",
		"settings-guest-checkout-get",
		"settings-guest-checkout-put",
		"settings-payment-policy-get",
		"settings-payment-policy-put",
		"settings-storefront-get",
		"settings-storefront-public-get",
		"settings-storefront-put",
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
		"webhooks-get",
		"webhooks-id-delete",
		"webhooks-id-deliveries-get",
		"webhooks-id-get",
		"webhooks-id-patch",
		"webhooks-id-test-post",
		"webhooks-post",
		"wishlists-get",
		"wishlists-peer-slug-delete",
		"wishlists-post",
		// AH-1.4 Batch 5: system, auth, crypto, moderators, blocklist, fx, peers
		"admin-password-post",
		"admin-version-get",
		"auth-identity-get",
		"auth-scopes-get",
		"auth-tokens-get",
		"auth-tokens-post",
		"auth-tokens-token-id-delete",
		"blocklist-peer-id-delete",
		"blocklist-peer-id-put",
		"config-get",
		"crypto-hash-post",
		"crypto-sign-post",
		"crypto-verify-post",
		"exchange-rates-currency-code-get",
		"exchange-rates-get",
		"moderators-delete",
		"moderators-get",
		"moderators-post",
		"peers-get",
		"system-cache-delete",
		"system-claim-store-post",
		"system-connect-platform-post",
		"system-diagnostics-get",
		"system-doctor-get",
		"system-domain-get",
		"system-domain-post",
		"system-health-get",
		"system-info-get",
		"system-logs-get",
		"system-mcp-capability-get",
		"system-mcp-clients-get",
		"system-mcp-connect-client-post",
		"system-mcp-connect-post",
		"system-mcp-disconnect-client-post",
		"system-mcp-disconnect-post",
		"system-network-get",
		"system-network-post",
		"system-publish-post",
		"system-setup-get",
		"system-setup-post",
		"system-shutdown-post",
		"system-update-config-get",
		"system-update-config-put",
		"system-update-trigger-post",
	}
	sort.Strings(expectedOps)

	const minOps = 279
	if len(got) < minOps {
		t.Errorf("Expected at least %d operations, got %d: %v", minOps, len(got), got)
	}

	missing := diffSlices(expectedOps, got)
	if len(missing) > 0 {
		t.Errorf("Missing operation IDs in OpenAPI spec: %v\nGot: %v", missing, got)
	}
}

func diffSlices(want, got []string) []string {
	set := make(map[string]bool, len(got))
	for _, s := range got {
		set[s] = true
	}
	var missing []string
	for _, s := range want {
		if !set[s] {
			missing = append(missing, s)
		}
	}
	return missing
}

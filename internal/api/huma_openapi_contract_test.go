package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humamux"
	"github.com/gorilla/mux"
)

// TestNodeOpenAPI_OperationIDSnapshot asserts that every huma-registered
// operation appears in the generated OpenAPI spec with the expected ID.
// Add new IDs to expectedOps when migrating handlers.
func TestNodeOpenAPI_OperationIDSnapshot(t *testing.T) {
	r := mux.NewRouter()

	cfg := huma.DefaultConfig(nodeHumaAPITitle, nodeHumaAPIVersion)
	cfg.OpenAPIPath = "/v1/openapi"
	cfg.DocsPath = "/v1/docs"
	cfg.SchemasPath = "/v1/schemas"
	installNodeHumaEnvelope(&cfg)
	api := humamux.New(r, cfg)

	g := &Gateway{}
	g.registerNodeHumaSmokeRoutes(api)
	g.registerNodeHumaWalletOperations(api)
	g.registerNodeHumaChatOperations(api)
	g.registerNodeHumaListingOperations(api)
	g.registerNodeHumaMediaOperations(api)
	g.registerNodeHumaProfileOperations(api)
	g.registerNodeHumaSocialOperations(api)
	g.registerNodeHumaOrderOperations(api)
	g.registerNodeHumaDisputeOperations(api)
	g.registerNodeHumaFiatOperations(api)
	g.registerNodeHumaFulfillmentOperations(api)
	g.registerNodeHumaCartOperations(api)

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
		"chat-post-room-avatar",
		"chat-media-upload",
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
		"listings-create",
		"listings-update",
		"listings-delete",
		"listings-import",
		"listings-import-json",
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
		"media-post-files",
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
		"fiat-public-capture-checkout-session",
		"fiat-public-create-checkout-session",
		"fiat-public-ingest-provider-webhook",
		"fiat-public-list-providers-by-peer",
		"fiat-refund-payment",
		"fiat-register-provider-webhook",
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
		"guest-orders-ship-token",
		"orders-delete-payment-watch",
		"orders-get-detail",
		"orders-get-payment-remaining",
		"orders-post-cancel",
		"orders-post-checkout-breakdown",
		"orders-post-complete",
		"orders-post-confirm",
		"orders-post-create-purchase",
		"orders-post-estimate-total",
		"orders-post-extend-protection",
		"orders-post-instructions-cancel",
		"orders-post-instructions-complete",
		"orders-post-instructions-confirm",
		"orders-post-instructions-decline",
		"orders-post-instructions-payment",
		"orders-post-instructions-refund",
		"orders-post-payment-cancel-partial",
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
	}
	sort.Strings(expectedOps)

	const minOps = 157
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

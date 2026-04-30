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
	}
	sort.Strings(expectedOps)

	const minOps = 40
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

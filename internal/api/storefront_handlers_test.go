package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// fakeStoreConfigNode satisfies contracts.NodeService via the embedded nil
// interface and storeConfigProvider via the in-memory slots below — the only
// surface the storefront handlers touch.
type fakeStoreConfigNode struct {
	contracts.NodeService
	live         string
	draft        string
	history      string
	previewToken string
}

func (f *fakeStoreConfigNode) rawOrNil(s string) (json.RawMessage, error) {
	if s == "" {
		return nil, nil
	}
	return json.RawMessage(s), nil
}

func (f *fakeStoreConfigNode) StoreConfig() (json.RawMessage, error) { return f.rawOrNil(f.live) }
func (f *fakeStoreConfigNode) SaveStoreConfig(cfg json.RawMessage) error {
	f.live = string(cfg)
	return nil
}
func (f *fakeStoreConfigNode) StoreDraftConfig() (json.RawMessage, error) { return f.rawOrNil(f.draft) }
func (f *fakeStoreConfigNode) SaveStoreDraftConfig(cfg json.RawMessage) error {
	f.draft = string(cfg)
	return nil
}
func (f *fakeStoreConfigNode) DeleteStoreDraftConfig() error { f.draft = ""; return nil }
func (f *fakeStoreConfigNode) PublishStoreConfig(cfg json.RawMessage) error {
	f.live = string(cfg)
	f.draft = ""
	return nil
}
func (f *fakeStoreConfigNode) StoreConfigHistory() (json.RawMessage, error) {
	if f.history == "" {
		return json.RawMessage("[]"), nil
	}
	return json.RawMessage(f.history), nil
}
func (f *fakeStoreConfigNode) StorefrontPreviewToken() (json.RawMessage, error) {
	return f.rawOrNil(f.previewToken)
}
func (f *fakeStoreConfigNode) SaveStorefrontPreviewToken(record json.RawMessage) error {
	f.previewToken = string(record)
	return nil
}

func storefrontTestRequest(method, target string, node *fakeStoreConfigNode, peerID string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
	if peerID != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("peerID", peerID)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	}
	return req.WithContext(ctx)
}

// TestStorefrontPreviewToken_RoundTrip issues a token and uses it to read the
// draft through the public endpoint — the full share-preview flow.
func TestStorefrontPreviewToken_RoundTrip(t *testing.T) {
	g := &Gateway{}
	node := &fakeStoreConfigNode{
		live:  `{"version":1,"status":"published","marker":"live"}`,
		draft: `{"version":1,"status":"draft","marker":"draft"}`,
	}

	w := httptest.NewRecorder()
	g.handlePOSTStorefrontPreviewToken(w, storefrontTestRequest(http.MethodPost, "/v1/settings/storefront/preview-token", node, ""))
	if w.Code != http.StatusOK {
		t.Fatalf("issue token: status %d, body %s", w.Code, w.Body.String())
	}
	var issued struct {
		Data storefrontPreviewTokenRecord `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &issued); err != nil {
		t.Fatalf("decode issue response: %v", err)
	}
	if len(issued.Data.Token) != 32 {
		t.Fatalf("token should be 32 hex chars, got %q", issued.Data.Token)
	}
	if !issued.Data.ExpiresAt.After(time.Now()) {
		t.Fatalf("token must expire in the future, got %v", issued.Data.ExpiresAt)
	}

	w = httptest.NewRecorder()
	g.handleGETStorefrontConfigPublic(w, storefrontTestRequest(http.MethodGet, "/v1/settings/storefront/peer-1?preview="+issued.Data.Token, node, "peer-1"))
	if w.Code != http.StatusOK {
		t.Fatalf("preview fetch: status %d, body %s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); !jsonContains(body, `"marker":"draft"`) {
		t.Fatalf("valid token must return the draft, got %s", body)
	}
}

// TestStorefrontPreviewToken_RejectsBadOrExpired covers the failure modes:
// wrong token, expired token, and no token issued — all must behave exactly
// like a store with no published config rather than leaking which case it is.
func TestStorefrontPreviewToken_RejectsBadOrExpired(t *testing.T) {
	g := &Gateway{}

	cases := []struct {
		name  string
		node  *fakeStoreConfigNode
		query string
	}{
		{
			name:  "no token ever issued",
			node:  &fakeStoreConfigNode{draft: `{"status":"draft"}`},
			query: "?preview=deadbeefdeadbeefdeadbeefdeadbeef",
		},
		{
			name: "wrong token",
			node: &fakeStoreConfigNode{
				draft:        `{"status":"draft"}`,
				previewToken: `{"token":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expiresAt":"2199-01-01T00:00:00Z"}`,
			},
			query: "?preview=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
		{
			name: "expired token",
			node: &fakeStoreConfigNode{
				draft:        `{"status":"draft"}`,
				previewToken: `{"token":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expiresAt":"2000-01-01T00:00:00Z"}`,
			},
			query: "?preview=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			g.handleGETStorefrontConfigPublic(w, storefrontTestRequest(http.MethodGet, "/v1/settings/storefront/peer-1"+tc.query, tc.node, "peer-1"))
			if w.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
			}
			if jsonContains(w.Body.String(), "draft") {
				t.Fatalf("draft leaked through invalid token: %s", w.Body.String())
			}
		})
	}
}

// TestStorefrontPreviewToken_FallsBackToLive: the seller published (clearing
// the draft) after sharing the link — the link should show the live config,
// not break.
func TestStorefrontPreviewToken_FallsBackToLive(t *testing.T) {
	g := &Gateway{}
	node := &fakeStoreConfigNode{
		live:         `{"version":1,"status":"published","marker":"live"}`,
		previewToken: `{"token":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expiresAt":"2199-01-01T00:00:00Z"}`,
	}
	w := httptest.NewRecorder()
	g.handleGETStorefrontConfigPublic(w, storefrontTestRequest(http.MethodGet, "/v1/settings/storefront/peer-1?preview=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", node, "peer-1"))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !jsonContains(w.Body.String(), `"marker":"live"`) {
		t.Fatalf("expected live config fallback, got %s", w.Body.String())
	}
}

// TestStorefrontConfig_HistoryVariant reads the archive through the owner
// endpoint.
func TestStorefrontConfig_HistoryVariant(t *testing.T) {
	g := &Gateway{}
	node := &fakeStoreConfigNode{
		history: `[{"publishedAt":"2026-07-01T00:00:00Z","config":{"marker":"old"}}]`,
	}
	w := httptest.NewRecorder()
	g.handleGETStorefrontConfig(w, storefrontTestRequest(http.MethodGet, "/v1/settings/storefront?variant=history", node, ""))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !jsonContains(w.Body.String(), `"marker":"old"`) {
		t.Fatalf("expected history entry, got %s", w.Body.String())
	}
}

// TestValidateStoreConfigJSON_RoleColors: the new role tokens go through the
// same hex gate as primaryColor.
func TestValidateStoreConfigJSON_RoleColors(t *testing.T) {
	valid := `{"version":1,"status":"draft","theme":{"primaryColor":"#123456","backgroundColor":"#ffffff","textColor":"#111827","surfaceColor":"#f3f4f6"},"sections":[]}`
	if err := validateStoreConfigJSON([]byte(valid)); err != nil {
		t.Fatalf("valid role colors rejected: %v", err)
	}
	invalid := `{"version":1,"status":"draft","theme":{"primaryColor":"#123456","backgroundColor":"not-a-color"},"sections":[]}`
	if err := validateStoreConfigJSON([]byte(invalid)); err == nil {
		t.Fatal("invalid backgroundColor accepted")
	}
}

func jsonContains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && bytesContains(haystack, needle)
}

func bytesContains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

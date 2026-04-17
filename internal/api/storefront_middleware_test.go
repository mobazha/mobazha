package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestStorefrontMiddleware_NoHeader ensures the middleware is a strict no-op
// when X-Storefront-ID is absent (main host traffic / internal API calls).
// StorefrontFromContext must return nil so downstream handlers short-circuit
// the filter path.
func TestStorefrontMiddleware_NoHeader(t *testing.T) {
	g := &Gateway{}
	var gotCtx *StorefrontContext
	h := g.StorefrontMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotCtx = StorefrontFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if gotCtx != nil {
		t.Fatalf("expected no storefront context without X-Storefront-ID header, got %+v", gotCtx)
	}
	if got := StorefrontFilterFromContext(req.Context()); got != nil {
		t.Fatalf("expected nil filter, got %+v", got)
	}
}

// TestStorefrontMiddleware_IDOnly ensures a bare X-Storefront-ID header
// (no filter headers) produces a context with empty filter — storefront
// identity is known but no restrictions apply. The handler should see
// Filter == nil so the empty filter short-circuit in handleGETListingIndex
// stays cheap.
func TestStorefrontMiddleware_IDOnly(t *testing.T) {
	g := &Gateway{}
	var gotCtx *StorefrontContext
	h := g.StorefrontMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotCtx = StorefrontFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	req.Header.Set(headerStorefrontID, "spring-sale")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if gotCtx == nil {
		t.Fatal("expected storefront context with X-Storefront-ID set")
	}
	if gotCtx.ID != "spring-sale" {
		t.Fatalf("unexpected storefront ID: got %q, want %q", gotCtx.ID, "spring-sale")
	}
	if gotCtx.Filter != nil {
		t.Fatalf("expected nil filter when no filter headers present, got %+v", gotCtx.Filter)
	}
}

// TestStorefrontMiddleware_AllFilterAxes verifies that all three filter
// headers are parsed correctly: CSV splitting, whitespace trimming, and
// empty-token dropping. This is the happy path the hosting Gateway will
// drive in production.
func TestStorefrontMiddleware_AllFilterAxes(t *testing.T) {
	g := &Gateway{}
	var gotCtx *StorefrontContext
	h := g.StorefrontMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotCtx = StorefrontFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	req.Header.Set(headerStorefrontID, "spring-sale")
	req.Header.Set(headerStorefrontFilterCollections, " col-1, col-2 ,,col-3")
	req.Header.Set(headerStorefrontFilterTags, "sale,clearance")
	req.Header.Set(headerStorefrontFilterExcludeTags, "adult")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if gotCtx == nil || gotCtx.Filter == nil {
		t.Fatalf("expected non-nil filter, got %+v", gotCtx)
	}
	wantCols := []string{"col-1", "col-2", "col-3"}
	if !stringSliceEqual(gotCtx.Filter.CollectionIDs, wantCols) {
		t.Errorf("CollectionIDs: got %v, want %v", gotCtx.Filter.CollectionIDs, wantCols)
	}
	wantTags := []string{"sale", "clearance"}
	if !stringSliceEqual(gotCtx.Filter.Tags, wantTags) {
		t.Errorf("Tags: got %v, want %v", gotCtx.Filter.Tags, wantTags)
	}
	wantExclude := []string{"adult"}
	if !stringSliceEqual(gotCtx.Filter.ExcludeTags, wantExclude) {
		t.Errorf("ExcludeTags: got %v, want %v", gotCtx.Filter.ExcludeTags, wantExclude)
	}
}

// TestStorefrontMiddleware_EmptyTokensCollapse defends against a subtle
// bug: a header like "," or ", ,," parses to zero tokens — the middleware
// must return a nil filter (not an empty-slice filter), because
// StorefrontFilter.IsEmpty relies on nil-ness for the happy-path check.
func TestStorefrontMiddleware_EmptyTokensCollapse(t *testing.T) {
	g := &Gateway{}
	var gotCtx *StorefrontContext
	h := g.StorefrontMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotCtx = StorefrontFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	req.Header.Set(headerStorefrontID, "spring-sale")
	req.Header.Set(headerStorefrontFilterCollections, ", ,,")
	req.Header.Set(headerStorefrontFilterTags, " ")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if gotCtx == nil {
		t.Fatal("expected storefront context")
	}
	if gotCtx.Filter != nil {
		t.Fatalf("expected nil filter when all filter tokens are empty, got %+v", gotCtx.Filter)
	}
}

// TestStorefrontMiddleware_WhitespaceIDIgnored ensures whitespace-only
// X-Storefront-ID is treated as absent (matches the TrimSpace check in the
// middleware). Protects against buggy proxies that blank-fill headers.
func TestStorefrontMiddleware_WhitespaceIDIgnored(t *testing.T) {
	g := &Gateway{}
	var gotCtx *StorefrontContext
	h := g.StorefrontMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotCtx = StorefrontFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	req.Header.Set(headerStorefrontID, "   ")
	req.Header.Set(headerStorefrontFilterCollections, "col-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if gotCtx != nil {
		t.Fatalf("expected no context when ID is whitespace-only, got %+v", gotCtx)
	}
}

// TestStorefrontFilter_IsEmpty covers the nil-vs-empty semantics
// handleGETListingIndex relies on for its short-circuit.
func TestStorefrontFilter_IsEmpty(t *testing.T) {
	var nilFilter *StorefrontFilter
	if !nilFilter.IsEmpty() {
		t.Error("nil filter should be empty")
	}
	if !(&StorefrontFilter{}).IsEmpty() {
		t.Error("zero-value filter should be empty")
	}
	f := &StorefrontFilter{CollectionIDs: []string{"c1"}}
	if f.IsEmpty() {
		t.Error("filter with CollectionIDs should not be empty")
	}
	f = &StorefrontFilter{Tags: []string{"t1"}}
	if f.IsEmpty() {
		t.Error("filter with Tags should not be empty")
	}
	f = &StorefrontFilter{ExcludeTags: []string{"t1"}}
	if f.IsEmpty() {
		t.Error("filter with ExcludeTags should not be empty")
	}
}

// TestStorefrontFromContext_NilCtx ensures the helper survives a nil
// context gracefully (defensive — shouldn't happen in HTTP flows, but
// handler unit tests sometimes pass context.TODO() with no value stored).
func TestStorefrontFromContext_NilCtx(t *testing.T) {
	if got := StorefrontFromContext(nil); got != nil {
		t.Errorf("nil ctx: expected nil, got %+v", got)
	}
	if got := StorefrontFromContext(context.Background()); got != nil {
		t.Errorf("empty ctx: expected nil, got %+v", got)
	}
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

//go:build !private_distribution

package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 1×1 PNG (transparent) used as a stand-in for Gumroad's thumbnail CDN.
var tinyPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
	0x42, 0x60, 0x82,
}

// newGumroadTestServer returns an httptest TLS server that fakes:
//   - GET /v2/products: returns the supplied products list
//   - GET /thumbs/{anything}: returns tinyPNG so downloadThumbnail succeeds
//
// TLS is required because the production downloadThumbnail enforces an
// https-only scheme as part of the SSRF defense. The returned `srv.Client()`
// trusts the in-memory cert so callers don't need to disable verification.
//
// The Authorization header check is best-effort (not strict) so a single
// fixture covers happy-path and "no thumbnail" tests via path manipulation.
func newGumroadTestServer(t *testing.T, products []gumroadAPIProduct, listStatus int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/products", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, `{"success":false,"message":"missing bearer"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if listStatus != 0 && listStatus != http.StatusOK {
			w.WriteHeader(listStatus)
			_, _ = w.Write([]byte(`{"success":false,"message":"forced error"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(gumroadListProductsResponse{
			Success:  true,
			Products: products,
		})
	})
	mux.HandleFunc("/thumbs/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(tinyPNG)
	})
	mux.HandleFunc("/missing-thumb/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	return httptest.NewTLSServer(mux)
}

func TestTransformGumroadProduct_Eligible(t *testing.T) {
	p := gumroadAPIProduct{
		ID:          "abc123",
		Name:        "My Notion Template",
		Description: "<p>Best template ever</p>",
		Price:       1500, // $15.00
		Currency:    "usd",
		Tags:        []string{"notion", " productivity ", "notion", ""},
		Published:   true,
	}
	in, skip := transformGumroadProduct(p, "gumroad-abc123.png", true)
	if skip != "" {
		t.Fatalf("unexpected skip reason: %s", skip)
	}
	if in.Title != "My Notion Template" {
		t.Errorf("title = %q", in.Title)
	}
	if in.ContractType != "DIGITAL_GOOD" {
		t.Errorf("contractType = %q", in.ContractType)
	}
	if in.Price != "15.00" {
		t.Errorf("price = %q (want 15.00)", in.Price)
	}
	if in.PricingCurrency != "USD" {
		t.Errorf("currency = %q (want USD)", in.PricingCurrency)
	}
	if in.Status != "draft" {
		t.Errorf("status = %q (want draft)", in.Status)
	}
	if got, want := in.Tags, []string{"notion", "productivity"}; len(got) != len(want) {
		t.Errorf("tags = %v (want %v)", got, want)
	} else {
		for i, tag := range want {
			if got[i] != tag {
				t.Errorf("tag[%d] = %q (want %q)", i, got[i], tag)
			}
		}
	}
	if len(in.Images) != 1 || in.Images[0] != "gumroad-abc123.png" {
		t.Errorf("images = %v", in.Images)
	}
	if in.Description != "<p>Best template ever</p>" {
		t.Errorf("description was stripped: %q", in.Description)
	}
}

func TestTransformGumroadProduct_AsPublished(t *testing.T) {
	in, skip := transformGumroadProduct(gumroadAPIProduct{
		ID: "x", Name: "x", Price: 100, Currency: "usd",
	}, "gumroad-x.jpg", false /* asDraft=false */)
	if skip != "" {
		t.Fatalf("unexpected skip: %s", skip)
	}
	if in.Status != "published" {
		t.Errorf("status = %q (want published)", in.Status)
	}
}

func TestTransformGumroadProduct_SkipReasons(t *testing.T) {
	cases := []struct {
		name        string
		product     gumroadAPIProduct
		thumbName   string
		wantSkipSub string
	}{
		{"deleted", gumroadAPIProduct{ID: "1", Name: "x", Deleted: true}, "f.png", "deleted"},
		{"shipping", gumroadAPIProduct{ID: "1", Name: "x", RequireShipping: true}, "f.png", "shipping"},
		{"tiered", gumroadAPIProduct{ID: "1", Name: "x", IsTieredMembership: true}, "f.png", "tiered membership"},
		{"customizable", gumroadAPIProduct{ID: "1", Name: "x", CustomizablePrice: true}, "f.png", "pay what you want"},
		{"negative price", gumroadAPIProduct{ID: "1", Name: "x", Price: -1}, "f.png", "invalid price"},
		{"empty name", gumroadAPIProduct{ID: "1", Name: ""}, "f.png", "no name"},
		{"no thumb", gumroadAPIProduct{ID: "1", Name: "x", Price: 100}, "", "no usable thumbnail"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, skip := transformGumroadProduct(tc.product, tc.thumbName, true)
			if !strings.Contains(strings.ToLower(skip), tc.wantSkipSub) {
				t.Errorf("skip = %q (want substring %q)", skip, tc.wantSkipSub)
			}
		})
	}
}

func TestTransformGumroadProduct_DefaultCurrency(t *testing.T) {
	in, skip := transformGumroadProduct(gumroadAPIProduct{
		ID: "x", Name: "x", Price: 100, Currency: "",
	}, "f.png", true)
	if skip != "" {
		t.Fatalf("unexpected skip: %s", skip)
	}
	if in.PricingCurrency != "USD" {
		t.Errorf("default currency = %q (want USD)", in.PricingCurrency)
	}
}

func TestSanitizeFilenameSegment(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"abc123", "abc123"},
		{"my-id_42", "my-id_42"},
		{"a/b\\c", "a_b_c"},
		{"foo bar", "foo_bar"},
		{"", "x"},
		{strings.Repeat("a", 100), strings.Repeat("a", 64)},
	}
	for _, tc := range cases {
		got := sanitizeFilenameSegment(tc.in)
		if got != tc.want {
			t.Errorf("sanitizeFilenameSegment(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDedupeTrimNonEmpty(t *testing.T) {
	got := dedupeTrimNonEmpty([]string{"a", " a ", "b", "", "  ", "B", "b"})
	want := []string{"a", "b", "B"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGumroadClient_FetchProducts_Success(t *testing.T) {
	srv := newGumroadTestServer(t, []gumroadAPIProduct{
		{ID: "p1", Name: "Product 1", Price: 500, Currency: "usd"},
		{ID: "p2", Name: "Product 2", Price: 1000, Currency: "eur"},
	}, http.StatusOK)
	defer srv.Close()

	c := &gumroadClient{baseURL: srv.URL + "/v2", httpc: srv.Client()}
	products, total, err := c.fetchProducts(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("fetchProducts: %v", err)
	}
	if len(products) != 2 {
		t.Fatalf("got %d products, want 2", len(products))
	}
	if total != 2 {
		t.Errorf("totalAvailable = %d, want 2 (no truncation)", total)
	}
	if products[0].ID != "p1" {
		t.Errorf("first product ID = %q", products[0].ID)
	}
}

func TestGumroadClient_FetchProducts_Unauthorized(t *testing.T) {
	srv := newGumroadTestServer(t, nil, http.StatusUnauthorized)
	defer srv.Close()
	c := &gumroadClient{baseURL: srv.URL + "/v2", httpc: srv.Client()}
	_, _, err := c.fetchProducts(context.Background(), "bogus")
	if err == nil {
		t.Fatal("expected error for 401 status")
	}
	if !strings.Contains(err.Error(), "invalid Gumroad access token") {
		t.Errorf("error message = %q", err.Error())
	}
}

func TestGumroadClient_FetchProducts_RateLimit(t *testing.T) {
	srv := newGumroadTestServer(t, nil, http.StatusTooManyRequests)
	defer srv.Close()
	c := &gumroadClient{baseURL: srv.URL + "/v2", httpc: srv.Client()}
	_, _, err := c.fetchProducts(context.Background(), "tok")
	if err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected rate limit error, got %v", err)
	}
}

func TestGumroadClient_FetchProducts_EmptyToken(t *testing.T) {
	c := newGumroadClient()
	_, _, err := c.fetchProducts(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "access token is required") {
		t.Errorf("got %v", err)
	}
	_, _, err = c.fetchProducts(context.Background(), "   ")
	if err == nil {
		t.Errorf("blank token should error")
	}
}

// TestGumroadClient_FetchProducts_TruncationReportsTotal ensures we surface
// the original count when Gumroad returns more than maxGumroadProductsPerImport
// so the caller can warn the operator instead of silently dropping products.
func TestGumroadClient_FetchProducts_TruncationReportsTotal(t *testing.T) {
	overflow := maxGumroadProductsPerImport + 5
	products := make([]gumroadAPIProduct, overflow)
	for i := range products {
		products[i] = gumroadAPIProduct{ID: "p", Name: "n", Price: 100, Currency: "usd"}
	}
	srv := newGumroadTestServer(t, products, http.StatusOK)
	defer srv.Close()
	c := &gumroadClient{baseURL: srv.URL + "/v2", httpc: srv.Client()}
	got, total, err := c.fetchProducts(context.Background(), "tok")
	if err != nil {
		t.Fatalf("fetchProducts: %v", err)
	}
	if len(got) != maxGumroadProductsPerImport {
		t.Errorf("returned slice = %d, want capped at %d", len(got), maxGumroadProductsPerImport)
	}
	if total != overflow {
		t.Errorf("totalAvailable = %d, want %d", total, overflow)
	}
}

func TestGumroadClient_DownloadThumbnail_OK(t *testing.T) {
	srv := newGumroadTestServer(t, nil, http.StatusOK)
	defer srv.Close()
	c := &gumroadClient{baseURL: srv.URL + "/v2", httpc: srv.Client()}
	data, ext, err := c.downloadThumbnail(context.Background(), srv.URL+"/thumbs/x.png")
	if err != nil {
		t.Fatalf("downloadThumbnail: %v", err)
	}
	if len(data) != len(tinyPNG) {
		t.Errorf("got %d bytes, want %d", len(data), len(tinyPNG))
	}
	if ext != "png" {
		t.Errorf("ext = %q (want png)", ext)
	}
}

func TestGumroadClient_DownloadThumbnail_Empty(t *testing.T) {
	c := newGumroadClient()
	data, ext, err := c.downloadThumbnail(context.Background(), "")
	if err != nil || data != nil || ext != "" {
		t.Errorf("empty URL should be no-op, got data=%d ext=%q err=%v", len(data), ext, err)
	}
}

func TestGumroadClient_DownloadThumbnail_NotFound(t *testing.T) {
	srv := newGumroadTestServer(t, nil, http.StatusOK)
	defer srv.Close()
	c := &gumroadClient{baseURL: srv.URL + "/v2", httpc: srv.Client()}
	_, _, err := c.downloadThumbnail(context.Background(), srv.URL+"/missing-thumb/x.png")
	if err == nil {
		t.Fatal("expected error for 404 thumbnail")
	}
}

// TestGumroadClient_DownloadThumbnail_RejectsUnsafeURLs covers the URL-time
// SSRF guard. Connect-time guarding (ssrfSafeDialControl + CheckRedirect)
// is verified separately by isPublicIP and ssrfSafeDialControl unit tests
// — exercising the full http.Client redirect path requires booting a
// listener bound to a private IP, which is portable but fragile in CI.
func TestGumroadClient_DownloadThumbnail_RejectsUnsafeURLs(t *testing.T) {
	c := newGumroadClient()
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"http scheme", "http://example.com/x.png", "must be https"},
		{"file scheme", "file:///etc/passwd", "must be https"},
		{"gopher scheme", "gopher://attacker.example/", "must be https"},
		{"embedded userinfo", "https://attacker@127.0.0.1/x.png", "must not embed credentials"},
		{"missing host", "https:///x.png", "missing host"},
		{"relative", "/x.png", "must be absolute"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := c.downloadThumbnail(context.Background(), tc.url)
			if err == nil {
				t.Fatalf("expected error for %q", tc.url)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q missing substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestIsPublicIP(t *testing.T) {
	cases := []struct {
		ip     string
		public bool
	}{
		// Non-public — must all be rejected by the SSRF guard.
		{"127.0.0.1", false},      // loopback
		{"169.254.169.254", false}, // AWS/GCP/Azure metadata service
		{"10.0.0.1", false},        // RFC1918
		{"172.16.0.1", false},      // RFC1918
		{"192.168.1.1", false},     // RFC1918
		{"100.64.0.1", false},      // RFC6598 CGNAT (Go IsPrivate covers)
		{"0.0.0.0", false},         // unspecified
		{"224.0.0.1", false},       // multicast
		{"::1", false},             // IPv6 loopback
		{"fe80::1", false},         // IPv6 link-local
		{"fc00::1", false},         // IPv6 unique-local
		// Public — must be allowed.
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"2606:4700:4700::1111", true}, // Cloudflare DNS IPv6
	}
	for _, tc := range cases {
		t.Run(tc.ip, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("invalid test fixture %q", tc.ip)
			}
			if got := isPublicIP(ip); got != tc.public {
				t.Errorf("isPublicIP(%s) = %v, want %v", tc.ip, got, tc.public)
			}
		})
	}
}

// TestSSRFSafeDialControl exercises the dialer hook directly.
func TestSSRFSafeDialControl(t *testing.T) {
	if err := ssrfSafeDialControl("tcp", "169.254.169.254:80", nil); err == nil {
		t.Error("expected metadata IP to be refused")
	}
	if err := ssrfSafeDialControl("tcp", "127.0.0.1:443", nil); err == nil {
		t.Error("expected loopback to be refused")
	}
	if err := ssrfSafeDialControl("tcp", "8.8.8.8:443", nil); err != nil {
		t.Errorf("expected public IP to dial, got %v", err)
	}
}

func TestProcessGumroadImport_DryRunClassifies(t *testing.T) {
	srv := newGumroadTestServer(t, []gumroadAPIProduct{
		{
			ID: "ok1", Name: "Eligible Product", Price: 1000, Currency: "usd",
			ThumbnailURL: "PLACEHOLDER",
			Published:    true,
		},
		{
			ID: "skip1", Name: "Physical", Price: 2000, Currency: "usd",
			ThumbnailURL: "PLACEHOLDER", RequireShipping: true,
		},
		{
			ID: "skip2", Name: "Pay-what-you-want", Price: 0, Currency: "usd",
			ThumbnailURL: "PLACEHOLDER", CustomizablePrice: true,
		},
		{
			ID: "skip3", Name: "Tiered Membership", Price: 500, Currency: "usd",
			ThumbnailURL: "PLACEHOLDER", IsTieredMembership: true,
		},
		{
			ID: "skip4", Name: "Deleted", Price: 100, Currency: "usd",
			ThumbnailURL: "PLACEHOLDER", Deleted: true,
		},
	}, http.StatusOK)
	defer srv.Close()

	// Patch ThumbnailURLs to the test server (can't do at struct literal time
	// because srv.URL is only known after newGumroadTestServer returns).
	c := &gumroadClient{baseURL: srv.URL + "/v2", httpc: srv.Client()}
	products, _, err := c.fetchProducts(context.Background(), "tok")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	for i := range products {
		products[i].ThumbnailURL = srv.URL + "/thumbs/" + products[i].ID + ".png"
	}

	// Re-serve with patched URLs by spinning up a fresh fixture server.
	srv2 := newGumroadTestServer(t, products, http.StatusOK)
	defer srv2.Close()
	c2 := &gumroadClient{baseURL: srv2.URL + "/v2", httpc: srv2.Client()}

	out, err := processGumroadImport(context.Background(), c2, gumroadImportRequest{
		AccessToken: "tok",
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("processGumroadImport: %v", err)
	}
	if out.resp.TotalFetched != 5 {
		t.Errorf("totalFetched = %d, want 5", out.resp.TotalFetched)
	}
	if out.resp.EligibleCount != 1 {
		t.Errorf("eligibleCount = %d, want 1", out.resp.EligibleCount)
	}
	if out.resp.SkippedCount != 4 {
		t.Errorf("skippedCount = %d, want 4", out.resp.SkippedCount)
	}
	if out.resp.FileUploadReminder == "" {
		t.Error("file upload reminder missing")
	}
	if !out.resp.DryRun {
		t.Error("dryRun flag not echoed")
	}
	// Eligible item should have a slug + WillImport=true
	var foundEligible bool
	for _, item := range out.resp.Items {
		if item.WillImport {
			foundEligible = true
			if item.MappedSlug != "gumroad-ok1" {
				t.Errorf("mappedSlug = %q (want gumroad-ok1)", item.MappedSlug)
			}
		}
	}
	if !foundEligible {
		t.Error("no eligible item found in items")
	}
	if len(out.inputs) != 1 {
		t.Errorf("got %d transformed inputs, want 1", len(out.inputs))
	}
	if len(out.thumbnails) != 1 {
		t.Errorf("got %d thumbnails, want 1", len(out.thumbnails))
	}
}

func TestProcessGumroadImport_TruncationWarning(t *testing.T) {
	overflow := maxGumroadProductsPerImport + 3
	products := make([]gumroadAPIProduct, overflow)
	for i := range products {
		products[i] = gumroadAPIProduct{ID: "p", Name: "n", Price: 100, Currency: "usd"}
	}
	srv := newGumroadTestServer(t, products, http.StatusOK)
	defer srv.Close()
	c := &gumroadClient{baseURL: srv.URL + "/v2", httpc: srv.Client()}
	out, err := processGumroadImport(context.Background(), c, gumroadImportRequest{
		AccessToken: "tok",
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("processGumroadImport: %v", err)
	}
	if len(out.resp.Warnings) == 0 {
		t.Fatalf("expected truncation warning, got none")
	}
	if !strings.Contains(out.resp.Warnings[0], "only the first") {
		t.Errorf("warning text = %q (want truncation explanation)", out.resp.Warnings[0])
	}
}

func TestProcessGumroadImport_ProductIDFilter(t *testing.T) {
	products := []gumroadAPIProduct{
		{ID: "p1", Name: "P1", Price: 100, Currency: "usd"},
		{ID: "p2", Name: "P2", Price: 200, Currency: "usd"},
		{ID: "p3", Name: "P3", Price: 300, Currency: "usd"},
	}
	srv := newGumroadTestServer(t, products, http.StatusOK)
	defer srv.Close()

	// Patch URLs after server start
	for i := range products {
		products[i].ThumbnailURL = srv.URL + "/thumbs/" + products[i].ID + ".png"
	}
	srv2 := newGumroadTestServer(t, products, http.StatusOK)
	defer srv2.Close()

	c := &gumroadClient{baseURL: srv2.URL + "/v2", httpc: srv2.Client()}
	out, err := processGumroadImport(context.Background(), c, gumroadImportRequest{
		AccessToken: "tok",
		DryRun:      true,
		ProductIDs:  []string{"p1", "p3"},
	})
	if err != nil {
		t.Fatalf("processGumroadImport: %v", err)
	}
	if out.resp.TotalFetched != 2 {
		t.Errorf("totalFetched = %d (want 2 after filter)", out.resp.TotalFetched)
	}
	if out.resp.EligibleCount != 2 {
		t.Errorf("eligibleCount = %d (want 2)", out.resp.EligibleCount)
	}
}

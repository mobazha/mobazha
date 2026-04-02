package ssr

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegration_FullChain spins up a mock Node API + SSR handler and
// verifies the entire request flow: crawler UA → meta-enriched HTML,
// browser UA → plain SPA, embed cards, and oEmbed API.
func TestIntegration_FullChain(t *testing.T) {
	mockNodeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/v1/listings/"):
			fmt.Fprint(w, `{"data":{"listing":{"slug":"test-product","vendorID":{"peerID":"QmTestPeer"},"metadata":{"pricingCurrency":{"code":"USD"}},"item":{"title":"Test Product","description":"A wonderful test product for integration testing","shortDescription":"Test product","price":"2999","images":[{"medium":"QmTestImage123","small":"","large":"","original":"","filename":"test.jpg"}]}}}}`)
		case strings.Contains(r.URL.Path, "/v1/profiles/"):
			fmt.Fprint(w, `{"data":{"peerID":"QmTestPeer","name":"Integration Store","handle":"@intstore","about":"The best store for testing","location":"Test City","avatarHashes":{"medium":"QmAvatar456","small":"","original":""},"headerHashes":{"large":"QmHeader789","medium":"","original":""},"stats":{"listingCount":15,"ratingCount":8,"averageRating":4.7}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockNodeAPI.Close()

	var port int
	fmt.Sscanf(mockNodeAPI.URL[strings.LastIndex(mockNodeAPI.URL, ":")+1:], "%d", &port)

	spaDir := t.TempDir()
	spaHTML := `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Mobazha</title></head><body><div id="root"></div></body></html>`
	os.WriteFile(filepath.Join(spaDir, "index.html"), []byte(spaHTML), 0644)

	handler, err := New(Config{
		NodePort:    port,
		SPADir:      spaDir,
		Domain:      "test.mobazha.org",
		LocalPeerID: "QmTestPeer",
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	t.Run("Crawler_gets_meta_enriched_product_page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/product/test-product", nil)
		req.Header.Set("User-Agent", "Googlebot/2.1")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("status=%d", w.Code)
		}
		body := w.Body.String()

		mustContain(t, body, `og:title" content="Test Product"`)
		mustContain(t, body, `og:image" content="https://test.mobazha.org/v1/media/images/QmTestImage123"`)
		mustContain(t, body, `"@type":"Product"`)
		mustContain(t, body, `"price":"2999"`)
		mustContain(t, body, `application/json+oembed`)
		mustContain(t, body, `<div id="root">`)
	})

	t.Run("Browser_gets_plain_SPA", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/product/test-product", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15) Chrome/120.0")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("status=%d", w.Code)
		}
		body := w.Body.String()

		if strings.Contains(body, "og:title") {
			t.Error("browser should NOT get OG tags")
		}
		mustContain(t, body, `<div id="root">`)
	})

	t.Run("Crawler_gets_meta_enriched_store_page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/store/QmTestPeer", nil)
		req.Header.Set("User-Agent", "Twitterbot/1.0")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("status=%d", w.Code)
		}
		body := w.Body.String()

		mustContain(t, body, `og:title" content="Integration Store — Mobazha Store"`)
		mustContain(t, body, `"@type":"Organization"`)
		mustContain(t, body, `twitter:card" content="summary"`)
	})

	t.Run("Embed_product_card", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/embed/product/test-product", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("status=%d", w.Code)
		}
		body := w.Body.String()

		mustContain(t, body, "Test Product")
		mustContain(t, body, "2999")
		mustContain(t, body, "on Mobazha")
		mustContain(t, body, "mobazha-embed-resize")
	})

	t.Run("Embed_product_card_dark_mode", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/embed/product/test-product?theme=dark", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		body := w.Body.String()
		mustContain(t, body, `class="dark"`)
	})

	t.Run("Embed_store_card", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/embed/store/QmTestPeer", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("status=%d", w.Code)
		}
		body := w.Body.String()

		mustContain(t, body, "Integration Store")
		mustContain(t, body, "15 products")
		mustContain(t, body, "on Mobazha")
	})

	t.Run("OEmbed_product", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/oembed?url=https://test.mobazha.org/product/test-product&format=json", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("status=%d", w.Code)
		}
		body := w.Body.String()

		mustContain(t, body, `"type":"rich"`)
		mustContain(t, body, `"title":"Test Product"`)
		mustContain(t, body, `iframe`)
		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Error("missing CORS header")
		}
	})

	t.Run("OEmbed_store", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/oembed?url=https://test.mobazha.org/store/QmTestPeer&format=json", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("status=%d", w.Code)
		}
		body := w.Body.String()

		mustContain(t, body, `"title":"Integration Store — Mobazha Store"`)
	})

	t.Run("OEmbed_disallowed_host", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/oembed?url=https://evil.com/product/x", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})
}

func mustContain(t *testing.T, body, substr string) {
	t.Helper()
	if !strings.Contains(body, substr) {
		t.Errorf("response missing %q\n\nbody (first 500 chars):\n%s", substr, truncateStr(body, 500))
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

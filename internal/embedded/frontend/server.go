package frontend

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ServerConfig configures the embedded frontend HTTP handler.
type ServerConfig struct {
	// OverrideDir, when set, serves files from this directory first,
	// falling back to the embedded DistFS. This allows operators to
	// replace the frontend without rebuilding the binary.
	OverrideDir string

	// SaaSURL is the SaaS platform URL for standalone buyer OAuth.
	// When set, the handler serves a dynamic /runtime-config.js that
	// switches the frontend to standalone mode.
	SaaSURL string

	// GuestCheckoutEnabled tells the frontend whether guest (anonymous)
	// checkout is available on this node.
	GuestCheckoutEnabled bool
}

// NewHandler returns an http.Handler that serves the SPA frontend.
// It tries (in order):
//  1. Brotli pre-compressed file (.br) if the client supports it
//  2. External override directory (if configured)
//  3. Embedded DistFS
//  4. Falls back to index.html for SPA client-side routing
func NewHandler(cfg ServerConfig) http.Handler {
	embeddedSub, _ := fs.Sub(DistFS, "dist")

	return &spaHandler{
		embedded:             embeddedSub,
		overrideDir:          cfg.OverrideDir,
		saasURL:              cfg.SaaSURL,
		guestCheckoutEnabled: cfg.GuestCheckoutEnabled,
	}
}

type spaHandler struct {
	embedded             fs.FS
	overrideDir          string
	saasURL              string
	guestCheckoutEnabled bool
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/runtime-config.js" {
		h.serveRuntimeConfig(w)
		return
	}

	urlPath := strings.TrimPrefix(r.URL.Path, "/")
	if urlPath == "" {
		urlPath = "index.html"
	}

	if h.tryServeBrotli(w, r, urlPath) {
		return
	}

	if h.overrideDir != "" {
		diskPath := filepath.Clean(filepath.Join(h.overrideDir, filepath.FromSlash(urlPath)))
		if !strings.HasPrefix(diskPath, filepath.Clean(h.overrideDir)+string(os.PathSeparator)) &&
			diskPath != filepath.Clean(h.overrideDir) {
			http.NotFound(w, r)
			return
		}
		if info, err := os.Stat(diskPath); err == nil && !info.IsDir() {
			http.ServeFile(w, r, diskPath)
			return
		}
	}

	if h.embedded != nil {
		if f, err := h.embedded.Open(urlPath); err == nil {
			f.Close()
			http.ServeFileFS(w, r, h.embedded, urlPath)
			return
		}
	}

	h.serveIndex(w, r)
}

func (h *spaHandler) tryServeBrotli(w http.ResponseWriter, r *http.Request, urlPath string) bool {
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "br") {
		return false
	}

	brPath := urlPath + ".br"

	if h.overrideDir != "" {
		diskBr := filepath.Clean(filepath.Join(h.overrideDir, filepath.FromSlash(brPath)))
		if !strings.HasPrefix(diskBr, filepath.Clean(h.overrideDir)+string(os.PathSeparator)) {
			return false
		}
		if info, err := os.Stat(diskBr); err == nil && !info.IsDir() {
			w.Header().Set("Content-Encoding", "br")
			w.Header().Set("Content-Type", sniffContentType(urlPath))
			setCacheHeaders(w, urlPath)
			http.ServeFile(w, r, diskBr)
			return true
		}
	}

	if h.embedded != nil {
		if f, err := h.embedded.Open(brPath); err == nil {
			f.Close()
			w.Header().Set("Content-Encoding", "br")
			w.Header().Set("Content-Type", sniffContentType(urlPath))
			setCacheHeaders(w, urlPath)
			http.ServeFileFS(w, r, h.embedded, brPath)
			return true
		}
	}

	return false
}

func (h *spaHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	if h.overrideDir != "" {
		indexPath := filepath.Join(h.overrideDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}
	}

	if h.embedded != nil {
		http.ServeFileFS(w, r, h.embedded, "index.html")
		return
	}
	http.NotFound(w, r)
}

// serveRuntimeConfig returns a JS snippet that tells the SPA it is
// running in standalone mode (native binary or Docker without build-time env).
func (h *spaHandler) serveRuntimeConfig(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "no-cache")

	saasURL := h.saasURL
	if saasURL == "" {
		saasURL = "https://app.mobazha.org"
	}

	guestCheckout := "false"
	if h.guestCheckoutEnabled {
		guestCheckout = "true"
	}

	fmt.Fprintf(w, `window.__RUNTIME_CONFIG__={saasUrl:"%s",authMode:"standalone",guestCheckoutEnabled:%s};`, saasURL, guestCheckout)
}

func sniffContentType(name string) string {
	switch {
	case strings.HasSuffix(name, ".js"):
		return "application/javascript"
	case strings.HasSuffix(name, ".css"):
		return "text/css"
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".json"):
		return "application/json"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(name, ".woff"):
		return "font/woff"
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	default:
		return "application/octet-stream"
	}
}

func setCacheHeaders(w http.ResponseWriter, name string) {
	if strings.Contains(name, "/assets/") || strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
}

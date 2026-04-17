package frontend

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// FeatureSnapshot is a flattened, frontend-facing view of a single registered
// feature flag. It is produced by the caller (the node's Resolver) and
// serialized into /runtime-config.js so the SPA can bootstrap synchronously
// without waiting for an API round-trip.
//
// Fields intentionally mirror FEATURE_FLAG_ARCHITECTURE.md §4.3.
type FeatureSnapshot struct {
	// Key is the canonical registry key (camelCase), e.g. "guestCheckout".
	Key string
	// Effective is the resolved boolean after all three layers
	// (platform_global AND tenant AND node_runtime) are applied.
	Effective bool
	// Overridable lists the scopes where an operator is allowed to change
	// this feature (e.g. ["tenant"]). Stable order: platform_global,
	// tenant, node_runtime.
	Overridable []string
}

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

	// FeaturesSnapshotFn returns the current set of feature flags and
	// their effective values for the requesting caller. It is invoked
	// per /runtime-config.js request so resolver updates (via PUT
	// /v1/settings/features/{key} or PATCH /platform/v1/features/{key})
	// propagate to the SPA without a process restart. A nil callback
	// yields an empty features map (fail-closed).
	FeaturesSnapshotFn func(context.Context) []FeatureSnapshot
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
		embedded:           embeddedSub,
		overrideDir:        cfg.OverrideDir,
		saasURL:            cfg.SaaSURL,
		featuresSnapshotFn: cfg.FeaturesSnapshotFn,
	}
}

type spaHandler struct {
	embedded           fs.FS
	overrideDir        string
	saasURL            string
	featuresSnapshotFn func(context.Context) []FeatureSnapshot
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/runtime-config.js" {
		h.serveRuntimeConfig(w, r)
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

// runtimeFeatureEntry is the per-feature shape inside
// window.__RUNTIME_CONFIG__.features. Keep it aligned with
// FEATURE_FLAG_ARCHITECTURE.md §4.3 and the frontend's
// featureFlags.initialize() parser.
type runtimeFeatureEntry struct {
	Effective   bool     `json:"effective"`
	Overridable []string `json:"overridable"`
}

// runtimeConfigPayload captures the fields embedded into
// window.__RUNTIME_CONFIG__ on every page load.
//
// Deprecated fields (guestCheckoutEnabled) exist for backward compatibility
// with older bundles that still read the flat boolean. Once the unified
// featureFlags service ships (Phase B of ff-impl-frontend), these flat
// fields move to TECHDEBT(TD-032) and get removed in Phase E.
type runtimeConfigPayload struct {
	SaasURL              string                         `json:"saasUrl"`
	AuthMode             string                         `json:"authMode"`
	GuestCheckoutEnabled bool                           `json:"guestCheckoutEnabled"`
	Features             map[string]runtimeFeatureEntry `json:"features"`
}

// serveRuntimeConfig emits a JS snippet that assigns window.__RUNTIME_CONFIG__
// synchronously before the SPA boots. The payload is marshalled via
// encoding/json so strings inside Overridable or future fields are safely
// escaped — do not revert to fmt.Fprintf string interpolation.
func (h *spaHandler) serveRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "no-cache")

	saasURL := h.saasURL
	if saasURL == "" {
		saasURL = "https://app.mobazha.org"
	}

	features := map[string]runtimeFeatureEntry{}
	guestCheckoutEnabled := false

	if h.featuresSnapshotFn != nil {
		for _, f := range h.featuresSnapshotFn(r.Context()) {
			if f.Key == "" {
				continue
			}
			overridable := f.Overridable
			if overridable == nil {
				// json.Marshal emits `null` for nil slices; the frontend
				// expects `[]` for "no overrides allowed", so normalize
				// here to avoid client-side null checks.
				overridable = []string{}
			}
			features[f.Key] = runtimeFeatureEntry{
				Effective:   f.Effective,
				Overridable: overridable,
			}
			if f.Key == "guestCheckout" {
				guestCheckoutEnabled = f.Effective
			}
		}
	}

	payload := runtimeConfigPayload{
		SaasURL:              saasURL,
		AuthMode:             "standalone",
		GuestCheckoutEnabled: guestCheckoutEnabled,
		Features:             features,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		// Marshal can only fail on unsupported types; payload is all
		// plain strings/bools/maps. Fall back to a minimal static config
		// rather than a 500 so the SPA still boots.
		fmt.Fprintf(w, `window.__RUNTIME_CONFIG__={saasUrl:%q,authMode:"standalone",guestCheckoutEnabled:false,features:{}};`, saasURL)
		return
	}

	fmt.Fprintf(w, "window.__RUNTIME_CONFIG__=%s;", body)
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

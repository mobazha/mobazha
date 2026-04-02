package ssr

import (
	"embed"
	"html/template"
	"net/http"
	"os"
	"strconv"

	"github.com/mobazha/mobazha3.0/pkg/logging"
)

var log = logging.MustGetLogger("SSR")

//go:embed templates/*.html
var templateFS embed.FS

// SSRHandler serves meta-enriched HTML for crawlers and embed cards
// for social-media link previews. Human browsers receive the SPA as-is.
type SSRHandler struct {
	nodePort    int
	spaDir      string
	spaHTML     []byte
	domain      string
	localPeerID string
	httpClient  *http.Client
	templates   *template.Template
}

// Config holds the SSR handler configuration.
type Config struct {
	NodePort    int
	SPADir      string
	Domain      string
	LocalPeerID string
}

// NewFromEnv creates SSRHandler by reading config from environment variables,
// falling back to sensible defaults. Returns nil if the SPA directory is missing.
func NewFromEnv(localPeerID string) *SSRHandler {
	spaDir := envOr("SSR_SPA_DIR", "/srv/www")
	indexPath := spaDir + "/index.html"
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return nil
	}

	port, _ := strconv.Atoi(envOr("NODE_PORT", "5102"))
	domain := os.Getenv("STORE_DOMAIN")

	h, err := New(Config{
		NodePort:    port,
		SPADir:      spaDir,
		Domain:      domain,
		LocalPeerID: localPeerID,
	})
	if err != nil {
		log.Warningf("SSR handler init failed: %v", err)
		return nil
	}
	return h
}

// New creates a new SSRHandler.
func New(cfg Config) (*SSRHandler, error) {
	indexPath := cfg.SPADir + "/index.html"
	spaHTML, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &SSRHandler{
		nodePort:    cfg.NodePort,
		spaDir:      cfg.SPADir,
		spaHTML:     spaHTML,
		domain:      cfg.Domain,
		localPeerID: cfg.LocalPeerID,
		httpClient:  newInternalClient(),
		templates:   tmpl,
	}, nil
}

// RegisterRoutes registers SSR routes on the given ServeMux.
// These routes take priority over the Caddy SPA catch-all.
func (h *SSRHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /product/{slug}", h.handleProductPage)
	mux.HandleFunc("GET /store/{peerId}", h.handleStorePage)
	mux.HandleFunc("GET /embed/product/{slug}", h.handleEmbedProduct)
	mux.HandleFunc("GET /embed/store/{peerId}", h.handleEmbedStore)
	mux.HandleFunc("GET /api/oembed", h.handleOEmbed)

	log.Infof("SSR handler registered (domain=%s, peerID=%s)", h.domain, truncatePeerID(h.localPeerID))
}

func (h *SSRHandler) handleProductPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		h.serveSPA(w)
		return
	}

	if !IsCrawler(r.UserAgent()) {
		h.serveSPA(w)
		return
	}

	product, err := h.fetchProduct(slug)
	if err != nil {
		log.Debugf("SSR product fetch failed for %q: %v", slug, err)
		h.serveSPA(w)
		return
	}

	enriched, err := h.injectProductMeta(product)
	if err != nil {
		log.Warningf("SSR meta injection failed for %q: %v", slug, err)
		h.serveSPA(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(enriched)
}

func (h *SSRHandler) handleStorePage(w http.ResponseWriter, r *http.Request) {
	peerID := r.PathValue("peerId")
	if peerID == "" {
		h.serveSPA(w)
		return
	}

	if !IsCrawler(r.UserAgent()) {
		h.serveSPA(w)
		return
	}

	profile, err := h.fetchProfile(peerID)
	if err != nil {
		log.Debugf("SSR profile fetch failed for %q: %v", peerID, err)
		h.serveSPA(w)
		return
	}

	enriched, err := h.injectProfileMeta(profile)
	if err != nil {
		log.Warningf("SSR meta injection failed for %q: %v", peerID, err)
		h.serveSPA(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(enriched)
}

func (h *SSRHandler) serveSPA(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(h.spaHTML)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func truncatePeerID(pid string) string {
	if len(pid) > 12 {
		return pid[:8] + "…"
	}
	return pid
}


package ssr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type oembedResponse struct {
	Type         string `json:"type"`
	Version      string `json:"version"`
	Title        string `json:"title,omitempty"`
	ProviderName string `json:"provider_name"`
	ProviderURL  string `json:"provider_url"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	HTML         string `json:"html"`
}

func (h *SSRHandler) handleOEmbed(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		http.Error(w, `{"error":"missing url parameter"}`, http.StatusBadRequest)
		return
	}

	format := r.URL.Query().Get("format")
	if format != "" && format != "json" {
		http.Error(w, `{"error":"only json format is supported"}`, http.StatusNotImplemented)
		return
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		http.Error(w, `{"error":"invalid url"}`, http.StatusBadRequest)
		return
	}

	if !h.isAllowedHost(parsed.Hostname()) {
		http.Error(w, `{"error":"url hostname not allowed"}`, http.StatusForbidden)
		return
	}

	maxWidth := intParam(r, "maxwidth", 480)
	maxHeight := intParam(r, "maxheight", 320)

	path := strings.TrimSuffix(parsed.Path, "/")
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	var resp *oembedResponse

	switch {
	case len(parts) == 2 && parts[0] == "product":
		resp, err = h.oembedProduct(parts[1], rawURL, maxWidth, maxHeight)
	case len(parts) == 2 && parts[0] == "store":
		resp, err = h.oembedStore(parts[1], rawURL, maxWidth, maxHeight)
	default:
		http.Error(w, `{"error":"unsupported url pattern"}`, http.StatusNotFound)
		return
	}

	if err != nil {
		log.Debugf("oEmbed data fetch error: %v", err)
		http.Error(w, `{"error":"resource not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(resp)
}

func (h *SSRHandler) oembedProduct(slug, originalURL string, maxW, maxH int) (*oembedResponse, error) {
	product, err := h.fetchProduct(slug)
	if err != nil {
		return nil, err
	}

	embedURL := fmt.Sprintf("https://%s/embed/product/%s", h.domain, slug)
	iframeHTML := buildIframe(embedURL, maxW, maxH)

	return &oembedResponse{
		Type:         "rich",
		Version:      "1.0",
		Title:        product.Title,
		ProviderName: "Mobazha",
		ProviderURL:  fmt.Sprintf("https://%s", h.domain),
		Width:        maxW,
		Height:       maxH,
		HTML:         iframeHTML,
	}, nil
}

func (h *SSRHandler) oembedStore(peerID, originalURL string, maxW, maxH int) (*oembedResponse, error) {
	profile, err := h.fetchProfile(peerID)
	if err != nil {
		return nil, err
	}

	embedURL := fmt.Sprintf("https://%s/embed/store/%s", h.domain, peerID)
	iframeHTML := buildIframe(embedURL, maxW, maxH)

	return &oembedResponse{
		Type:         "rich",
		Version:      "1.0",
		Title:        displayName(profile) + " — Mobazha Store",
		ProviderName: "Mobazha",
		ProviderURL:  fmt.Sprintf("https://%s", h.domain),
		Width:        maxW,
		Height:       maxH,
		HTML:         iframeHTML,
	}, nil
}

func (h *SSRHandler) isAllowedHost(hostname string) bool {
	allowed := map[string]bool{
		"app.mobazha.org":   true,
		"store.mobazha.org": true,
		"localhost":         true,
	}
	if h.domain != "" {
		allowed[h.domain] = true
	}
	return allowed[hostname]
}

func buildIframe(embedURL string, maxW, maxH int) string {
	return fmt.Sprintf(
		`<iframe src="%s" width="%d" height="%d" frameborder="0" `+
			`scrolling="no" style="border:none;max-width:100%%;overflow:hidden" `+
			`sandbox="allow-scripts allow-same-origin allow-popups" `+
			`loading="lazy"></iframe>`,
		embedURL, maxW, maxH,
	)
}

func intParam(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}

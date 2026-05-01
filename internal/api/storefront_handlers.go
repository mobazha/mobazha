package api

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"

	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

const maxStoreConfigSize = 100 * 1024 // 100 KB

// storeConfigProvider is implemented by MobazhaNode.
type storeConfigProvider interface {
	StoreConfig() (json.RawMessage, error)
	SaveStoreConfig(json.RawMessage) error
}

func getStoreConfigProvider(r *http.Request) storeConfigProvider {
	node := getNodeService(r)
	if p, ok := node.(storeConfigProvider); ok {
		return p
	}
	return nil
}

// handleGETStorefrontConfig returns the owner's store config (may include draft).
func (g *Gateway) handleGETStorefrontConfig(w http.ResponseWriter, r *http.Request) {
	p := getStoreConfigProvider(r)
	if p == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Storefront config not available")
		return
	}

	cfg, err := p.StoreConfig()
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to read storefront config")
		return
	}
	if cfg == nil {
		responsePkg.Success(w, nil)
		return
	}
	responsePkg.Success(w, cfg)
}

// handlePUTStorefrontConfig replaces the owner's store config.
func (g *Gateway) handlePUTStorefrontConfig(w http.ResponseWriter, r *http.Request) {
	p := getStoreConfigProvider(r)
	if p == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Storefront config not available")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxStoreConfigSize)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "Request body too large or unreadable")
		return
	}

	if err := validateStoreConfigJSON(body); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
		return
	}

	cfg := json.RawMessage(body)
	if err := p.SaveStoreConfig(cfg); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to save storefront config")
		return
	}

	responsePkg.Success(w, cfg)
}

// handleGETStorefrontConfigPublic returns the published store config for a peer (no auth).
func (g *Gateway) handleGETStorefrontConfigPublic(w http.ResponseWriter, r *http.Request) {
	peerID := chi.URLParam(r, "peerID")
	if peerID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "Missing peerID")
		return
	}

	p := getStoreConfigProvider(r)
	if p == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Storefront config not available")
		return
	}

	cfg, err := p.StoreConfig()
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to read storefront config")
		return
	}
	if cfg == nil {
		responsePkg.Success(w, nil)
		return
	}

	// Filter to published-only if status field exists
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(cfg, &parsed); err == nil {
		if statusRaw, ok := parsed["status"]; ok {
			var status string
			if json.Unmarshal(statusRaw, &status) == nil && status != "published" {
				responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "No published storefront config")
				return
			}
		}
	}

	responsePkg.Success(w, cfg)
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

var hexColorRe = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

var validSectionTypes = map[string]bool{
	"hero": true, "announcement-bar": true, "featured-products": true,
	"product-grid": true, "about": true, "trust-badges": true,
	"testimonials": true, "faq": true, "collections": true,
	"gallery": true, "rich-text": true, "contact": true, "store-tabs": true,
}

func validateStoreConfigJSON(data []byte) error {
	var raw struct {
		Version  int    `json:"version"`
		Status   string `json:"status"`
		Theme    *struct {
			PrimaryColor string `json:"primaryColor"`
		} `json:"theme"`
		Sections []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return &validationError{"Invalid JSON structure"}
	}
	if raw.Version != 1 {
		return &validationError{"version must be 1"}
	}
	if raw.Status != "draft" && raw.Status != "published" {
		return &validationError{"status must be 'draft' or 'published'"}
	}
	if raw.Theme == nil {
		return &validationError{"theme is required"}
	}
	if raw.Theme.PrimaryColor != "" && !hexColorRe.MatchString(raw.Theme.PrimaryColor) {
		return &validationError{"theme.primaryColor must be a valid hex color"}
	}
	if len(raw.Sections) > 20 {
		return &validationError{"sections cannot exceed 20"}
	}
	for _, s := range raw.Sections {
		if s.ID == "" {
			return &validationError{"each section must have an id"}
		}
		if !validSectionTypes[s.Type] {
			return &validationError{"unknown section type: " + s.Type}
		}
	}
	return nil
}

type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }

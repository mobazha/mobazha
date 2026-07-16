package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"

	responsePkg "github.com/mobazha/mobazha/pkg/response"
)

const maxStoreConfigSize = 100 * 1024 // 100 KB

// storefrontPreviewTokenTTL bounds how long a draft-preview share link works.
// Long enough to collect feedback over a week; short enough that a leaked
// link does not expose the seller's drafts indefinitely.
const storefrontPreviewTokenTTL = 7 * 24 * time.Hour

// storeConfigProvider is implemented by MobazhaNode.
type storeConfigProvider interface {
	StoreConfig() (json.RawMessage, error)
	SaveStoreConfig(json.RawMessage) error
	StoreDraftConfig() (json.RawMessage, error)
	SaveStoreDraftConfig(json.RawMessage) error
	DeleteStoreDraftConfig() error
	// PublishStoreConfig atomically replaces the live config AND clears the draft.
	PublishStoreConfig(json.RawMessage) error
	// StoreConfigHistory returns previously-published configs, newest first.
	StoreConfigHistory() (json.RawMessage, error)
	StorefrontPreviewToken() (json.RawMessage, error)
	SaveStorefrontPreviewToken(json.RawMessage) error
}

// storefrontPreviewTokenRecord is the stored shape of the share token.
type storefrontPreviewTokenRecord struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func getStoreConfigProvider(r *http.Request) storeConfigProvider {
	node := getNodeService(r)
	if p, ok := node.(storeConfigProvider); ok {
		return p
	}
	return nil
}

// handleGETStorefrontConfig returns the owner's store config.
// ?variant=draft returns the unpublished draft slot (null when absent);
// ?variant=history returns previously-published configs, newest first.
func (g *Gateway) handleGETStorefrontConfig(w http.ResponseWriter, r *http.Request) {
	p := getStoreConfigProvider(r)
	if p == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Storefront config not available")
		return
	}

	var cfg json.RawMessage
	var err error
	switch r.URL.Query().Get("variant") {
	case "draft":
		cfg, err = p.StoreDraftConfig()
	case "history":
		cfg, err = p.StoreConfigHistory()
	default:
		cfg, err = p.StoreConfig()
	}
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

// handlePOSTStorefrontPreviewToken issues (and rotates) the draft-preview
// share token. Anyone holding the link can see the seller's draft until it
// expires, so issuing is owner-only and re-issuing revokes the old link.
func (g *Gateway) handlePOSTStorefrontPreviewToken(w http.ResponseWriter, r *http.Request) {
	p := getStoreConfigProvider(r)
	if p == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Storefront config not available")
		return
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to generate preview token")
		return
	}
	record := storefrontPreviewTokenRecord{
		Token:     hex.EncodeToString(buf),
		ExpiresAt: time.Now().UTC().Add(storefrontPreviewTokenTTL),
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to encode preview token")
		return
	}
	if err := p.SaveStorefrontPreviewToken(encoded); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to store preview token")
		return
	}
	responsePkg.Success(w, json.RawMessage(encoded))
}

// validPreviewToken reports whether the presented token matches the stored,
// unexpired record. Comparison is constant-time; every failure mode
// (no token issued, expired, mismatch, corrupt record) is indistinguishable
// to the caller.
func validPreviewToken(p storeConfigProvider, presented string) bool {
	if presented == "" {
		return false
	}
	raw, err := p.StorefrontPreviewToken()
	if err != nil || raw == nil {
		return false
	}
	var record storefrontPreviewTokenRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return false
	}
	if record.Token == "" || time.Now().UTC().After(record.ExpiresAt) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(record.Token), []byte(presented)) == 1
}

// handlePUTStorefrontConfig replaces the owner's store config. Routing is by
// the config's own status field: drafts land in the draft slot and leave the
// live config untouched; publishing writes the live slot and clears the draft.
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
	if storeConfigStatus(body) == "draft" {
		if err := p.SaveStoreDraftConfig(cfg); err != nil {
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to save storefront draft")
			return
		}
		responsePkg.Success(w, cfg)
		return
	}

	// Publishing supersedes any pending draft — one transaction, so we can
	// never report failure after the live config already changed.
	if err := p.PublishStoreConfig(cfg); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to save storefront config")
		return
	}

	responsePkg.Success(w, cfg)
}

// handleDELETEStorefrontDraft discards the owner's storefront draft.
func (g *Gateway) handleDELETEStorefrontDraft(w http.ResponseWriter, r *http.Request) {
	p := getStoreConfigProvider(r)
	if p == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Storefront config not available")
		return
	}
	if err := p.DeleteStoreDraftConfig(); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to discard storefront draft")
		return
	}
	responsePkg.Success(w, nil)
}

func storeConfigStatus(data []byte) string {
	var raw struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	return raw.Status
}

// handleGETStorefrontConfigPublic returns the published store config for a peer (no auth).
// With a valid ?preview=<token> it returns the unpublished draft instead —
// that is the whole point of the share-preview link.
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

	if token := r.URL.Query().Get("preview"); token != "" {
		if !validPreviewToken(p, token) {
			// Same status as "no published storefront": an invalid token must
			// not confirm whether a draft, a token, or the store itself exists.
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "No published storefront config")
			return
		}
		draft, err := p.StoreDraftConfig()
		if err != nil {
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to read storefront config")
			return
		}
		if draft != nil {
			responsePkg.Success(w, draft)
			return
		}
		// Seller published (or discarded) since sharing the link — fall
		// through to the live config so the link keeps showing something.
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
	"video": true, "countdown": true,
}

func validateStoreConfigJSON(data []byte) error {
	var raw struct {
		Version  int    `json:"version"`
		Status   string `json:"status"`
		Theme    *struct {
			PrimaryColor    string `json:"primaryColor"`
			BackgroundColor string `json:"backgroundColor"`
			TextColor       string `json:"textColor"`
			SurfaceColor    string `json:"surfaceColor"`
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
	for name, v := range map[string]string{
		"theme.backgroundColor": raw.Theme.BackgroundColor,
		"theme.textColor":       raw.Theme.TextColor,
		"theme.surfaceColor":    raw.Theme.SurfaceColor,
	} {
		if v != "" && !hexColorRe.MatchString(v) {
			return &validationError{name + " must be a valid hex color"}
		}
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

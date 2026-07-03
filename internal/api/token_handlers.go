package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha/pkg/apitoken"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/response"
)

type createTokenRequest struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	ExpiresIn *int     `json:"expires_in_days,omitempty"`
}

type createTokenResponse struct {
	Token string       `json:"token"`
	Info  tokenSummary `json:"info"`
}

type tokenSummary struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Prefix     string   `json:"prefix"`
	Scopes     []string `json:"scopes"`
	Active     bool     `json:"active"`
	Revoked    bool     `json:"revoked"`
	CreatedAt  string   `json:"created_at"`
	ExpiresAt  *string  `json:"expires_at,omitempty"`
	LastUsedAt *string  `json:"last_used_at,omitempty"`
}

// toTokenSummary builds the API view of an apitoken.Token. Centralising the
// formatting here keeps the create / list responses field-compatible.
func toTokenSummary(t *apitoken.Token) tokenSummary {
	scopes := t.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	v := tokenSummary{
		ID:        t.ID,
		Name:      t.Name,
		Prefix:    t.TokenPrefix,
		Scopes:    scopes,
		Active:    t.IsActive(),
		Revoked:   t.RevokedAt != nil,
		CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if t.ExpiresAt != nil {
		s := t.ExpiresAt.Format("2006-01-02T15:04:05Z")
		v.ExpiresAt = &s
	}
	if t.LastUsedAt != nil {
		s := t.LastUsedAt.Format("2006-01-02T15:04:05Z")
		v.LastUsedAt = &s
	}
	return v
}

// handlePOSTAuthToken creates a new API token (standalone-only).
//
// Token-management routes are NOT in routeScopeMap, so ScopeEnforcementMiddleware
// already rejects mbz_ API tokens with 403 before reaching here. The IsAPIToken
// check below is defense-in-depth: it survives any future map regression and
// gives a precise error code/message ("api tokens cannot create new tokens"
// instead of the generic "this route is not accessible to API tokens").
func (g *Gateway) handlePOSTAuthToken(w http.ResponseWriter, r *http.Request) {
	if id := GetAuthIdentity(r.Context()); id != nil && id.IsAPIToken {
		response.Error(w, http.StatusForbidden, response.CodeForbidden,
			"api tokens cannot create new tokens")
		return
	}

	store := g.getTokenStore()
	if store == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"API tokens are not available in this mode")
		return
	}

	count, err := store.CountActive()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "failed to count tokens")
		return
	}
	if count >= int64(apitoken.MaxPerUser) {
		response.Error(w, http.StatusConflict, response.CodeConflict,
			"maximum number of API tokens reached")
		return
	}

	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		req.Name = "api-token"
	}
	if len(req.Scopes) == 0 {
		req.Scopes = defaultTokenScopes()
	}

	// Expand role presets (e.g. "seller:*", "buyer:*") into their canonical
	// scope lists before validation. This mirrors the SaaS gateway behavior so
	// the same client payload works against either deployment.
	req.Scopes = contracts.ExpandScopePresets(req.Scopes)

	if invalid := contracts.ValidateScopes(contracts.ParseScopes(req.Scopes)); invalid != "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"invalid scope: "+string(invalid))
		return
	}

	var expiresAt *time.Time
	if req.ExpiresIn != nil {
		if *req.ExpiresIn < 1 || *req.ExpiresIn > 365 {
			response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
				"expires_in_days must be between 1 and 365")
			return
		}
		t := time.Now().Add(time.Duration(*req.ExpiresIn) * 24 * time.Hour)
		expiresAt = &t
	}

	rawToken, record, err := apitoken.Generate(req.Name, req.Scopes, expiresAt)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "failed to generate token")
		return
	}

	if err := store.Create(record); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "failed to save token")
		return
	}

	response.Created(w, createTokenResponse{
		Token: rawToken,
		Info:  toTokenSummary(record),
	})
}

// handleGETAuthTokens lists all API tokens (standalone-only).
func (g *Gateway) handleGETAuthTokens(w http.ResponseWriter, r *http.Request) {
	store := g.getTokenStore()
	if store == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"API tokens are not available in this mode")
		return
	}

	tokens, err := store.List()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list tokens")
		return
	}

	views := make([]tokenSummary, 0, len(tokens))
	for i := range tokens {
		views = append(views, toTokenSummary(&tokens[i]))
	}

	response.Success(w, views)
}

// handleDELETEAuthToken revokes an API token by ID (standalone-only).
//
// Same defense-in-depth note as handlePOSTAuthToken: this route is not in
// routeScopeMap, so mbz_ API tokens are already blocked by
// ScopeEnforcementMiddleware. The explicit IsAPIToken check guarantees a
// stable, precise error contract regardless of route-map drift.
func (g *Gateway) handleDELETEAuthToken(w http.ResponseWriter, r *http.Request) {
	if id := GetAuthIdentity(r.Context()); id != nil && id.IsAPIToken {
		response.Error(w, http.StatusForbidden, response.CodeForbidden,
			"api tokens cannot revoke tokens")
		return
	}

	store := g.getTokenStore()
	if store == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"API tokens are not available in this mode")
		return
	}

	idStr := chi.URLParam(r, "tokenID")
	tokenID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "invalid token ID")
		return
	}

	if err := store.Revoke(tokenID); err != nil {
		if err == apitoken.ErrTokenNotFound {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "token not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "failed to revoke token")
		return
	}

	response.NoContent(w)
}

func defaultTokenScopes() []string {
	return []string{
		"listings:read", "listings:write",
		"orders:read", "orders:manage",
		"profiles:read", "profiles:write",
		"media:read", "media:write",
		"settings:read",
		"ai:use",
	}
}

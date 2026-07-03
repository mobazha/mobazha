package api

import (
	"net/http"
	"sort"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/response"
)

// authIdentityResponse is the wire shape of GET /v1/auth/identity.
//
// Mirrors mobazha_hosting/api/auth_identity_handler.go's authIdentityResponse so
// the unified frontend (apps/web) can use a single TypeScript model regardless
// of mode.
type authIdentityResponse struct {
	UserID     string   `json:"user_id"`
	PeerID     string   `json:"peer_id,omitempty"`
	IsAPIToken bool     `json:"is_api_token"`
	Scopes     []string `json:"scopes"`
}

// handleGETAuthIdentity returns the resolved identity and scopes for the current
// request. JWT / Basic Auth admins (Scopes == nil) are reported as having all
// known scopes (full access). API tokens get exactly their granted scopes.
//
// GET /v1/auth/identity
//
// Used by the MCP SSE/Streamable identity hook (pkg/mcp/auth.go) to drive
// per-tool scope filtering, and by the AI-agent admin UI to know whether the
// caller is admin or a constrained API token.
func (g *Gateway) handleGETAuthIdentity(w http.ResponseWriter, r *http.Request) {
	identity := GetAuthIdentity(r.Context())
	if identity == nil {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized,
			"authentication required")
		return
	}

	var scopes []string
	if identity.IsAPIToken && identity.Scopes != nil {
		scopes = make([]string, 0, len(identity.Scopes))
		for s := range identity.Scopes {
			scopes = append(scopes, string(s))
		}
		sort.Strings(scopes)
	} else {
		all := contracts.AllScopes()
		scopes = make([]string, len(all))
		for i, s := range all {
			scopes[i] = string(s)
		}
	}

	response.Success(w, authIdentityResponse{
		UserID:     identity.UserID,
		PeerID:     identity.PeerID,
		IsAPIToken: identity.IsAPIToken,
		Scopes:     scopes,
	})
}

// scopeInfo is one entry in the response of GET /v1/auth/scopes.
type scopeInfo struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Action string `json:"action"`
}

// handleGETAuthScopes returns all recognized scopes for the token-creation UI.
//
// GET /v1/auth/scopes
//
// This endpoint must succeed for any authenticated caller (admin or API token).
// It's a pure metadata read, registered under the standalone /v1/auth/scopes
// path so the unified frontend can construct a token-creation form without
// reaching out to the SaaS platform.
func (g *Gateway) handleGETAuthScopes(w http.ResponseWriter, r *http.Request) {
	all := contracts.AllScopes()
	result := make([]scopeInfo, len(all))
	for i, s := range all {
		result[i] = scopeInfo{
			Name:   string(s),
			Domain: s.Domain(),
			Action: s.Action(),
		}
	}
	response.Success(w, result)
}

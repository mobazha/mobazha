//go:build !private_distribution

package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// registerAgentChatStreamRoute mounts POST /v1/agent/chat as a raw chi handler before
// huma registration. Huma's recorder bridge buffers the entire SSE body and
// returns {"stream":true} JSON — the frontend expects text/event-stream chunks.
func (g *Gateway) registerAgentChatStreamRoute(r chi.Router) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return g.AuthenticationMiddleware(g.ScopeEnforcementMiddleware(h))
	}
	r.Method(http.MethodPost, "/v1/agent/chat", wrap(g.handlePOSTAgentChat))
}

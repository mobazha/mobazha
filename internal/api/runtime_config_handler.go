package api

import (
	"net/http"

	"github.com/mobazha/mobazha3.0/internal/embedded/frontend"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

// handleGETRuntimeConfig returns the same versioned snapshot used by
// /runtime-config.js. Hosted and Vite clients use it to refresh backend
// features and capabilities after the application has booted.
func (g *Gateway) handleGETRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	payload := frontend.BuildRuntimeConfigPayload(r.Context(), g.runtimeFrontendConfig())
	responsePkg.Success(w, payload)
}

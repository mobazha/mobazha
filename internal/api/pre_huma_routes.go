//go:build !private_distribution

package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// registerPreHumaRoutes mounts raw chi handlers that MUST be registered
// before huma adds parameterized siblings on the same path prefix.
//
// Chi's radix trie resolves static path segments before param nodes at
// the same tree level, but only when the static node was inserted first.
// The digital-assets upload-stream endpoint (/v1/digital-assets/upload-stream)
// would otherwise be swallowed by huma's /v1/digital-assets/{assetID}
// (GET/PATCH/DELETE), causing a 405 Method Not Allowed for POST.
func (g *Gateway) registerPreHumaRoutes(r chi.Router) {
	if !g.config.PublicOnly {
		g.registerDigitalAssetStreamRoute(r)
		g.registerExportRoutes(r)
	}
}

// registerExportRoutes mounts the seller data-portability endpoints
// (DG-1.10). Registered here rather than via Huma because Huma's response
// pipeline is JSON-centric — the CSV path needs to write text/csv
// directly to ResponseWriter without the {data: ...} envelope. The auth
// chain mirrors digital-asset upload-stream:
//
//	AuthenticationMiddleware → ScopeEnforcementMiddleware → handler
//
// so admin JWT/Basic accounts get full access, and API tokens are gated
// by routeScopeMap (`/v1/exports/*` entries in scope_mapping.go).
func (g *Gateway) registerExportRoutes(r chi.Router) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return g.AuthenticationMiddleware(g.ScopeEnforcementMiddleware(h))
	}
	r.Method(http.MethodGet, "/v1/exports/listings", wrap(g.handleExportListings))
	r.Method(http.MethodGet, "/v1/exports/sales", wrap(g.handleExportSales))
	r.Method(http.MethodGet, "/v1/exports/customers", wrap(g.handleExportCustomers))
}

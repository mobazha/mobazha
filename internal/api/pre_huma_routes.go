package api

import (
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
	if g.config.PublicOnly {
		return
	}
	g.registerDigitalAssetStreamRoute(r)
	g.registerBillingHoldRoutes(r)
	g.registerDistributionPreHumaRoutes(r)
}

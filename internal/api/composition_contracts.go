package api

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
)

// Optional routes are selected explicitly by the product-surface profile.
// Direct calls keep signature drift compile-time visible.
func (g *Gateway) registerDistributionPreHumaRoutes(r chi.Router) {
	if g.restrictedProductSurface() {
		return
	}
	if g.activeAIHTTPPolicy().AllowsAgentWorkspace() {
		g.registerAgentChatStreamRoute(r)
	}
	g.registerExportRoutes(r)
}

func (g *Gateway) registerDistributionHumaSystemAdminOperations(api huma.API) {
	if g.restrictedProductSurface() {
		return
	}
	g.registerFullNodeHumaSystemAdminOperations(api)
}

func (g *Gateway) registerDistributionHumaSystemPublicOperations(api huma.API) {
	if g.restrictedProductSurface() {
		return
	}
	g.registerFullNodeHumaSystemPublicOperations(api)
}

func (g *Gateway) registerDistributionListingImportVendorOperations(api huma.API) {
	if g.restrictedProductSurface() {
		return
	}
	g.registerListingImportVendorCapability(api)
}

func (g *Gateway) registerDistributionListingSupplySummaryOperations(api huma.API) {
	if g.restrictedProductSurface() {
		return
	}
	g.registerListingSupplySummaryCapability(api)
}

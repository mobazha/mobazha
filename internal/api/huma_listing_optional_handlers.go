package api

import "github.com/danielgtaylor/huma/v2"

// Optional listing surfaces are selected by the runtime composition policy.
// Restricted distributions keep the implementations unregistered.
func (g *Gateway) registerListingImportVendorOps(api huma.API) {
	g.registerDistributionListingImportVendorOperations(api)
}

func (g *Gateway) registerListingSupplySummary(api huma.API) {
	g.registerDistributionListingSupplySummaryOperations(api)
}

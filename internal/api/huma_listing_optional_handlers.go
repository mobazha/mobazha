package api

import "github.com/danielgtaylor/huma/v2"

// Optional listing surfaces are selected by the build composition. Their
// implementations stay in full-only files, so PrivateDistribution does not link the
// integrations.
func (g *Gateway) registerListingImportVendorOps(api huma.API) {
	g.registerDistributionListingImportVendorOperations(api)
}

func (g *Gateway) registerListingSupplySummary(api huma.API) {
	g.registerDistributionListingSupplySummaryOperations(api)
}

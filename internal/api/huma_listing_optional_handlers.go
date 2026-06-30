package api

import "github.com/danielgtaylor/huma/v2"

// Optional listing surfaces are discovered from the selected build. Their
// implementations stay in full-only files, so PrivateDistribution neither links the
// integrations nor needs empty method stubs.
func (g *Gateway) registerListingImportVendorOps(api huma.API) {
	if registrar, ok := any(g).(listingImportVendorRegistrar); ok {
		registrar.registerListingImportVendorCapability(api)
	}
}

func (g *Gateway) registerListingSupplySummary(api huma.API) {
	if registrar, ok := any(g).(listingSupplySummaryRegistrar); ok {
		registrar.registerListingSupplySummaryCapability(api)
	}
}

type listingImportVendorRegistrar interface {
	registerListingImportVendorCapability(huma.API)
}

type listingSupplySummaryRegistrar interface {
	registerListingSupplySummaryCapability(huma.API)
}

package api

import "github.com/danielgtaylor/huma/v2"

// System operations always expose the provider-neutral Core surface. The
// selected build may add full-node operations; private commercial operations
// are registered independently through TrustedHumaModule.
func (g *Gateway) registerNodeHumaSystemAdminOperations(api huma.API) {
	g.registerCommonSystemAdminOps(api)
	g.registerDistributionHumaSystemAdminOperations(api)
}

func (g *Gateway) registerNodeHumaSystemPublicOperations(api huma.API) {
	g.registerCommonSystemPublicOps(api)
	g.registerDistributionHumaSystemPublicOperations(api)
}

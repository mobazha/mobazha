package api

import "github.com/danielgtaylor/huma/v2"

// System operations always expose the provider-neutral Core surface. The
// selected build may add full-node operations; private commercial operations
// are registered independently through TrustedHumaModule.
func (g *Gateway) registerNodeHumaSystemAdminOperations(api huma.API) {
	g.registerCommonSystemAdminOps(api)
	if registrar, ok := any(g).(fullNodeHumaSystemRegistrar); ok {
		registrar.registerFullNodeHumaSystemAdminOperations(api)
	}
}

func (g *Gateway) registerNodeHumaSystemPublicOperations(api huma.API) {
	g.registerCommonSystemPublicOps(api)
	if registrar, ok := any(g).(fullNodeHumaSystemRegistrar); ok {
		registrar.registerFullNodeHumaSystemPublicOperations(api)
	}
}

type fullNodeHumaSystemRegistrar interface {
	registerFullNodeHumaSystemAdminOperations(huma.API)
	registerFullNodeHumaSystemPublicOperations(huma.API)
}

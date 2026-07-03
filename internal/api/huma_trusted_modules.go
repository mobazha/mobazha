package api

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/mobazha/mobazha/pkg/distribution"
)

func (g *Gateway) registerTrustedHumaModules(api huma.API) {
	if g == nil || g.config == nil || g.config.PublicOnly {
		return
	}
	registration := distribution.TrustedHumaRegistration{
		API:               api,
		NodeAuthSecurity:  cloneSecurityRequirements(nodeAuthSecurity),
		AdminOnlySecurity: cloneSecurityRequirements(adminOnlyAuthSecurity),
	}
	for _, module := range g.config.TrustedHumaModules {
		if module != nil {
			module.RegisterTrustedHuma(registration)
		}
	}
}

func cloneSecurityRequirements(in []map[string][]string) []map[string][]string {
	out := make([]map[string][]string, 0, len(in))
	for _, requirement := range in {
		copyRequirement := make(map[string][]string, len(requirement))
		for scheme, scopes := range requirement {
			copyScopes := make([]string, len(scopes))
			copy(copyScopes, scopes)
			copyRequirement[scheme] = copyScopes
		}
		out = append(out, copyRequirement)
	}
	return out
}

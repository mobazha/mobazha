package api

import (
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/distribution"
)

func (g *Gateway) registerTrustedHumaModules(api huma.API) error {
	if g == nil || g.config == nil || g.config.PublicOnly {
		return nil
	}
	for _, module := range g.config.TrustedHumaModules {
		if module == nil {
			continue
		}
		descriptor := module.TrustedHumaModuleDescriptor()
		registration, err := distribution.NewTrustedHumaRegistration(distribution.TrustedHumaRegistrationConfig{
			API:                   api,
			Descriptor:            descriptor,
			NodeAuthSecurity:      cloneSecurityRequirements(nodeAuthSecurity),
			AdminOnlySecurity:     cloneSecurityRequirements(adminOnlyAuthSecurity),
			ValidateAPITokenScope: validateTrustedHumaAPITokenScope,
		})
		if err != nil {
			return fmt.Errorf("trusted Huma module %q: %w", descriptor.Owner, err)
		}
		if err := module.RegisterTrustedHuma(registration); err != nil {
			return fmt.Errorf("trusted Huma module %q: register: %w", descriptor.Owner, err)
		}
	}
	return nil
}

func validateTrustedHumaAPITokenScope(method, path string, declared contracts.Scope) error {
	required, ok := routeScopeForOperation(method, path)
	if !ok {
		return fmt.Errorf("API token route %s %s has no scope mapping", method, path)
	}
	if required != declared {
		return fmt.Errorf("API token route %s %s requires scope %q, not %q", method, path, required, declared)
	}
	return nil
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

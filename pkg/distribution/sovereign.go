package distribution

import (
	"fmt"
	"reflect"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

// SovereignNodeConfig is an atomic local-first composition. Product-specific
// wallet administration remains in TrustedHumaModules; Core receives only the
// payment lifecycle and provider-neutral policies it must enforce.
type SovereignNodeConfig struct {
	ExternalPaymentRuntime ExternalPaymentRuntime
	Policy                 SovereignNodePolicy
	TrustedHumaModules     []TrustedHumaModule
	ContentStore           contracts.ContentStore
}

// Clone returns an owned configuration safe from caller slice mutation.
func (config SovereignNodeConfig) Clone() SovereignNodeConfig {
	config.TrustedHumaModules = append([]TrustedHumaModule(nil), config.TrustedHumaModules...)
	return config
}

// Validate rejects partial sovereign compositions before resources are opened.
func (config SovereignNodeConfig) Validate() error {
	if nilCompositionPort(config.ExternalPaymentRuntime) {
		return fmt.Errorf("sovereign external payment runtime is required")
	}
	if nilCompositionPort(config.Policy) {
		return fmt.Errorf("sovereign node policy is required")
	}
	if config.Policy.CoreAPISurface() != CoreAPISurfaceRestricted {
		return fmt.Errorf("sovereign node requires the restricted Core API surface")
	}
	if config.Policy.MCPToolCatalog() != MCPToolCatalogRestricted {
		return fmt.Errorf("sovereign node requires the restricted MCP tool catalog")
	}
	if config.Policy.ExternalExchangeRatesEnabled() {
		return fmt.Errorf("sovereign node cannot enable external exchange rates")
	}
	for index, module := range config.TrustedHumaModules {
		if nilCompositionPort(module) {
			return fmt.Errorf("sovereign Huma module %d is nil", index)
		}
	}
	return nil
}

func nilCompositionPort(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

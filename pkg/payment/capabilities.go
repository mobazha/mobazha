package payment

import iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"

// RuntimeCapability identifies an optional payment or settlement behavior
// selected by a distribution composition root.
type RuntimeCapability string

const (
	// CapabilityManagedEVMEscrowV2 enables the managed EVM escrow V2 rail for
	// a chain. Core deliberately does not encode how a distribution implements
	// or operates the rail.
	CapabilityManagedEVMEscrowV2 RuntimeCapability = "payment.managed-evm-escrow-v2"
)

// ChainCapabilityProvider answers concrete payment capability questions.
// Implementations must be immutable after node construction.
type ChainCapabilityProvider interface {
	Enabled(capability RuntimeCapability, chain iwallet.ChainType) bool
}

type staticChainCapabilityProvider struct {
	enabled map[RuntimeCapability]map[iwallet.ChainType]struct{}
}

// NewStaticChainCapabilityProvider creates an immutable positive allowlist.
// Nil or omitted capabilities fail closed.
func NewStaticChainCapabilityProvider(
	capabilities map[RuntimeCapability][]iwallet.ChainType,
) ChainCapabilityProvider {
	enabled := make(map[RuntimeCapability]map[iwallet.ChainType]struct{}, len(capabilities))
	for capability, chains := range capabilities {
		set := make(map[iwallet.ChainType]struct{}, len(chains))
		for _, chain := range chains {
			set[chain] = struct{}{}
		}
		enabled[capability] = set
	}
	return &staticChainCapabilityProvider{enabled: enabled}
}

func (p *staticChainCapabilityProvider) Enabled(
	capability RuntimeCapability,
	chain iwallet.ChainType,
) bool {
	if p == nil {
		return false
	}
	chains, ok := p.enabled[capability]
	if !ok {
		return false
	}
	_, ok = chains[chain]
	return ok
}

// EnabledChains filters candidates through a provider while preserving order.
func EnabledChains(
	provider ChainCapabilityProvider,
	capability RuntimeCapability,
	candidates []iwallet.ChainType,
) []iwallet.ChainType {
	if provider == nil {
		return nil
	}
	result := make([]iwallet.ChainType, 0, len(candidates))
	for _, chain := range candidates {
		if provider.Enabled(capability, chain) {
			result = append(result, chain)
		}
	}
	return result
}

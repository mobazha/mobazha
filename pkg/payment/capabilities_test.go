package payment

import (
	"testing"

	"github.com/stretchr/testify/assert"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestStaticChainCapabilityProvider_FailsClosedAndCopiesInput(t *testing.T) {
	chains := []iwallet.ChainType{iwallet.ChainBSC}
	provider := NewStaticChainCapabilityProvider(map[RuntimeCapability][]iwallet.ChainType{
		CapabilityManagedEVMEscrowV2: chains,
	})
	chains[0] = iwallet.ChainEthereum

	assert.True(t, provider.Enabled(CapabilityManagedEVMEscrowV2, iwallet.ChainBSC))
	assert.False(t, provider.Enabled(CapabilityManagedEVMEscrowV2, iwallet.ChainEthereum))
	assert.False(t, provider.Enabled("unknown", iwallet.ChainBSC))
	assert.Empty(t, EnabledChains(nil, CapabilityManagedEVMEscrowV2, chains))
}

func TestEnabledChains_PreservesCandidateOrder(t *testing.T) {
	provider := NewStaticChainCapabilityProvider(map[RuntimeCapability][]iwallet.ChainType{
		CapabilityManagedEVMEscrowV2: {iwallet.ChainEthereum, iwallet.ChainBSC},
	})
	candidates := []iwallet.ChainType{iwallet.ChainBSC, iwallet.ChainPolygon, iwallet.ChainEthereum}

	assert.Equal(t,
		[]iwallet.ChainType{iwallet.ChainBSC, iwallet.ChainEthereum},
		EnabledChains(provider, CapabilityManagedEVMEscrowV2, candidates),
	)
}

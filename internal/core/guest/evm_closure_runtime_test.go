package guest

import (
	"testing"

	"github.com/stretchr/testify/require"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestSetEVMManagedEscrowClosureRuntime_ObservationAvailableIdempotent(t *testing.T) {
	svc := &GuestOrderAppService{}
	ethChain := map[iwallet.ChainType]struct{}{iwallet.ChainEthereum: {}}

	svc.SetEVMManagedEscrowClosureRuntime(EVMManagedEscrowClosureRuntime{
		ObservationReady:  true,
		ManagedEscrowMonitorChains: ethChain,
	})
	require.True(t, svc.evmObservationAvailable)

	svc.SetEVMManagedEscrowClosureRuntime(EVMManagedEscrowClosureRuntime{
		ObservationReady:  false,
		ManagedEscrowMonitorChains: ethChain,
	})
	require.False(t, svc.evmObservationAvailable, "must clear observation when ObservationReady is false")

	svc.SetEVMManagedEscrowClosureRuntime(EVMManagedEscrowClosureRuntime{
		ObservationReady:  true,
		ManagedEscrowMonitorChains: nil,
	})
	require.False(t, svc.evmObservationAvailable, "must clear observation when no ManagedEscrow monitors are wired")
}

package guest

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
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

func TestEvaluateEVMClosureReadiness_RelayGasUnhealthyIncludesReason(t *testing.T) {
	svc := &GuestOrderAppService{
		directPayment: &DirectPaymentService{
			evmManagedEscrowFunding: &EVMManagedEscrowFundingAdapter{},
		},
		evmManagedEscrowSettlement: &EVMManagedEscrowSettlementService{},
	}
	ethChain := map[iwallet.ChainType]struct{}{iwallet.ChainEthereum: {}}
	const unhealthyReason = "gas wallet balance below low-watermark (0.01 ETH)"
	svc.SetEVMManagedEscrowClosureRuntime(EVMManagedEscrowClosureRuntime{
		FundingReady:          true,
		ObservationReady:      true,
		SettlementReady:       true,
		RelayReady:            true,
		ManagedEscrowMonitorChains:     ethChain,
		RelayGasHealthyChains: map[iwallet.ChainType]struct{}{},
		RelayGasUnhealthyReason: map[iwallet.ChainType]string{
			iwallet.ChainEthereum: unhealthyReason,
		},
	})

	coinType, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainEthereum)
	require.True(t, ok)
	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	require.NoError(t, err)

	err = svc.evaluateEVMClosureReadiness(coinType, coinInfo)
	require.Error(t, err)
	require.True(t, errors.Is(err, contracts.ErrCoinUnavailable))
	require.Contains(t, err.Error(), string(coinInfo.Chain))
	require.Contains(t, err.Error(), unhealthyReason)
	require.NotContains(t, strings.ToLower(err.Error()), "relay is not configured")
}

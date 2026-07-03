package guest

import (
	"fmt"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/distribution"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ManagedEscrowClosureRuntime carries Phase 3D runtime gates for buyer-visible EVM guest checkout.
type ManagedEscrowClosureRuntime struct {
	FundingReady               bool
	ObservationReady           bool
	SettlementReady            bool
	RelayReady                 bool
	ManagedEscrowMonitorChains map[iwallet.ChainType]struct{}
	RelayGasHealthyChains      map[iwallet.ChainType]struct{}
	RelayGasUnhealthyReason    map[iwallet.ChainType]string
	HealthProvider             distribution.ManagedEscrowHealthProvider
}

// SetManagedEscrowClosureRuntime updates managed EVM closure wiring (called after managed module registration).
func (s *GuestOrderAppService) SetManagedEscrowClosureRuntime(cfg ManagedEscrowClosureRuntime) {
	if s == nil {
		return
	}
	s.evmRuntimeMu.Lock()
	defer s.evmRuntimeMu.Unlock()
	s.managedEscrowFundingReady = cfg.FundingReady
	s.managedEscrowObservationReady = cfg.ObservationReady
	s.managedEscrowSettlementReady = cfg.SettlementReady
	s.managedEscrowRelayReady = cfg.RelayReady
	s.evmManagedEscrowMonitorChains = cloneChainSet(cfg.ManagedEscrowMonitorChains)
	s.evmRelayGasHealthyChains = cloneChainSet(cfg.RelayGasHealthyChains)
	s.evmRelayGasUnhealthyReason = cloneChainReasons(cfg.RelayGasUnhealthyReason)
	s.evmHealthProvider = cfg.HealthProvider
	s.evmObservationAvailable = cfg.ObservationReady && len(s.evmManagedEscrowMonitorChains) > 0
}

func cloneChainReasons(in map[iwallet.ChainType]string) map[iwallet.ChainType]string {
	if in == nil {
		return nil
	}
	out := make(map[iwallet.ChainType]string, len(in))
	for chain, reason := range in {
		out[chain] = reason
	}
	return out
}

func cloneChainSet(in map[iwallet.ChainType]struct{}) map[iwallet.ChainType]struct{} {
	if in == nil {
		return nil
	}
	out := make(map[iwallet.ChainType]struct{}, len(in))
	for chain := range in {
		out[chain] = struct{}{}
	}
	return out
}

func (s *GuestOrderAppService) HasManagedEscrowSettlement() bool {
	return s != nil && s.managedEscrowSettlement != nil && managedEscrowGuestSettlementActive
}

func (s *GuestOrderAppService) evaluateEVMClosureReadiness(coinType iwallet.CoinType, coinInfo iwallet.CoinInfo) error {
	if chosenEVMSettlementStrategy != EVMSettlementManagedSession {
		return fmt.Errorf("%w: EVM guest checkout strategy is not managed escrow/PaymentSession", contracts.ErrCoinUnavailable)
	}
	s.evmRuntimeMu.RLock()
	fundingReady := s.managedEscrowFundingReady && s.directPayment != nil && s.directPayment.HasManagedEscrowFunding()
	obsReady := s.managedEscrowObservationReady
	settleReady := s.managedEscrowSettlementReady && s.managedEscrowSettlement != nil && managedEscrowGuestSettlementActive
	relayReady := s.managedEscrowRelayReady
	_, monitorOK := s.evmManagedEscrowMonitorChains[coinInfo.Chain]
	_, relayGasOK := s.evmRelayGasHealthyChains[coinInfo.Chain]
	relayGasReason := s.evmRelayGasUnhealthyReason[coinInfo.Chain]
	healthProvider := s.evmHealthProvider
	s.evmRuntimeMu.RUnlock()
	if healthProvider != nil {
		health := healthProvider.ManagedEscrowHealth(coinInfo.Chain)
		relayReady = health.RelayReady
		relayGasOK = health.RelayGasHealthy
		relayGasReason = health.Reason
	}

	if !fundingReady {
		return fmt.Errorf("%w: managed EVM funding adapter is not configured", contracts.ErrCoinUnavailable)
	}
	if !obsReady || !monitorOK {
		return fmt.Errorf("%w: managed EVM observation is not configured for %s", contracts.ErrCoinUnavailable, coinInfo.Chain)
	}
	if !settleReady {
		return fmt.Errorf("%w: managed EVM settlement is not configured", contracts.ErrCoinUnavailable)
	}
	if !relayReady {
		return fmt.Errorf("%w: EVM managed relay is not configured", contracts.ErrCoinUnavailable)
	}
	if !relayGasOK {
		if relayGasReason == "" {
			relayGasReason = "relay gas wallet not healthy"
		}
		return fmt.Errorf("%w: EVM managed relay gas wallet is not healthy for %s: %s",
			contracts.ErrCoinUnavailable, coinInfo.Chain, relayGasReason)
	}
	if !s.hasActiveReceivingAccount(coinInfo.Chain) {
		return fmt.Errorf("%w: no active seller receiving account for %s", contracts.ErrCoinUnavailable, coinInfo.Chain)
	}
	return nil
}

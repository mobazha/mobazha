package guest

import (
	"sort"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/managedescrow"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func (s *GuestOrderAppService) evmReadinessChainList() []iwallet.ChainType {
	if s == nil {
		return nil
	}
	s.evmRuntimeMu.RLock()
	defer s.evmRuntimeMu.RUnlock()
	out := make([]iwallet.ChainType, 0, len(s.evmManagedEscrowMonitorChains))
	for chain := range s.evmManagedEscrowMonitorChains {
		out = append(out, chain)
	}
	sort.Slice(out, func(i, j int) bool {
		return string(out[i]) < string(out[j])
	})
	return out
}

func (s *GuestOrderAppService) appendEVMReadiness(out *contracts.GuestCheckoutReadiness) {
	if s == nil || out == nil {
		return
	}
	chains := s.evmReadinessChainList()
	if len(chains) == 0 {
		// Report canonical ManagedEscrow-enabled EVM chains even when monitors are not wired yet.
		// Non-nil empty config = all Ready chains (standalone default); nil would disable ManagedEscrow.
		for _, chain := range managed_escrow.ManagedEscrowEnabledChains(&managed_escrow.ChainCapabilityConfig{}) {
			chains = append(chains, chain)
		}
	}
	s.evmRuntimeMu.RLock()
	fundingReady := s.evmManagedEscrowFundingReady
	obsReady := s.evmManagedEscrowObservationReady
	settleReady := s.evmManagedEscrowSettlementReady
	relayReady := s.evmManagedEscrowRelayReady
	monitors := s.evmManagedEscrowMonitorChains
	relayGasHealthy := s.evmRelayGasHealthyChains
	relayGasReasons := s.evmRelayGasUnhealthyReason
	healthProvider := s.evmHealthProvider
	s.evmRuntimeMu.RUnlock()

	seen := make(map[iwallet.ChainType]struct{}, len(chains))
	for _, chain := range chains {
		if _, ok := seen[chain]; ok {
			continue
		}
		seen[chain] = struct{}{}
		coinType, ok := iwallet.CanonicalNativeCoinType(chain)
		if !ok {
			continue
		}
		coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
		if err != nil {
			continue
		}
		_, monitorOK := monitors[chain]
		_, relayGasOK := relayGasHealthy[chain]
		relayGasReason := relayGasReasons[chain]
		chainRelayReady := relayReady
		if healthProvider != nil {
			health := healthProvider.ManagedEscrowHealth(chain)
			chainRelayReady = health.RelayReady
			relayGasOK = health.RelayGasHealthy
			relayGasReason = health.Reason
		}
		if chainRelayReady && !relayGasOK && relayGasReason == "" {
			relayGasReason = "relay gas wallet not healthy"
		}
		entry := contracts.GuestEVMChainReadiness{
			Chain:                  string(chain),
			Coin:                   string(coinType),
			ManagedEscrowMonitorActive:      monitorOK,
			RelayReady:             chainRelayReady,
			RelayGasHealthy:        chainRelayReady && relayGasOK,
			RelayGasReason:         relayGasReason,
			SettlementReady:        settleReady && guestEVMManagedEscrowSettlementActive,
			FundingReady:           fundingReady && s.directPayment != nil && s.directPayment.HasEVMManagedEscrowFunding(),
			ObservationReady:       obsReady && monitorOK,
			ReceivingAccountActive: s.hasActiveReceivingAccount(chain),
		}
		cap := s.evaluateGuestPaymentCapability(coinType, coinInfo)
		entry.BuyerVisible = cap.BuyerVisible
		entry.Reason = cap.Reason
		out.EVMChains = append(out.EVMChains, entry)
	}
}

//go:build !private_distribution

package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/internal/core/guest"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type distributionManagedEscrowGuestRuntimeBinder struct {
	node   *MobazhaNode
	source distribution.ManagedEscrowGuestSettlementSource
}

func (b distributionManagedEscrowGuestRuntimeBinder) BindManagedEscrowGuestRuntime(
	ctx context.Context,
	runtime distribution.ManagedEscrowGuestRuntime,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.node == nil || b.node.guestOrderService == nil || b.node.guestPaymentMonitor == nil {
		return fmt.Errorf("managed escrow guest runtime: guest checkout services are unavailable")
	}
	if b.source == nil || runtime.WatchRegistrar == nil || runtime.SettlementExecutor == nil {
		return fmt.Errorf("managed escrow guest runtime: source, watch registrar, and settlement executor are required")
	}
	chainSet := make(map[iwallet.ChainType]struct{}, len(runtime.MonitorChains))
	for _, chain := range runtime.MonitorChains {
		if strings.TrimSpace(string(chain)) == "" {
			return fmt.Errorf("managed escrow guest runtime: empty monitor chain")
		}
		chainSet[chain] = struct{}{}
	}
	if len(chainSet) == 0 {
		return fmt.Errorf("managed escrow guest runtime: at least one monitor chain is required")
	}
	for chain := range runtime.RelayGasHealthyChains {
		if _, ok := chainSet[chain]; !ok {
			return fmt.Errorf("managed escrow guest runtime: gas health reported for unmonitored chain %s", chain)
		}
	}
	confirmed, err := b.source.ListConfirmedManagedEscrowGuestSettlements(ctx)
	if err != nil {
		return fmt.Errorf("managed escrow guest runtime: replay confirmed settlements: %w", err)
	}

	watcher := guest.NewDistributionManagedEscrowWatcher(runtime.WatchRegistrar, b.node.walletTestnet)
	settlement := guest.NewDistributionManagedEscrowGuestSettlementService(b.source, runtime.SettlementExecutor)
	b.node.guestPaymentMonitor.SetEVMManagedEscrowWatch(watcher)
	b.node.guestOrderService.SetEVMManagedEscrowSettlement(settlement)
	b.node.guestOrderService.SetEVMManagedEscrowClosureRuntime(guest.EVMManagedEscrowClosureRuntime{
		FundingReady:            b.node.directPaymentService != nil && b.node.directPaymentService.HasEVMManagedEscrowFunding(),
		ObservationReady:        true,
		SettlementReady:         true,
		RelayReady:              runtime.RelayReady,
		ManagedEscrowMonitorChains:       chainSet,
		RelayGasHealthyChains:   runtime.RelayGasHealthyChains,
		RelayGasUnhealthyReason: runtime.RelayGasUnhealthyReason,
	})

	for _, orderID := range confirmed {
		b.node.guestOrderService.OnEVMManagedEscrowSettlementConfirmed(orderID)
	}
	go settlement.RecoverPendingSettlements(ctx)
	return nil
}

var _ distribution.ManagedEscrowGuestRuntimeBinder = distributionManagedEscrowGuestRuntimeBinder{}

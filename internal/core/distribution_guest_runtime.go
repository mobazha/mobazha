//go:build !private_distribution

package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/mobazha/mobazha3.0/internal/core/guest"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type distributionManagedEscrowGuestRuntimeBinder struct {
	mu         sync.Mutex
	node       *MobazhaNode
	source     distribution.ManagedEscrowGuestSettlementSource
	bound      bool
	started    bool
	settlement *guest.DistributionManagedEscrowGuestSettlementService
}

func (b *distributionManagedEscrowGuestRuntimeBinder) BindManagedEscrowGuestRuntime(
	ctx context.Context,
	runtime distribution.ManagedEscrowGuestRuntime,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.node == nil || b.node.guestOrderService == nil || b.node.guestPaymentMonitor == nil {
		return fmt.Errorf("managed escrow guest runtime: guest checkout services are unavailable")
	}
	if b.source == nil || runtime.WatchRegistrar == nil || runtime.SettlementExecutor == nil || runtime.HealthProvider == nil {
		return fmt.Errorf("managed escrow guest runtime: source, watch registrar, settlement executor, and health provider are required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.bound {
		return fmt.Errorf("managed escrow guest runtime: already bound")
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
	watcher := guest.NewDistributionManagedEscrowWatcher(runtime.WatchRegistrar, b.node.walletTestnet)
	settlement := guest.NewDistributionManagedEscrowGuestSettlementService(b.source, runtime.SettlementExecutor)
	b.node.guestPaymentMonitor.SetEVMManagedEscrowWatch(watcher)
	b.node.guestOrderService.SetEVMManagedEscrowSettlement(settlement)
	b.node.guestOrderService.SetEVMManagedEscrowClosureRuntime(guest.EVMManagedEscrowClosureRuntime{
		FundingReady:      b.node.directPaymentService != nil && b.node.directPaymentService.HasEVMManagedEscrowFunding(),
		ObservationReady:  true,
		SettlementReady:   true,
		RelayReady:        true,
		ManagedEscrowMonitorChains: chainSet,
		HealthProvider:    runtime.HealthProvider,
	})
	b.settlement = settlement
	b.bound = true
	return nil
}

func (b *distributionManagedEscrowGuestRuntimeBinder) StartManagedEscrowGuestRuntime(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	if !b.bound || b.settlement == nil {
		b.mu.Unlock()
		return fmt.Errorf("managed escrow guest runtime: must be bound before start")
	}
	if b.started {
		b.mu.Unlock()
		return nil
	}
	b.started = true
	settlement := b.settlement
	source := b.source
	node := b.node
	b.mu.Unlock()
	confirmed, err := source.ListConfirmedManagedEscrowGuestSettlements(ctx)
	if err != nil {
		b.mu.Lock()
		b.started = false
		b.mu.Unlock()
		return fmt.Errorf("managed escrow guest runtime: replay confirmed settlements: %w", err)
	}
	for _, orderID := range confirmed {
		node.guestOrderService.OnEVMManagedEscrowSettlementConfirmed(orderID)
	}
	go settlement.RunPendingSettlementRecovery(ctx)
	return nil
}

func (b *distributionManagedEscrowGuestRuntimeBinder) UnbindManagedEscrowGuestRuntime(ctx context.Context) error {
	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.bound {
		return nil
	}
	b.node.guestPaymentMonitor.SetEVMManagedEscrowWatch(nil)
	b.node.guestOrderService.SetEVMManagedEscrowSettlement(nil)
	b.node.guestOrderService.SetEVMManagedEscrowClosureRuntime(guest.EVMManagedEscrowClosureRuntime{})
	b.settlement = nil
	b.bound = false
	b.started = false
	return nil
}

var _ distribution.ManagedEscrowGuestRuntimeBinder = (*distributionManagedEscrowGuestRuntimeBinder)(nil)

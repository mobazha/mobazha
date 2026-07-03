package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/mobazha/mobazha/internal/core/guest"
	"github.com/mobazha/mobazha/pkg/distribution"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type distributionManagedEscrowGuestRuntimeBinder struct {
	mu          sync.Mutex
	node        *MobazhaNode
	source      *guest.ManagedEscrowGuestSettlementSource
	watchSource *distributionManagedEscrowWatchSource
	bound       bool
	started     bool
	settlement  *guest.DistributionManagedEscrowGuestSettlementService
}

func (b *distributionManagedEscrowGuestRuntimeBinder) BindManagedEscrowGuestRuntime(
	ctx context.Context,
	runtime distribution.ManagedEscrowGuestRuntime,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.node == nil || b.node.guestOrderService == nil || b.node.guestPaymentMonitor == nil || b.node.directPaymentService == nil {
		return fmt.Errorf("managed escrow guest runtime: guest checkout services are unavailable")
	}
	if b.source == nil || b.watchSource == nil || runtime.Projector == nil || runtime.WatchRegistrar == nil || runtime.SettlementExecutor == nil || runtime.ReceiptValidator == nil || runtime.HealthProvider == nil {
		return fmt.Errorf("managed escrow guest runtime: projector, sources, watch registrar, settlement executor, receipt validator, and health provider are required")
	}
	if b.node.keyProvider == nil {
		return fmt.Errorf("managed escrow guest runtime: key provider is required")
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
	watcher := guest.NewDistributionManagedEscrowWatcher(runtime.WatchRegistrar, runtime.Projector)
	settlement := guest.NewDistributionManagedEscrowGuestSettlementService(b.source, runtime.SettlementExecutor)
	b.source.SetProjector(runtime.Projector)
	b.watchSource.setProjector(runtime.Projector)
	b.node.directPaymentService.SetManagedEscrowFunding(runtime.Projector, &guest.NodeEVMSellerOwnerResolver{Keys: b.node.keyProvider})
	b.node.managedEscrowReceiptValidator = runtime.ReceiptValidator
	b.node.guestPaymentMonitor.SetEVMManagedEscrowWatch(watcher)
	b.node.guestOrderService.SetManagedEscrowSettlement(settlement)
	b.node.guestOrderService.SetManagedEscrowClosureRuntime(guest.ManagedEscrowClosureRuntime{
		FundingReady:               b.node.directPaymentService.HasManagedEscrowFunding(),
		ObservationReady:           true,
		SettlementReady:            true,
		RelayReady:                 true,
		ManagedEscrowMonitorChains: chainSet,
		HealthProvider:             runtime.HealthProvider,
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
		node.guestOrderService.OnManagedEscrowSettlementConfirmed(orderID)
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
	b.node.guestOrderService.SetManagedEscrowSettlement(nil)
	b.node.guestOrderService.SetManagedEscrowClosureRuntime(guest.ManagedEscrowClosureRuntime{})
	b.node.directPaymentService.SetManagedEscrowFunding(nil, nil)
	b.node.managedEscrowReceiptValidator = nil
	b.source.SetProjector(nil)
	b.watchSource.setProjector(nil)
	b.settlement = nil
	b.bound = false
	b.started = false
	return nil
}

var _ distribution.ManagedEscrowGuestRuntimeBinder = (*distributionManagedEscrowGuestRuntimeBinder)(nil)

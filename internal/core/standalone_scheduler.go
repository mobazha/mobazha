//go:build !private_distribution

package core

import (
	"context"
)

// startStandaloneScheduler creates and starts a lightweight process-local
// scheduler for standalone (non-SaaS) nodes. It registers the same periodic
// jobs that the hosting shared scheduler drives in SaaS mode, but uses
// GlobalFn (no NodeRegistry, no lease-based locking) since there is exactly
// one node in the process.
//
// The scheduler respects ctx cancellation and is stopped automatically when
// the node shuts down.
func (n *MobazhaNode) startStandaloneScheduler(ctx context.Context) {
	// hookFns maps job name → GlobalFn that delegates to the corresponding
	// SchedulerHooks method. Metadata (interval, overlap) comes from the
	// shared pkg/scheduler.Jobs registry — single source of truth.
	hookFns := map[string]func(ctx context.Context) error{
		"order-timeout":                   func(ctx context.Context) error { n.RunOrderTimeoutOnce(ctx); return nil },
		"outbox-poll":                     func(ctx context.Context) error { n.RunOutboxPollOnce(ctx); return nil },
		"outbox-cleanup":                  func(ctx context.Context) error { n.RunOutboxCleanupOnce(ctx); return nil },
		"payment-reconcile-scan":          func(ctx context.Context) error { n.RunPaymentReconcileScanOnce(ctx); return nil },
		"payment-verification":            func(ctx context.Context) error { n.RunPaymentVerificationOnce(ctx); return nil },
		"settlement-action-confirmations": func(ctx context.Context) error { n.RunSettlementActionConfirmationsOnce(ctx); return nil },
		"webhook-delivery":                func(ctx context.Context) error { n.RunWebhookDeliveryOnce(ctx); return nil },
		"webhook-cleanup":                 func(ctx context.Context) error { n.RunWebhookCleanupOnce(ctx); return nil },
		"analytics-cleanup":               func(ctx context.Context) error { n.RunAnalyticsCleanupOnce(ctx); return nil },
		"fiat-reconciliation":             func(ctx context.Context) error { n.RunFiatReconciliationOnce(ctx); return nil },
		"fiat-cleanup":                    func(ctx context.Context) error { n.RunFiatCleanupOnce(ctx); return nil },
		"guest-order-cleanup":             func(ctx context.Context) error { n.RunGuestOrderCleanupOnce(ctx); return nil },
		"follower-connect":                func(ctx context.Context) error { n.RunFollowerConnectOnce(ctx); return nil },
		"netdb-reconcile":                 func(ctx context.Context) error { n.RunNetDBReconcileOnce(ctx); return nil },
		"order-lock-cleanup":              func(ctx context.Context) error { n.RunOrderLockCleanupOnce(ctx); return nil },

		"supply-chain-retry":           func(ctx context.Context) error { n.RunSupplyChainRetryOnce(ctx); return nil },
		"supply-chain-reconcile":       func(ctx context.Context) error { n.RunSupplyChainReconcileOnce(ctx); return nil },
		"supply-chain-cleanup":         func(ctx context.Context) error { n.RunSupplyChainCleanupOnce(ctx); return nil },
		"supply-chain-inventory-check": func(ctx context.Context) error { n.RunSupplyChainInventoryCheckOnce(ctx); return nil },
		"supply-chain-price-drift":     func(ctx context.Context) error { n.RunSupplyChainPriceDriftOnce(ctx); return nil },
	}

	n.startStandaloneSchedulerPlan(ctx, nil, hookFns)
}

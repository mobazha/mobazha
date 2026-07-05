// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"fmt"
	"time"

	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/distribution"
)

// distributionDirectObservedRuntimeBinder exposes only the Core wiring needed
// by a trusted direct-observed payment module. The provider owns lifecycle,
// chain clients, credentials, and observation implementation.
type distributionDirectObservedRuntimeBinder struct {
	node *MobazhaNode
}

func (b *distributionDirectObservedRuntimeBinder) BindExternalPaymentRuntime(
	registration distribution.ExternalPaymentRuntimeRegistration,
) error {
	if b == nil || b.node == nil {
		return fmt.Errorf("direct observed runtime binder is unavailable")
	}
	if registration.Runtime == nil {
		return fmt.Errorf("direct observed runtime is required")
	}
	b.node.externalPaymentMu.Lock()
	if b.node.externalPayments == nil {
		b.node.externalPayments = distribution.NewExternalPaymentRuntimeCatalog()
	}
	if err := b.node.externalPayments.Register(registration); err != nil {
		b.node.externalPaymentMu.Unlock()
		return err
	}
	if b.node.directPaymentService != nil {
		b.node.directPaymentService.SetExternalPaymentRuntimeCatalog(b.node.externalPayments)
	}
	if b.node.guestPaymentMonitor != nil {
		b.node.guestPaymentMonitor.SetExternalPaymentRuntimeCatalog(b.node.externalPayments)
	}
	if registration.ActiveForNewWork && b.node.guestOrderService != nil {
		b.node.guestOrderService.SetDirectObservedGraceProvider(registration.Runtime)
	}
	directPayment := b.node.directPaymentService
	b.node.externalPaymentMu.Unlock()

	// Module binding is the first point at which the exact historical runtime
	// is guaranteed to exist after restart. Kick recovery immediately without
	// delaying module readiness; the scheduled reconcile scan remains the
	// durable retry loop.
	if directPayment != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			if _, err := directPayment.RecoverPendingExternalPaymentAddresses(ctx); err != nil && ctx.Err() == nil {
				logger.LogWarningWithIDf(log, b.node.nodeID, "direct observed address recovery: %v", err)
			}
		}()
	}
	return nil
}

func (b *distributionDirectObservedRuntimeBinder) UnbindExternalPaymentRuntime(
	registration distribution.ExternalPaymentRuntimeRegistration,
) error {
	if b == nil || b.node == nil {
		return nil
	}
	b.node.externalPaymentMu.Lock()
	defer b.node.externalPaymentMu.Unlock()
	if b.node.externalPayments != nil {
		b.node.externalPayments.Unregister(registration.Route)
	}
	// Keep the last grace policy bound to the order service. Existing order and
	// inventory deadlines are durable business policy, not live-runtime state.
	return nil
}

var _ distribution.DirectObservedRuntimeBinder = (*distributionDirectObservedRuntimeBinder)(nil)

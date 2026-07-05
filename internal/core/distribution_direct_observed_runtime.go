// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"fmt"

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
	defer b.node.externalPaymentMu.Unlock()
	if b.node.externalPayments == nil {
		b.node.externalPayments = distribution.NewExternalPaymentRuntimeCatalog()
	}
	if err := b.node.externalPayments.Register(registration); err != nil {
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

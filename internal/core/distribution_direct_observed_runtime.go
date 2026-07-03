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
	runtime distribution.ExternalPaymentRuntime,
) error {
	if b == nil || b.node == nil {
		return fmt.Errorf("direct observed runtime binder is unavailable")
	}
	if runtime == nil {
		return fmt.Errorf("direct observed runtime is required")
	}
	if b.node.externalPayment != nil {
		return fmt.Errorf("a direct observed runtime is already bound")
	}
	b.node.externalPayment = runtime
	if b.node.directPaymentService != nil {
		b.node.directPaymentService.SetExternalPaymentRuntime(runtime)
	}
	if b.node.guestPaymentMonitor != nil {
		b.node.guestPaymentMonitor.SetExternalPaymentRuntime(runtime)
	}
	if b.node.guestOrderService != nil {
		b.node.guestOrderService.SetDirectObservedGraceProvider(runtime)
	}
	return nil
}

func (b *distributionDirectObservedRuntimeBinder) UnbindExternalPaymentRuntime(
	_ distribution.ExternalPaymentRuntime,
) error {
	if b == nil || b.node == nil {
		return nil
	}
	if b.node.externalPayment == nil {
		return nil
	}
	b.node.externalPayment = nil
	if b.node.directPaymentService != nil {
		b.node.directPaymentService.SetExternalPaymentRuntime(nil)
	}
	if b.node.guestPaymentMonitor != nil {
		b.node.guestPaymentMonitor.SetExternalPaymentRuntime(nil)
	}
	if b.node.guestOrderService != nil {
		b.node.guestOrderService.SetDirectObservedGraceProvider(nil)
	}
	return nil
}

var _ distribution.DirectObservedRuntimeBinder = (*distributionDirectObservedRuntimeBinder)(nil)

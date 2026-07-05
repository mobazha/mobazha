// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/distribution"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const coreFiatPaymentModuleID = "mobazha.core.fiat"

var coreFiatProviderIDs = []string{"paypal", "stripe"}

// coreFiatPaymentModule contributes Core's statically linked provider-session
// drivers to the same trusted manager used by Safe, Solana, and direct-observed
// modules. Provider credentials and tenant bindings remain runtime data.
type coreFiatPaymentModule struct{}

func newCoreFiatPaymentModule() distribution.PaymentModule { return &coreFiatPaymentModule{} }

func (*coreFiatPaymentModule) Descriptor() distribution.PaymentModuleDescriptor {
	chains := make([]iwallet.ChainType, 0, len(coreFiatProviderIDs))
	for _, providerID := range coreFiatProviderIDs {
		chains = append(chains, FiatChainType(providerID))
	}
	return distribution.PaymentModuleDescriptor{
		ID: coreFiatPaymentModuleID, Version: "v1",
		Rails:              []distribution.PaymentRailKind{distribution.PaymentRailProviderSession},
		Chains:             chains,
		Assets:             []iwallet.CoinType{distribution.PaymentAssetAny},
		Activation:         distribution.PaymentModuleOptional,
		ProtocolVersion:    "provider-v1",
		StateSchemaVersion: "1",
	}
}

func (m *coreFiatPaymentModule) Register(_ context.Context, _ distribution.PaymentRuntime, registrar distribution.PaymentRegistrar) error {
	if registrar == nil {
		return fmt.Errorf("Core fiat payment module: registrar is required")
	}
	for _, providerID := range coreFiatProviderIDs {
		if err := registrar.RegisterRail(coreFiatPaymentContribution(providerID)); err != nil {
			return fmt.Errorf("Core fiat payment module: register %s: %w", providerID, err)
		}
	}
	return nil
}

func (*coreFiatPaymentModule) RollbackRegistration(context.Context) error { return nil }

func coreFiatPaymentContribution(providerID string) distribution.PaymentRailContribution {
	providerID = strings.ToLower(strings.TrimSpace(providerID))
	return distribution.PaymentRailContribution{
		ContributionID: coreFiatPaymentContributionID(providerID),
		Rail:           distribution.PaymentRailProviderSession,
		Network:        FiatChainType(providerID),
		Asset:          distribution.PaymentAssetAny,
		Operations: []distribution.PaymentRailOperation{
			distribution.PaymentOperationSetup,
			distribution.PaymentOperationObserve,
			distribution.PaymentOperationVerify,
			distribution.PaymentOperationConfirm,
			distribution.PaymentOperationCancel,
			distribution.PaymentOperationRefund,
			distribution.PaymentOperationReconcile,
		},
	}
}

func coreFiatPaymentContributionID(providerID string) string {
	return coreFiatPaymentModuleID + "." + strings.ToLower(strings.TrimSpace(providerID))
}

var _ distribution.PaymentModule = (*coreFiatPaymentModule)(nil)

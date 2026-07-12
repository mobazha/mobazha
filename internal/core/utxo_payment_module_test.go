// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha/pkg/distribution"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoreNativeUTXOPaymentModule_RegistersStableNativeRoutes(t *testing.T) {
	module := newCoreNativeUTXOPaymentModule()
	descriptor := module.Descriptor()
	require.NoError(t, distribution.ValidatePaymentModuleDescriptor(descriptor))
	assert.Equal(t, coreNativeUTXOPaymentModuleID, descriptor.ID)

	registrar := new(coreFiatModuleRegistrar)
	require.NoError(t, module.Register(context.Background(), distribution.PaymentRuntime{}, registrar))
	require.Len(t, registrar.contributions, len(coreNativeUTXOChains))
	for _, contribution := range registrar.contributions {
		require.NoError(t, distribution.ValidatePaymentRailContribution(descriptor, contribution))
		assert.Equal(t, distribution.PaymentRailDirectObserved, contribution.Rail)
		assert.True(t, contribution.Network.IsUTXOChain())
	}
}

func TestRegisterSovereignPaymentModules_ComposesNativeBitcoinRoute(t *testing.T) {
	node := &MobazhaNode{
		identityFields: identityFields{nodeID: "standalone-test", nodeCtx: context.Background()},
	}
	require.NoError(t, node.registerSovereignPaymentModules())
	require.NoError(t, node.runDistributionPaymentModules(context.Background()))
	t.Cleanup(func() {
		require.NoError(t, node.paymentModuleManager.Stop(context.Background()))
	})

	bitcoin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.NoError(t, err)
	route, err := node.ResolveNewPaymentRouteIdentity(context.Background(), distribution.PaymentCapabilityRequest{
		Rail:      distribution.PaymentRailDirectObserved,
		Network:   iwallet.ChainBitcoin,
		Asset:     bitcoin,
		Operation: distribution.PaymentOperationSetup,
	})
	require.NoError(t, err)
	assert.Equal(t, coreNativeUTXOPaymentModuleID, route.ModuleID)
	assert.Equal(t, string(bitcoin), route.AssetID)
}

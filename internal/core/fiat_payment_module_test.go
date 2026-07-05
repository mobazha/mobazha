// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type coreFiatModuleRegistrar struct {
	contributions []distribution.PaymentRailContribution
}

func (*coreFiatModuleRegistrar) RegisterEscrowV2(distribution.PaymentRailContribution, payment.ChainEscrowV2) error {
	return nil
}

func (r *coreFiatModuleRegistrar) RegisterRail(contribution distribution.PaymentRailContribution) error {
	r.contributions = append(r.contributions, contribution)
	return nil
}

func TestCoreFiatPaymentModule_ProviderSessionContract(t *testing.T) {
	module := newCoreFiatPaymentModule()
	descriptor := module.Descriptor()
	require.NoError(t, distribution.ValidatePaymentModuleDescriptor(descriptor))
	assert.Equal(t, coreFiatPaymentModuleID, descriptor.ID)
	assert.Equal(t, []distribution.PaymentRailKind{distribution.PaymentRailProviderSession}, descriptor.Rails)

	registrar := &coreFiatModuleRegistrar{}
	require.NoError(t, module.Register(context.Background(), distribution.PaymentRuntime{}, registrar))
	require.Len(t, registrar.contributions, len(coreFiatProviderIDs))
	for _, contribution := range registrar.contributions {
		require.NoError(t, distribution.ValidatePaymentRailContribution(descriptor, contribution))
		assert.Equal(t, distribution.PaymentRailProviderSession, contribution.Rail)
		assert.Equal(t, distribution.PaymentAssetAny, contribution.Asset)
	}
	assert.Equal(t, coreFiatPaymentContributionID("paypal"), registrar.contributions[0].ContributionID)
	assert.Equal(t, coreFiatPaymentContributionID("stripe"), registrar.contributions[1].ContributionID)
}

// SPDX-License-Identifier: MPL-2.0

package distribution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectPaymentModules_SelectsInProfileOrderAndOmitsUnselected(t *testing.T) {
	first := &metadataPaymentModule{id: "module.first", network: "first", asset: PaymentAssetAny}
	second := &metadataPaymentModule{id: "module.second", network: "second", asset: PaymentAssetAny}
	selected, err := SelectPaymentModules(PaymentModuleProfile{
		ID: "commercial", Version: "1",
		Modules: []PaymentProfileModule{
			{ModuleID: "module.second", Requirement: PaymentProfileRequired},
		},
	}, []PaymentModule{first, second})
	require.NoError(t, err)
	require.Len(t, selected, 1)
	assert.Same(t, second, selected[0])
}

func TestSelectPaymentModules_RequiredMissingFailsOptionalMissingIsSkipped(t *testing.T) {
	profile := PaymentModuleProfile{
		ID: "required-only", Version: "1",
		Modules: []PaymentProfileModule{{ModuleID: "module.xmr", Requirement: PaymentProfileRequired}},
	}
	_, err := SelectPaymentModules(profile, nil)
	require.ErrorContains(t, err, "requires unavailable module")

	profile.Modules[0].Requirement = PaymentProfileOptional
	selected, err := SelectPaymentModules(profile, nil)
	require.NoError(t, err)
	assert.Empty(t, selected)
}

func TestSelectPaymentModules_RejectsDuplicateSelection(t *testing.T) {
	module := &metadataPaymentModule{id: "module.duplicate", network: "network", asset: PaymentAssetAny}
	_, err := SelectPaymentModules(PaymentModuleProfile{
		ID: "invalid", Version: "1",
		Modules: []PaymentProfileModule{
			{ModuleID: "module.duplicate", Requirement: PaymentProfileRequired},
			{ModuleID: "module.duplicate", Requirement: PaymentProfileOptional},
		},
	}, []PaymentModule{module})
	require.ErrorContains(t, err, "more than once")
}

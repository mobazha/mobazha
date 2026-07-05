// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/stretchr/testify/require"
)

func TestOrderExtensionProvisioningPolicyRunsCollateralAdmission(t *testing.T) {
	extension, err := extensions.NewOrderExtension("order-collateral", "provider", "source-custody", "v1", "resource", map[string]string{"mode": "M2"})
	require.NoError(t, err)
	denied := errors.New("collateral allocation is stale")
	admissionCalls := 0
	policy := NewOrderExtensionsProvisioningPolicy(
		func(SessionProvisioningPolicyInput) ([]extensions.OrderExtension, error) {
			return []extensions.OrderExtension{extension}, nil
		},
		nil,
		nil,
		func(_ context.Context, input SessionProvisioningPolicyInput) error {
			admissionCalls++
			require.Equal(t, "order-collateral", input.OrderID)
			return denied
		},
	)

	err = policy.AuthorizeSessionProvisioning(context.Background(), SessionProvisioningPolicyInput{OrderID: "order-collateral"})
	require.ErrorIs(t, err, ErrOrderExtensionCollateral)
	require.ErrorIs(t, err, denied)
	require.Equal(t, 1, admissionCalls)
}

func TestOrderExtensionProvisioningPolicySkipsCollateralAdmissionWithoutHook(t *testing.T) {
	policy := NewOrderExtensionsProvisioningPolicy(
		func(SessionProvisioningPolicyInput) ([]extensions.OrderExtension, error) { return nil, nil },
		nil,
		nil,
	)
	require.NoError(t, policy.AuthorizeSessionProvisioning(context.Background(), SessionProvisioningPolicyInput{OrderID: "order-v1"}))
}

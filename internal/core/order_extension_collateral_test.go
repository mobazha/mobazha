// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

type collateralRequirementTestModule struct {
	requirement extensions.CollateralRequirement
}

func (collateralRequirementTestModule) Descriptor() extensions.ModuleDescriptor {
	return extensions.ModuleDescriptor{
		ID: "io.mobazha.collectibles", Version: "1.0.0",
		Contracts: []string{
			extensions.ContractOrderExtensionDeclarationV1,
			extensions.ContractOrderExtensionCollateralRequirementV1,
		},
	}
}

func (collateralRequirementTestModule) DeclarationPort() extensions.DeclarationPort {
	return collateralRequirementTestDeclaration{}
}

func (m collateralRequirementTestModule) CollateralRequirementPort() extensions.CollateralRequirementPort {
	return collateralRequirementTestPort{requirement: m.requirement}
}

type collateralRequirementTestDeclaration struct{}

func (collateralRequirementTestDeclaration) DeclareOrderExtensions(context.Context, extensions.DeclarationInput) ([]extensions.OrderExtension, error) {
	return nil, nil
}

type collateralRequirementTestPort struct {
	requirement extensions.CollateralRequirement
}

func (p collateralRequirementTestPort) CollateralRequirement(context.Context, extensions.CollateralRequirementInput) (extensions.CollateralRequirement, bool, error) {
	return p.requirement, true, nil
}

func TestCollateralRequirementFailsClosedWithoutCoreIssuedV2Binding(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.OrderExtensionRecord{}); err != nil {
			return err
		}
		return tx.Migrate(&models.CollateralOrderExtensionBindingRecord{})
	}))

	extension, err := extensions.NewOrderExtension(
		"order-requires-collateral", "io.mobazha.collectibles", "source-custody", "v1", "source-1", map[string]string{"mode": "M2"},
	)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return orderextensions.PersistTx(tx, "order-requires-collateral", extension)
	}))
	requirement := extensions.CollateralRequirement{
		ProviderID: extension.ProviderID, ResourceID: extension.ResourceID, PrincipalID: "seller-1",
		AssetID: "crypto:eip155:56:erc20:0x55d398326f99059fF775485246999027B3197955", Amount: "1000000",
		PolicyID: "collectibles-source-custody", PolicyVersion: "1",
	}
	node := &MobazhaNode{orderExtensionFields: orderExtensionFields{
		orderExtensionModules: mustRegisterOrderExtensionModules(t, collateralRequirementTestModule{requirement: requirement}),
	}}
	err = db.View(func(tx database.Tx) error {
		return node.admitOrderExtensionCollateralRequirementsTx(
			context.Background(), tx, "order-requires-collateral", nil, []extensions.OrderExtension{extension},
		)
	})
	require.ErrorContains(t, err, "requires a Core-issued collateral allocation binding")
}

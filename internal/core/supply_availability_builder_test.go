package core

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/fulfillment"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestSupplyAvailabilityProvidersForNodeIncludesBaseProvidersWithoutSupplyChainRegistry(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.InventoryReservation{})
	providers := supplyAvailabilityProvidersForNode(&MobazhaNode{
		storageFields: storageFields{db: db},
	})

	require.Equal(t, []contracts.SupplyKind{
		contracts.SupplyKindSkuQuantity,
		contracts.SupplyKindLicenseKeyPool,
		contracts.SupplyKindUnlimitedDigital,
	}, supplyProviderKinds(providers))
}

func TestSupplyAvailabilityProvidersForNodeIncludesExternalWhenSupplyChainRegistryExists(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.InventoryReservation{}, &models.SyncedProductMapping{})
	providers := supplyAvailabilityProvidersForNode(&MobazhaNode{
		storageFields: storageFields{db: db},
		appServices: appServices{
			supplyChainRegistry: fulfillment.NewRegistry(),
		},
	})

	require.Equal(t, []contracts.SupplyKind{
		contracts.SupplyKindSkuQuantity,
		contracts.SupplyKindLicenseKeyPool,
		contracts.SupplyKindUnlimitedDigital,
		contracts.SupplyKindExternalSupply,
	}, supplyProviderKinds(providers))
}

func TestInitSupplyAvailabilitySubsystemRegistersExternalProviderWhenAvailable(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.InventoryReservation{}, &models.SyncedProductMapping{})
	node := &MobazhaNode{
		storageFields: storageFields{db: db},
		appServices: appServices{
			supplyChainRegistry: fulfillment.NewRegistry(),
		},
	}

	initSupplyAvailabilitySubsystem(node)

	service, ok := node.supplyAvailabilityService.(*SupplyAvailabilityAppService)
	require.True(t, ok)
	require.Contains(t, service.providers, contracts.SupplyKindSkuQuantity)
	require.Contains(t, service.providers, contracts.SupplyKindLicenseKeyPool)
	require.Contains(t, service.providers, contracts.SupplyKindUnlimitedDigital)
	require.Contains(t, service.providers, contracts.SupplyKindExternalSupply)
}

func supplyProviderKinds(providers []contracts.SupplyProvider) []contracts.SupplyKind {
	kinds := make([]contracts.SupplyKind, 0, len(providers))
	for _, provider := range providers {
		kinds = append(kinds, provider.Kind())
	}
	return kinds
}

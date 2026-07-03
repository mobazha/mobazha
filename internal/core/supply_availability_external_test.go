package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/fulfillment"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestExternalSupplyProvider_GetAvailabilityRequiresSyncedMappingAndRegisteredProvider(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.SyncedProductMapping{})
	reg := fulfillment.NewRegistry()
	require.NoError(t, reg.Register(&externalSupplyStubProvider{id: "printful"}))
	seedExternalSupplyMapping(t, db, models.SyncedProductMapping{
		ID:            "map-1",
		ProviderID:    "printful",
		ListingSlug:   "supplier-shirt",
		SyncProductID: "sync-123",
		Status:        "synced",
		LastSyncAt:    time.Now(),
	})

	provider := NewExternalSupplyProvider(db, reg)
	result, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{
		Line: externalSupplyLine("supplier-shirt", 2),
	})

	require.NoError(t, err)
	require.True(t, result.Available)
	require.Equal(t, contracts.SupplyAvailabilityAvailable, result.Status)
	require.Equal(t, int64(2), result.AvailableQuantity)
	require.Equal(t, "printful", result.ProviderID)
	require.Equal(t, "sync-123", result.ProviderRef)
}

func TestExternalSupplyProvider_GetAvailabilityMissingMappingRequiresManualAction(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.SyncedProductMapping{})
	provider := NewExternalSupplyProvider(db, fulfillment.NewRegistry())

	result, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{
		Line: externalSupplyLine("missing-shirt", 1),
	})

	require.NoError(t, err)
	require.False(t, result.Available)
	require.True(t, result.ManualActionRequired)
	require.Equal(t, contracts.SupplyAvailabilityManualActionRequired, result.Status)
	require.Equal(t, "external_mapping_missing", result.Reason)
}

func TestExternalSupplyProvider_GetAvailabilityUnregisteredProviderIsSupplierUnavailable(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.SyncedProductMapping{})
	seedExternalSupplyMapping(t, db, models.SyncedProductMapping{
		ID:          "map-1",
		ProviderID:  "printful",
		ListingSlug: "supplier-shirt",
		Status:      "synced",
		LastSyncAt:  time.Now(),
	})

	provider := NewExternalSupplyProvider(db, fulfillment.NewRegistry())
	result, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{
		Line: externalSupplyLine("supplier-shirt", 1),
	})

	require.NoError(t, err)
	require.False(t, result.Available)
	require.Equal(t, contracts.SupplyAvailabilitySupplierUnavailable, result.Status)
	require.Equal(t, "supplier_provider_unavailable", result.Reason)
}

func TestExternalSupplyProvider_GetAvailabilityMappingStatusIsSupplierUnavailable(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.SyncedProductMapping{})
	reg := fulfillment.NewRegistry()
	require.NoError(t, reg.Register(&externalSupplyStubProvider{id: "printful"}))
	seedExternalSupplyMapping(t, db, models.SyncedProductMapping{
		ID:          "map-1",
		ProviderID:  "printful",
		ListingSlug: "supplier-shirt",
		Status:      "stock_out",
		LastSyncAt:  time.Now(),
	})

	provider := NewExternalSupplyProvider(db, reg)
	result, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{
		Line: externalSupplyLine("supplier-shirt", 1),
	})

	require.NoError(t, err)
	require.False(t, result.Available)
	require.Equal(t, contracts.SupplyAvailabilitySupplierUnavailable, result.Status)
	require.Equal(t, "supplier_mapping_stock_out", result.Reason)
}

func TestExternalSupplyProvider_ReserveFailsClosedWithoutExternalHold(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.SyncedProductMapping{})
	provider := NewExternalSupplyProvider(db, fulfillment.NewRegistry())
	req := contracts.ReserveSupplyRequest{
		OrderRef:  "order-1",
		OrderType: models.OrderTypeStandard,
		Line:      externalSupplyLine("supplier-shirt", 1),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	_, err := provider.Reserve(context.Background(), req)
	require.ErrorIs(t, err, contracts.ErrSupplyManualActionRequired)

	err = db.Update(func(tx database.Tx) error {
		_, err := provider.ReserveTx(context.Background(), tx, req)
		return err
	})
	require.ErrorIs(t, err, contracts.ErrSupplyManualActionRequired)

	require.NoError(t, provider.Commit(context.Background(), contracts.CommitSupplyRequest{
		OrderRef:  "order-1",
		OrderType: models.OrderTypeStandard,
	}))
	require.NoError(t, provider.Release(context.Background(), contracts.ReleaseSupplyRequest{
		OrderRef:  "order-1",
		OrderType: models.OrderTypeStandard,
		Reason:    "cancelled",
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := provider.CommitTx(context.Background(), tx, contracts.CommitSupplyRequest{
			OrderRef:  "order-1",
			OrderType: models.OrderTypeStandard,
		}); err != nil {
			return err
		}
		return provider.ReleaseTx(context.Background(), tx, contracts.ReleaseSupplyRequest{
			OrderRef:  "order-1",
			OrderType: models.OrderTypeStandard,
			Reason:    "cancelled",
		})
	}))
}

func TestSupplyAvailabilityAppService_ExternalReserveTxFailsClosedWithoutInventoryHold(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.SyncedProductMapping{}, &models.InventoryReservation{})
	reg := fulfillment.NewRegistry()
	require.NoError(t, reg.Register(&externalSupplyStubProvider{id: "printful"}))
	seedExternalSupplyMapping(t, db, models.SyncedProductMapping{
		ID:            "map-1",
		ProviderID:    "printful",
		ListingSlug:   "supplier-shirt",
		SyncProductID: "sync-123",
		Status:        "synced",
		LastSyncAt:    time.Now(),
	})
	service, err := NewSupplyAvailabilityAppService(NewExternalSupplyProvider(db, reg))
	require.NoError(t, err)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		_, reserveErr := service.ReserveOrderTx(context.Background(), tx, contracts.ReserveOrderSupplyRequest{
			OrderRef:  "order-external",
			OrderType: models.OrderTypeStandard,
			ExpiresAt: time.Now().Add(time.Hour),
			Lines: []contracts.SupplyLine{
				externalSupplyLine("supplier-shirt", 1),
			},
		})
		require.ErrorIs(t, reserveErr, contracts.ErrSupplyManualActionRequired)
		return nil
	}))

	var count int64
	require.NoError(t, db.gormDB.Model(&models.InventoryReservation{}).Count(&count).Error)
	require.Zero(t, count)
}

func externalSupplyLine(listingSlug string, quantity int) contracts.SupplyLine {
	return contracts.SupplyLine{
		LineID:      listingSlug + "-line",
		ListingSlug: listingSlug,
		Quantity:    quantity,
		SupplyKind:  contracts.SupplyKindExternalSupply,
	}
}

func seedExternalSupplyMapping(t *testing.T, db *featureTestDatabase, mapping models.SyncedProductMapping) {
	t.Helper()
	if mapping.TenantID == "" {
		mapping.TenantID = database.StandaloneTenantID
	}
	require.NoError(t, db.gormDB.Create(&mapping).Error)
}

type externalSupplyStubProvider struct {
	id string
}

func (p *externalSupplyStubProvider) ProviderID() string   { return p.id }
func (p *externalSupplyStubProvider) ProviderType() string { return "pod" }
func (p *externalSupplyStubProvider) ValidateCredentials(context.Context, contracts.ProviderCredentials) error {
	return nil
}
func (p *externalSupplyStubProvider) CreateFulfillmentOrder(context.Context, contracts.CreateFulfillmentParams) (*contracts.FulfillmentOrder, error) {
	return nil, errors.New("not used")
}
func (p *externalSupplyStubProvider) GetFulfillmentOrder(context.Context, string) (*contracts.FulfillmentOrder, error) {
	return nil, errors.New("not used")
}
func (p *externalSupplyStubProvider) CancelFulfillmentOrder(context.Context, string) error {
	return errors.New("not used")
}
func (p *externalSupplyStubProvider) ParseWebhook(context.Context, []byte, map[string]string) (*contracts.FulfillmentWebhookEvent, error) {
	return nil, errors.New("not used")
}
func (p *externalSupplyStubProvider) EstimateShipping(context.Context, contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
	return nil, errors.New("not used")
}

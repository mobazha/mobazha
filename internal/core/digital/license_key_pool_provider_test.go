package digital

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestLicenseKeyPoolProvider_ReserveReleaseAndCommitLifecycle(t *testing.T) {
	assetSvc, db := newTestAssetService(t)
	licenseExpiresAt := time.Date(2035, 1, 2, 3, 4, 5, 0, time.UTC)
	_, err := assetSvc.ImportLicenseKeys("listing-lic", "", "app-lic", []string{"KEY-001", "KEY-002"}, "perpetual", 1, licenseExpiresAt)
	require.NoError(t, err)
	provider := NewLicenseKeyPoolProvider(db)

	availability, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{
		Line: licenseKeySupplyLine("listing-lic", 2),
	})
	require.NoError(t, err)
	require.True(t, availability.Available)
	require.Equal(t, int64(2), availability.AvailableQuantity)

	reservation, err := provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:    "order-reserve",
		OrderType:   models.OrderTypeStandard,
		BuyerPeerID: "buyer-1",
		Line:        licenseKeySupplyLine("listing-lic", 2),
		ExpiresAt:   time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, contracts.SupplyReservationReserved, reservation.Status)
	require.Equal(t, 2, reservation.Quantity)
	require.NotEmpty(t, reservation.ReservationRef)

	keys := licenseKeysByOrder(t, db, "order-reserve")
	require.Len(t, keys, 2)
	for _, key := range keys {
		require.Equal(t, models.LicenseKeyStatusReserved, key.Status)
		require.Equal(t, models.OrderTypeStandard, key.OrderType)
		require.Equal(t, "buyer-1", key.BuyerPeerID)
		require.True(t, key.DispensedAt.IsZero())
		require.Equal(t, licenseExpiresAt, key.ExpiresAt)
	}

	availability, err = provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{
		Line: licenseKeySupplyLine("listing-lic", 1),
	})
	require.NoError(t, err)
	require.False(t, availability.Available)
	require.Equal(t, contracts.SupplyAvailabilityOutOfStock, availability.Status)

	stats, err := assetSvc.GetLicenseKeyPoolStats("listing-lic", "")
	require.NoError(t, err)
	require.Equal(t, int64(0), stats.Available)
	require.Equal(t, int64(0), stats.Dispensed)
	require.Equal(t, int64(2), stats.Total)

	_, err = assetSvc.AllocateLicenseKey("listing-lic", "", "order-other", "buyer-other")
	require.Error(t, err, "reserved keys must not be dispensed by the old allocator")

	require.NoError(t, provider.Release(context.Background(), contracts.ReleaseSupplyRequest{
		OrderRef:  "order-reserve",
		OrderType: models.OrderTypeStandard,
		Reason:    "cancelled",
	}))
	keys = licenseKeysByListing(t, db, "listing-lic")
	for _, key := range keys {
		require.Equal(t, models.LicenseKeyStatusAvailable, key.Status)
		require.Empty(t, key.OrderID)
		require.Empty(t, key.OrderType)
		require.Empty(t, key.BuyerPeerID)
	}

	_, err = provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:    "order-commit",
		OrderType:   models.OrderTypeStandard,
		BuyerPeerID: "buyer-commit",
		Line:        licenseKeySupplyLine("listing-lic", 1),
		ExpiresAt:   time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.NoError(t, provider.Commit(context.Background(), contracts.CommitSupplyRequest{
		OrderRef:  "order-commit",
		OrderType: models.OrderTypeStandard,
	}))
	require.NoError(t, provider.Commit(context.Background(), contracts.CommitSupplyRequest{
		OrderRef:  "order-commit",
		OrderType: models.OrderTypeStandard,
	}))

	keys = licenseKeysByOrder(t, db, "order-commit")
	require.Len(t, keys, 1)
	require.Equal(t, models.LicenseKeyStatusDispensed, keys[0].Status)
	require.False(t, keys[0].DispensedAt.IsZero())
	require.Equal(t, int64(1), assetSvc.CountAllocatedKeys("order-commit", "listing-lic", ""))

	require.NoError(t, provider.Release(context.Background(), contracts.ReleaseSupplyRequest{
		OrderRef:  "order-commit",
		OrderType: models.OrderTypeStandard,
		Reason:    "refund",
	}))
	keys = licenseKeysByOrder(t, db, "order-commit")
	require.Len(t, keys, 1)
	require.Equal(t, models.LicenseKeyStatusDispensed, keys[0].Status, "release must not return committed keys to the pool")
}

func TestLicenseKeyPoolProvider_ReserveFailsWhenPoolExhausted(t *testing.T) {
	assetSvc, db := newTestAssetService(t)
	_, err := assetSvc.ImportLicenseKeys("listing-small", "", "app-small", []string{"ONLY-KEY"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)
	provider := NewLicenseKeyPoolProvider(db)

	_, err = provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:  "order-too-large",
		OrderType: models.OrderTypeStandard,
		Line:      licenseKeySupplyLine("listing-small", 2),
		ExpiresAt: time.Now().Add(time.Hour),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, contracts.ErrSupplyUnavailable))

	keys := licenseKeysByListing(t, db, "listing-small")
	require.Len(t, keys, 1)
	require.Equal(t, models.LicenseKeyStatusAvailable, keys[0].Status)
	require.Empty(t, keys[0].OrderID)
	require.Empty(t, keys[0].OrderType)
}

func TestLicenseKeyPoolProvider_ReserveTxCleansPartialHoldOnFailure(t *testing.T) {
	assetSvc, db := newTestAssetService(t)
	_, err := assetSvc.ImportLicenseKeys("listing-partial", "", "app-partial", []string{"ONLY-KEY"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)
	provider := NewLicenseKeyPoolProvider(db)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		_, reserveErr := provider.ReserveTx(context.Background(), tx, contracts.ReserveSupplyRequest{
			OrderRef:  "order-partial",
			OrderType: models.OrderTypeStandard,
			Line:      licenseKeySupplyLine("listing-partial", 2),
			ExpiresAt: time.Now().Add(time.Hour),
		})
		require.Error(t, reserveErr)
		require.True(t, errors.Is(reserveErr, contracts.ErrSupplyUnavailable))
		return nil
	}))

	keys := licenseKeysByListing(t, db, "listing-partial")
	require.Len(t, keys, 1)
	require.Equal(t, models.LicenseKeyStatusAvailable, keys[0].Status)
	require.Empty(t, keys[0].OrderID)
	require.Empty(t, keys[0].OrderType)
}

func TestLicenseKeyPoolProvider_ReserveIsIdempotentForSameOrder(t *testing.T) {
	assetSvc, db := newTestAssetService(t)
	_, err := assetSvc.ImportLicenseKeys("listing-repeat", "", "app-repeat", []string{"KEY-1", "KEY-2"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)
	provider := NewLicenseKeyPoolProvider(db)
	req := contracts.ReserveSupplyRequest{
		OrderRef:  "order-repeat",
		OrderType: models.OrderTypeStandard,
		Line:      licenseKeySupplyLine("listing-repeat", 1),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	first, err := provider.Reserve(context.Background(), req)
	require.NoError(t, err)
	second, err := provider.Reserve(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, first.ReservationRef, second.ReservationRef)
	keys := licenseKeysByOrder(t, db, "order-repeat")
	require.Len(t, keys, 1)
	require.Equal(t, models.LicenseKeyStatusReserved, keys[0].Status)
	require.Equal(t, models.OrderTypeStandard, keys[0].OrderType)
}

func TestLicenseKeyPoolProvider_IsolatesSameOrderRefByOrderType(t *testing.T) {
	assetSvc, db := newTestAssetService(t)
	_, err := assetSvc.ImportLicenseKeys("listing-type", "", "app-type", []string{"KEY-STD", "KEY-GUEST"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)
	provider := NewLicenseKeyPoolProvider(db)
	expiresAt := time.Now().Add(time.Hour)

	_, err = provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:  "same-ref",
		OrderType: models.OrderTypeStandard,
		Line:      licenseKeySupplyLine("listing-type", 1),
		ExpiresAt: expiresAt,
	})
	require.NoError(t, err)
	_, err = provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:  "same-ref",
		OrderType: models.OrderTypeGuest,
		Line:      licenseKeySupplyLine("listing-type", 1),
		ExpiresAt: expiresAt,
	})
	require.NoError(t, err)

	require.Len(t, licenseKeysByOrderType(t, db, "same-ref", models.OrderTypeStandard), 1)
	require.Len(t, licenseKeysByOrderType(t, db, "same-ref", models.OrderTypeGuest), 1)

	require.NoError(t, provider.Commit(context.Background(), contracts.CommitSupplyRequest{
		OrderRef:  "same-ref",
		OrderType: models.OrderTypeStandard,
	}))
	require.NoError(t, provider.Release(context.Background(), contracts.ReleaseSupplyRequest{
		OrderRef:  "same-ref",
		OrderType: models.OrderTypeGuest,
		Reason:    "guest_expired",
	}))

	standardKeys := licenseKeysByOrderType(t, db, "same-ref", models.OrderTypeStandard)
	require.Len(t, standardKeys, 1)
	require.Equal(t, models.LicenseKeyStatusDispensed, standardKeys[0].Status)
	require.Empty(t, licenseKeysByOrderType(t, db, "same-ref", models.OrderTypeGuest))

	keys := licenseKeysByListing(t, db, "listing-type")
	require.ElementsMatch(t, []string{models.LicenseKeyStatusAvailable, models.LicenseKeyStatusDispensed}, []string{keys[0].Status, keys[1].Status})
}

func TestLicenseKeyPoolProvider_RefundRevokeDoesNotRestockCommittedKeys(t *testing.T) {
	assetSvc, db := newTestAssetService(t)
	_, err := assetSvc.ImportLicenseKeys("listing-refund", "", "app-refund", []string{"REFUND-KEY"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)
	provider := NewLicenseKeyPoolProvider(db)

	_, err = provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:    "order-refund",
		OrderType:   models.OrderTypeStandard,
		BuyerPeerID: "buyer-refund",
		Line:        licenseKeySupplyLine("listing-refund", 1),
		ExpiresAt:   time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.NoError(t, provider.Commit(context.Background(), contracts.CommitSupplyRequest{
		OrderRef:  "order-refund",
		OrderType: models.OrderTypeStandard,
	}))

	entitlement := &DigitalEntitlementAppService{db: db, assets: assetSvc}
	entitlement.revokeLicensesByOrder("order-refund")

	keys := licenseKeysByOrder(t, db, "order-refund")
	require.Len(t, keys, 1)
	require.Equal(t, models.LicenseKeyStatusRevoked, keys[0].Status)

	require.NoError(t, provider.Release(context.Background(), contracts.ReleaseSupplyRequest{
		OrderRef:  "order-refund",
		OrderType: models.OrderTypeStandard,
		Reason:    "refund",
	}))
	keys = licenseKeysByOrder(t, db, "order-refund")
	require.Len(t, keys, 1)
	require.Equal(t, models.LicenseKeyStatusRevoked, keys[0].Status, "refund revoke must not be converted into stock release")

	availability, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{
		Line: licenseKeySupplyLine("listing-refund", 1),
	})
	require.NoError(t, err)
	require.False(t, availability.Available)
	require.Equal(t, int64(0), availability.AvailableQuantity)

	validation, err := assetSvc.ValidateLicense("REFUND-KEY", "app-refund")
	require.NoError(t, err)
	require.False(t, validation.Valid)
	require.Equal(t, "revoked", validation.Reason)
}

func TestLicenseKeyPoolProvider_DisputeSuspendRestoreDoesNotRestock(t *testing.T) {
	assetSvc, db := newTestAssetService(t)
	_, err := assetSvc.ImportLicenseKeys("listing-dispute", "", "app-dispute", []string{"DISPUTE-KEY"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)
	provider := NewLicenseKeyPoolProvider(db)

	_, err = provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:    "order-dispute",
		OrderType:   models.OrderTypeStandard,
		BuyerPeerID: "buyer-dispute",
		Line:        licenseKeySupplyLine("listing-dispute", 1),
		ExpiresAt:   time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.NoError(t, provider.Commit(context.Background(), contracts.CommitSupplyRequest{
		OrderRef:  "order-dispute",
		OrderType: models.OrderTypeStandard,
	}))

	entitlement := &DigitalEntitlementAppService{db: db, assets: assetSvc}
	entitlement.suspendLicensesByOrder("order-dispute")

	keys := licenseKeysByOrder(t, db, "order-dispute")
	require.Len(t, keys, 1)
	require.Equal(t, models.LicenseKeyStatusSuspended, keys[0].Status)

	validation, err := assetSvc.ValidateLicense("DISPUTE-KEY", "app-dispute")
	require.NoError(t, err)
	require.False(t, validation.Valid)
	require.Equal(t, "suspended", validation.Reason)

	require.NoError(t, provider.Release(context.Background(), contracts.ReleaseSupplyRequest{
		OrderRef:  "order-dispute",
		OrderType: models.OrderTypeStandard,
		Reason:    "dispute_opened",
	}))
	keys = licenseKeysByOrder(t, db, "order-dispute")
	require.Len(t, keys, 1)
	require.Equal(t, models.LicenseKeyStatusSuspended, keys[0].Status, "dispute suspension must not be returned to available stock")

	entitlement.restoreLicensesByOrder("order-dispute")

	keys = licenseKeysByOrder(t, db, "order-dispute")
	require.Len(t, keys, 1)
	require.Equal(t, models.LicenseKeyStatusDispensed, keys[0].Status)

	validation, err = assetSvc.ValidateLicense("DISPUTE-KEY", "app-dispute")
	require.NoError(t, err)
	require.True(t, validation.Valid)
}

func licenseKeySupplyLine(listingSlug string, quantity int) contracts.SupplyLine {
	return contracts.SupplyLine{
		ListingSlug: listingSlug,
		Quantity:    quantity,
		SupplyKind:  contracts.SupplyKindLicenseKeyPool,
	}
}

func licenseKeysByOrder(t *testing.T, db *testDatabase, orderID string) []models.DigitalLicenseKey {
	t.Helper()
	var keys []models.DigitalLicenseKey
	require.NoError(t, db.gormDB.Where("order_id = ?", orderID).Order("id ASC").Find(&keys).Error)
	return keys
}

func licenseKeysByOrderType(t *testing.T, db *testDatabase, orderID string, orderType string) []models.DigitalLicenseKey {
	t.Helper()
	var keys []models.DigitalLicenseKey
	require.NoError(t, db.gormDB.Where("order_id = ? AND order_type = ?", orderID, orderType).Order("id ASC").Find(&keys).Error)
	return keys
}

func licenseKeysByListing(t *testing.T, db *testDatabase, listingSlug string) []models.DigitalLicenseKey {
	t.Helper()
	var keys []models.DigitalLicenseKey
	require.NoError(t, db.gormDB.Where("listing_slug = ?", listingSlug).Order("id ASC").Find(&keys).Error)
	return keys
}

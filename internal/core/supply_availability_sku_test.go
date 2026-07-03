package core

import (
	"context"
	"errors"
	"testing"
	"time"

	dbstore "github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSkuQuantityProvider_GetAvailabilitySubtractsUnreleasedReservations(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.InventoryReservation{})
	now := time.Now()
	seedSkuReservation(t, db, 1, models.InventoryReservation{
		OrderRef:    "guest-1",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "camera",
		VariantHash: "red",
		Quantity:    2,
		ReservedAt:  now,
		ExpiresAt:   now.Add(time.Hour),
	})
	releasedAt := now.Add(-time.Minute)
	seedSkuReservation(t, db, 2, models.InventoryReservation{
		OrderRef:    "guest-2",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "camera",
		VariantHash: "red",
		Quantity:    2,
		ReservedAt:  now.Add(-2 * time.Hour),
		ExpiresAt:   now.Add(-time.Hour),
		ReleasedAt:  &releasedAt,
	})
	seedSkuReservation(t, db, 3, models.InventoryReservation{
		OrderRef:    "standard-1",
		OrderType:   models.OrderTypeStandard,
		ListingSlug: "camera",
		VariantHash: "red",
		Quantity:    1,
		ReservedAt:  now,
		ExpiresAt:   now.Add(time.Hour),
		Confirmed:   true,
	})

	provider := NewSkuQuantityProvider(db)
	result, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{
		Line: trackedSkuLine("camera", "red", 2, 5),
	})

	require.NoError(t, err)
	require.Equal(t, contracts.SupplyAvailabilityAvailable, result.Status)
	require.True(t, result.Available)
	require.Equal(t, int64(2), result.AvailableQuantity)
}

func TestSkuQuantityProvider_ReserveFailsWhenTrackedStockIsUnavailable(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.InventoryReservation{})
	now := time.Now()
	seedSkuReservation(t, db, 1, models.InventoryReservation{
		OrderRef:    "guest-1",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "camera",
		VariantHash: "red",
		Quantity:    4,
		ReservedAt:  now,
		ExpiresAt:   now.Add(time.Hour),
	})

	provider := NewSkuQuantityProvider(db)
	_, err := provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:  "guest-2",
		OrderType: models.OrderTypeGuest,
		Line:      trackedSkuLine("camera", "red", 2, 5),
		ExpiresAt: now.Add(time.Hour),
	})

	require.Error(t, err)
	require.True(t, errors.Is(err, contracts.ErrSupplyUnavailable))
	var count int64
	require.NoError(t, db.gormDB.Model(&models.InventoryReservation{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSkuQuantityProvider_ReserveCommitAndReleaseUseInventoryReservation(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.InventoryReservation{})
	provider := NewSkuQuantityProvider(db)
	expiresAt := time.Now().Add(time.Hour)

	reservation, err := provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:  "guest-1",
		OrderType: models.OrderTypeGuest,
		Line:      trackedSkuLine("camera", "red", 2, 5),
		ExpiresAt: expiresAt,
	})

	require.NoError(t, err)
	require.Equal(t, contracts.SupplyReservationReserved, reservation.Status)
	require.Equal(t, "camera", reservation.ListingSlug)
	require.Equal(t, "red", reservation.VariantHash)

	row := loadSkuReservation(t, db, 1)
	require.Equal(t, "guest-1", row.OrderRef)
	require.False(t, row.Confirmed)
	require.Nil(t, row.ReleasedAt)

	require.NoError(t, provider.Commit(context.Background(), contracts.CommitSupplyRequest{
		OrderRef:  "guest-1",
		OrderType: models.OrderTypeGuest,
	}))
	require.NoError(t, provider.Commit(context.Background(), contracts.CommitSupplyRequest{
		OrderRef:  "guest-1",
		OrderType: models.OrderTypeGuest,
	}))
	row = loadSkuReservation(t, db, 1)
	require.True(t, row.Confirmed)
	require.Equal(t, 2099, row.ExpiresAt.Year())

	require.NoError(t, provider.Release(context.Background(), contracts.ReleaseSupplyRequest{
		OrderRef:  "guest-1",
		OrderType: models.OrderTypeGuest,
		Reason:    "cancelled",
	}))
	row = loadSkuReservation(t, db, 1)
	require.Nil(t, row.ReleasedAt)
}

func TestSkuQuantityProvider_ReserveIsIdempotentForSameOrderBucket(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.InventoryReservation{})
	provider := NewSkuQuantityProvider(db)
	expiresAt := time.Now().Add(time.Hour)
	req := contracts.ReserveSupplyRequest{
		OrderRef:  "standard-1",
		OrderType: models.OrderTypeStandard,
		Line:      trackedSkuLine("camera", "red", 1, 5),
		ExpiresAt: expiresAt,
	}

	first, err := provider.Reserve(context.Background(), req)
	require.NoError(t, err)
	second, err := provider.Reserve(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID)
	var count int64
	require.NoError(t, db.gormDB.Model(&models.InventoryReservation{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSkuQuantityProvider_ReleaseMarksUncommittedReservations(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.InventoryReservation{})
	provider := NewSkuQuantityProvider(db)
	expiresAt := time.Now().Add(time.Hour)

	_, err := provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:  "guest-1",
		OrderType: models.OrderTypeGuest,
		Line:      trackedSkuLine("camera", "red", 2, 5),
		ExpiresAt: expiresAt,
	})
	require.NoError(t, err)

	require.NoError(t, provider.Release(context.Background(), contracts.ReleaseSupplyRequest{
		OrderRef:  "guest-1",
		OrderType: models.OrderTypeGuest,
		Reason:    "cancelled",
	}))
	require.NoError(t, provider.Release(context.Background(), contracts.ReleaseSupplyRequest{
		OrderRef:  "guest-1",
		OrderType: models.OrderTypeGuest,
		Reason:    "cancelled",
	}))
	row := loadSkuReservation(t, db, 1)
	require.NotNil(t, row.ReleasedAt)
}

func TestSkuQuantityProvider_UntrackedStockIsUnlimitedButStillReservable(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.InventoryReservation{})
	provider := NewSkuQuantityProvider(db)

	availability, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{
		Line: contracts.SupplyLine{
			ListingSlug:  "ebook",
			Quantity:     1,
			SupplyKind:   contracts.SupplyKindSkuQuantity,
			StockTracked: false,
		},
	})
	require.NoError(t, err)
	require.Equal(t, contracts.SupplyAvailabilityUnlimited, availability.Status)
	require.True(t, availability.Available)
	require.True(t, availability.Unlimited)

	reservation, err := provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:  "guest-1",
		OrderType: models.OrderTypeGuest,
		Line: contracts.SupplyLine{
			ListingSlug:  "ebook",
			Quantity:     1,
			SupplyKind:   contracts.SupplyKindSkuQuantity,
			StockTracked: false,
		},
		ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, contracts.SupplyReservationReserved, reservation.Status)
}

func TestSkuQuantityProvider_StandardAndGuestShareSameSkuBucket(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.InventoryReservation{})
	now := time.Now()
	seedSkuReservation(t, db, 1, models.InventoryReservation{
		OrderRef:    "guest-1",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "camera",
		VariantHash: "red",
		Quantity:    1,
		ReservedAt:  now,
		ExpiresAt:   now.Add(time.Hour),
	})
	provider := NewSkuQuantityProvider(db)

	standardReservation, err := provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:  "standard-1",
		OrderType: models.OrderTypeStandard,
		Line:      trackedSkuLine("camera", "red", 1, 2),
		ExpiresAt: now.Add(time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, models.OrderTypeStandard, standardReservation.OrderType)

	availability, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{
		Line: trackedSkuLine("camera", "red", 1, 2),
	})
	require.NoError(t, err)
	require.False(t, availability.Available)
	require.Equal(t, contracts.SupplyAvailabilityOutOfStock, availability.Status)
	require.Equal(t, int64(0), availability.AvailableQuantity)
}

func TestSkuQuantityProvider_UsesTenantScopedReservations(t *testing.T) {
	sharedDB := newSupplyAvailabilitySharedDB(t)
	require.NoError(t, sharedDB.AutoMigrate(&models.InventoryReservation{}))
	dbA, err := dbstore.NewTenantDBWithPublicData(sharedDB, "tenant-a", dbstore.NewDBPublicData(sharedDB, "tenant-a"))
	require.NoError(t, err)
	dbB, err := dbstore.NewTenantDBWithPublicData(sharedDB, "tenant-b", dbstore.NewDBPublicData(sharedDB, "tenant-b"))
	require.NoError(t, err)

	providerA := NewSkuQuantityProvider(dbA)
	providerB := NewSkuQuantityProvider(dbB)
	expiresAt := time.Now().Add(time.Hour)
	_, err = providerA.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:  "tenant-a-guest",
		OrderType: models.OrderTypeGuest,
		Line:      trackedSkuLine("camera", "red", 1, 1),
		ExpiresAt: expiresAt,
	})
	require.NoError(t, err)

	availabilityA, err := providerA.GetAvailability(context.Background(), contracts.AvailabilityRequest{
		Line: trackedSkuLine("camera", "red", 1, 1),
	})
	require.NoError(t, err)
	require.False(t, availabilityA.Available)
	require.Equal(t, int64(0), availabilityA.AvailableQuantity)

	availabilityB, err := providerB.GetAvailability(context.Background(), contracts.AvailabilityRequest{
		Line: trackedSkuLine("camera", "red", 1, 1),
	})
	require.NoError(t, err)
	require.True(t, availabilityB.Available)
	require.Equal(t, int64(1), availabilityB.AvailableQuantity)

	var tenantA, tenantB int64
	require.NoError(t, sharedDB.Model(&models.InventoryReservation{}).Where("tenant_id = ?", "tenant-a").Count(&tenantA).Error)
	require.NoError(t, sharedDB.Model(&models.InventoryReservation{}).Where("tenant_id = ?", "tenant-b").Count(&tenantB).Error)
	require.Equal(t, int64(1), tenantA)
	require.Equal(t, int64(0), tenantB)
}

func trackedSkuLine(slug, variantHash string, quantity int, stockLimit int64) contracts.SupplyLine {
	return contracts.SupplyLine{
		ListingSlug:  slug,
		VariantHash:  variantHash,
		Quantity:     quantity,
		SupplyKind:   contracts.SupplyKindSkuQuantity,
		StockTracked: true,
		StockLimit:   stockLimit,
	}
}

func newSupplyAvailabilitySharedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	return db
}

func seedSkuReservation(t *testing.T, db *featureTestDatabase, id int, r models.InventoryReservation) {
	t.Helper()
	r.ID = id
	if r.TenantID == "" {
		r.TenantID = database.StandaloneTenantID
	}
	require.NoError(t, db.gormDB.Create(&r).Error)
}

func loadSkuReservation(t *testing.T, db *featureTestDatabase, id int) models.InventoryReservation {
	t.Helper()
	var row models.InventoryReservation
	require.NoError(t, db.gormDB.Where("id = ?", id).First(&row).Error)
	return row
}

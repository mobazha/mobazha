package dbstore

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestShippingStore(t *testing.T) *database.GormShippingStore {
	t.Helper()
	tmpDir := t.TempDir()
	db, err := NewMemoryDB(tmpDir)
	require.NoError(t, err)
	require.NoError(t, database.MigrateShippingModels(db))
	t.Cleanup(func() { db.Close() })
	return database.NewGormShippingStore(db)
}

func makeProfile(id, name string, isDefault bool) *models.ShippingProfileEntity {
	return &models.ShippingProfileEntity{
		ID:                 id,
		Name:               name,
		IsDefault:          isDefault,
		LocationGroupsJSON: `[{"id":"lg-1","locationIds":[],"zones":[{"id":"z-1","name":"Global","regions":["ALL"],"rates":[{"id":"r-1","name":"Standard","price":"500","currency":"USD","estimatedDelivery":"7-14 days"}]}]}]`,
	}
}

func makeLocation(id, name string, isDefault bool) *models.ShippingLocationEntity {
	return &models.ShippingLocationEntity{
		ID:        id,
		Name:      name,
		Address:   "Test Address",
		IsDefault: isDefault,
	}
}

// --- Profile CRUD ---

func TestShippingStore_Profile_CreateAndGet(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	p := makeProfile("p1", "Default Shipping", true)
	require.NoError(t, store.CreateProfile(ctx, p))

	got, err := store.GetProfile(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, "p1", got.ID)
	assert.Equal(t, "Default Shipping", got.Name)
	assert.True(t, got.IsDefault)
	assert.Equal(t, 1, got.Version)

	groups, err := got.GetLocationGroups()
	require.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, "lg-1", groups[0].ID)
}

func TestShippingStore_Profile_GetNotFound(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	_, err := store.GetProfile(ctx, "nonexistent")
	assert.ErrorIs(t, err, database.ErrShippingProfileNotFound)
}

func TestShippingStore_Profile_List(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateProfile(ctx, makeProfile("p1", "Profile 1", true)))
	require.NoError(t, store.CreateProfile(ctx, makeProfile("p2", "Profile 2", false)))

	profiles, err := store.ListProfiles(ctx)
	require.NoError(t, err)
	assert.Len(t, profiles, 2)
}

func TestShippingStore_Profile_Update(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	p := makeProfile("p1", "Original", false)
	require.NoError(t, store.CreateProfile(ctx, p))

	p.Name = "Updated"
	p.Version = 2
	require.NoError(t, store.UpdateProfile(ctx, p))

	got, err := store.GetProfile(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.Name)
	assert.Equal(t, 2, got.Version)
}

func TestShippingStore_Profile_Delete(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateProfile(ctx, makeProfile("p1", "To Delete", false)))
	require.NoError(t, store.DeleteProfile(ctx, "p1"))

	_, err := store.GetProfile(ctx, "p1")
	assert.ErrorIs(t, err, database.ErrShippingProfileNotFound)
}

func TestShippingStore_Profile_GetDefault(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateProfile(ctx, makeProfile("p1", "Non-default", false)))
	require.NoError(t, store.CreateProfile(ctx, makeProfile("p2", "Default", true)))

	got, err := store.GetDefaultProfile(ctx)
	require.NoError(t, err)
	assert.Equal(t, "p2", got.ID)
}

func TestShippingStore_Profile_SetDefault(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateProfile(ctx, makeProfile("p1", "Profile 1", true)))
	require.NoError(t, store.CreateProfile(ctx, makeProfile("p2", "Profile 2", false)))

	require.NoError(t, store.SetDefaultProfile(ctx, "p2"))

	p1, err := store.GetProfile(ctx, "p1")
	require.NoError(t, err)
	assert.False(t, p1.IsDefault)

	p2, err := store.GetProfile(ctx, "p2")
	require.NoError(t, err)
	assert.True(t, p2.IsDefault)
}

func TestShippingStore_Profile_SetDefault_NotFound(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	err := store.SetDefaultProfile(ctx, "nonexistent")
	assert.ErrorIs(t, err, database.ErrShippingProfileNotFound)
}

// --- Location CRUD ---

func TestShippingStore_Location_CreateAndGet(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	loc := makeLocation("loc1", "Beijing Warehouse", true)
	require.NoError(t, store.CreateLocation(ctx, loc))

	got, err := store.GetLocation(ctx, "loc1")
	require.NoError(t, err)
	assert.Equal(t, "Beijing Warehouse", got.Name)
	assert.True(t, got.IsDefault)
}

func TestShippingStore_Location_List(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateLocation(ctx, makeLocation("loc1", "Warehouse 1", true)))
	require.NoError(t, store.CreateLocation(ctx, makeLocation("loc2", "Warehouse 2", false)))

	locs, err := store.ListLocations(ctx)
	require.NoError(t, err)
	assert.Len(t, locs, 2)
}

func TestShippingStore_Location_Delete(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateLocation(ctx, makeLocation("loc1", "To Delete", false)))
	require.NoError(t, store.DeleteLocation(ctx, "loc1"))

	_, err := store.GetLocation(ctx, "loc1")
	assert.ErrorIs(t, err, database.ErrShippingLocationNotFound)
}

// --- Listing-Profile Refs ---

func TestShippingStore_Ref_UpsertAndGet(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateProfile(ctx, makeProfile("p1", "Profile 1", true)))

	ref := &models.ListingShippingRef{
		ListingSlug:       "my-product",
		ShippingProfileID: "p1",
		SnapshotVersion:   1,
		IsStale:           false,
	}
	require.NoError(t, store.UpsertListingRef(ctx, ref))

	got, err := store.GetListingRef(ctx, "my-product")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "p1", got.ShippingProfileID)
	assert.Equal(t, 1, got.SnapshotVersion)
	assert.False(t, got.IsStale)
}

func TestShippingStore_Ref_Upsert_UpdateExisting(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateProfile(ctx, makeProfile("p1", "Profile 1", true)))
	require.NoError(t, store.CreateProfile(ctx, makeProfile("p2", "Profile 2", false)))

	ref := &models.ListingShippingRef{
		ListingSlug:       "my-product",
		ShippingProfileID: "p1",
		SnapshotVersion:   1,
		IsStale:           false,
	}
	require.NoError(t, store.UpsertListingRef(ctx, ref))

	ref2 := &models.ListingShippingRef{
		ListingSlug:       "my-product",
		ShippingProfileID: "p2",
		SnapshotVersion:   2,
		IsStale:           false,
	}
	require.NoError(t, store.UpsertListingRef(ctx, ref2))

	got, err := store.GetListingRef(ctx, "my-product")
	require.NoError(t, err)
	assert.Equal(t, "p2", got.ShippingProfileID)
	assert.Equal(t, 2, got.SnapshotVersion)
}

func TestShippingStore_Ref_GetNonexistent(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	got, err := store.GetListingRef(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestShippingStore_Ref_Delete(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateProfile(ctx, makeProfile("p1", "Profile 1", true)))
	ref := &models.ListingShippingRef{
		ListingSlug:       "my-product",
		ShippingProfileID: "p1",
		SnapshotVersion:   1,
		IsStale:           false,
	}
	require.NoError(t, store.UpsertListingRef(ctx, ref))

	require.NoError(t, store.DeleteListingRef(ctx, "my-product"))

	got, err := store.GetListingRef(ctx, "my-product")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestShippingStore_Ref_MarkProfileStale(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateProfile(ctx, makeProfile("p1", "Profile 1", true)))

	for _, slug := range []string{"product-1", "product-2", "product-3"} {
		ref := &models.ListingShippingRef{
			ListingSlug:       slug,
			ShippingProfileID: "p1",
			SnapshotVersion:   1,
			IsStale:           false,
		}
		require.NoError(t, store.UpsertListingRef(ctx, ref))
	}

	require.NoError(t, store.MarkProfileStale(ctx, "p1"))

	refs, total, err := store.ListStaleRefs(ctx, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, refs, 3)
	for _, r := range refs {
		assert.True(t, r.IsStale)
	}
}

func TestShippingStore_Ref_ListByProfile_Paginated(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateProfile(ctx, makeProfile("p1", "Profile 1", true)))

	for i := 0; i < 5; i++ {
		ref := &models.ListingShippingRef{
			ListingSlug:       "product-" + string(rune('a'+i)),
			ShippingProfileID: "p1",
			SnapshotVersion:   1,
			IsStale:           false,
		}
		require.NoError(t, store.UpsertListingRef(ctx, ref))
	}

	refs, total, err := store.ListRefsByProfile(ctx, "p1", 1, 3)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, refs, 3)

	refs2, _, err := store.ListRefsByProfile(ctx, "p1", 2, 3)
	require.NoError(t, err)
	assert.Len(t, refs2, 2)
}

func TestShippingStore_Ref_CountListingsByProfile(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateProfile(ctx, makeProfile("p1", "Profile 1", true)))
	require.NoError(t, store.CreateProfile(ctx, makeProfile("p2", "Profile 2", false)))

	for _, slug := range []string{"prod-1", "prod-2"} {
		ref := &models.ListingShippingRef{
			ListingSlug:       slug,
			ShippingProfileID: "p1",
			SnapshotVersion:   1,
		}
		require.NoError(t, store.UpsertListingRef(ctx, ref))
	}
	ref := &models.ListingShippingRef{
		ListingSlug:       "prod-3",
		ShippingProfileID: "p2",
		SnapshotVersion:   1,
	}
	require.NoError(t, store.UpsertListingRef(ctx, ref))

	count, err := store.CountListingsByProfile(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	count2, err := store.CountListingsByProfile(ctx, "p2")
	require.NoError(t, err)
	assert.Equal(t, 1, count2)
}

func TestShippingStore_Ref_MigrateRefs(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateProfile(ctx, makeProfile("p1", "Source", false)))
	require.NoError(t, store.CreateProfile(ctx, makeProfile("p2", "Target", true)))

	for _, slug := range []string{"prod-1", "prod-2", "prod-3"} {
		ref := &models.ListingShippingRef{
			ListingSlug:       slug,
			ShippingProfileID: "p1",
			SnapshotVersion:   1,
			IsStale:           false,
		}
		require.NoError(t, store.UpsertListingRef(ctx, ref))
	}

	migrated, err := store.MigrateRefs(ctx, "p1", "p2")
	require.NoError(t, err)
	assert.Equal(t, 3, migrated)

	countP1, err := store.CountListingsByProfile(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, 0, countP1)

	countP2, err := store.CountListingsByProfile(ctx, "p2")
	require.NoError(t, err)
	assert.Equal(t, 3, countP2)

	stale, total, err := store.ListStaleRefs(ctx, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	for _, r := range stale {
		assert.True(t, r.IsStale)
		assert.Equal(t, "p2", r.ShippingProfileID)
	}
}

func TestShippingStore_Ref_ListStaleRefs_Paginated(t *testing.T) {
	store := newTestShippingStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateProfile(ctx, makeProfile("p1", "Profile 1", true)))

	for i := 0; i < 5; i++ {
		ref := &models.ListingShippingRef{
			ListingSlug:       "product-" + string(rune('a'+i)),
			ShippingProfileID: "p1",
			SnapshotVersion:   1,
			IsStale:           true,
		}
		require.NoError(t, store.UpsertListingRef(ctx, ref))
	}

	refs, total, err := store.ListStaleRefs(ctx, 1, 3)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, refs, 3)
}

// --- JSON serialization ---

func TestShippingProfile_LocationGroups_JSON(t *testing.T) {
	p := &models.ShippingProfileEntity{
		ID:   "p1",
		Name: "Test",
	}

	groups := []*models.LocationGroup{
		{
			ID:          "lg-1",
			LocationIDs: []string{},
			Zones: []*models.ShippingZone{
				{
					ID:      "z-1",
					Name:    "Global",
					Regions: []string{"ALL"},
					Rates: []*models.ShippingRate{
						{
							ID:                "r-1",
							Name:              "Standard",
							Price:             "500",
							Currency:          "USD",
							EstimatedDelivery: "7-14 days",
							FreeShippingThreshold: &models.FreeShippingThreshold{
								Enabled:   true,
								MinAmount: "5000",
							},
						},
					},
				},
			},
		},
	}

	require.NoError(t, p.SetLocationGroups(groups))
	assert.NotEmpty(t, p.LocationGroupsJSON)

	got, err := p.GetLocationGroups()
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "lg-1", got[0].ID)
	require.Len(t, got[0].Zones, 1)
	require.Len(t, got[0].Zones[0].Rates, 1)
	assert.Equal(t, "Standard", got[0].Zones[0].Rates[0].Name)
	assert.True(t, got[0].Zones[0].Rates[0].FreeShippingThreshold.Enabled)
}

func TestShippingProfile_LocationGroups_EmptyJSON(t *testing.T) {
	p := &models.ShippingProfileEntity{LocationGroupsJSON: ""}
	groups, err := p.GetLocationGroups()
	require.NoError(t, err)
	assert.Nil(t, groups)

	require.NoError(t, p.SetLocationGroups(nil))
	assert.Equal(t, "[]", p.LocationGroupsJSON)
}

package dbstore_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mobazha/mobazha/internal/core"
	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/database/dbstore"
	pkgdb "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMigrationDB(t *testing.T) (pkgdb.Database, *database.GormShippingStore) {
	t.Helper()
	tmpDir := t.TempDir()
	db, err := dbstore.NewMemoryDB(tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	require.NoError(t, db.Update(func(tx pkgdb.Tx) error {
		return tx.Migrate(&models.UserPreferences{})
	}))
	require.NoError(t, database.MigrateShippingModels(db))

	store := database.NewGormShippingStore(db)
	return db, store
}

func seedPreferences(t *testing.T, db pkgdb.Database, prefs *models.UserPreferences) {
	t.Helper()
	require.NoError(t, db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(prefs)
	}))
}

func TestMigrateShipping_FromShippingProfiles(t *testing.T) {
	db, store := setupMigrationDB(t)
	ctx := context.Background()

	profiles := []*models.ShippingProfile{
		{
			ProfileID: "prof-1",
			Name:      "Domestic",
			IsDefault: true,
			LocationGroups: []*models.LocationGroup{
				{
					ID: "lg-1",
					Zones: []*models.ShippingZone{
						{
							ID:      "z-1",
							Name:    "US",
							Regions: []string{"US"},
							Rates: []*models.ShippingRate{
								{ID: "r-1", Name: "Standard", Price: "500", Currency: "USD"},
							},
						},
					},
				},
			},
		},
		{
			ProfileID: "prof-2",
			Name:      "International",
			IsDefault: false,
			LocationGroups: []*models.LocationGroup{
				{
					ID: "lg-2",
					Zones: []*models.ShippingZone{
						{ID: "z-2", Name: "Global", Regions: []string{"ALL"}},
					},
				},
			},
		},
	}
	locations := []*models.ShippingLocation{
		{ID: "loc-1", Name: "Warehouse A", Address: "123 Main St", IsDefault: true},
		{ID: "loc-2", Name: "Warehouse B", Address: "456 Oak Ave", IsDefault: false},
	}

	profilesJSON, _ := json.Marshal(profiles)
	locationsJSON, _ := json.Marshal(locations)

	seedPreferences(t, db, &models.UserPreferences{
		ShippingProfiles:  profilesJSON,
		ShippingLocations: locationsJSON,
	})

	require.NoError(t, core.MigrateShippingFromPreferences(db, store))

	got, err := store.ListProfiles(ctx)
	require.NoError(t, err)
	assert.Len(t, got, 2)

	p1, err := store.GetProfile(ctx, "prof-1")
	require.NoError(t, err)
	assert.Equal(t, "Domestic", p1.Name)
	assert.True(t, p1.IsDefault)
	groups, err := p1.GetLocationGroups()
	require.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, "lg-1", groups[0].ID)

	p2, err := store.GetProfile(ctx, "prof-2")
	require.NoError(t, err)
	assert.Equal(t, "International", p2.Name)
	assert.False(t, p2.IsDefault)

	locs, err := store.ListLocations(ctx)
	require.NoError(t, err)
	assert.Len(t, locs, 2)

	l1, err := store.GetLocation(ctx, "loc-1")
	require.NoError(t, err)
	assert.Equal(t, "Warehouse A", l1.Name)
	assert.True(t, l1.IsDefault)
}

func TestMigrateShipping_Idempotent(t *testing.T) {
	db, store := setupMigrationDB(t)
	ctx := context.Background()

	profiles := []*models.ShippingProfile{
		{ProfileID: "prof-1", Name: "Only", IsDefault: true},
	}
	profilesJSON, _ := json.Marshal(profiles)
	seedPreferences(t, db, &models.UserPreferences{
		ShippingProfiles: profilesJSON,
	})

	require.NoError(t, core.MigrateShippingFromPreferences(db, store))
	require.NoError(t, core.MigrateShippingFromPreferences(db, store))

	got, err := store.ListProfiles(ctx)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestMigrateShipping_EmptyPreferences(t *testing.T) {
	db, store := setupMigrationDB(t)
	ctx := context.Background()

	seedPreferences(t, db, &models.UserPreferences{})

	require.NoError(t, core.MigrateShippingFromPreferences(db, store))

	got, err := store.ListProfiles(ctx)
	require.NoError(t, err)
	assert.Len(t, got, 0)
}

func TestMigrateShipping_NoPreferencesRow(t *testing.T) {
	db, store := setupMigrationDB(t)
	ctx := context.Background()

	require.NoError(t, core.MigrateShippingFromPreferences(db, store))

	got, err := store.ListProfiles(ctx)
	require.NoError(t, err)
	assert.Len(t, got, 0)
}

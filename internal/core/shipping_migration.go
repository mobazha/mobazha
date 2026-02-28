package core

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// MigrateShippingFromPreferences migrates shipping data from legacy
// UserPreferences JSON blobs (ShippingProfiles, ShippingLocations, ShippingOptions)
// into the dedicated shipping_profiles / shipping_locations tables.
//
// Idempotent: skips if the new tables already contain profiles.
// Called once during initShippingSubsystem at node startup.
func MigrateShippingFromPreferences(db pkgdb.Database, store contracts.ShippingStore) error {
	ctx := context.Background()

	existing, err := store.ListProfiles(ctx)
	if err != nil {
		return fmt.Errorf("check existing profiles: %w", err)
	}
	if len(existing) > 0 {
		return nil
	}

	var prefs models.UserPreferences
	err = db.View(func(tx pkgdb.Tx) error {
		return tx.Read().First(&prefs).Error
	})
	if err != nil {
		return nil
	}

	if prefs.NeedsMigrationFromLegacy() {
		if err := prefs.MigrateFromLegacyShippingOptions(
			uuid.New().String(), "Default Shipping",
			uuid.New().String(), "Default Location",
		); err != nil {
			return fmt.Errorf("legacy option conversion: %w", err)
		}
	}

	profiles, err := prefs.GetShippingProfiles()
	if err != nil {
		return fmt.Errorf("parse shipping profiles: %w", err)
	}
	if len(profiles) == 0 {
		return nil
	}

	for _, p := range profiles {
		entity := &models.ShippingProfileEntity{
			ID:        p.ProfileID,
			Name:      p.Name,
			IsDefault: p.IsDefault,
			Version:   1,
		}
		if err := entity.SetLocationGroups(p.LocationGroups); err != nil {
			return fmt.Errorf("serialize location groups for profile %s: %w", p.ProfileID, err)
		}
		if err := store.CreateProfile(ctx, entity); err != nil {
			return fmt.Errorf("create profile %s: %w", p.ProfileID, err)
		}
	}

	locations, err := prefs.GetShippingLocations()
	if err != nil {
		return fmt.Errorf("parse shipping locations: %w", err)
	}
	for _, loc := range locations {
		entity := &models.ShippingLocationEntity{
			ID:        loc.ID,
			Name:      loc.Name,
			Address:   loc.Address,
			IsDefault: loc.IsDefault,
		}
		if err := store.CreateLocation(ctx, entity); err != nil {
			return fmt.Errorf("create location %s: %w", loc.ID, err)
		}
	}

	return nil
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mobazha/mobazha/internal/core"
	"github.com/mobazha/mobazha/pkg/models"
)

// devnetShippingProfileID is the profile ID that factory.NewPhysicalListing
// embeds into the listings devnet publishes (pkg/models/factory/listings.go).
// ListingAppService resolves that exact ID before saving a listing, so a devnet
// vendor must own a row with this ID, otherwise SaveListing fails with
// "shipping profile not found" and the devnet command aborts during startup.
const devnetShippingProfileID = "factory-default-profile"

// ensureDevnetShippingProfile seeds the shipping profile that devnet listings
// reference. It is idempotent: when the profile already exists (for example on a
// second "mobazha devnet" run against the same data directory) nothing changes.
//
// This mirrors internal/core seedMockShippingProfile, which exists for the same
// reason in the test harness.
func ensureDevnetShippingProfile(node *core.MobazhaNode, role string) error {
	svc := node.Shipping()
	if svc == nil {
		return fmt.Errorf("%s: shipping subsystem unavailable", role)
	}

	ctx := context.Background()
	if existing, err := svc.GetProfile(ctx, devnetShippingProfileID); err == nil && existing != nil {
		return nil
	}

	groups := []*models.LocationGroup{{
		ID: "default-lg",
		Zones: []*models.ShippingZone{{
			ID:      "zone-all",
			Name:    "Worldwide",
			Regions: []string{"ALL"},
			Rates: []*models.ShippingRate{{
				ID:                "rate-standard",
				Name:              "Standard",
				Price:             "20",
				Currency:          "USD",
				EstimatedDelivery: "3 days",
			}},
		}},
	}}
	groupsJSON, err := json.Marshal(groups)
	if err != nil {
		return fmt.Errorf("%s: marshal shipping location groups: %w", role, err)
	}

	if err := svc.CreateProfile(ctx, &models.ShippingProfileEntity{
		ID:                 devnetShippingProfileID,
		Name:               "Default Shipping",
		IsDefault:          true,
		LocationGroupsJSON: string(groupsJSON),
	}); err != nil {
		return fmt.Errorf("%s: create default shipping profile: %w", role, err)
	}
	return nil
}

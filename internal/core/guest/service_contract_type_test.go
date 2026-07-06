package guest

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	pkgconfig "github.com/mobazha/mobazha/pkg/config"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubGuestListings struct {
	bySlug map[string]*pb.SignedListing
}

func (s *stubGuestListings) GetMyListings() (models.ListingIndex, error) {
	index := make(models.ListingIndex, 0, len(s.bySlug))
	for slug := range s.bySlug {
		index = append(index, models.ListingMetadata{Slug: slug})
	}
	return index, nil
}

func (s *stubGuestListings) GetMyListingBySlug(slug string) (*pb.SignedListing, error) {
	sl, ok := s.bySlug[slug]
	if !ok {
		return nil, fmt.Errorf("listing %q not found", slug)
	}
	return sl, nil
}

type alwaysEnabledResolver struct{}

func (alwaysEnabledResolver) IsEnabled(context.Context, string) bool { return true }

func (alwaysEnabledResolver) Evaluate(context.Context, string) pkgconfig.Evaluation {
	return pkgconfig.Evaluation{Enabled: true}
}

func (alwaysEnabledResolver) List(context.Context) []pkgconfig.EffectiveFeature {
	return nil
}

func guestListing(slug string, ct pb.Listing_Metadata_ContractType) *pb.SignedListing {
	return &pb.SignedListing{
		Listing: &pb.Listing{
			Slug: slug,
			Metadata: &pb.Listing_Metadata{
				ContractType: ct,
				PricingCurrency: &pb.Currency{
					Code: "USD",
				},
			},
			Item: &pb.Listing_Item{
				Title: slug,
				Price: "1000",
			},
		},
	}
}

func physicalGuestListingWithShipping() *pb.SignedListing {
	listing := guestListing("physical-item", pb.Listing_Metadata_PHYSICAL_GOOD)
	listing.Listing.Item.Grams = 500
	listing.Listing.ShippingProfile = &pb.ShippingProfile{
		LocationGroups: []*pb.LocationGroup{{
			Zones: []*pb.ShippingZone{{
				Id:      "us-zone",
				Name:    "United States",
				Regions: []string{"US"},
				Rates: []*pb.ShippingRate{{
					Id:       "weight-rate",
					Name:     "Weight rate",
					Price:    "250",
					Currency: "USD",
					Condition: &pb.ShippingRate_RateCondition{
						Type:     pb.ShippingRate_RateCondition_WEIGHT,
						MinValue: 500,
						MaxValue: 1000,
					},
					FreeShippingThreshold: &pb.ShippingRate_FreeShippingThreshold{
						Enabled:   true,
						MinAmount: "2000",
					},
				}},
			}},
		}},
	}
	return listing
}

func TestCreateGuestOrder_RejectsMixedContractTypes(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	svc.resolver = alwaysEnabledResolver{}
	svc.listings = &stubGuestListings{
		bySlug: map[string]*pb.SignedListing{
			"digital-item":  guestListing("digital-item", pb.Listing_Metadata_DIGITAL_GOOD),
			"physical-item": guestListing("physical-item", pb.Listing_Metadata_PHYSICAL_GOOD),
		},
	}

	_, err := svc.CreateGuestOrder(context.Background(), contracts.CreateGuestOrderRequest{
		PaymentCoin: "LTC",
		Items: []contracts.GuestOrderItemRequest{
			{ListingSlug: "digital-item", Quantity: 1},
			{ListingSlug: "physical-item", Quantity: 1},
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, contracts.ErrInvalidGuestRequest))
	assert.Contains(t, err.Error(), "mixed contract types")
}

func TestCreateGuestOrder_PhysicalRequiresShippingAddress(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	svc.resolver = alwaysEnabledResolver{}
	svc.listings = &stubGuestListings{bySlug: map[string]*pb.SignedListing{
		"physical-item": guestListing("physical-item", pb.Listing_Metadata_PHYSICAL_GOOD),
	}}

	_, err := svc.CreateGuestOrder(context.Background(), contracts.CreateGuestOrderRequest{
		PaymentCoin: "LTC",
		Items:       []contracts.GuestOrderItemRequest{{ListingSlug: "physical-item", Quantity: 1}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrInvalidGuestRequest)
	assert.Contains(t, err.Error(), "physical orders require a shipping address")
}

func TestCreateGuestOrder_RejectsPlaintextWhenAddressEncryptionRequired(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	db := svc.db.(*testDatabase)
	require.NoError(t, db.gormDB.AutoMigrate(&models.GuestCheckoutConfig{}))
	svc.resolver = alwaysEnabledResolver{}
	svc.listings = &stubGuestListings{bySlug: map[string]*pb.SignedListing{
		"physical-item": guestListing("physical-item", pb.Listing_Metadata_PHYSICAL_GOOD),
	}}
	require.NoError(t, svc.SaveGuestCheckoutConfig(context.Background(), &models.GuestCheckoutConfig{
		Enabled:                   true,
		AcceptedCoins:             "LTC",
		AddressEncryptionRequired: true,
		PGPPublicKey:              "configured",
		PGPKeyFingerprint:         "AABBCCDD",
	}))

	_, err := svc.CreateGuestOrder(context.Background(), contracts.CreateGuestOrderRequest{
		PaymentCoin:     "LTC",
		ShippingAddress: map[string]string{"country": "US"},
		Items:           []contracts.GuestOrderItemRequest{{ListingSlug: "physical-item", Quantity: 1}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrInvalidGuestRequest)
	assert.Contains(t, err.Error(), "must be PGP-encrypted")
}

func TestResolveShippingCost_EnforcesCountryConditionAndFreeThreshold(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	svc.listings = &stubGuestListings{bySlug: map[string]*pb.SignedListing{
		"physical-item": physicalGuestListingWithShipping(),
	}}
	resolved := &resolvedItem{PriceCurrencyCode: "USD", UnitWeightGrams: 500}
	item := contracts.GuestOrderItemRequest{
		ListingSlug:     "physical-item",
		Quantity:        2,
		ShippingOption:  "us-zone",
		ShippingService: "weight-rate",
	}

	fee, err := svc.resolveShippingCost(item, "US", big.NewInt(2000), resolved)
	require.NoError(t, err)
	require.Zero(t, fee.Sign(), "free-shipping threshold should override the base rate")

	_, err = svc.resolveShippingCost(item, "CA", big.NewInt(1000), resolved)
	require.ErrorContains(t, err, "not available for country CA")

	item.Quantity = 3
	_, err = svc.resolveShippingCost(item, "US", big.NewInt(1000), resolved)
	require.ErrorContains(t, err, "conditions are not satisfied")
}

func TestGuestCheckoutConfig_FieldScopedSavesPreserveIndependentSettings(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	db := svc.db.(*testDatabase)
	require.NoError(t, db.gormDB.AutoMigrate(&models.GuestCheckoutConfig{}))
	require.NoError(t, svc.SaveGuestCheckoutConfig(t.Context(), &models.GuestCheckoutConfig{
		Enabled:                   true,
		AcceptedCoins:             "XMR",
		PGPPublicKey:              "old-key",
		PGPKeyFingerprint:         "OLD",
		PGPKeyVersion:             1,
		PGPEncryptedPrivateKey:    "old-backup",
		AddressEncryptionRequired: true,
	}))

	// Simulate the settings form saving a snapshot that does not contain the
	// concurrently managed encryption fields.
	require.NoError(t, svc.SaveGuestCheckoutBusinessConfig(t.Context(), &models.GuestCheckoutConfig{
		Enabled:           false,
		AcceptedCoins:     "",
		PaymentTimeout:    30,
		MaxOrderAmount:    "1000",
		PGPPublicKey:      "stale-key",
		PGPKeyFingerprint: "STALE",
	}))
	cfg, err := svc.GetGuestCheckoutConfig(t.Context())
	require.NoError(t, err)
	require.Equal(t, "old-key", cfg.PGPPublicKey)
	require.Equal(t, "OLD", cfg.PGPKeyFingerprint)
	require.True(t, cfg.AddressEncryptionRequired)

	// Encryption updates must likewise preserve the latest business fields.
	cfg.PGPPublicKey = "new-key"
	cfg.PGPKeyFingerprint = "NEW"
	cfg.PGPKeyVersion = 2
	cfg.PGPEncryptedPrivateKey = "new-backup"
	require.NoError(t, svc.SaveGuestCheckoutEncryptionConfig(t.Context(), cfg))
	updated, err := svc.GetGuestCheckoutConfig(t.Context())
	require.NoError(t, err)
	require.False(t, updated.Enabled)
	require.Equal(t, 30, updated.PaymentTimeout)
	require.Equal(t, "1000", updated.MaxOrderAmount)
	require.Equal(t, "new-key", updated.PGPPublicKey)
}

package guest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubGuestListings struct {
	bySlug map[string]*pb.SignedListing
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

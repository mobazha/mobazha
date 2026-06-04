package checkoutsupply

import (
	"context"
	"fmt"
	"testing"

	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
)

type stubListingReader struct {
	bySlug map[string]*pb.SignedListing
}

func (s *stubListingReader) GetMyListingBySlug(slug string) (*pb.SignedListing, error) {
	sl, ok := s.bySlug[slug]
	if !ok {
		return nil, fmt.Errorf("listing %q not found", slug)
	}
	return sl, nil
}

type disabledSupplyFeatureResolver struct{}

func (disabledSupplyFeatureResolver) IsEnabled(_ context.Context, key string) bool {
	return key == pkgconfig.FeatureGuestCheckoutEnabled.Key
}

func (disabledSupplyFeatureResolver) Evaluate(_ context.Context, key string) pkgconfig.Evaluation {
	return pkgconfig.Evaluation{Enabled: key == pkgconfig.FeatureGuestCheckoutEnabled.Key}
}

func (disabledSupplyFeatureResolver) List(context.Context) []pkgconfig.EffectiveFeature {
	return nil
}

type enabledSupplyFeatureResolver struct{}

func (enabledSupplyFeatureResolver) IsEnabled(context.Context, string) bool { return true }

func (enabledSupplyFeatureResolver) Evaluate(context.Context, string) pkgconfig.Evaluation {
	return pkgconfig.Evaluation{Enabled: true}
}

func (enabledSupplyFeatureResolver) List(context.Context) []pkgconfig.EffectiveFeature {
	return nil
}

type recordingSupplyAvailability struct {
	quoteResult   *contracts.SupplyQuoteResult
	quoteRequests []contracts.SupplyQuoteRequest
}

func (r *recordingSupplyAvailability) Quote(_ context.Context, req contracts.SupplyQuoteRequest) (*contracts.SupplyQuoteResult, error) {
	r.quoteRequests = append(r.quoteRequests, req)
	if r.quoteResult != nil {
		return r.quoteResult, nil
	}
	return &contracts.SupplyQuoteResult{CanSell: true}, nil
}

func (r *recordingSupplyAvailability) ReserveOrder(context.Context, contracts.ReserveOrderSupplyRequest) (*contracts.ReserveOrderSupplyResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *recordingSupplyAvailability) CommitOrder(context.Context, string, string) error {
	return fmt.Errorf("not implemented")
}

func (r *recordingSupplyAvailability) ReleaseOrder(context.Context, string, string, string) error {
	return fmt.Errorf("not implemented")
}

var _ contracts.SupplyAvailabilityService = (*recordingSupplyAvailability)(nil)

func listingWithSku(slug, option, variant, quantity, price string) *pb.SignedListing {
	return &pb.SignedListing{
		Listing: &pb.Listing{
			Slug: slug,
			Metadata: &pb.Listing_Metadata{
				ContractType: pb.Listing_Metadata_PHYSICAL_GOOD,
				PricingCurrency: &pb.Currency{
					Code: "USD",
				},
			},
			Item: &pb.Listing_Item{
				Title: slug,
				Price: "1000",
				Skus: []*pb.Listing_Item_Sku{{
					Selections: []*pb.Listing_Item_Sku_Selection{{
						Option:  option,
						Variant: variant,
					}},
					Quantity: quantity,
					Price:    price,
				}},
			},
		},
	}
}

func TestQuote_RequiresItems(t *testing.T) {
	svc := NewCheckoutSupplyQuoteService(CheckoutSupplyQuoteServiceConfig{
		Listings: &stubListingReader{},
	})
	_, err := svc.Quote(context.Background(), models.OrderTypeStandard, "checkout_quote", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one item is required")
}

func TestQuote_ReturnsUnknownWhenFeatureDisabled(t *testing.T) {
	svc := NewCheckoutSupplyQuoteService(CheckoutSupplyQuoteServiceConfig{
		Resolver: disabledSupplyFeatureResolver{},
		Listings: &stubListingReader{
			bySlug: map[string]*pb.SignedListing{
				"camera": listingWithSku("camera", "Color", "Red", "3", "1200"),
			},
		},
	})

	resp, err := svc.Quote(context.Background(), models.OrderTypeGuest, "guest_quote", []contracts.CheckoutSupplyItemRequest{{
		ListingSlug: "camera",
		Quantity:    1,
		Options:     []map[string]string{{"Color": "Red"}},
	}})
	require.NoError(t, err)
	require.True(t, resp.CanSell)
	require.Equal(t, "supply_availability_disabled", resp.Reason)
	require.Len(t, resp.Items, 1)
	require.Equal(t, contracts.SupplyAvailabilityUnknown, resp.Items[0].Status)
}

func TestQuote_ReturnsBuyerManagedEscrowAvailability(t *testing.T) {
	recorder := &recordingSupplyAvailability{
		quoteResult: &contracts.SupplyQuoteResult{
			CanSell: true,
			Results: []contracts.AvailabilityResult{{
				LineID:            "checkout_quote:0",
				SupplyKind:        contracts.SupplyKindSkuQuantity,
				Status:            contracts.SupplyAvailabilityLowStock,
				Available:         true,
				AvailableQuantity: 2,
				ProviderID:        "internal-provider",
				ProviderRef:       "private-ref",
			}},
		},
	}
	svc := NewCheckoutSupplyQuoteService(CheckoutSupplyQuoteServiceConfig{
		Resolver:           enabledSupplyFeatureResolver{},
		SupplyAvailability: recorder,
		Listings: &stubListingReader{
			bySlug: map[string]*pb.SignedListing{
				"camera": listingWithSku("camera", "Color", "Red", "3", "1200"),
			},
		},
	})

	resp, err := svc.Quote(context.Background(), models.OrderTypeStandard, "checkout_quote", []contracts.CheckoutSupplyItemRequest{{
		ListingSlug: "camera",
		Quantity:    1,
		Options:     []map[string]string{{"Color": "Red"}},
	}})
	require.NoError(t, err)
	require.True(t, resp.CanSell)
	require.Len(t, recorder.quoteRequests, 1)
	require.Equal(t, models.OrderTypeStandard, recorder.quoteRequests[0].OrderType)
	require.Len(t, recorder.quoteRequests[0].Lines, 1)
	require.Equal(t, contracts.SupplyKindSkuQuantity, recorder.quoteRequests[0].Lines[0].SupplyKind)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "camera", resp.Items[0].ListingSlug)
	require.Equal(t, contracts.SupplyAvailabilityLowStock, resp.Items[0].Status)
	require.Equal(t, int64(2), resp.Items[0].AvailableQuantity)
	require.NotContains(t, fmt.Sprintf("%+v", resp), "internal-provider")
}

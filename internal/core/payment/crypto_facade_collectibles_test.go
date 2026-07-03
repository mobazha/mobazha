package payment

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	porderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
)

func TestIsManagedCollectibleFirstSale(t *testing.T) {
	valid := managedCollectibleFirstSaleOrderOpen()
	if !models.IsManagedCollectibleFirstSale(valid) {
		t.Fatal("authoritative source-custody first sale was rejected")
	}
	if !testOrderOpenContainsRWA(valid) {
		t.Fatal("RWA listing was not detected")
	}

	tests := []struct {
		name   string
		mutate func(*porderpb.OrderOpen)
	}{
		{name: "legacy evm", mutate: func(open *porderpb.OrderOpen) {
			open.Listings[0].Listing.Item.Blockchain = "ETH"
		}},
		{name: "pre-minted", mutate: func(open *porderpb.OrderOpen) {
			open.Listings[0].Listing.Item.TokenAddress = "mint-address"
		}},
		{name: "wrong token standard", mutate: func(open *porderpb.OrderOpen) {
			open.Listings[0].Listing.Item.TokenStandard = "erc721"
		}},
		{name: "missing hub slot", mutate: func(open *porderpb.OrderOpen) {
			open.Items[0].OptionalFeatures = removeCollectibleFeature(open.Items[0].OptionalFeatures, models.CollectibleFeatureHubSlotID)
		}},
		{name: "missing cert", mutate: func(open *porderpb.OrderOpen) {
			open.Items[0].OptionalFeatures = removeCollectibleFeature(open.Items[0].OptionalFeatures, models.CollectibleFeatureCertNumber)
		}},
		{name: "missing buyer wallet", mutate: func(open *porderpb.OrderOpen) {
			open.Items[0].OptionalFeatures = removeCollectibleFeature(open.Items[0].OptionalFeatures, models.CollectibleFeatureHolderWallet)
		}},
		{name: "multiple listings", mutate: func(open *porderpb.OrderOpen) {
			open.Listings = append(open.Listings, open.Listings[0])
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			open := managedCollectibleFirstSaleOrderOpen()
			test.mutate(open)
			if models.IsManagedCollectibleFirstSale(open) {
				t.Fatal("unsupported RWA shape was accepted")
			}
		})
	}
}

func TestOrderExtensionProvisioningPolicy_ReservesCollectibleBeforeCryptoProvisioning(t *testing.T) {
	open := managedCollectibleFirstSaleOrderOpen()
	open.Listings[0].Listing.VendorID = &porderpb.ID{PeerID: "seller-peer"}
	wantErr := errors.New("source deposit unavailable")
	var got extensions.ReservationRequest
	policy := NewOrderExtensionProvisioningPolicy(resolveCollectibleFirstSaleExtension, func(_ context.Context, request extensions.ReservationRequest) (extensions.Reservation, error) {
		got = request
		return extensions.Reservation{}, wantErr
	})
	expiresAt := time.Now().Add(time.Hour).UTC()

	err := policy.AuthorizeSessionProvisioning(context.Background(), SessionProvisioningPolicyInput{
		OrderID: "order-1", PaymentCoin: "crypto:eip155:1:native", ExpiresAt: expiresAt, OrderOpen: open,
	})
	if !errors.Is(err, ErrOrderExtensionReservation) {
		t.Fatalf("reservation error = %v, want ErrOrderExtensionReservation", err)
	}
	metadata, ok := models.CollectibleOrderMetadataFromExtension(got.Extension)
	if !ok || got.OrderID != "order-1" || metadata.HubSlotID != "source_550e8400-e29b-41d4-a716-446655440000" || metadata.CertNumber != "PSA-123" || metadata.SellerPeerID != "seller-peer" || got.PaymentCoin != "crypto:eip155:1:native" || !got.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("reservation request = %+v, metadata = %+v", got, metadata)
	}
}

func TestOrderExtensionProvisioningPolicy_RequiresReservationPort(t *testing.T) {
	err := NewOrderExtensionProvisioningPolicy(resolveCollectibleFirstSaleExtension, nil).AuthorizeSessionProvisioning(context.Background(), SessionProvisioningPolicyInput{
		OrderID: "order-1", PaymentCoin: "crypto:eip155:1:native", ExpiresAt: time.Now().Add(time.Hour), OrderOpen: managedCollectibleFirstSaleOrderOpen(),
	})
	if !errors.Is(err, ErrOrderExtensionReservation) {
		t.Fatalf("reservation error = %v, want ErrOrderExtensionReservation", err)
	}
}

func TestOrderExtensionProvisioningPolicy_RequiresDurableReservationResult(t *testing.T) {
	policy := NewOrderExtensionProvisioningPolicy(resolveCollectibleFirstSaleExtension, func(context.Context, extensions.ReservationRequest) (extensions.Reservation, error) {
		return extensions.Reservation{}, nil
	})
	err := policy.AuthorizeSessionProvisioning(context.Background(), SessionProvisioningPolicyInput{
		OrderID: "order-invalid-reservation", PaymentCoin: "crypto:eip155:1:native", ExpiresAt: time.Now().Add(time.Hour), OrderOpen: managedCollectibleFirstSaleOrderOpen(),
	})
	if !errors.Is(err, ErrOrderExtensionReservation) || err == nil {
		t.Fatalf("reservation error = %v, want invalid reservation result", err)
	}
}

func TestOrderExtensionProvisioningPolicy_AcceptsDurableReservationResult(t *testing.T) {
	recorded := false
	policy := NewOrderExtensionProvisioningPolicy(resolveCollectibleFirstSaleExtension, func(context.Context, extensions.ReservationRequest) (extensions.Reservation, error) {
		return extensions.Reservation{ID: "reservation-1", Version: 1, Status: "reserved"}, nil
	}, func(request extensions.ReservationRequest, reservation extensions.Reservation) error {
		recorded = true
		require.Equal(t, "reservation-1", reservation.ID)
		require.NotEmpty(t, request.IdempotencyKey)
		return nil
	})
	err := policy.AuthorizeSessionProvisioning(context.Background(), SessionProvisioningPolicyInput{
		OrderID: "order-valid-reservation", PaymentCoin: "crypto:eip155:1:native", ExpiresAt: time.Now().Add(time.Hour), OrderOpen: managedCollectibleFirstSaleOrderOpen(),
	})
	if err != nil {
		t.Fatalf("reservation error = %v", err)
	}
	require.True(t, recorded)
}

func TestOrderExtensionProvisioningPolicy_RejectsCollectibleFiatBeforeReservation(t *testing.T) {
	called := false
	policy := NewOrderExtensionProvisioningPolicy(resolveCollectibleFirstSaleExtension, func(context.Context, extensions.ReservationRequest) (extensions.Reservation, error) {
		called = true
		return extensions.Reservation{}, nil
	})
	err := policy.AuthorizeSessionProvisioning(context.Background(), SessionProvisioningPolicyInput{
		OrderID: "order-fiat", PaymentCoin: "fiat:stripe:USD", OrderOpen: managedCollectibleFirstSaleOrderOpen(),
	})
	if !errors.Is(err, ErrRWAPaymentSessionUnsupported) {
		t.Fatalf("fiat policy error = %v, want ErrRWAPaymentSessionUnsupported", err)
	}
	if called {
		t.Fatal("unsupported fiat rail must be rejected before reserving the source deposit")
	}
}

func resolveCollectibleFirstSaleExtension(input SessionProvisioningPolicyInput) (extensions.OrderExtension, bool, error) {
	if !testOrderOpenContainsRWA(input.OrderOpen) {
		return extensions.OrderExtension{}, false, nil
	}
	if !models.IsManagedCollectibleFirstSale(input.OrderOpen) {
		return extensions.OrderExtension{}, false, ErrRWAPaymentSessionUnsupported
	}
	if strings.HasPrefix(input.PaymentCoin, "fiat:") || !strings.HasPrefix(input.PaymentCoin, "crypto:") {
		return extensions.OrderExtension{}, false, ErrRWAPaymentSessionUnsupported
	}
	extension, ok, err := models.CollectibleOrderExtensionFromOrderOpen(input.OrderID, input.OrderOpen)
	return extension, ok, err
}

func testOrderOpenContainsRWA(orderOpen *porderpb.OrderOpen) bool {
	if orderOpen == nil {
		return false
	}
	for _, signed := range orderOpen.GetListings() {
		if signed.GetListing().GetMetadata().GetContractType() == porderpb.Listing_Metadata_RWA_TOKEN {
			return true
		}
	}
	return false
}

func managedCollectibleFirstSaleOrderOpen() *porderpb.OrderOpen {
	features := []string{
		models.CollectibleOptionalFeature(models.CollectibleFeatureFulfillment, models.CollectibleFulfillmentNFT),
		models.CollectibleOptionalFeature(models.CollectibleFeatureHubSlotID, "source_550e8400-e29b-41d4-a716-446655440000"),
		models.CollectibleOptionalFeature(models.CollectibleFeatureCertNumber, "PSA-123"),
		models.CollectibleOptionalFeature(models.CollectibleFeatureHolderWallet, "11111111111111111111111111111111"),
	}
	return &porderpb.OrderOpen{
		Listings: []*porderpb.SignedListing{{Listing: &porderpb.Listing{
			Metadata: &porderpb.Listing_Metadata{ContractType: porderpb.Listing_Metadata_RWA_TOKEN},
			Item: &porderpb.Listing_Item{
				Blockchain:    "SOL",
				TokenStandard: "metaplex_pnft",
			},
		}}},
		Items: []*porderpb.OrderOpen_Item{{OptionalFeatures: features}},
	}
}

func removeCollectibleFeature(features []string, key string) []string {
	prefix := models.CollectibleOptionalFeature(key, "")
	if prefix == "" {
		prefix = "collectibles." + key + "="
	}
	out := make([]string, 0, len(features))
	for _, feature := range features {
		if len(feature) >= len(prefix) && feature[:len(prefix)] == prefix {
			continue
		}
		out = append(out, feature)
	}
	return out
}

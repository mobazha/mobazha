package payment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
	porderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

func TestIsManagedCollectibleFirstSale(t *testing.T) {
	valid := managedCollectibleFirstSaleOrderOpen()
	if !models.IsManagedCollectibleFirstSale(valid) {
		t.Fatal("authoritative source-custody first sale was rejected")
	}
	if !orderOpenContainsRWA(valid) {
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

func TestCollectibleFirstSaleProvisioningPolicyAuthorizesCrypto(t *testing.T) {
	open := managedCollectibleFirstSaleOrderOpen()
	open.Listings[0].Listing.VendorID = &porderpb.ID{PeerID: "seller-peer"}
	wantErr := errors.New("source deposit unavailable")
	var got CollectibleFirstSaleAuthorizationSignal
	policy := NewCollectibleFirstSaleProvisioningPolicy(func(_ context.Context, signal CollectibleFirstSaleAuthorizationSignal) error {
		got = signal
		return wantErr
	})
	expiresAt := time.Now().Add(time.Hour).UTC()

	err := policy.AuthorizeSessionProvisioning(context.Background(), SessionProvisioningPolicyInput{
		OrderID: "order-1", PaymentCoin: "crypto:eip155:1:native", ExpiresAt: expiresAt, OrderOpen: open,
	})
	if !errors.Is(err, ErrCollectibleFirstSalePreflight) {
		t.Fatalf("preflight error = %v, want ErrCollectibleFirstSalePreflight", err)
	}
	if got.OrderID != "order-1" || got.HubSlotID != "source_550e8400-e29b-41d4-a716-446655440000" || got.CertNumber != "PSA-123" || got.SellerPeerID != "seller-peer" || got.PaymentCoin != "crypto:eip155:1:native" || !got.ReservationExpiresAt.Equal(expiresAt) {
		t.Fatalf("preflight signal = %+v", got)
	}
}

func TestCollectibleFirstSaleProvisioningPolicyRequiresHook(t *testing.T) {
	err := NewCollectibleFirstSaleProvisioningPolicy(nil).AuthorizeSessionProvisioning(context.Background(), SessionProvisioningPolicyInput{
		OrderID: "order-1", PaymentCoin: "crypto:eip155:1:native", OrderOpen: managedCollectibleFirstSaleOrderOpen(),
	})
	if !errors.Is(err, ErrCollectibleFirstSalePreflight) {
		t.Fatalf("preflight error = %v, want ErrCollectibleFirstSalePreflight", err)
	}
}

func TestCollectibleFirstSaleProvisioningPolicyRejectsFiatBeforeAuthorization(t *testing.T) {
	called := false
	policy := NewCollectibleFirstSaleProvisioningPolicy(func(context.Context, CollectibleFirstSaleAuthorizationSignal) error {
		called = true
		return nil
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

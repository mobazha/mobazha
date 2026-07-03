package models

import (
	"testing"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

func TestPurchaseItemOptionalFeaturesWithCollectibleMetadata(t *testing.T) {
	item := PurchaseItem{
		OptionalFeatures: []string{
			"gift-wrap",
			CollectibleOptionalFeature(CollectibleFeatureHubSlotID, "slot-existing"),
		},
		Fulfillment:  CollectibleFulfillmentNFT,
		HubSlotID:    "slot-new",
		NFTMint:      "mint-1",
		CertNumber:   "cert-1",
		HolderWallet: "holder-wallet-1",
	}

	features := PurchaseItemOptionalFeaturesWithCollectibleMetadata(item)

	want := map[string]bool{
		"gift-wrap": true,
		CollectibleOptionalFeature(CollectibleFeatureHubSlotID, "slot-existing"):             true,
		CollectibleOptionalFeature(CollectibleFeatureFulfillment, CollectibleFulfillmentNFT): true,
		CollectibleOptionalFeature(CollectibleFeatureNFTMint, "mint-1"):                      true,
		CollectibleOptionalFeature(CollectibleFeatureCertNumber, "cert-1"):                   true,
		CollectibleOptionalFeature(CollectibleFeatureHolderWallet, "holder-wallet-1"):        true,
	}
	if len(features) != len(want) {
		t.Fatalf("expected %d features, got %d: %#v", len(want), len(features), features)
	}
	for _, feature := range features {
		if !want[feature] {
			t.Fatalf("unexpected feature %q in %#v", feature, features)
		}
	}

	nonCollectible := PurchaseItemOptionalFeaturesWithCollectibleMetadata(PurchaseItem{
		OptionalFeatures: []string{"delivery=instant"},
		Fulfillment:      "digital",
	})
	if len(nonCollectible) != 1 || nonCollectible[0] != "delivery=instant" {
		t.Fatalf("expected non-collectible fulfillment to be ignored, got %#v", nonCollectible)
	}
}

func TestPurchaseItemHasCollectibleMetadata(t *testing.T) {
	if !PurchaseItemHasCollectibleMetadata(PurchaseItem{Fulfillment: CollectibleFulfillmentNFT}) {
		t.Fatal("expected NFT fulfillment to be detected")
	}
	if !PurchaseItemHasCollectibleMetadata(PurchaseItem{HubSlotID: "slot-1"}) {
		t.Fatal("expected explicit hub slot metadata to be detected")
	}
	if !PurchaseItemHasCollectibleMetadata(PurchaseItem{
		OptionalFeatures: []string{CollectibleOptionalFeature(CollectibleFeatureNFTMint, "mint-1")},
	}) {
		t.Fatal("expected optional feature metadata to be detected")
	}
	if PurchaseItemHasCollectibleMetadata(PurchaseItem{OptionalFeatures: []string{"gift-wrap"}}) {
		t.Fatal("expected ordinary optional feature to be ignored")
	}
	if PurchaseItemHasCollectibleMetadata(PurchaseItem{Fulfillment: "digital"}) {
		t.Fatal("expected non-collectible fulfillment to be ignored")
	}
}

func TestCollectibleOrderMetadataFromOrderOpen(t *testing.T) {
	orderOpen := &pb.OrderOpen{
		BuyerID: &pb.ID{PeerID: "buyer-peer"},
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				Slug:     "psa-card",
				VendorID: &pb.ID{PeerID: "seller-peer"},
				Metadata: &pb.Listing_Metadata{
					ContractType: pb.Listing_Metadata_RWA_TOKEN,
				},
				Item: &pb.Listing_Item{
					TokenStandard: "pNFT",
					TokenAddress:  "mint-from-listing",
				},
			},
		}},
		Items: []*pb.OrderOpen_Item{{
			ListingHash: "listing-hash",
			OptionalFeatures: []string{
				CollectibleOptionalFeature(CollectibleFeatureHubSlotID, "slot-1"),
				CollectibleOptionalFeature(CollectibleFeatureNFTMint, "mint-from-feature"),
				CollectibleOptionalFeature(CollectibleFeatureCertNumber, "cert-1"),
				CollectibleOptionalFeature(CollectibleFeatureHolderWallet, "holder-wallet-1"),
			},
		}},
	}

	meta, ok := CollectibleOrderMetadataFromOrderOpen(orderOpen)
	if !ok {
		t.Fatal("expected collectible metadata")
	}
	if meta.Type != CollectibleMetadataTypePrimarySale {
		t.Fatalf("unexpected type %q", meta.Type)
	}
	if meta.Fulfillment != CollectibleFulfillmentNFT {
		t.Fatalf("unexpected fulfillment %q", meta.Fulfillment)
	}
	if meta.HubSlotID != "slot-1" {
		t.Fatalf("unexpected hub slot %q", meta.HubSlotID)
	}
	if meta.NFTMint != "mint-from-feature" {
		t.Fatalf("unexpected NFT mint %q", meta.NFTMint)
	}
	if meta.CertNumber != "cert-1" {
		t.Fatalf("unexpected cert number %q", meta.CertNumber)
	}
	if meta.HolderWallet != "holder-wallet-1" {
		t.Fatalf("unexpected holder wallet %q", meta.HolderWallet)
	}
	if meta.ListingHash != "listing-hash" || meta.ListingSlug != "psa-card" {
		t.Fatalf("unexpected listing metadata: %#v", meta)
	}
	if meta.BuyerPeerID != "buyer-peer" || meta.SellerPeerID != "seller-peer" {
		t.Fatalf("unexpected peers: %#v", meta)
	}
	if meta.ContractType != "RWA_TOKEN" || meta.TokenStandard != "pNFT" || meta.TokenAddress != "mint-from-listing" {
		t.Fatalf("unexpected token metadata: %#v", meta)
	}
}

func TestCollectibleOrderMetadataFromOrderOpenIgnoresNonCollectible(t *testing.T) {
	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_PHYSICAL_GOOD},
				Item:     &pb.Listing_Item{Title: "shirt"},
			},
		}},
		Items: []*pb.OrderOpen_Item{{ListingHash: "listing-hash"}},
	}

	if _, ok := CollectibleOrderMetadataFromOrderOpen(orderOpen); ok {
		t.Fatal("expected non-collectible order to be ignored")
	}
}

func TestIsHubManagedCollectiblePrimarySaleIncludesLegacyHubListing(t *testing.T) {
	orderOpen := &pb.OrderOpen{
		BuyerID: &pb.ID{PeerID: "buyer-peer"},
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				Slug:     "legacy-hub-card",
				VendorID: &pb.ID{PeerID: "seller-peer"},
				Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_PHYSICAL_GOOD},
				Item:     &pb.Listing_Item{Title: "Hub-held card"},
			},
		}},
		Items: []*pb.OrderOpen_Item{{
			ListingHash: "listing-hash",
			OptionalFeatures: []string{
				CollectibleOptionalFeature(CollectibleFeatureFulfillment, CollectibleFulfillmentNFT),
				CollectibleOptionalFeature(CollectibleFeatureHubSlotID, "slot-1"),
				CollectibleOptionalFeature(CollectibleFeatureCertNumber, "cert-1"),
				CollectibleOptionalFeature(CollectibleFeatureHolderWallet, "holder-wallet-1"),
			},
		}},
	}

	if !IsHubManagedCollectiblePrimarySale(orderOpen) {
		t.Fatal("expected complete Hub collectible metadata to enable lifecycle delivery")
	}
	if IsManagedCollectibleFirstSale(orderOpen) {
		t.Fatal("legacy Hub listing must not bypass source-custody payment authorization")
	}
}

func TestCollectibleOrderMetadataFromFiatMetadata(t *testing.T) {
	meta, ok := CollectibleOrderMetadataFromFiatMetadata(map[string]string{
		CollectibleMetadataKeyType:         CollectibleMetadataTypePrimarySale,
		CollectibleMetadataKeyFulfillment:  CollectibleFulfillmentNFT,
		CollectibleMetadataKeyHubSlotID:    "slot-1",
		CollectibleMetadataKeyNFTMint:      "mint-1",
		CollectibleMetadataKeyCertNumber:   "cert-1",
		CollectibleMetadataKeyBuyerPeerID:  "buyer-peer",
		CollectibleMetadataKeySellerPeerID: "seller-peer",
		CollectibleMetadataKeyHolderWallet: "holder-wallet-1",
	})
	if !ok {
		t.Fatal("expected collectible fiat metadata")
	}
	if meta.HubSlotID != "slot-1" || meta.NFTMint != "mint-1" || meta.BuyerPeerID != "buyer-peer" || meta.SellerPeerID != "seller-peer" || meta.HolderWallet != "holder-wallet-1" {
		t.Fatalf("unexpected metadata: %#v", meta)
	}

	if _, ok := CollectibleOrderMetadataFromFiatMetadata(map[string]string{"fiat_provider": "stripe"}); ok {
		t.Fatal("expected ordinary fiat metadata to be ignored")
	}
}

package models

import (
	"testing"

	"github.com/mobazha/mobazha/pkg/extensions"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

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

func TestCollectibleOrderExtension_RequiresAttestedSettlement(t *testing.T) {
	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{{Listing: &pb.Listing{
			Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_RWA_TOKEN},
		}}},
		Items: []*pb.OrderOpen_Item{{OptionalFeatures: []string{
			CollectibleOptionalFeature(CollectibleFeatureHubSlotID, "slot-1"),
		}}},
	}
	extension, ok, err := CollectibleOrderExtensionFromOrderOpen("order-1", orderOpen)
	if err != nil || !ok {
		t.Fatalf("build extension: ok=%v err=%v", ok, err)
	}
	if extension.SettlementPolicy != extensions.SettlementPolicyExtensionAttested {
		t.Fatalf("settlement policy = %q", extension.SettlementPolicy)
	}
	if !extension.ReservationRequired {
		t.Fatal("collectible extension must require a pre-funding reservation")
	}
}

func TestCollectibleOrderMetadataFromExtensionRejectsResourceMismatch(t *testing.T) {
	extension, err := extensions.NewOrderExtension(
		"order-resource-mismatch",
		CollectibleExtensionProviderID,
		CollectibleExtensionTypePrimarySale,
		extensions.ContractVersionV1,
		"slot-envelope",
		CollectibleOrderMetadata{
			Type: CollectibleMetadataTypePrimarySale, Fulfillment: CollectibleFulfillmentNFT,
			HubSlotID: "slot-payload", HolderWallet: "holder",
		},
	)
	if err != nil {
		t.Fatalf("NewOrderExtension: %v", err)
	}
	if _, ok := CollectibleOrderMetadataFromExtension(extension); ok {
		t.Fatal("accepted collectible payload whose Hub slot differs from the envelope resource")
	}
}

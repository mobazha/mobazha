//go:build !private_distribution

package order

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestPersistCollectibleOrderMetadataMergesFiatMetadata(t *testing.T) {
	db := newTestDatabase(t)
	orderID := "order_collectible_1"
	err := db.Update(func(tx database.Tx) error {
		order := &models.Order{ID: models.OrderID(orderID), MyRole: string(models.RoleBuyer)}
		if err := order.MergeFiatMetadata(map[string]string{"fiat_provider": "stripe"}); err != nil {
			return err
		}
		return tx.Save(order)
	})
	if err != nil {
		t.Fatal(err)
	}

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
					TokenAddress:  "mint-1",
				},
			},
		}},
		Items: []*pb.OrderOpen_Item{{
			ListingHash: "listing-hash",
			OptionalFeatures: []string{
				models.CollectibleOptionalFeature(models.CollectibleFeatureHubSlotID, "slot-1"),
				models.CollectibleOptionalFeature(models.CollectibleFeatureCertNumber, "cert-1"),
			},
		}},
	}

	err = db.Update(func(tx database.Tx) error {
		return persistCollectibleOrderMetadata(tx, orderID, orderOpen)
	})
	if err != nil {
		t.Fatal(err)
	}

	var order models.Order
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	meta, err := order.GetFiatMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if meta["fiat_provider"] != "stripe" {
		t.Fatalf("expected existing fiat metadata to be preserved, got %#v", meta)
	}
	if meta[models.CollectibleMetadataKeyType] != models.CollectibleMetadataTypePrimarySale {
		t.Fatalf("unexpected collectible metadata: %#v", meta)
	}
	if meta[models.CollectibleMetadataKeyHubSlotID] != "slot-1" {
		t.Fatalf("unexpected hub slot metadata: %#v", meta)
	}
	if meta[models.CollectibleMetadataKeyNFTMint] != "mint-1" {
		t.Fatalf("unexpected NFT mint metadata: %#v", meta)
	}
	if meta[models.CollectibleMetadataKeySellerPeerID] != "seller-peer" {
		t.Fatalf("unexpected seller metadata: %#v", meta)
	}
}

func TestPersistCollectibleOrderMetadataNoopsForNonCollectible(t *testing.T) {
	db := newTestDatabase(t)
	orderID := "order_physical_1"
	err := db.Update(func(tx database.Tx) error {
		return tx.Save(&models.Order{ID: models.OrderID(orderID), MyRole: string(models.RoleBuyer)})
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.Update(func(tx database.Tx) error {
		return persistCollectibleOrderMetadata(tx, orderID, &pb.OrderOpen{
			Listings: []*pb.SignedListing{{
				Listing: &pb.Listing{
					Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_PHYSICAL_GOOD},
					Item:     &pb.Listing_Item{Title: "shirt"},
				},
			}},
			Items: []*pb.OrderOpen_Item{{ListingHash: "listing-hash"}},
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	var order models.Order
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	meta, err := order.GetFiatMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if len(meta) != 0 {
		t.Fatalf("expected no metadata for non-collectible order, got %#v", meta)
	}
}

func TestPostProcessOrderOpenPersistsVendorCollectibleMetadata(t *testing.T) {
	db := newTestDatabase(t)
	svc := &OrderAppService{db: db}
	orderID := "order_vendor_collectible_1"
	err := db.Update(func(tx database.Tx) error {
		return tx.Save(&models.Order{
			ID:     models.OrderID(orderID),
			MyRole: string(models.RoleVendor),
			Open:   true,
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	orderOpen := &pb.OrderOpen{
		BuyerID: &pb.ID{PeerID: "buyer-peer"},
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				Slug:     "psa-card",
				VendorID: &pb.ID{PeerID: "seller-peer"},
				Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_RWA_TOKEN},
				Item:     &pb.Listing_Item{TokenStandard: "pNFT"},
			},
		}},
		Items: []*pb.OrderOpen_Item{{
			ListingHash: "listing-hash",
			OptionalFeatures: []string{
				models.CollectibleOptionalFeature(models.CollectibleFeatureHubSlotID, "slot-vendor"),
			},
		}},
	}
	payload, err := anypb.New(orderOpen)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(func(tx database.Tx) error {
		return svc.postProcessOrderOpenInTx(tx, &npb.OrderMessage{
			OrderID:     orderID,
			MessageType: npb.OrderMessage_ORDER_OPEN,
			Message:     payload,
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	var order models.Order
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	meta, err := order.GetFiatMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if meta[models.CollectibleMetadataKeyHubSlotID] != "slot-vendor" {
		t.Fatalf("vendor collectible metadata missing: %#v", meta)
	}
	if meta[models.CollectibleMetadataKeySellerPeerID] != "seller-peer" {
		t.Fatalf("vendor seller metadata missing: %#v", meta)
	}
}

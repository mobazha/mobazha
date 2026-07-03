package order

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	solana "github.com/gagliardetto/solana-go"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestPersistCollectibleOrderExtension_PersistsEnvelopeWithoutFiatMetadata(t *testing.T) {
	db := newTestDatabase(t)
	orderID := "order_collectible_1"
	holderWallet := solana.NewWallet().PublicKey().String()
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
				models.CollectibleOptionalFeature(models.CollectibleFeatureHolderWallet, holderWallet),
			},
		}},
	}
	svc := &OrderAppService{orderExtensionDeclarer: collectibleTestDeclarer}

	err = db.Update(func(tx database.Tx) error {
		return svc.persistOrderExtensions(context.Background(), tx, orderID, orderOpen)
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
	if len(meta) != 1 {
		t.Fatalf("collectible data leaked into FiatMetadata: %#v", meta)
	}
	var extension models.OrderExtensionRecord
	if err := db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_id = ? AND provider_id = ?", orderID, models.CollectibleExtensionProviderID).First(&extension).Error
	}); err != nil {
		t.Fatal(err)
	}
	if extension.ExtensionType != models.CollectibleExtensionTypePrimarySale || extension.Revision != 1 || extension.PayloadHash == "" {
		t.Fatalf("unexpected order extension envelope: %+v", extension)
	}
	var payload models.CollectibleOrderMetadata
	if err := json.Unmarshal(extension.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.HubSlotID != "slot-1" || payload.NFTMint != "mint-1" || payload.SellerPeerID != "seller-peer" || payload.HolderWallet != holderWallet {
		t.Fatalf("unexpected collectible extension payload: %+v", payload)
	}
}

func TestPersistCollectibleOrderExtension_RejectsMissingHolderWallet(t *testing.T) {
	db := newTestDatabase(t)
	orderID := "order_collectible_missing_holder"
	if err := db.Update(func(tx database.Tx) error {
		return tx.Save(&models.Order{ID: models.OrderID(orderID), MyRole: string(models.RoleVendor)})
	}); err != nil {
		t.Fatal(err)
	}
	orderOpen := &pb.OrderOpen{
		BuyerID: &pb.ID{PeerID: "buyer-peer"},
		Listings: []*pb.SignedListing{{Listing: &pb.Listing{
			VendorID: &pb.ID{PeerID: "seller-peer"},
			Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_RWA_TOKEN},
		}}},
		Items: []*pb.OrderOpen_Item{{
			ListingHash: "listing-hash",
			OptionalFeatures: []string{
				models.CollectibleOptionalFeature(models.CollectibleFeatureHubSlotID, "slot-1"),
			},
		}},
	}
	svc := &OrderAppService{orderExtensionDeclarer: collectibleTestDeclarer}
	err := db.Update(func(tx database.Tx) error {
		return svc.persistOrderExtensions(context.Background(), tx, orderID, orderOpen)
	})
	if err == nil {
		t.Fatal("module declaration accepted a missing holder wallet")
	}
}

func TestPersistCollectibleOrderExtension_NoopsForNonCollectible(t *testing.T) {
	db := newTestDatabase(t)
	orderID := "order_physical_1"
	err := db.Update(func(tx database.Tx) error {
		return tx.Save(&models.Order{ID: models.OrderID(orderID), MyRole: string(models.RoleBuyer)})
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.Update(func(tx database.Tx) error {
		svc := &OrderAppService{orderExtensionDeclarer: collectibleTestDeclarer}
		return svc.persistOrderExtensions(context.Background(), tx, orderID, &pb.OrderOpen{
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

	var count int64
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.OrderExtensionRecord{}).Where("order_id = ?", orderID).Count(&count).Error
	})
	if err != nil || count != 0 {
		t.Fatalf("expected no extension for non-collectible order, count=%d err=%v", count, err)
	}
}

func TestPersistOrderExtensions_RequiresDeclarationForRWAOrder(t *testing.T) {
	db := newTestDatabase(t)
	orderOpen := &pb.OrderOpen{Listings: []*pb.SignedListing{{Listing: &pb.Listing{
		Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_RWA_TOKEN},
	}}}}
	svc := &OrderAppService{}
	err := db.Update(func(tx database.Tx) error {
		return svc.persistOrderExtensions(context.Background(), tx, "order-rwa", orderOpen)
	})
	if err == nil {
		t.Fatal("RWA order without a declaration module was accepted")
	}

	svc.orderExtensionDeclarer = func(context.Context, extensions.DeclarationInput) ([]extensions.OrderExtension, error) {
		return nil, nil
	}
	err = db.Update(func(tx database.Tx) error {
		return svc.persistOrderExtensions(context.Background(), tx, "order-rwa", orderOpen)
	})
	if err == nil {
		t.Fatal("RWA order without a declared extension was accepted")
	}
}

func TestPostProcessOrderOpen_PersistsVendorCollectibleExtension(t *testing.T) {
	db := newTestDatabase(t)
	svc := &OrderAppService{db: db, orderExtensionDeclarer: collectibleTestDeclarer}
	orderID := "order_vendor_collectible_1"
	holderWallet := solana.NewWallet().PublicKey().String()
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
				models.CollectibleOptionalFeature(models.CollectibleFeatureHolderWallet, holderWallet),
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

	var extension models.OrderExtensionRecord
	if err := db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_id = ? AND provider_id = ?", orderID, models.CollectibleExtensionProviderID).First(&extension).Error
	}); err != nil {
		t.Fatal(err)
	}
	var payloadMetadata models.CollectibleOrderMetadata
	if err := json.Unmarshal(extension.Payload, &payloadMetadata); err != nil {
		t.Fatal(err)
	}
	if payloadMetadata.HubSlotID != "slot-vendor" || payloadMetadata.SellerPeerID != "seller-peer" || payloadMetadata.HolderWallet != holderWallet {
		t.Fatalf("vendor collectible extension payload missing: %+v", payloadMetadata)
	}
}

func collectibleTestDeclarer(_ context.Context, input extensions.DeclarationInput) ([]extensions.OrderExtension, error) {
	extension, ok, err := models.CollectibleOrderExtensionFromOrderOpen(input.OrderID, input.OrderOpen)
	if err != nil || !ok {
		return nil, err
	}
	metadata, ok := models.CollectibleOrderMetadataFromExtension(extension)
	if !ok {
		return nil, fmt.Errorf("invalid collectible extension")
	}
	if metadata.HolderWallet == "" {
		return nil, fmt.Errorf("%w: collectible declaration requires a holderWallet", coreiface.ErrBadRequest)
	}
	if _, err := solana.PublicKeyFromBase58(metadata.HolderWallet); err != nil {
		return nil, fmt.Errorf("%w: collectible holderWallet must be a valid Solana public key", coreiface.ErrBadRequest)
	}
	return []extensions.OrderExtension{extension}, nil
}

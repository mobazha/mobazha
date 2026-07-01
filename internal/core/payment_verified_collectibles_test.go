package core

import (
	"context"
	"testing"
	"time"

	solana "github.com/gagliardetto/solana-go"
	coreorder "github.com/mobazha/mobazha3.0/internal/core/order"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestCollectiblePrimarySalePaidUsesSignedOrderOpenWithoutFiatMetadata(t *testing.T) {
	db, err := repo.MockDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := coreorder.NewTestOrderAppService(t, coreorder.OrderAppServiceConfig{DB: db})
	orderID := "order-collectible-paid"
	coreorder.SeedOrder(t, svc, orderID, string(models.RoleVendor), models.OrderState_AWAITING_PAYMENT)
	buyerWallet := solana.NewWallet()
	nodeIdentityWallet := solana.NewWallet()

	paidAt := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	err = db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.CollectibleLifecycleDelivery{}); err != nil {
			return err
		}
		var order models.Order
		if err := tx.Read().Where("id = ?", orderID).First(&order).Error; err != nil {
			return err
		}
		orderOpen := collectibleLifecycleOrderOpen(
			buyerWallet.PublicKey().String(),
			nodeIdentityWallet.PublicKey().Bytes(),
		)
		serializedOrderOpen, err := protojson.Marshal(orderOpen)
		if err != nil {
			return err
		}
		order.SerializedOrderOpen = serializedOrderOpen
		paymentSent := &pb.PaymentSent{
			TransactionID:  "escrow-tx-1",
			Coin:           "crypto:solana:devnet:usdc",
			Amount:         "2500000",
			ToAddress:      "escrow-address",
			SettlementSpec: payment.NewDirectSpec().ToPaymentSent(),
		}
		serializedPaymentSent, err := protojson.Marshal(paymentSent)
		if err != nil {
			return err
		}
		order.SerializedPaymentSent = serializedPaymentSent
		order.PaidAt = &paidAt
		if err := tx.Save(&order); err != nil {
			return err
		}
		return tx.Save(&models.CollectibleLifecycleDelivery{
			JobID:   models.CollectibleLifecyclePaid + ":" + orderID,
			OrderID: orderID,
			Kind:    models.CollectibleLifecyclePaid,
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	var got CollectiblePrimarySalePaidSignal
	node := &MobazhaNode{
		storageFields: storageFields{db: db},
		appServices:   appServices{orderService: svc},
		collectiblesFields: collectiblesFields{
			collectiblePrimarySalePaidHook: func(_ context.Context, signal CollectiblePrimarySalePaidSignal) error {
				got = signal
				return nil
			},
		},
	}

	node.handleCryptoPaymentVerified(orderID, &pb.PaymentSent{
		TransactionID:  "escrow-tx-1",
		Coin:           "crypto:solana:devnet:usdc",
		Amount:         "2500000",
		ToAddress:      "escrow-address",
		SettlementSpec: payment.NewDirectSpec().ToPaymentSent(),
	})

	if got.OrderID != orderID || got.HubSlotID != "slot-1" || got.NFTMint != "mint-1" {
		t.Fatalf("unexpected collectible signal ids: %#v", got)
	}
	if got.EscrowID != "escrow-tx-1" || got.PriceAmount != "2500000" || got.CurrencyCode != "crypto:solana:devnet:usdc" {
		t.Fatalf("unexpected collectible signal payment fields: %#v", got)
	}
	if got.BuyerPeerID != "buyer-peer" || got.SellerPeerID != "seller-peer" || !got.PaidAt.Equal(paidAt) {
		t.Fatalf("unexpected collectible signal parties/time: %#v", got)
	}
	if got.BuyerSolanaAddress != buyerWallet.PublicKey().String() {
		t.Fatalf("buyer solana address = %q, want %q", got.BuyerSolanaAddress, buyerWallet.PublicKey().String())
	}
}

func collectibleLifecycleOrderOpen(holderWallet string, buyerIdentityKey []byte) *pb.OrderOpen {
	return &pb.OrderOpen{
		BuyerID: &pb.ID{
			PeerID:  "buyer-peer",
			Pubkeys: &pb.ID_Pubkeys{Solana: buyerIdentityKey},
		},
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				Slug:     "collectible-card",
				VendorID: &pb.ID{PeerID: "seller-peer"},
				Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_RWA_TOKEN},
				Item: &pb.Listing_Item{
					Blockchain:    "solana",
					TokenStandard: "metaplex_pnft",
				},
			},
		}},
		Items: []*pb.OrderOpen_Item{{
			ListingHash: "listing-hash",
			OptionalFeatures: []string{
				models.CollectibleOptionalFeature(models.CollectibleFeatureFulfillment, models.CollectibleFulfillmentNFT),
				models.CollectibleOptionalFeature(models.CollectibleFeatureHubSlotID, "slot-1"),
				models.CollectibleOptionalFeature(models.CollectibleFeatureNFTMint, "mint-1"),
				models.CollectibleOptionalFeature(models.CollectibleFeatureCertNumber, "cert-1"),
				models.CollectibleOptionalFeature(models.CollectibleFeatureHolderWallet, holderWallet),
			},
		}},
	}
}

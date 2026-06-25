//go:build !private_distribution

package core

import (
	"context"
	"testing"
	"time"

	coreorder "github.com/mobazha/mobazha3.0/internal/core/order"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
)

func TestSignalCollectiblePrimarySalePaidFromVerifiedPayment(t *testing.T) {
	db, err := repo.MockDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := coreorder.NewTestOrderAppService(t, coreorder.OrderAppServiceConfig{DB: db})
	orderID := "order-collectible-paid"
	coreorder.SeedOrder(t, svc, orderID, string(models.RoleVendor), models.OrderState_AWAITING_PAYMENT)

	paidAt := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	err = db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().Where("id = ?", orderID).First(&order).Error; err != nil {
			return err
		}
		if err := order.MergeFiatMetadata(map[string]string{
			models.CollectibleMetadataKeyType:         models.CollectibleMetadataTypePrimarySale,
			models.CollectibleMetadataKeyFulfillment:  models.CollectibleFulfillmentNFT,
			models.CollectibleMetadataKeyHubSlotID:    "slot-1",
			models.CollectibleMetadataKeyNFTMint:      "mint-1",
			models.CollectibleMetadataKeyCertNumber:   "cert-1",
			models.CollectibleMetadataKeyBuyerPeerID:  "buyer-peer",
			models.CollectibleMetadataKeySellerPeerID: "seller-peer",
		}); err != nil {
			return err
		}
		order.PaidAt = &paidAt
		return tx.Save(&order)
	})
	if err != nil {
		t.Fatal(err)
	}

	var got CollectiblePrimarySalePaidSignal
	node := &MobazhaNode{
		appServices: appServices{orderService: svc},
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
}

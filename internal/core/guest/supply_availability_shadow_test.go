package guest

import (
	"context"
	"errors"
	"io"
	"math/big"
	"strconv"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/gagliardetto/solana-go"
	"github.com/mobazha/mobazha/internal/core/checkoutsupply"
	"github.com/mobazha/mobazha/internal/core/digital"
	"github.com/mobazha/mobazha/internal/wallet"
	pkgconfig "github.com/mobazha/mobazha/pkg/config"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

func TestGuestSupplyLinesForItemsMapsStockTracking(t *testing.T) {
	items := []models.GuestOrderItem{
		{
			OrderToken:  "gst_test",
			ListingSlug: "camera",
			VariantHash: "red",
			Quantity:    2,
		},
		{
			OrderToken:  "gst_test",
			ListingSlug: "ebook",
			Quantity:    1,
		},
	}
	trackedBucket := guestInventoryBucketKey{Slug: "camera", VariantHash: "red"}
	lines := guestSupplyLinesForItems(
		items,
		[]guestInventoryBucketKey{
			trackedBucket,
			{Slug: "ebook"},
		},
		map[guestInventoryBucketKey]int64{
			trackedBucket: 5,
		},
	)

	require.Len(t, lines, 2)
	require.Equal(t, contracts.SupplyKindSkuQuantity, lines[0].SupplyKind)
	require.Equal(t, "camera", lines[0].ListingSlug)
	require.Equal(t, "red", lines[0].VariantHash)
	require.Equal(t, 2, lines[0].Quantity)
	require.True(t, lines[0].StockTracked)
	require.Equal(t, int64(5), lines[0].StockLimit)

	require.Equal(t, "ebook", lines[1].ListingSlug)
	require.Equal(t, 1, lines[1].Quantity)
	require.False(t, lines[1].StockTracked)
	require.Zero(t, lines[1].StockLimit)
}

func TestGuestSupplyLinesForItemsPrefersExternalMapping(t *testing.T) {
	items := []models.GuestOrderItem{
		{
			OrderToken:   "gst_test",
			ListingSlug:  "supplier-shirt",
			ContractType: pb.Listing_Metadata_PHYSICAL_GOOD.String(),
			VariantHash:  "red",
			Quantity:     2,
		},
	}
	bucket := guestInventoryBucketKey{Slug: "supplier-shirt", VariantHash: "red"}
	lines := guestSupplyLinesForItemsWithExternalMappings(
		items,
		[]guestInventoryBucketKey{bucket},
		map[guestInventoryBucketKey]int64{bucket: 5},
		map[string]models.SyncedProductMapping{
			"supplier-shirt": {
				ID:            "spm-1",
				ProviderID:    "printful",
				ListingSlug:   "supplier-shirt",
				SyncProductID: "sync-123",
			},
		},
	)

	require.Len(t, lines, 1)
	require.Equal(t, contracts.SupplyKindExternalSupply, lines[0].SupplyKind)
	require.Equal(t, "supplier-shirt", lines[0].ListingSlug)
	require.Equal(t, 2, lines[0].Quantity)
	require.Equal(t, "printful", lines[0].ProviderID)
	require.Equal(t, "sync-123", lines[0].ProviderRef)
	require.False(t, lines[0].StockTracked)
	require.Empty(t, lines[0].VariantHash)
}

func TestGuestSupplyLinesUsesDigitalResolverForDigitalGoods(t *testing.T) {
	resolver := &recordingGuestDigitalSupplyLineResolver{
		lines: []contracts.SupplyLine{{
			LineID:      "digital:0:ebook:unlimited_digital",
			ListingSlug: "ebook",
			Quantity:    2,
			SupplyKind:  contracts.SupplyKindUnlimitedDigital,
		}},
	}
	svc := &GuestOrderAppService{
		digitalSupplyLines: resolver,
		resolver:           alwaysEnabledResolver{},
	}

	lines, err := svc.supplyAvailabilityLinesForGuestItems(context.Background(),
		[]models.GuestOrderItem{{
			OrderToken:   "gst_test",
			ListingSlug:  "ebook",
			ContractType: pb.Listing_Metadata_DIGITAL_GOOD.String(),
			VariantSKU:   "sku-blue",
			Quantity:     2,
		}},
		[]guestInventoryBucketKey{{Slug: "ebook"}},
		map[guestInventoryBucketKey]int64{},
		nil,
	)
	require.NoError(t, err)
	require.Len(t, resolver.items, 1)
	require.Equal(t, "ebook", resolver.items[0][0].ListingSlug)
	require.Equal(t, "sku-blue", resolver.items[0][0].VariantSKU)
	require.Equal(t, uint32(2), resolver.items[0][0].Quantity)
	require.Equal(t, resolver.lines, lines)
}

func TestResolveItemPriceSnapshotsVariantSKU(t *testing.T) {
	svc := &GuestOrderAppService{
		listings: &stubGuestListings{
			bySlug: map[string]*pb.SignedListing{
				"ebook": guestListingWithSku("ebook", "Color", "Blue", "3", "1200"),
			},
		},
	}
	svc.listings.(*stubGuestListings).bySlug["ebook"].Listing.Metadata.ContractType = pb.Listing_Metadata_DIGITAL_GOOD
	svc.listings.(*stubGuestListings).bySlug["ebook"].Listing.Item.Skus[0].ProductID = "sku-blue"

	resolved, err := svc.resolveItemPrice(contracts.GuestOrderItemRequest{
		ListingSlug: "ebook",
		Quantity:    1,
		Options: []map[string]string{
			{"Color": "Blue"},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, resolved.VariantHash)
	require.Equal(t, "sku-blue", resolved.VariantSKU)
}

func TestGuestSupplyLinesSkipsDigitalGoodsWithoutResolver(t *testing.T) {
	svc := &GuestOrderAppService{}

	lines, err := svc.supplyAvailabilityLinesForGuestItems(context.Background(),
		[]models.GuestOrderItem{{
			OrderToken:   "gst_test",
			ListingSlug:  "ebook",
			ContractType: pb.Listing_Metadata_DIGITAL_GOOD.String(),
			Quantity:     1,
		}},
		[]guestInventoryBucketKey{{Slug: "ebook"}},
		map[guestInventoryBucketKey]int64{},
		nil,
	)
	require.NoError(t, err)
	require.Empty(t, lines)
}

func TestGuestSupplyLinesFailsDigitalGoodsWithoutResolverWhenRequired(t *testing.T) {
	svc := &GuestOrderAppService{
		resolver:           alwaysEnabledResolver{},
		supplyAvailability: &recordingSupplyAvailability{},
	}

	_, err := svc.supplyAvailabilityLinesForGuestItems(context.Background(),
		[]models.GuestOrderItem{{
			OrderToken:   "gst_test",
			ListingSlug:  "ebook",
			ContractType: pb.Listing_Metadata_DIGITAL_GOOD.String(),
			Quantity:     1,
		}},
		[]guestInventoryBucketKey{{Slug: "ebook"}},
		map[guestInventoryBucketKey]int64{},
		nil,
	)
	require.ErrorContains(t, err, "digital supply resolver unavailable")
}

func TestGuestSupplyLinesReturnsDigitalResolverError(t *testing.T) {
	resolver := &recordingGuestDigitalSupplyLineResolver{err: errors.New("digital resolver offline")}
	svc := &GuestOrderAppService{
		digitalSupplyLines: resolver,
		resolver:           alwaysEnabledResolver{},
	}

	_, err := svc.supplyAvailabilityLinesForGuestItems(context.Background(),
		[]models.GuestOrderItem{{
			OrderToken:   "gst_test",
			ListingSlug:  "ebook",
			ContractType: pb.Listing_Metadata_DIGITAL_GOOD.String(),
			Quantity:     1,
		}},
		[]guestInventoryBucketKey{{Slug: "ebook"}},
		map[guestInventoryBucketKey]int64{},
		nil,
	)
	require.ErrorContains(t, err, "digital resolver offline")
}

func TestQuoteSupplyAvailabilityShadowPassesGuestOrderLines(t *testing.T) {
	recorder := &recordingSupplyAvailability{
		quoteResult: &contracts.SupplyQuoteResult{CanSell: true},
	}
	svc := &GuestOrderAppService{supplyAvailability: recorder}
	bucket := guestInventoryBucketKey{Slug: "camera", VariantHash: "red"}

	svc.quoteSupplyAvailabilityShadow(
		context.Background(),
		"gst_test",
		[]models.GuestOrderItem{{
			OrderToken:  "gst_test",
			ListingSlug: "camera",
			VariantHash: "red",
			Quantity:    2,
		}},
		[]guestInventoryBucketKey{bucket},
		map[guestInventoryBucketKey]int64{bucket: 5},
	)

	require.Len(t, recorder.quoteRequests, 1)
	req := recorder.quoteRequests[0]
	require.Equal(t, "gst_test", req.OrderRef)
	require.Equal(t, models.OrderTypeGuest, req.OrderType)
	require.Len(t, req.Lines, 1)
	require.Equal(t, "camera", req.Lines[0].ListingSlug)
	require.True(t, req.Lines[0].StockTracked)
	require.Equal(t, int64(5), req.Lines[0].StockLimit)
}

func wireGuestCheckoutSupplyQuoter(svc *GuestOrderAppService) {
	if svc == nil {
		return
	}
	quoter := checkoutsupply.NewCheckoutSupplyQuoteService(checkoutsupply.CheckoutSupplyQuoteServiceConfig{
		DB:                 svc.db,
		SupplyAvailability: svc.supplyAvailability,
		Resolver:           svc.resolver,
		DigitalSupplyLines: svc.digitalSupplyLines,
		Listings:           svc.listings,
	})
	svc.SetCheckoutSupplyQuoter(quoter)
}

func TestQuoteGuestOrderSupplyReturnsUnknownWhenFeatureDisabled(t *testing.T) {
	svc := &GuestOrderAppService{
		resolver: guestCheckoutOnlyResolver{},
		listings: &stubGuestListings{
			bySlug: map[string]*pb.SignedListing{
				"camera": guestListingWithSku("camera", "Color", "Red", "3", "1200"),
			},
		},
	}
	svc.db = newGuestQuoteConfigDB(t)
	wireGuestCheckoutSupplyQuoter(svc)

	resp, err := svc.QuoteGuestOrderSupply(context.Background(), contracts.QuoteGuestOrderSupplyRequest{
		Items: []contracts.GuestOrderItemRequest{{
			ListingSlug: "camera",
			Quantity:    1,
			Options:     []map[string]string{{"Color": "Red"}},
		}},
	})
	require.NoError(t, err)
	require.True(t, resp.CanSell)
	require.Equal(t, "supply_availability_disabled", resp.Reason)
	require.Len(t, resp.Items, 1)
	require.Equal(t, contracts.SupplyAvailabilityUnknown, resp.Items[0].Status)
}

func TestQuoteGuestOrderSupplyReturnsBuyerManagedAvailability(t *testing.T) {
	recorder := &recordingSupplyAvailability{
		quoteResult: &contracts.SupplyQuoteResult{
			CanSell: true,
			Results: []contracts.AvailabilityResult{{
				LineID:            "guest_quote:0",
				SupplyKind:        contracts.SupplyKindSkuQuantity,
				Status:            contracts.SupplyAvailabilityLowStock,
				Available:         true,
				AvailableQuantity: 2,
				ProviderID:        "internal-provider",
				ProviderRef:       "private-ref",
			}},
		},
	}
	svc := &GuestOrderAppService{
		resolver:           alwaysEnabledResolver{},
		supplyAvailability: recorder,
		listings: &stubGuestListings{
			bySlug: map[string]*pb.SignedListing{
				"camera": guestListingWithSku("camera", "Color", "Red", "3", "1200"),
			},
		},
	}
	svc.db = newGuestQuoteConfigDB(t)
	wireGuestCheckoutSupplyQuoter(svc)

	resp, err := svc.QuoteGuestOrderSupply(context.Background(), contracts.QuoteGuestOrderSupplyRequest{
		Items: []contracts.GuestOrderItemRequest{{
			ListingSlug: "camera",
			Quantity:    1,
			Options:     []map[string]string{{"Color": "Red"}},
		}},
	})
	require.NoError(t, err)
	require.True(t, resp.CanSell)
	require.Len(t, recorder.quoteRequests, 1)
	require.Len(t, recorder.quoteRequests[0].Lines, 1)
	require.Equal(t, contracts.SupplyKindSkuQuantity, recorder.quoteRequests[0].Lines[0].SupplyKind)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "camera", resp.Items[0].ListingSlug)
	require.Equal(t, contracts.SupplyAvailabilityLowStock, resp.Items[0].Status)
	require.Equal(t, int64(2), resp.Items[0].AvailableQuantity)
}

func TestQuoteGuestOrderSupplyUsesVariantDigitalLicensePool(t *testing.T) {
	db := newGuestQuoteConfigDB(t)
	assetSvc := newGuestQuoteDigitalAssetService(t, db)
	_, err := assetSvc.ImportLicenseKeys("ebook", "", "app-universal", []string{"UNIVERSAL-1"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)
	_, err = assetSvc.ImportLicenseKeys("ebook", "sku-blue", "app-blue", []string{"BLUE-1", "BLUE-2"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)

	supply := newProviderBackedGuestSupplyAvailability(
		digital.NewLicenseKeyPoolProvider(db),
		digital.NewUnlimitedDigitalProvider(db),
	)
	listing := guestListingWithSku("ebook", "Color", "Blue", "3", "1200")
	listing.Listing.Metadata.ContractType = pb.Listing_Metadata_DIGITAL_GOOD
	listing.Listing.Item.Skus[0].ProductID = "sku-blue"
	svc := &GuestOrderAppService{
		db:                 db,
		resolver:           alwaysEnabledResolver{},
		digitalSupplyLines: assetSvc,
		supplyAvailability: supply,
		listings: &stubGuestListings{
			bySlug: map[string]*pb.SignedListing{
				"ebook": listing,
			},
		},
	}
	wireGuestCheckoutSupplyQuoter(svc)

	resp, err := svc.QuoteGuestOrderSupply(context.Background(), contracts.QuoteGuestOrderSupplyRequest{
		Items: []contracts.GuestOrderItemRequest{{
			ListingSlug: "ebook",
			Quantity:    2,
			Options:     []map[string]string{{"Color": "Blue"}},
		}},
	})
	require.NoError(t, err)
	require.True(t, resp.CanSell)
	require.False(t, resp.ManualActionRequired)
	require.Len(t, resp.Items, 1)
	require.Equal(t, contracts.SupplyKindLicenseKeyPool, resp.Items[0].SupplyKind)
	require.Equal(t, contracts.SupplyAvailabilityAvailable, resp.Items[0].Status)
	require.True(t, resp.Items[0].Available)
	require.Equal(t, int64(2), resp.Items[0].AvailableQuantity)

	require.Len(t, supply.quoteRequests, 1)
	require.Len(t, supply.quoteRequests[0].Lines, 1)
	require.Equal(t, "sku-blue", supply.quoteRequests[0].Lines[0].VariantSKU)
}

func TestQuoteGuestOrderSupplyReportsMissingDigitalAssetManualAction(t *testing.T) {
	db := newGuestQuoteConfigDB(t)
	assetSvc := newGuestQuoteDigitalAssetService(t, db)
	supply := newProviderBackedGuestSupplyAvailability(
		digital.NewLicenseKeyPoolProvider(db),
		digital.NewUnlimitedDigitalProvider(db),
	)
	svc := &GuestOrderAppService{
		db:                 db,
		resolver:           alwaysEnabledResolver{},
		digitalSupplyLines: assetSvc,
		supplyAvailability: supply,
		listings: &stubGuestListings{
			bySlug: map[string]*pb.SignedListing{
				"ebook-missing": guestListing("ebook-missing", pb.Listing_Metadata_DIGITAL_GOOD),
			},
		},
	}
	wireGuestCheckoutSupplyQuoter(svc)

	resp, err := svc.QuoteGuestOrderSupply(context.Background(), contracts.QuoteGuestOrderSupplyRequest{
		Items: []contracts.GuestOrderItemRequest{{
			ListingSlug: "ebook-missing",
			Quantity:    1,
		}},
	})
	require.NoError(t, err)
	require.False(t, resp.CanSell)
	require.True(t, resp.ManualActionRequired)
	require.Equal(t, "manual_action_required", resp.Reason)
	require.Len(t, resp.Items, 1)
	require.Equal(t, contracts.SupplyKindUnlimitedDigital, resp.Items[0].SupplyKind)
	require.Equal(t, contracts.SupplyAvailabilityManualActionRequired, resp.Items[0].Status)
	require.False(t, resp.Items[0].Available)
	require.True(t, resp.Items[0].ManualActionRequired)
	require.Equal(t, "digital_asset_missing", resp.Items[0].Reason)

	require.Len(t, supply.quoteRequests, 1)
	require.Len(t, supply.quoteRequests[0].Lines, 1)
	require.Equal(t, "digital_asset_missing", supply.quoteRequests[0].Lines[0].Metadata["manualActionReason"])
}

func TestQuoteSupplyAvailabilityShadowSwallowsQuoteError(t *testing.T) {
	recorder := &recordingSupplyAvailability{
		quoteErr: errors.New("quote unavailable"),
	}
	svc := &GuestOrderAppService{supplyAvailability: recorder}
	bucket := guestInventoryBucketKey{Slug: "camera", VariantHash: "red"}

	require.NotPanics(t, func() {
		svc.quoteSupplyAvailabilityShadow(
			context.Background(),
			"gst_test",
			[]models.GuestOrderItem{{
				OrderToken:  "gst_test",
				ListingSlug: "camera",
				VariantHash: "red",
				Quantity:    1,
			}},
			[]guestInventoryBucketKey{bucket},
			map[guestInventoryBucketKey]int64{bucket: 1},
		)
	})
	require.Len(t, recorder.quoteRequests, 1)
}

func newGuestQuoteConfigDB(t *testing.T) *testDatabase {
	t.Helper()
	db := newGuestTestDB(t)
	require.NoError(t, db.gormDB.AutoMigrate(&models.GuestCheckoutConfig{}))
	require.NoError(t, db.gormDB.Create(&models.GuestCheckoutConfig{
		Enabled:       true,
		AcceptedCoins: "LTC",
	}).Error)
	return db
}

type recordingGuestAffiliateService struct {
	facts    models.AffiliateOrderFacts
	recorded bool
}

func (s *recordingGuestAffiliateService) PrepareOrderAttribution(_ context.Context, facts models.AffiliateOrderFacts) (*models.AffiliateOrderResult, error) {
	s.facts = facts
	return &models.AffiliateOrderResult{
		Attribution: models.AffiliateAttribution{
			OrderID: facts.OrderID, BuyerKind: facts.BuyerKind, GuestBuyerID: facts.GuestBuyerID,
			PromoterUTXOPayoutAddresses: models.AffiliateUTXOPayoutAddresses{
				models.AffiliatePayoutRailBitcoin: "btc-promoter", models.AffiliatePayoutRailBitcoinCash: "bch-promoter",
				models.AffiliatePayoutRailLitecoin: "ltc-promoter",
			},
		},
		Lines: []models.AffiliateCommissionLine{{CommissionAtomic: "1500"}},
	}, nil
}

func (s *recordingGuestAffiliateService) RecordPreparedOrderTx(_ database.Tx, result *models.AffiliateOrderResult) (*models.AffiliateOrderResult, error) {
	s.recorded = true
	return result, nil
}

func (*recordingGuestAffiliateService) TransitionCommissionTx(database.Tx, string, models.AffiliateCommissionStatus, models.AffiliateCommissionReversalReason, time.Time) ([]models.AffiliateCommissionLine, error) {
	return nil, nil
}

func TestCreateGuestOrder_FreezesAffiliateTermsInPaymentCoin(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	db := svc.db.(*testDatabase)
	require.NoError(t, db.gormDB.AutoMigrate(
		&models.DirectPaymentAddressCounter{},
		&models.GuestCheckoutConfig{},
	))
	require.NoError(t, db.gormDB.Create(&models.DirectPaymentAddressCounter{
		TenantID: testTenantID, ID: 1, ChainKey: "LTC",
	}).Error)
	recorder := &recordingSupplyAvailability{quoteResult: &contracts.SupplyQuoteResult{CanSell: true}}
	affiliate := &recordingGuestAffiliateService{}
	svc.resolver = alwaysEnabledResolver{}
	svc.directPayment = NewDirectPaymentService(db, fixedBIP44KeyDeriver{})
	svc.exchangeRates = wallet.NewFixedRateProvider("LTC", map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(8000),
	})
	svc.supplyAvailability = recorder
	svc.sellerAffiliate = affiliate
	svc.listings = &stubGuestListings{bySlug: map[string]*pb.SignedListing{
		"ebook": guestListing("ebook", pb.Listing_Metadata_DIGITAL_GOOD),
	}}
	require.NoError(t, svc.SaveGuestCheckoutConfig(context.Background(), &models.GuestCheckoutConfig{
		Enabled: true, AcceptedCoins: "LTC",
	}))

	resp, err := svc.CreateGuestOrder(context.Background(), contracts.CreateGuestOrderRequest{
		PaymentCoin: "LTC", AffiliateReferralSessionID: "referral-session-1",
		Items: []contracts.GuestOrderItemRequest{{ListingSlug: "ebook", Quantity: 1}},
	})
	require.NoError(t, err)
	require.True(t, affiliate.recorded)
	require.Equal(t, models.AffiliateBuyerKindGuest, affiliate.facts.BuyerKind)
	require.Empty(t, affiliate.facts.BuyerPeerID)
	require.NotEmpty(t, affiliate.facts.GuestBuyerID)
	require.Len(t, affiliate.facts.Lines, 1)
	require.Equal(t, resp.PaymentCoin, affiliate.facts.Lines[0].Currency)

	order := loadGuestOrder(t, db, resp.OrderToken)
	require.Equal(t, "referral-session-1", order.AffiliateReferralSessionID)
	require.Equal(t, affiliate.facts.GuestBuyerID, order.AffiliateGuestBuyerID)
	require.Equal(t, "ltc-promoter", order.AffiliatePayoutAddress)
	require.Equal(t, "1500", order.AffiliatePayoutAmount)
}

func TestValidateGuestAffiliateUTXOPayout_RejectsDustBeforePayment(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	err := svc.validateGuestAffiliateUTXOPayout(iwallet.ChainLitecoin, "ltc-promoter", big.NewInt(10))
	require.ErrorIs(t, err, contracts.ErrInvalidGuestRequest)
	require.Contains(t, err.Error(), "dust")
}

func TestCreateGuestOrder_ShadowQuotePreservesGuestInventoryReservation(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	db := svc.db.(*testDatabase)
	require.NoError(t, db.gormDB.AutoMigrate(
		&models.DirectPaymentAddressCounter{},
		&models.GuestCheckoutConfig{},
	))
	require.NoError(t, db.gormDB.Create(&models.DirectPaymentAddressCounter{
		TenantID: testTenantID,
		ID:       1,
		ChainKey: "LTC",
	}).Error)

	recorder := &recordingSupplyAvailability{
		quoteResult: &contracts.SupplyQuoteResult{CanSell: true},
	}
	svc.resolver = alwaysEnabledResolver{}
	svc.directPayment = NewDirectPaymentService(db, fixedBIP44KeyDeriver{})
	svc.exchangeRates = wallet.NewFixedRateProvider("LTC", map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(8000),
	})
	svc.supplyAvailability = recorder
	svc.listings = &stubGuestListings{
		bySlug: map[string]*pb.SignedListing{
			"camera": guestListingWithSku("camera", "Color", "Red", "3", "1200"),
		},
	}

	require.NoError(t, svc.SaveGuestCheckoutConfig(context.Background(), &models.GuestCheckoutConfig{
		Enabled:       true,
		AcceptedCoins: "LTC",
	}))

	resp, err := svc.CreateGuestOrder(context.Background(), contracts.CreateGuestOrderRequest{
		PaymentCoin:     "LTC",
		ShippingAddress: map[string]string{"country": "US"},
		ShippingCountry: "US",
		Items: []contracts.GuestOrderItemRequest{{
			ListingSlug:     "camera",
			Quantity:        1,
			ShippingOption:  "test-zone",
			ShippingService: "standard",
			Options: []map[string]string{
				{"Color": "Red"},
			},
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.OrderToken)

	require.Len(t, recorder.quoteRequests, 1)
	quoteReq := recorder.quoteRequests[0]
	require.Equal(t, resp.OrderToken, quoteReq.OrderRef)
	require.Equal(t, models.OrderTypeGuest, quoteReq.OrderType)
	require.Len(t, quoteReq.Lines, 1)
	require.Equal(t, contracts.SupplyKindSkuQuantity, quoteReq.Lines[0].SupplyKind)
	require.Equal(t, "camera", quoteReq.Lines[0].ListingSlug)
	require.True(t, quoteReq.Lines[0].StockTracked)
	require.Equal(t, int64(3), quoteReq.Lines[0].StockLimit)
	require.Equal(t, 1, quoteReq.Lines[0].Quantity)
	require.NotEmpty(t, quoteReq.Lines[0].VariantHash)

	var reservations []models.InventoryReservation
	require.NoError(t, db.gormDB.Where("order_ref = ?", resp.OrderToken).Find(&reservations).Error)
	require.Len(t, reservations, 1)
	require.Equal(t, models.OrderTypeGuest, reservations[0].OrderType)
	require.Equal(t, "camera", reservations[0].ListingSlug)
	require.Equal(t, quoteReq.Lines[0].VariantHash, reservations[0].VariantHash)
	require.Equal(t, 1, reservations[0].Quantity)
	require.False(t, reservations[0].Confirmed)
	require.Nil(t, reservations[0].ReleasedAt)
}

func TestCreateGuestOrder_ShadowQuoteUsesExternalSupplyLineForSyncedListing(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	db := svc.db.(*testDatabase)
	require.NoError(t, db.gormDB.AutoMigrate(
		&models.DirectPaymentAddressCounter{},
		&models.GuestCheckoutConfig{},
		&models.SyncedProductMapping{},
	))
	require.NoError(t, db.gormDB.Create(&models.DirectPaymentAddressCounter{
		TenantID: testTenantID,
		ID:       1,
		ChainKey: "LTC",
	}).Error)
	seedGuestExternalSupplyMapping(t, db, models.SyncedProductMapping{
		ID:            "spm-1",
		ProviderID:    "printful",
		ListingSlug:   "supplier-shirt",
		SyncProductID: "sync-123",
		Status:        "synced",
		LastSyncAt:    time.Now(),
	})

	recorder := &recordingSupplyAvailability{
		quoteResult: &contracts.SupplyQuoteResult{CanSell: true},
	}
	svc.resolver = alwaysEnabledResolver{}
	svc.directPayment = NewDirectPaymentService(db, fixedBIP44KeyDeriver{})
	svc.exchangeRates = wallet.NewFixedRateProvider("LTC", map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(8000),
	})
	svc.supplyAvailability = recorder
	svc.listings = &stubGuestListings{
		bySlug: map[string]*pb.SignedListing{
			"supplier-shirt": guestListingWithSku("supplier-shirt", "Color", "Red", "3", "1200"),
		},
	}

	require.NoError(t, svc.SaveGuestCheckoutConfig(context.Background(), &models.GuestCheckoutConfig{
		Enabled:       true,
		AcceptedCoins: "LTC",
	}))

	resp, err := svc.CreateGuestOrder(context.Background(), contracts.CreateGuestOrderRequest{
		PaymentCoin:     "LTC",
		ShippingAddress: map[string]string{"country": "US"},
		ShippingCountry: "US",
		Items: []contracts.GuestOrderItemRequest{{
			ListingSlug:     "supplier-shirt",
			Quantity:        1,
			ShippingOption:  "test-zone",
			ShippingService: "standard",
			Options: []map[string]string{
				{"Color": "Red"},
			},
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Len(t, recorder.quoteRequests, 1)
	require.Len(t, recorder.quoteRequests[0].Lines, 1)
	line := recorder.quoteRequests[0].Lines[0]
	require.Equal(t, contracts.SupplyKindExternalSupply, line.SupplyKind)
	require.Equal(t, "supplier-shirt", line.ListingSlug)
	require.Equal(t, "printful", line.ProviderID)
	require.Equal(t, "sync-123", line.ProviderRef)

	var reservations []models.InventoryReservation
	require.NoError(t, db.gormDB.Where("order_ref = ?", resp.OrderToken).Find(&reservations).Error)
	require.Len(t, reservations, 1, "non-transactional supply service still leaves legacy guest SKU reservation authoritative")
}

func TestCreateGuestOrder_AuthoritativeSupplyReserveUsesTransactionalService(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	db := svc.db.(*testDatabase)
	require.NoError(t, db.gormDB.AutoMigrate(
		&models.DirectPaymentAddressCounter{},
		&models.GuestCheckoutConfig{},
	))
	require.NoError(t, db.gormDB.Create(&models.DirectPaymentAddressCounter{
		TenantID: testTenantID,
		ID:       1,
		ChainKey: "LTC",
	}).Error)

	recorder := &transactionalRecordingSupplyAvailability{}
	svc.resolver = alwaysEnabledResolver{}
	svc.directPayment = NewDirectPaymentService(db, fixedBIP44KeyDeriver{})
	svc.exchangeRates = wallet.NewFixedRateProvider("LTC", map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(8000),
	})
	svc.supplyAvailability = recorder
	svc.listings = &stubGuestListings{
		bySlug: map[string]*pb.SignedListing{
			"camera": guestListingWithSku("camera", "Color", "Red", "3", "1200"),
		},
	}

	require.NoError(t, svc.SaveGuestCheckoutConfig(context.Background(), &models.GuestCheckoutConfig{
		Enabled:       true,
		AcceptedCoins: "LTC",
	}))

	resp, err := svc.CreateGuestOrder(context.Background(), contracts.CreateGuestOrderRequest{
		PaymentCoin:     "LTC",
		ShippingAddress: map[string]string{"country": "US"},
		ShippingCountry: "US",
		Items: []contracts.GuestOrderItemRequest{{
			ListingSlug:     "camera",
			Quantity:        1,
			ShippingOption:  "test-zone",
			ShippingService: "standard",
			Options: []map[string]string{
				{"Color": "Red"},
			},
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Len(t, recorder.quoteRequests, 1)
	require.Len(t, recorder.reserveTxRequests, 1)
	require.Equal(t, resp.OrderToken, recorder.reserveTxRequests[0].OrderRef)
	require.Equal(t, models.OrderTypeGuest, recorder.reserveTxRequests[0].OrderType)

	var reservations []models.InventoryReservation
	require.NoError(t, db.gormDB.Where("order_ref = ?", resp.OrderToken).Find(&reservations).Error)
	require.Len(t, reservations, 1)
	require.Equal(t, "camera", reservations[0].ListingSlug)
	require.Equal(t, recorder.reserveTxRequests[0].Lines[0].VariantHash, reservations[0].VariantHash)
	require.Equal(t, 1, reservations[0].Quantity)
}

func TestCreateGuestOrder_PreparesDigitalSupplyBeforeWriteTransaction(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	baseDB := svc.db.(*testDatabase)
	require.NoError(t, baseDB.gormDB.AutoMigrate(
		&models.DirectPaymentAddressCounter{},
		&models.GuestCheckoutConfig{},
	))
	require.NoError(t, baseDB.gormDB.Create(&models.DirectPaymentAddressCounter{
		TenantID: testTenantID,
		ID:       1,
		ChainKey: "LTC",
	}).Error)

	guardDB := &nestedViewGuardDatabase{Database: baseDB}
	listing := guestListingWithSku("ebook", "License", "Standard", "3", "1200")
	listing.Listing.Metadata.ContractType = pb.Listing_Metadata_DIGITAL_GOOD

	svc.db = guardDB
	svc.resolver = alwaysEnabledResolver{}
	svc.directPayment = NewDirectPaymentService(guardDB, fixedBIP44KeyDeriver{})
	svc.exchangeRates = wallet.NewFixedRateProvider("LTC", map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(8000),
	})
	svc.supplyAvailability = &transactionalRecordingSupplyAvailability{}
	svc.digitalSupplyLines = databaseReadingDigitalSupplyLineResolver{db: guardDB}
	svc.listings = &stubGuestListings{bySlug: map[string]*pb.SignedListing{"ebook": listing}}

	require.NoError(t, svc.SaveGuestCheckoutConfig(context.Background(), &models.GuestCheckoutConfig{
		Enabled:       true,
		AcceptedCoins: "LTC",
	}))

	resp, err := svc.CreateGuestOrder(context.Background(), contracts.CreateGuestOrderRequest{
		PaymentCoin: "LTC",
		Items: []contracts.GuestOrderItemRequest{{
			ListingSlug: "ebook",
			Quantity:    1,
			Options:     []map[string]string{{"License": "Standard"}},
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.False(t, guardDB.nestedView, "digital supply resolution must not open a nested DB view")
}

func TestCreateGuestOrder_AuthoritativeSupplyReserveSkipsExternalSyncedListing(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	db := svc.db.(*testDatabase)
	require.NoError(t, db.gormDB.AutoMigrate(
		&models.DirectPaymentAddressCounter{},
		&models.GuestCheckoutConfig{},
		&models.SyncedProductMapping{},
	))
	require.NoError(t, db.gormDB.Create(&models.DirectPaymentAddressCounter{
		TenantID: testTenantID,
		ID:       1,
		ChainKey: "LTC",
	}).Error)
	seedGuestExternalSupplyMapping(t, db, models.SyncedProductMapping{
		ID:            "spm-1",
		ProviderID:    "printful",
		ListingSlug:   "supplier-shirt",
		SyncProductID: "sync-123",
		Status:        "synced",
		LastSyncAt:    time.Now(),
	})

	recorder := &transactionalRecordingSupplyAvailability{}
	svc.resolver = alwaysEnabledResolver{}
	svc.directPayment = NewDirectPaymentService(db, fixedBIP44KeyDeriver{})
	svc.exchangeRates = wallet.NewFixedRateProvider("LTC", map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(8000),
	})
	svc.supplyAvailability = recorder
	svc.listings = &stubGuestListings{
		bySlug: map[string]*pb.SignedListing{
			"supplier-shirt": guestListingWithSku("supplier-shirt", "Color", "Red", "3", "1200"),
		},
	}

	require.NoError(t, svc.SaveGuestCheckoutConfig(context.Background(), &models.GuestCheckoutConfig{
		Enabled:       true,
		AcceptedCoins: "LTC",
	}))

	resp, err := svc.CreateGuestOrder(context.Background(), contracts.CreateGuestOrderRequest{
		PaymentCoin:     "LTC",
		ShippingAddress: map[string]string{"country": "US"},
		ShippingCountry: "US",
		Items: []contracts.GuestOrderItemRequest{{
			ListingSlug:     "supplier-shirt",
			Quantity:        1,
			ShippingOption:  "test-zone",
			ShippingService: "standard",
			Options: []map[string]string{
				{"Color": "Red"},
			},
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Empty(t, recorder.reserveTxRequests, "external-only synced listing must not enter authoritative reserve")

	var reservations []models.InventoryReservation
	require.NoError(t, db.gormDB.Where("order_ref = ?", resp.OrderToken).Find(&reservations).Error)
	require.Empty(t, reservations, "external-only listing must not fall back to legacy SKU reservation when authoritative path is active")
}

func TestGuestReservableSupplyLines_SkipExternalKeepSku(t *testing.T) {
	externalMappings := map[string]models.SyncedProductMapping{
		"supplier-shirt": {
			ProviderID:    "printful",
			SyncProductID: "sync-123",
		},
	}
	lines := guestSupplyLinesForItemsWithExternalMappings(
		[]models.GuestOrderItem{
			{OrderToken: "gst_test", ListingSlug: "supplier-shirt", Quantity: 1},
			{OrderToken: "gst_test", ListingSlug: "camera", Quantity: 1, VariantHash: "red"},
		},
		[]guestInventoryBucketKey{
			{Slug: "supplier-shirt", VariantHash: "vh1"},
			{Slug: "camera", VariantHash: "red"},
		},
		map[guestInventoryBucketKey]int64{
			{Slug: "camera", VariantHash: "red"}: 3,
		},
		externalMappings,
	)
	reservable, manualAction := contracts.PartitionReservableSupplyLines(lines)
	require.Len(t, manualAction, 1)
	require.Equal(t, contracts.SupplyKindExternalSupply, manualAction[0].SupplyKind)
	require.Len(t, reservable, 1)
	require.Equal(t, contracts.SupplyKindSkuQuantity, reservable[0].SupplyKind)
	require.Equal(t, "camera", reservable[0].ListingSlug)
}

func TestFinalizeGuestSupplyCommitBlocksFundedWhenAuthoritative(t *testing.T) {
	recorder := &transactionalRecordingSupplyAvailability{
		commitErr: errors.New("commit failed"),
	}
	svc := &GuestOrderAppService{
		resolver:           alwaysEnabledResolver{},
		supplyAvailability: recorder,
	}

	err := svc.finalizeGuestSupplyCommitInTx(context.Background(), nil, "gst_test")
	require.ErrorContains(t, err, "commit guest supply")
	require.ErrorContains(t, err, "commit failed")
	require.Empty(t, recorder.commitTxRequests)
}

func TestCommitGuestSupplyInTx_UsesAuthoritativeServiceWhenEnabled(t *testing.T) {
	db := newGuestTestDB(t)
	recorder := &transactionalRecordingSupplyAvailability{}
	svc := &GuestOrderAppService{
		db:                 db,
		resolver:           alwaysEnabledResolver{},
		supplyAvailability: recorder,
	}
	seedReservation(t, db, 1, models.InventoryReservation{
		OrderRef:    "gst_test",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "camera",
		VariantHash: "red",
		Quantity:    1,
		ReservedAt:  time.Now(),
		ExpiresAt:   time.Now().Add(time.Hour),
	})

	require.NoError(t, db.Update(func(tx database.Tx) error {
		return svc.commitGuestSupplyInTx(context.Background(), tx, "gst_test")
	}))

	require.Equal(t, []string{"gst_test"}, recorder.commitTxRequests)
	row := loadReservation(t, db, 1)
	require.True(t, row.Confirmed)
	require.Equal(t, 2099, row.ExpiresAt.Year())
}

func TestReleaseGuestSupplyInTx_UsesAuthoritativeServiceWhenEnabled(t *testing.T) {
	db := newGuestTestDB(t)
	recorder := &transactionalRecordingSupplyAvailability{}
	svc := &GuestOrderAppService{
		db:                 db,
		resolver:           alwaysEnabledResolver{},
		supplyAvailability: recorder,
	}
	seedReservation(t, db, 1, models.InventoryReservation{
		OrderRef:    "gst_test",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "camera",
		VariantHash: "red",
		Quantity:    1,
		ReservedAt:  time.Now(),
		ExpiresAt:   time.Now().Add(time.Hour),
	})

	require.NoError(t, db.Update(func(tx database.Tx) error {
		return svc.releaseGuestSupplyInTx(context.Background(), tx, "gst_test", "expired")
	}))

	require.Equal(t, []string{"expired"}, recorder.releaseTxRequests)
	row := loadReservation(t, db, 1)
	require.NotNil(t, row.ReleasedAt)
}

func guestListingWithSku(slug, option, variant, quantity, price string) *pb.SignedListing {
	listing := guestListing(slug, pb.Listing_Metadata_PHYSICAL_GOOD)
	listing.Listing.ShippingProfile = &pb.ShippingProfile{
		LocationGroups: []*pb.LocationGroup{{
			Zones: []*pb.ShippingZone{{
				Id:      "test-zone",
				Name:    "Worldwide",
				Regions: []string{"ALL"},
				Rates: []*pb.ShippingRate{{
					Id:       "standard",
					Name:     "Standard",
					Price:    "0",
					Currency: "USD",
				}},
			}},
		}},
	}
	listing.Listing.Item.Skus = []*pb.Listing_Item_Sku{{
		Selections: []*pb.Listing_Item_Sku_Selection{{
			Option:  option,
			Variant: variant,
		}},
		Quantity: quantity,
		Price:    price,
	}}
	return listing
}

type fixedBIP44KeyDeriver struct{}

func (fixedBIP44KeyDeriver) DeriveAddress(iwallet.ChainType, uint32) (string, error) {
	return "ltc1q_shadow_quote_test", nil
}

func (fixedBIP44KeyDeriver) DerivePrivateKey(iwallet.ChainType, uint32) ([]byte, error) {
	return []byte{1}, nil
}

func newGuestQuoteDigitalAssetService(t *testing.T, db *testDatabase) *digital.DigitalAssetAppService {
	t.Helper()
	require.NoError(t, db.gormDB.AutoMigrate(
		&models.DigitalAsset{},
		&models.DigitalLicenseKey{},
		&models.LicenseActivation{},
		&models.DownloadGrant{},
		&models.DigitalDownloadLog{},
	))
	return digital.NewDigitalAssetAppService(db, guestQuoteNoopBlobStore{}, guestQuoteKeyProvider{})
}

type providerBackedGuestSupplyAvailability struct {
	providers     map[contracts.SupplyKind]contracts.SupplyProvider
	quoteRequests []contracts.SupplyQuoteRequest
}

func newProviderBackedGuestSupplyAvailability(providers ...contracts.SupplyProvider) *providerBackedGuestSupplyAvailability {
	svc := &providerBackedGuestSupplyAvailability{
		providers: make(map[contracts.SupplyKind]contracts.SupplyProvider, len(providers)),
	}
	for _, provider := range providers {
		svc.providers[provider.Kind()] = provider
	}
	return svc
}

func (s *providerBackedGuestSupplyAvailability) Quote(ctx context.Context, req contracts.SupplyQuoteRequest) (*contracts.SupplyQuoteResult, error) {
	s.quoteRequests = append(s.quoteRequests, req)
	result := &contracts.SupplyQuoteResult{
		CanSell: true,
		Results: make([]contracts.AvailabilityResult, 0, len(req.Lines)),
	}
	for _, line := range req.Lines {
		provider := s.providers[line.SupplyKind]
		if provider == nil {
			return nil, contracts.ErrSupplyKindUnsupported
		}
		availability, err := provider.GetAvailability(ctx, contracts.AvailabilityRequest{Line: line})
		if err != nil {
			return nil, err
		}
		result.Results = append(result.Results, *availability)
		if availability.ManualActionRequired || availability.Status == contracts.SupplyAvailabilityManualActionRequired {
			result.ManualActionRequired = true
		}
		if !availability.Available || availability.ManualActionRequired {
			result.CanSell = false
		}
	}
	if result.ManualActionRequired {
		result.Reason = "manual_action_required"
	} else if !result.CanSell {
		result.Reason = "supply_unavailable"
	}
	return result, nil
}

func (*providerBackedGuestSupplyAvailability) ReserveOrder(context.Context, contracts.ReserveOrderSupplyRequest) (*contracts.ReserveOrderSupplyResult, error) {
	return &contracts.ReserveOrderSupplyResult{}, nil
}

func (*providerBackedGuestSupplyAvailability) CommitOrder(context.Context, string, string) error {
	return nil
}

func (*providerBackedGuestSupplyAvailability) ReleaseOrder(context.Context, string, string, string) error {
	return nil
}

type guestQuoteNoopBlobStore struct{}

func (guestQuoteNoopBlobStore) Put(context.Context, string, []byte, string) error {
	return nil
}

func (guestQuoteNoopBlobStore) PutStream(_ context.Context, _ string, r io.Reader, _ int64, _ string) error {
	_, err := io.Copy(io.Discard, r)
	return err
}

func (guestQuoteNoopBlobStore) Get(context.Context, string) (io.ReadCloser, string, error) {
	return nil, "", contracts.ErrBlobNotFound
}

func (guestQuoteNoopBlobStore) Exists(context.Context, string) (bool, error) {
	return false, nil
}

func (guestQuoteNoopBlobStore) PublicURL(string) string { return "" }

type guestQuoteKeyProvider struct{}

func (guestQuoteKeyProvider) DigitalContentMasterKey(int) ([]byte, error) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return key, nil
}

func (guestQuoteKeyProvider) EVMMasterKey() (*btcec.PrivateKey, error)     { return nil, nil }
func (guestQuoteKeyProvider) SolanaMasterKey() (*solana.PrivateKey, error) { return nil, nil }
func (guestQuoteKeyProvider) EscrowMasterKey() (*btcec.PrivateKey, error)  { return nil, nil }
func (guestQuoteKeyProvider) RatingMasterKey() (*btcec.PrivateKey, error)  { return nil, nil }
func (guestQuoteKeyProvider) TRONMasterKey() (*btcec.PrivateKey, error)    { return nil, nil }

var _ contracts.SupplyAvailabilityService = (*providerBackedGuestSupplyAvailability)(nil)

type recordingSupplyAvailability struct {
	quoteRequests []contracts.SupplyQuoteRequest
	quoteResult   *contracts.SupplyQuoteResult
	quoteErr      error
}

type recordingGuestDigitalSupplyLineResolver struct {
	items [][]digital.OrderLineItem
	lines []contracts.SupplyLine
	err   error
}

type nestedViewGuardDatabase struct {
	database.Database
	inUpdate   bool
	nestedView bool
}

func (db *nestedViewGuardDatabase) View(fn func(database.Tx) error) error {
	if db.inUpdate {
		db.nestedView = true
		return errors.New("nested Database.View during Database.Update")
	}
	return db.Database.View(fn)
}

func (db *nestedViewGuardDatabase) Update(fn func(database.Tx) error) error {
	db.inUpdate = true
	defer func() { db.inUpdate = false }()
	return db.Database.Update(fn)
}

type databaseReadingDigitalSupplyLineResolver struct {
	db database.Database
}

func (resolver databaseReadingDigitalSupplyLineResolver) SupplyAvailabilityLinesForOrderItems(
	items []digital.OrderLineItem,
) ([]contracts.SupplyLine, error) {
	if err := resolver.db.View(func(database.Tx) error { return nil }); err != nil {
		return nil, err
	}
	return []contracts.SupplyLine{{
		LineID:      "digital:0:ebook:unlimited_digital",
		ListingSlug: items[0].ListingSlug,
		Quantity:    int(items[0].Quantity),
		SupplyKind:  contracts.SupplyKindUnlimitedDigital,
	}}, nil
}

func (r *recordingGuestDigitalSupplyLineResolver) SupplyAvailabilityLinesForOrderItems(items []digital.OrderLineItem) ([]contracts.SupplyLine, error) {
	r.items = append(r.items, append([]digital.OrderLineItem(nil), items...))
	if r.err != nil {
		return nil, r.err
	}
	lines := make([]contracts.SupplyLine, len(r.lines))
	copy(lines, r.lines)
	return lines, nil
}

func (r *recordingSupplyAvailability) Quote(_ context.Context, req contracts.SupplyQuoteRequest) (*contracts.SupplyQuoteResult, error) {
	r.quoteRequests = append(r.quoteRequests, req)
	if r.quoteErr != nil {
		return nil, r.quoteErr
	}
	if r.quoteResult != nil {
		return r.quoteResult, nil
	}
	return &contracts.SupplyQuoteResult{CanSell: true}, nil
}

func (r *recordingSupplyAvailability) ReserveOrder(context.Context, contracts.ReserveOrderSupplyRequest) (*contracts.ReserveOrderSupplyResult, error) {
	return &contracts.ReserveOrderSupplyResult{}, nil
}

func (r *recordingSupplyAvailability) CommitOrder(context.Context, string, string) error {
	return nil
}

func (r *recordingSupplyAvailability) ReleaseOrder(context.Context, string, string, string) error {
	return nil
}

var _ contracts.SupplyAvailabilityService = (*recordingSupplyAvailability)(nil)

type transactionalRecordingSupplyAvailability struct {
	recordingSupplyAvailability
	reserveTxRequests []contracts.ReserveOrderSupplyRequest
	commitTxRequests  []string
	releaseTxRequests []string
	commitErr         error
}

func (r *transactionalRecordingSupplyAvailability) ReserveOrderTx(_ context.Context, tx database.Tx, req contracts.ReserveOrderSupplyRequest) (*contracts.ReserveOrderSupplyResult, error) {
	r.reserveTxRequests = append(r.reserveTxRequests, req)
	result := &contracts.ReserveOrderSupplyResult{
		Reservations: make([]contracts.SupplyReservation, 0, len(req.Lines)),
	}
	for i, line := range req.Lines {
		row := models.InventoryReservation{
			ID: 1000 + i,
			TenantMixin: models.TenantMixin{
				TenantID: testTenantID,
			},
			OrderRef:    req.OrderRef,
			OrderType:   req.OrderType,
			ListingSlug: line.ListingSlug,
			VariantHash: line.VariantHash,
			Quantity:    line.Quantity,
			ReservedAt:  time.Now(),
			ExpiresAt:   req.ExpiresAt,
		}
		if err := tx.Save(&row); err != nil {
			return nil, err
		}
		result.Reservations = append(result.Reservations, contracts.SupplyReservation{
			ID:          strconv.Itoa(row.ID),
			OrderRef:    row.OrderRef,
			OrderType:   row.OrderType,
			LineID:      line.LineID,
			SupplyKind:  line.SupplyKind,
			ListingSlug: row.ListingSlug,
			VariantHash: row.VariantHash,
			Quantity:    row.Quantity,
			Status:      contracts.SupplyReservationReserved,
			ExpiresAt:   row.ExpiresAt,
		})
	}
	return result, nil
}

func (r *transactionalRecordingSupplyAvailability) CommitOrderTx(_ context.Context, tx database.Tx, orderRef string, orderType string) error {
	if r.commitErr != nil {
		return r.commitErr
	}
	r.commitTxRequests = append(r.commitTxRequests, orderRef)
	farFuture := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	var reservations []models.InventoryReservation
	if err := tx.Read().Where("order_ref = ? AND order_type = ? AND released_at IS NULL",
		orderRef, orderType).Find(&reservations).Error; err != nil {
		return err
	}
	for i := range reservations {
		reservations[i].Confirmed = true
		reservations[i].ExpiresAt = farFuture
		if err := tx.Save(&reservations[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *transactionalRecordingSupplyAvailability) ReleaseOrderTx(_ context.Context, tx database.Tx, orderRef string, orderType string, reason string) error {
	r.releaseTxRequests = append(r.releaseTxRequests, reason)
	now := time.Now()
	var reservations []models.InventoryReservation
	if err := tx.Read().Where("order_ref = ? AND order_type = ? AND released_at IS NULL",
		orderRef, orderType).Find(&reservations).Error; err != nil {
		return err
	}
	for i := range reservations {
		reservations[i].ReleasedAt = &now
		if err := tx.Save(&reservations[i]); err != nil {
			return err
		}
	}
	return nil
}

func TestQuoteSupplyAvailabilityShadowIgnoresNilService(t *testing.T) {
	svc := &GuestOrderAppService{}
	require.NotPanics(t, func() {
		svc.quoteSupplyAvailabilityShadow(
			context.Background(),
			"gst_test",
			[]models.GuestOrderItem{{
				OrderToken:  "gst_test",
				ListingSlug: "camera",
				VariantHash: "red",
				Quantity:    1,
			}},
			[]guestInventoryBucketKey{{Slug: "camera", VariantHash: "red"}},
			map[guestInventoryBucketKey]int64{},
		)
	})
}

func TestQuoteSupplyAvailabilityShadowSkipsWhenFeatureDisabled(t *testing.T) {
	recorder := &recordingSupplyAvailability{
		quoteResult: &contracts.SupplyQuoteResult{CanSell: true},
	}
	svc := &GuestOrderAppService{
		supplyAvailability: recorder,
		resolver:           disabledSupplyAvailabilityResolver{},
	}

	svc.quoteSupplyAvailabilityShadow(
		context.Background(),
		"gst_test",
		[]models.GuestOrderItem{{
			OrderToken:  "gst_test",
			ListingSlug: "camera",
			VariantHash: "red",
			Quantity:    1,
		}},
		[]guestInventoryBucketKey{{Slug: "camera", VariantHash: "red"}},
		map[guestInventoryBucketKey]int64{},
	)

	require.Empty(t, recorder.quoteRequests)
}

type disabledSupplyAvailabilityResolver struct{}

func (disabledSupplyAvailabilityResolver) IsEnabled(context.Context, string) bool { return false }

func (disabledSupplyAvailabilityResolver) Evaluate(context.Context, string) pkgconfig.Evaluation {
	return pkgconfig.Evaluation{Enabled: false}
}

func (disabledSupplyAvailabilityResolver) List(context.Context) []pkgconfig.EffectiveFeature {
	return nil
}

type guestCheckoutOnlyResolver struct{}

func (guestCheckoutOnlyResolver) IsEnabled(_ context.Context, key string) bool {
	return key == pkgconfig.FeatureGuestCheckoutEnabled.Key
}

func (guestCheckoutOnlyResolver) Evaluate(_ context.Context, key string) pkgconfig.Evaluation {
	return pkgconfig.Evaluation{Key: key, Enabled: key == pkgconfig.FeatureGuestCheckoutEnabled.Key}
}

func (guestCheckoutOnlyResolver) List(context.Context) []pkgconfig.EffectiveFeature {
	return nil
}

func seedGuestExternalSupplyMapping(t *testing.T, db *testDatabase, mapping models.SyncedProductMapping) {
	t.Helper()
	if mapping.TenantID == "" {
		mapping.TenantID = testTenantID
	}
	require.NoError(t, db.gormDB.Create(&mapping).Error)
}

package guest

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/wallet"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
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
		PaymentCoin: "LTC",
		Items: []contracts.GuestOrderItemRequest{{
			ListingSlug: "camera",
			Quantity:    1,
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
		PaymentCoin: "LTC",
		Items: []contracts.GuestOrderItemRequest{{
			ListingSlug: "supplier-shirt",
			Quantity:    1,
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
		PaymentCoin: "LTC",
		Items: []contracts.GuestOrderItemRequest{{
			ListingSlug: "camera",
			Quantity:    1,
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
		PaymentCoin: "LTC",
		Items: []contracts.GuestOrderItemRequest{{
			ListingSlug: "supplier-shirt",
			Quantity:    1,
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

type recordingSupplyAvailability struct {
	quoteRequests []contracts.SupplyQuoteRequest
	quoteResult   *contracts.SupplyQuoteResult
	quoteErr      error
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

func seedGuestExternalSupplyMapping(t *testing.T, db *testDatabase, mapping models.SyncedProductMapping) {
	t.Helper()
	if mapping.TenantID == "" {
		mapping.TenantID = testTenantID
	}
	require.NoError(t, db.gormDB.Create(&mapping).Error)
}

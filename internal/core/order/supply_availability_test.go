package order

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mobazha/mobazha/internal/core/digital"
	"github.com/mobazha/mobazha/internal/orders"
	"github.com/mobazha/mobazha/internal/orders/utils"
	pkgconfig "github.com/mobazha/mobazha/pkg/config"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSelectedStandardOrderSku_ResolvesSingleSkuWithoutVariantOptions(t *testing.T) {
	listing := orderSupplyListingNoVariant(t, "simple-shirt", "5")
	sku, err := selectedStandardOrderSku(listing.Listing, nil)
	require.NoError(t, err)
	require.NotNil(t, sku)
	require.Equal(t, "5", sku.Quantity)
}

func TestSelectedStandardOrderSku_ReturnsNilWhenListingHasNoSkus(t *testing.T) {
	listing := orderSupplyListingNoVariant(t, "empty-shirt", "5")
	listing.Listing.Item.Skus = nil
	sku, err := selectedStandardOrderSku(listing.Listing, nil)
	require.NoError(t, err)
	require.Nil(t, sku)
}

func TestSelectedStandardOrderSku_ReturnsErrorWhenVariantListingHasNoSelection(t *testing.T) {
	listing := orderSupplyListing(t, "camera", "Color", "Red", "3")
	_, err := selectedStandardOrderSku(listing.Listing, nil)
	require.ErrorContains(t, err, "selected sku not found")
}

func TestMatchingLocalSku_MatchesByProductID(t *testing.T) {
	embedded := orderSupplyListingNoVariant(t, "shirt", "5")
	embedded.Listing.Item.Skus[0].ProductID = "sku-001"

	local := proto.Clone(embedded.Listing).(*pb.Listing)
	local.Item.Skus[0].Quantity = "8"

	matched := matchingLocalSku(local, embedded.Listing.Item.Skus[0])
	require.NotNil(t, matched)
	require.Equal(t, "8", matched.Quantity)
}

func TestMatchingLocalSku_DoesNotMatchSelectionlessCandidateForVariant(t *testing.T) {
	embedded := orderSupplyListing(t, "shirt", "Size", "M", "")
	local := orderSupplyListing(t, "shirt", "Size", "S", "2")
	local.Listing.Item.Skus = append([]*pb.Listing_Item_Sku{
		{Quantity: "99"},
	}, local.Listing.Item.Skus...)

	matched := matchingLocalSku(local.Listing, embedded.Listing.Item.Skus[0])
	require.Nil(t, matched)
}

func TestAuthoritativeStandardStockSku_UsesLocalWhenEmbeddedSkuOmitsQuantity(t *testing.T) {
	local := orderSupplyListingNoVariant(t, "shirt", "8")
	embedded := proto.Clone(local.Listing).(*pb.Listing)
	embedded.Item.Skus[0].Quantity = ""
	embedded.Item.Skus[0].ProductID = ""

	stockSku := authoritativeStandardStockSku(local.Listing, embedded.Item.Skus[0], nil)
	require.NotNil(t, stockSku)
	require.Equal(t, "8", stockSku.Quantity)
}

func TestPostProcessOrderOpen_UsesLocalVariantHashWhenEmbeddedSkuOnlyHasProductID(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})
	local := orderSupplyListing(t, "variant-shirt", "Size", "M", "7")
	local.Listing.Item.Skus[0].ProductID = "sku-m"
	embedded := orderSupplyListingNoVariant(t, "variant-shirt", "")
	embedded.Listing.Item.Skus[0].ProductID = "sku-m"
	listingHash := orderSupplyListingHash(t, embedded)

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		if err := tx.SetListing(local); err != nil {
			return err
		}
		order := &models.Order{
			ID:     models.OrderID("order-embedded-product-id-only"),
			MyRole: string(models.RoleVendor),
			Open:   true,
		}
		exp := time.Now().Add(time.Hour)
		order.ExpiresAt = &exp
		order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
		if err := tx.Save(order); err != nil {
			return err
		}
		return svc.postProcessOrderOpenInTx(tx, orderSupplyOrderOpenMessageNoOptions(t, "order-embedded-product-id-only", embedded, listingHash), nil)
	}))

	require.Len(t, recorder.reserveTxRequests, 1)
	require.NotEmpty(t, recorder.reserveTxRequests[0].Lines[0].VariantHash)
	require.Equal(t, int64(7), recorder.reserveTxRequests[0].Lines[0].StockLimit)
}

func TestPostProcessOrderOpen_ReservesWhenOrderListingOmitsSkuQuantity(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})
	local := orderSupplyListingNoVariant(t, "public-shirt", "5")
	orderEmbed := proto.Clone(local).(*pb.SignedListing)
	orderEmbed.Listing.Item.Skus[0].Quantity = ""
	orderEmbed.Listing.Item.Skus[0].ProductID = ""
	listingHash := orderSupplyListingHash(t, orderEmbed)

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		if err := tx.SetListing(local); err != nil {
			return err
		}
		order := &models.Order{
			ID:     models.OrderID("order-public-embed-no-qty"),
			MyRole: string(models.RoleVendor),
			Open:   true,
		}
		exp := time.Now().Add(time.Hour)
		order.ExpiresAt = &exp
		order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
		if err := tx.Save(order); err != nil {
			return err
		}
		return svc.postProcessOrderOpenInTx(tx, orderSupplyOrderOpenMessageNoOptions(t, "order-public-embed-no-qty", orderEmbed, listingHash), nil)
	}))

	require.Len(t, recorder.reserveTxRequests, 1)
	require.Equal(t, int64(5), recorder.reserveTxRequests[0].Lines[0].StockLimit)
}

func TestPostProcessOrderOpen_ReservesUsingEmbeddedSkuWithLocalStockLimit(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})
	embedded := orderSupplyListingNoVariant(t, "embedded-shirt", "5")
	embedded.Listing.Item.Skus[0].ProductID = "sku-001"
	local := proto.Clone(embedded).(*pb.SignedListing)
	local.Listing.Item.Skus[0].Quantity = "8"
	listingHash := orderSupplyListingHash(t, embedded)

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		if err := tx.SetListing(local); err != nil {
			return err
		}
		order := &models.Order{
			ID:     models.OrderID("order-embedded-local-stock"),
			MyRole: string(models.RoleVendor),
			Open:   true,
		}
		exp := time.Now().Add(time.Hour)
		order.ExpiresAt = &exp
		order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
		if err := tx.Save(order); err != nil {
			return err
		}
		return svc.postProcessOrderOpenInTx(tx, orderSupplyOrderOpenMessageNoOptions(t, "order-embedded-local-stock", embedded, listingHash), nil)
	}))

	require.Len(t, recorder.reserveTxRequests, 1)
	require.Equal(t, int64(8), recorder.reserveTxRequests[0].Lines[0].StockLimit)
}

func TestPostProcessOrderOpen_ReservesStandardOrderSupplyForVendorWithoutVariantOptions(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})
	listing := orderSupplyListingNoVariant(t, "simple-shirt", "5")
	listingHash := orderSupplyListingHash(t, listing)

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		if err := tx.SetListing(listing); err != nil {
			return err
		}
		order := &models.Order{
			ID:     models.OrderID("order-standard-no-variant"),
			MyRole: string(models.RoleVendor),
			Open:   true,
		}
		exp := time.Now().Add(time.Hour)
		order.ExpiresAt = &exp
		order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
		if err := tx.Save(order); err != nil {
			return err
		}
		return svc.postProcessOrderOpenInTx(tx, orderSupplyOrderOpenMessageNoOptions(t, "order-standard-no-variant", listing, listingHash), nil)
	}))

	require.Len(t, recorder.reserveTxRequests, 1)
	req := recorder.reserveTxRequests[0]
	require.Equal(t, "order-standard-no-variant", req.OrderRef)
	require.Equal(t, models.OrderTypeStandard, req.OrderType)
	require.Len(t, req.Lines, 1)
	require.Equal(t, contracts.SupplyKindSkuQuantity, req.Lines[0].SupplyKind)
	require.Equal(t, "simple-shirt", req.Lines[0].ListingSlug)
	require.Equal(t, 1, req.Lines[0].Quantity)
	require.Equal(t, int64(5), req.Lines[0].StockLimit)
	require.True(t, req.Lines[0].StockTracked)
	require.Empty(t, req.Lines[0].VariantHash)
}

func TestPostProcessOrderOpen_ReservesWhenEmbeddedListingHasNoSkus(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})
	local := orderSupplyListingNoVariant(t, "local-shirt", "5")
	embedded := proto.Clone(local).(*pb.SignedListing)
	embedded.Listing.Item.Skus = nil
	listingHash := orderSupplyListingHash(t, embedded)

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		if err := tx.SetListing(local); err != nil {
			return err
		}
		order := &models.Order{
			ID:     models.OrderID("order-embedded-no-skus"),
			MyRole: string(models.RoleVendor),
			Open:   true,
		}
		exp := time.Now().Add(time.Hour)
		order.ExpiresAt = &exp
		order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
		if err := tx.Save(order); err != nil {
			return err
		}
		return svc.postProcessOrderOpenInTx(tx, orderSupplyOrderOpenMessageNoOptions(t, "order-embedded-no-skus", embedded, listingHash), nil)
	}))

	require.Len(t, recorder.reserveTxRequests, 1)
	require.Equal(t, int64(5), recorder.reserveTxRequests[0].Lines[0].StockLimit)
}

func TestPostProcessOrderOpen_ReservesStandardOrderSupplyForVendor(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})
	listing := orderSupplyListing(t, "camera", "Color", "Red", "3")
	listingHash := orderSupplyListingHash(t, listing)

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		if err := tx.SetListing(listing); err != nil {
			return err
		}
		order := &models.Order{
			ID:     models.OrderID("order-standard-1"),
			MyRole: string(models.RoleVendor),
			Open:   true,
		}
		exp := time.Now().Add(time.Hour)
		order.ExpiresAt = &exp
		order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
		if err := tx.Save(order); err != nil {
			return err
		}
		return svc.postProcessOrderOpenInTx(tx, orderSupplyOrderOpenMessage(t, "order-standard-1", listing, listingHash), nil)
	}))

	require.Len(t, recorder.reserveTxRequests, 1)
	req := recorder.reserveTxRequests[0]
	require.Equal(t, "order-standard-1", req.OrderRef)
	require.Equal(t, models.OrderTypeStandard, req.OrderType)
	require.Len(t, req.Lines, 1)
	require.Equal(t, contracts.SupplyKindSkuQuantity, req.Lines[0].SupplyKind)
	require.Equal(t, "camera", req.Lines[0].ListingSlug)
	require.Equal(t, 1, req.Lines[0].Quantity)
	require.True(t, req.Lines[0].StockTracked)
	require.Equal(t, int64(3), req.Lines[0].StockLimit)
	require.NotEmpty(t, req.Lines[0].VariantHash)
}

func TestPostProcessOrderOpen_LeavesExternalSupplyLineForManualAction(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})
	listing := orderSupplyListing(t, "supplier-shirt", "Color", "Red", "3")
	listingHash := orderSupplyListingHash(t, listing)

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.SyncedProductMapping{}); err != nil {
			return err
		}
		if err := tx.Save(&models.SyncedProductMapping{
			ID:            "spm-1",
			ProviderID:    "printful",
			ListingSlug:   "supplier-shirt",
			SyncProductID: "sync-123",
			Status:        "synced",
			LastSyncAt:    time.Now(),
		}); err != nil {
			return err
		}
		order := &models.Order{
			ID:     models.OrderID("order-standard-external"),
			MyRole: string(models.RoleVendor),
			Open:   true,
		}
		exp := time.Now().Add(time.Hour)
		order.ExpiresAt = &exp
		order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
		if err := tx.Save(order); err != nil {
			return err
		}
		lines, err := standardOrderSupplyLinesFromOrderOpen(tx, "order-standard-external", orderSupplyOrderOpen(listing, listingHash), nil, false)
		require.NoError(t, err)
		require.Len(t, lines, 1)
		require.Equal(t, contracts.SupplyKindExternalSupply, lines[0].SupplyKind)
		require.Equal(t, "supplier-shirt", lines[0].ListingSlug)
		require.Equal(t, 1, lines[0].Quantity)
		require.Equal(t, "printful", lines[0].ProviderID)
		require.Equal(t, "sync-123", lines[0].ProviderRef)
		require.False(t, lines[0].StockTracked)
		require.Empty(t, lines[0].VariantHash)
		return svc.postProcessOrderOpenInTx(tx, orderSupplyOrderOpenMessage(t, "order-standard-external", listing, listingHash), nil)
	}))

	require.Empty(t, recorder.reserveTxRequests)
}

func TestStandardOrderSupplyLinesUsesDigitalResolverForDigitalGoods(t *testing.T) {
	resolver := &recordingDigitalSupplyLineResolver{
		lines: []contracts.SupplyLine{{
			LineID:      "digital:0:ebook:license_key_pool",
			ListingSlug: "ebook",
			Quantity:    2,
			SupplyKind:  contracts.SupplyKindLicenseKeyPool,
		}},
	}
	listing := orderSupplyListing(t, "ebook", "Color", "Red", "")
	listing.Listing.Metadata.ContractType = pb.Listing_Metadata_DIGITAL_GOOD
	listing.Listing.Item.Skus[0].ProductID = "sku-red"
	listingHash := orderSupplyListingHash(t, listing)

	lines, err := standardOrderSupplyLinesFromOrderOpen(nil, "order-standard-digital", orderSupplyOrderOpenWithQuantity(listing, listingHash, "2"), resolver, false)
	require.NoError(t, err)
	require.Len(t, resolver.items, 1)
	require.Equal(t, "ebook", resolver.items[0][0].ListingSlug)
	require.Equal(t, "sku-red", resolver.items[0][0].VariantSKU)
	require.Equal(t, uint32(2), resolver.items[0][0].Quantity)
	require.Equal(t, []contracts.SupplyLine{{
		LineID:      "digital:0:ebook:license_key_pool",
		ListingSlug: "ebook",
		Quantity:    2,
		SupplyKind:  contracts.SupplyKindLicenseKeyPool,
	}}, lines)
}

func TestStandardOrderSupplyLinesUsesExistingTransactionForDigitalGoods(t *testing.T) {
	resolver := &recordingDigitalSupplyLineResolver{
		lines: []contracts.SupplyLine{{
			LineID:      "digital:0:ebook:unlimited_digital",
			ListingSlug: "ebook",
			Quantity:    1,
			SupplyKind:  contracts.SupplyKindUnlimitedDigital,
		}},
	}
	listing := orderSupplyListingNoVariant(t, "ebook", "")
	listing.Listing.Metadata.ContractType = pb.Listing_Metadata_DIGITAL_GOOD
	listingHash := orderSupplyListingHash(t, listing)
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})

	require.NoError(t, svc.db.View(func(tx database.Tx) error {
		lines, err := standardOrderSupplyLinesFromOrderOpen(tx, "order-standard-digital-tx", orderSupplyOrderOpen(listing, listingHash), resolver, false)
		require.NoError(t, err)
		require.Len(t, lines, 1)
		return nil
	}))
	require.True(t, resolver.txSeen)
	require.Len(t, resolver.txItems, 1)
	require.Empty(t, resolver.items, "transactional resolution must not re-enter Database.View")
}

func TestStandardOrderSupplyLinesRejectsNonTransactionalResolverInsideWrite(t *testing.T) {
	listing := orderSupplyListingNoVariant(t, "ebook", "")
	listing.Listing.Metadata.ContractType = pb.Listing_Metadata_DIGITAL_GOOD
	listingHash := orderSupplyListingHash(t, listing)
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})

	err := svc.db.Update(func(tx database.Tx) error {
		_, err := standardOrderSupplyLinesFromOrderOpen(
			tx,
			"order-standard-digital-tx",
			orderSupplyOrderOpen(listing, listingHash),
			nonTransactionalDigitalSupplyLineResolver{},
			true,
		)
		return err
	})
	require.ErrorContains(t, err, "does not support caller transactions")
}

func TestStandardOrderSupplyLinesSkipsDigitalGoodsWithoutResolver(t *testing.T) {
	listing := orderSupplyListingNoVariant(t, "ebook", "")
	listing.Listing.Metadata.ContractType = pb.Listing_Metadata_DIGITAL_GOOD
	listingHash := orderSupplyListingHash(t, listing)

	lines, err := standardOrderSupplyLinesFromOrderOpen(nil, "order-standard-digital", orderSupplyOrderOpen(listing, listingHash), nil, false)
	require.NoError(t, err)
	require.Empty(t, lines)
}

func TestStandardOrderSupplyLinesFailsDigitalGoodsWithoutResolverWhenRequired(t *testing.T) {
	listing := orderSupplyListingNoVariant(t, "ebook", "")
	listing.Listing.Metadata.ContractType = pb.Listing_Metadata_DIGITAL_GOOD
	listingHash := orderSupplyListingHash(t, listing)

	_, err := standardOrderSupplyLinesFromOrderOpen(nil, "order-standard-digital", orderSupplyOrderOpen(listing, listingHash), nil, true)
	require.ErrorContains(t, err, "digital supply resolver unavailable")
}

func TestStandardOrderSupplyLinesReturnsDigitalResolverError(t *testing.T) {
	resolver := &recordingDigitalSupplyLineResolver{err: fmt.Errorf("digital resolver offline")}
	listing := orderSupplyListingNoVariant(t, "ebook", "")
	listing.Listing.Metadata.ContractType = pb.Listing_Metadata_DIGITAL_GOOD
	listingHash := orderSupplyListingHash(t, listing)

	_, err := standardOrderSupplyLinesFromOrderOpen(nil, "order-standard-digital", orderSupplyOrderOpen(listing, listingHash), resolver, false)
	require.ErrorContains(t, err, "digital resolver offline")
}

func TestPostProcessOrderOpen_SkipsClosedOrDeclinedStandardOrder(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})
	listing := orderSupplyListing(t, "camera", "Color", "Red", "3")
	listingHash := orderSupplyListingHash(t, listing)

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		if err := tx.SetListing(listing); err != nil {
			return err
		}
		order := &models.Order{
			ID:                     models.OrderID("order-standard-declined"),
			MyRole:                 string(models.RoleVendor),
			Open:                   false,
			SerializedOrderDecline: []byte{1},
			SerializedOrderOpen:    []byte{1},
		}
		if err := tx.Save(order); err != nil {
			return err
		}
		return svc.postProcessOrderOpenInTx(tx, orderSupplyOrderOpenMessage(t, "order-standard-declined", listing, listingHash), nil)
	}))

	require.Empty(t, recorder.reserveTxRequests)
}

func TestStandardOrderSupplyLifecycle_CommitAndReleaseUseTransactionalService(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		order := &models.Order{
			ID:     models.OrderID("order-standard-lifecycle"),
			MyRole: string(models.RoleVendor),
			Open:   true,
		}
		if err := tx.Save(order); err != nil {
			return err
		}
		if err := svc.commitStandardOrderSupplyInTx(tx, "order-standard-lifecycle"); err != nil {
			return err
		}
		return svc.releaseStandardOrderSupplyInTx(tx, "order-standard-lifecycle", "cancelled")
	}))

	require.Len(t, recorder.commitTxRequests, 1)
	require.Equal(t, standardOrderSupplyCommitRequest{
		orderRef:  "order-standard-lifecycle",
		orderType: models.OrderTypeStandard,
		txSeen:    true,
	}, recorder.commitTxRequests[0])
	require.Len(t, recorder.releaseTxRequests, 1)
	require.Equal(t, standardOrderSupplyReleaseRequest{
		orderRef:  "order-standard-lifecycle",
		orderType: models.OrderTypeStandard,
		reason:    "cancelled",
		txSeen:    true,
	}, recorder.releaseTxRequests[0])
}

func TestPostProcessInTx_DispatchesStandardOrderSupplyLifecycle(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		order := &models.Order{
			ID:     models.OrderID("order-standard-dispatch"),
			MyRole: string(models.RoleVendor),
			Open:   true,
		}
		if err := tx.Save(order); err != nil {
			return err
		}
		for _, msgType := range []npb.OrderMessage_MessageType{
			npb.OrderMessage_ORDER_CONFIRMATION,
			npb.OrderMessage_ORDER_CANCEL,
			npb.OrderMessage_ORDER_DECLINE,
		} {
			msg := &npb.OrderMessage{
				OrderID:     "order-standard-dispatch",
				MessageType: msgType,
			}
			if err := svc.postProcessInTx(tx, msg, nil, &models.Order{}); err != nil {
				return err
			}
		}
		return nil
	}))

	require.Len(t, recorder.commitTxRequests, 1)
	require.Equal(t, "order-standard-dispatch", recorder.commitTxRequests[0].orderRef)
	require.Equal(t, models.OrderTypeStandard, recorder.commitTxRequests[0].orderType)
	require.Len(t, recorder.releaseTxRequests, 2)
	require.Equal(t, "cancelled", recorder.releaseTxRequests[0].reason)
	require.Equal(t, "declined", recorder.releaseTxRequests[1].reason)
}

func TestDeclineOrder_ReleasesStandardOrderSupplyHold(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	privKey, _, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	marshaledKey, err := libp2pcrypto.MarshalPrivateKey(privKey)
	require.NoError(t, err)
	signer, err := contracts.NewKeyPairSignerFromMarshaledKey(marshaledKey)
	require.NoError(t, err)

	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Signer:             signer,
		Messenger:          noopMessenger{},
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})
	svc.orderProcessor = orders.NewOrderProcessor(&orders.Config{
		Db:        svc.db,
		Signer:    signer,
		Messenger: noopMessenger{},
		EventBus:  svc.eventBus,
	})

	listing := orderSupplyListingNoVariant(t, "decline-shirt", "5")
	listing.Listing.VendorID = &pb.ID{PeerID: signer.PeerID().String()}
	listing.Listing.Item.Images = []*pb.Image{{Tiny: "tiny", Small: "small"}}
	listingHash := orderSupplyListingHash(t, listing)
	orderOpen := orderSupplyOrderOpenNoOptions(listing, listingHash)
	orderOpen.BuyerID = &pb.ID{PeerID: "12D3KooWG1kQhGJSYgbPibZVUcny4U28tRZZSStDq8FdpbUoGsah"}
	serializedOrderOpen, err := protojson.Marshal(orderOpen)
	require.NoError(t, err)

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		order := &models.Order{
			ID:                  models.OrderID("order-local-decline-release"),
			MyRole:              string(models.RoleVendor),
			Open:                true,
			SerializedOrderOpen: serializedOrderOpen,
		}
		exp := time.Now().Add(time.Hour)
		order.ExpiresAt = &exp
		order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
		return tx.Save(order)
	}))

	require.NoError(t, svc.DeclineOrder("order-local-decline-release", "", "seller declined", nil))

	require.Len(t, recorder.releaseTxRequests, 1)
	require.Equal(t, standardOrderSupplyReleaseRequest{
		orderRef:  "order-local-decline-release",
		orderType: models.OrderTypeStandard,
		reason:    "declined",
		txSeen:    true,
	}, recorder.releaseTxRequests[0])
}

func TestStandardOrderSupplyLifecycle_SkipsBuyerRole(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		order := &models.Order{
			ID:     models.OrderID("order-standard-buyer"),
			MyRole: string(models.RoleBuyer),
			Open:   true,
		}
		if err := tx.Save(order); err != nil {
			return err
		}
		if err := svc.commitStandardOrderSupplyInTx(tx, "order-standard-buyer"); err != nil {
			return err
		}
		return svc.releaseStandardOrderSupplyInTx(tx, "order-standard-buyer", "cancelled")
	}))

	require.Empty(t, recorder.commitTxRequests)
	require.Empty(t, recorder.releaseTxRequests)
}

func TestStandardOrderSupplyLifecycle_UsesVendorRowWhenBuyerAndVendorShareOrderID(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		buyerOrder := &models.Order{
			TenantMixin: models.TenantMixin{TenantID: "buyer-tenant"},
			ID:          models.OrderID("order-standard-shared"),
			MyRole:      string(models.RoleBuyer),
			Open:        false,
		}
		if err := tx.Save(buyerOrder); err != nil {
			return err
		}
		vendorOrder := &models.Order{
			TenantMixin: models.TenantMixin{TenantID: "vendor-tenant"},
			ID:          models.OrderID("order-standard-shared"),
			MyRole:      string(models.RoleVendor),
			Open:        false,
		}
		if err := tx.Save(vendorOrder); err != nil {
			return err
		}
		return svc.releaseStandardOrderSupplyInTx(tx, "order-standard-shared", "declined")
	}))

	require.Len(t, recorder.releaseTxRequests, 1)
	require.Equal(t, standardOrderSupplyReleaseRequest{
		orderRef:  "order-standard-shared",
		orderType: models.OrderTypeStandard,
		reason:    "declined",
		txSeen:    true,
	}, recorder.releaseTxRequests[0])
}

type standardOrderSupplyCommitRequest struct {
	orderRef  string
	orderType string
	txSeen    bool
}

type standardOrderSupplyReleaseRequest struct {
	orderRef  string
	orderType string
	reason    string
	txSeen    bool
}

type recordingOrderSupplyAvailability struct {
	reserveTxRequests []contracts.ReserveOrderSupplyRequest
	commitTxRequests  []standardOrderSupplyCommitRequest
	releaseTxRequests []standardOrderSupplyReleaseRequest
}

type recordingDigitalSupplyLineResolver struct {
	items   [][]digital.OrderLineItem
	txItems [][]digital.OrderLineItem
	txSeen  bool
	lines   []contracts.SupplyLine
	err     error
}

type nonTransactionalDigitalSupplyLineResolver struct{}

func (nonTransactionalDigitalSupplyLineResolver) SupplyAvailabilityLinesForOrderItems([]digital.OrderLineItem) ([]contracts.SupplyLine, error) {
	return nil, nil
}

func (r *recordingDigitalSupplyLineResolver) SupplyAvailabilityLinesForOrderItems(items []digital.OrderLineItem) ([]contracts.SupplyLine, error) {
	r.items = append(r.items, append([]digital.OrderLineItem(nil), items...))
	if r.err != nil {
		return nil, r.err
	}
	lines := make([]contracts.SupplyLine, len(r.lines))
	copy(lines, r.lines)
	return lines, nil
}

func (r *recordingDigitalSupplyLineResolver) SupplyAvailabilityLinesForOrderItemsTx(tx database.Tx, items []digital.OrderLineItem) ([]contracts.SupplyLine, error) {
	r.txSeen = tx != nil
	r.txItems = append(r.txItems, append([]digital.OrderLineItem(nil), items...))
	if r.err != nil {
		return nil, r.err
	}
	lines := make([]contracts.SupplyLine, len(r.lines))
	copy(lines, r.lines)
	return lines, nil
}

func (r *recordingOrderSupplyAvailability) Quote(context.Context, contracts.SupplyQuoteRequest) (*contracts.SupplyQuoteResult, error) {
	return &contracts.SupplyQuoteResult{CanSell: true}, nil
}

func (r *recordingOrderSupplyAvailability) ReserveOrder(context.Context, contracts.ReserveOrderSupplyRequest) (*contracts.ReserveOrderSupplyResult, error) {
	return &contracts.ReserveOrderSupplyResult{}, nil
}

func (r *recordingOrderSupplyAvailability) ReserveOrderTx(_ context.Context, _ database.Tx, req contracts.ReserveOrderSupplyRequest) (*contracts.ReserveOrderSupplyResult, error) {
	r.reserveTxRequests = append(r.reserveTxRequests, req)
	return &contracts.ReserveOrderSupplyResult{}, nil
}

func (r *recordingOrderSupplyAvailability) CommitOrderTx(_ context.Context, tx database.Tx, orderRef string, orderType string) error {
	r.commitTxRequests = append(r.commitTxRequests, standardOrderSupplyCommitRequest{
		orderRef:  orderRef,
		orderType: orderType,
		txSeen:    tx != nil,
	})
	return nil
}

func (r *recordingOrderSupplyAvailability) ReleaseOrderTx(_ context.Context, tx database.Tx, orderRef string, orderType string, reason string) error {
	r.releaseTxRequests = append(r.releaseTxRequests, standardOrderSupplyReleaseRequest{
		orderRef:  orderRef,
		orderType: orderType,
		reason:    reason,
		txSeen:    tx != nil,
	})
	return nil
}

func (r *recordingOrderSupplyAvailability) CommitOrder(context.Context, string, string) error {
	return nil
}

func (r *recordingOrderSupplyAvailability) ReleaseOrder(context.Context, string, string, string) error {
	return nil
}

var _ contracts.SupplyAvailabilityService = (*recordingOrderSupplyAvailability)(nil)

type orderSupplyAlwaysEnabledResolver struct{}

func (orderSupplyAlwaysEnabledResolver) IsEnabled(context.Context, string) bool { return true }

func (orderSupplyAlwaysEnabledResolver) Evaluate(context.Context, string) pkgconfig.Evaluation {
	return pkgconfig.Evaluation{Enabled: true}
}

func (orderSupplyAlwaysEnabledResolver) List(context.Context) []pkgconfig.EffectiveFeature {
	return nil
}

func orderSupplyListingNoVariant(t *testing.T, slug, quantity string) *pb.SignedListing {
	t.Helper()
	return &pb.SignedListing{
		Listing: &pb.Listing{
			Slug: slug,
			Metadata: &pb.Listing_Metadata{
				Version:      ListingVersion,
				ContractType: pb.Listing_Metadata_PHYSICAL_GOOD,
				PricingCurrency: &pb.Currency{
					Code: "USD",
				},
			},
			Item: &pb.Listing_Item{
				Title: slug,
				Price: "1000",
				Skus: []*pb.Listing_Item_Sku{{
					Quantity: quantity,
					Price:    "1000",
				}},
			},
		},
	}
}

func orderSupplyListing(t *testing.T, slug, option, variant, quantity string) *pb.SignedListing {
	t.Helper()
	return &pb.SignedListing{
		Listing: &pb.Listing{
			Slug: slug,
			Metadata: &pb.Listing_Metadata{
				Version:      ListingVersion,
				ContractType: pb.Listing_Metadata_PHYSICAL_GOOD,
				PricingCurrency: &pb.Currency{
					Code: "USD",
				},
			},
			Item: &pb.Listing_Item{
				Title: slug,
				Price: "1000",
				Options: []*pb.Listing_Item_Option{{
					Name: option,
					Variants: []*pb.Listing_Item_Option_Variant{{
						Name: variant,
					}},
				}},
				Skus: []*pb.Listing_Item_Sku{{
					Selections: []*pb.Listing_Item_Sku_Selection{{
						Option:  option,
						Variant: variant,
					}},
					Quantity: quantity,
					Price:    "1000",
				}},
			},
		},
	}
}

func orderSupplyListingHash(t *testing.T, sl *pb.SignedListing) string {
	t.Helper()
	ser, err := proto.Marshal(sl)
	require.NoError(t, err)
	hash, err := utils.MultihashSha256(ser)
	require.NoError(t, err)
	return hash.B58String()
}

func orderSupplyOrderOpen(listing *pb.SignedListing, listingHash string) *pb.OrderOpen {
	return orderSupplyOrderOpenWithQuantity(listing, listingHash, "1")
}

func orderSupplyOrderOpenWithQuantity(listing *pb.SignedListing, listingHash string, quantity string) *pb.OrderOpen {
	return &pb.OrderOpen{
		Timestamp: timestamppb.Now(),
		BuyerID: &pb.ID{
			PeerID: "buyer",
		},
		Listings: []*pb.SignedListing{listing},
		Items: []*pb.OrderOpen_Item{{
			ListingHash: listingHash,
			Quantity:    quantity,
			Options: []*pb.OrderOpen_Item_Option{{
				Name:  "Color",
				Value: "Red",
			}},
		}},
		RatingKeys: [][]byte{{1}},
	}
}

func orderSupplyOrderOpenNoOptions(listing *pb.SignedListing, listingHash string) *pb.OrderOpen {
	return orderSupplyOrderOpenNoOptionsWithQuantity(listing, listingHash, "1")
}

func orderSupplyOrderOpenNoOptionsWithQuantity(listing *pb.SignedListing, listingHash string, quantity string) *pb.OrderOpen {
	return &pb.OrderOpen{
		Timestamp: timestamppb.Now(),
		BuyerID: &pb.ID{
			PeerID: "buyer",
		},
		Listings: []*pb.SignedListing{listing},
		Items: []*pb.OrderOpen_Item{{
			ListingHash: listingHash,
			Quantity:    quantity,
		}},
		RatingKeys: [][]byte{{1}},
	}
}

func orderSupplyOrderOpenMessage(t *testing.T, orderID string, listing *pb.SignedListing, listingHash string) *npb.OrderMessage {
	t.Helper()
	orderOpen := orderSupplyOrderOpen(listing, listingHash)
	anyMsg, err := anypb.New(orderOpen)
	require.NoError(t, err)
	return &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Message:     anyMsg,
	}
}

func orderSupplyOrderOpenMessageNoOptions(t *testing.T, orderID string, listing *pb.SignedListing, listingHash string) *npb.OrderMessage {
	t.Helper()
	orderOpen := orderSupplyOrderOpenNoOptions(listing, listingHash)
	anyMsg, err := anypb.New(orderOpen)
	require.NoError(t, err)
	return &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Message:     anyMsg,
	}
}

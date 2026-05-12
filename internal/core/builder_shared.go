package core

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"

	"github.com/mobazha/mobazha3.0/internal/core/digital"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/storage"
	pkgcontracts "github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

// initDiscountSubsystem initializes the per-node discount subsystem:
// migrates DB models, creates DiscountStore, and wires up DiscountAppService.
// Shared between full and private_distribution builds (no build tags).
func initDiscountSubsystem(obNode *MobazhaNode) {
	if err := database.MigrateDiscountModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Discount: failed to migrate models: %v", err)
		return
	}
	store := database.NewGormDiscountStore(obNode.db)
	obNode.discountService = NewDiscountAppService(store, nil, obNode.eventBus, obNode.nodeID)
	logger.LogInfoWithID(log, obNode.nodeID, "Discount subsystem initialized")
}

// initCollectionSubsystem initializes the per-node collection subsystem:
// migrates DB models, creates CollectionStore, and wires up CollectionAppService.
// Shared between full and private_distribution builds (no build tags).
func initCollectionSubsystem(obNode *MobazhaNode) {
	if err := database.MigrateCollectionModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Collection: failed to migrate models: %v", err)
		return
	}
	store := database.NewGormCollectionStore(obNode.db)
	obNode.collectionService = NewCollectionAppService(store, obNode.eventBus, obNode.nodeID)

	if obNode.discountService != nil {
		obNode.discountService.collectionStore = store
	}

	logger.LogInfoWithID(log, obNode.nodeID, "Collection subsystem initialized")
}

// initShippingSubsystem initializes the per-node shipping subsystem:
// migrates DB models, creates ShippingStore, and wires up ShippingAppService.
// Shared between full and private_distribution builds (no build tags).
func initShippingSubsystem(obNode *MobazhaNode) {
	if err := database.MigrateShippingModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Shipping: failed to migrate models: %v", err)
		return
	}
	store := database.NewGormShippingStore(obNode.db)

	if err := MigrateShippingFromPreferences(obNode.db, store); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Shipping: data migration failed (non-fatal): %v", err)
	}

	publisher := &managed_escrowListingPublisher{node: obNode}
	svc := NewShippingAppService(store, publisher)
	svc.SetEventBus(obNode.eventBus)
	obNode.shippingService = svc
	logger.LogInfoWithID(log, obNode.nodeID, "Shipping subsystem initialized")
}

// managed_escrowListingPublisher wraps MobazhaNode to implement contracts.ListingPublisher
// using closure-style deferred evaluation with nil-safety. Works in both full
// and private_distribution builds.
type managed_escrowListingPublisher struct {
	node *MobazhaNode
}

func (p *managed_escrowListingPublisher) RepublishListing(ctx context.Context, slug string) error {
	if p.node == nil || p.node.listingService == nil {
		return nil
	}
	return p.node.listingService.RepublishListing(ctx, slug)
}

// initDigitalSubsystem initializes the per-node digital goods subsystem:
// creates DigitalAssetAppService + DigitalEntitlementAppService,
// and starts the entitlement event listener.
// Shared between full and private_distribution builds (no build tags).
func initDigitalSubsystem(obNode *MobazhaNode) {
	var blob pkgcontracts.BlobStore
	blob = getHostBlobStore(obNode)
	if blob == nil && obNode.repo != nil {
		blobDir := filepath.Join(obNode.repo.DataDir(), "blobs")
		if bs, bsErr := storage.NewLocalFSAdapter(blobDir); bsErr != nil {
			logger.LogErrorWithIDf(log, obNode.nodeID, "Digital: blob store init failed: %v", bsErr)
		} else {
			blob = bs
		}
	}

	if blob == nil {
		logger.LogWarningWithID(log, obNode.nodeID, "Digital: blob store unavailable — file-based digital assets will not work (license keys and links are unaffected)")
	}

	assetSvc := digital.NewDigitalAssetAppService(obNode.db, blob, obNode.keyProvider)
	obNode.digitalAssetService = assetSvc

	if obNode.eventBus == nil {
		logger.LogInfoWithID(log, obNode.nodeID, "Digital asset subsystem initialized (entitlement disabled: no event bus)")
		return
	}

	orders := &dbOrderQuerier{db: obNode.db}
	entitlementSvc := digital.NewDigitalEntitlementAppService(obNode.nodeCtx, obNode.db, obNode.featureResolver, assetSvc, orders, obNode.eventBus)
	if err := entitlementSvc.Start(); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Digital: entitlement start failed: %v", err)
		return
	}
	obNode.digitalEntitlementService = entitlementSvc
	logger.LogInfoWithID(log, obNode.nodeID, "Digital subsystem initialized")
}

// TECHDEBT(TD-099): dbOrderQuerier loads the Order GORM row and decodes the
// embedded OrderOpen / PaymentSent protobufs to extract entitlement metadata.
// The mobazha3.0 orders schema does not store contract type / listing slug /
// payment method as flat columns — they only exist inside the serialized
// protobufs. A thin adapter is acceptable as long as we don't expand the
// queried fields; if more callers need this metadata, promote it to
// contracts.OrderRepo and have OrderAppService implement it.
//
// VariantSKU is intentionally left blank in this adapter: resolving the SKU
// requires the listing's variant table (selected options → SKU mapping),
// which is not loaded here. DigitalAssetAppService.getAssetModelsByListing
// already treats empty SKU as "any variant" (`variant_sku IN (?, '')`), so
// listings without per-variant assets work correctly. Per-variant license
// pools therefore depend on listings keeping a single asset row per SKU, or
// future work to plumb the resolved SKU through OrderConfirmation events.
//
// 清除条件: contracts.OrderRepo exposes GetOrderMetadata or OrderConfirmation
// events carry the resolved (slug, variantSKU) pair directly.
type dbOrderQuerier struct {
	db database.Database
}

func (q *dbOrderQuerier) GetOrderMetadata(orderID string) (*digital.OrderMetadata, error) {
	var ord models.Order
	err := q.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&ord).Error
	})
	if err != nil {
		// Phase 1.0: orderID may also be a GuestOrder.OrderToken
		// (anonymous buyer flow). Guest order metadata is stored as flat
		// columns (no embedded protobuf), so the lookup is simpler than
		// the escrow path above. We only fall through on RecordNotFound;
		// any other DB error (connection, schema) propagates as-is.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return q.getGuestOrderMetadata(orderID)
		}
		return nil, err
	}

	meta := &digital.OrderMetadata{
		ContractType:  ord.ContractType().String(),
		PaymentMethod: ord.PaymentMethod().String(),
	}

	if oo, err := ord.OrderOpenMessage(); err == nil && oo != nil {
		if oo.BuyerID != nil {
			meta.BuyerPeerID = oo.BuyerID.PeerID
		}

		// Items and Listings are NOT 1:1 index-aligned. Items reference their
		// listing by ListingHash (== SignedListing.Cid). Build a lookup map.
		cidToSlug := make(map[string]string, len(oo.Listings))
		for _, sl := range oo.Listings {
			if sl != nil && sl.Listing != nil && sl.Cid != "" {
				cidToSlug[sl.Cid] = sl.Listing.Slug
			}
		}

		for _, oi := range oo.Items {
			if oi == nil {
				continue
			}
			slug := cidToSlug[oi.ListingHash]
			if slug == "" {
				continue
			}
			item := digital.OrderLineItem{
				ListingSlug: slug,
				// TECHDEBT(TD-099): VariantSKU requires variant table
				// lookup (selected options → SKU mapping). Left blank;
				// getAssetModelsByListing treats "" as "universal assets only".
			}
			if oi.Quantity != "" {
				if q, err := strconv.ParseUint(oi.Quantity, 10, 32); err == nil {
					item.Quantity = uint32(q)
				}
			}
			if item.Quantity == 0 {
				item.Quantity = 1
			}
			meta.LineItems = append(meta.LineItems, item)
		}
	}

	return meta, nil
}

// getGuestOrderMetadata builds OrderMetadata from a GuestOrder row keyed by
// order_token. Used by the entitlement service when the buyer is anonymous
// (no peer ID, no embedded protobufs). PaymentMethod is reported as "DIRECT"
// to match guest checkout's on-chain settlement model — DigitalEntitlement
// uses this to pick the initial grant status (active, not protected).
//
// BuyerPeerID is intentionally empty: the orderToken itself is the only
// capability the buyer has, mirroring the existing public buyer-portal
// trust model (see huma_digital_asset_handlers.go::digital-assets-buyer-get).
func (q *dbOrderQuerier) getGuestOrderMetadata(orderToken string) (*digital.OrderMetadata, error) {
	var go_ models.GuestOrder
	if err := q.db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_token = ?", orderToken).
			Preload("Items").First(&go_).Error
	}); err != nil {
		return nil, err
	}

	meta := &digital.OrderMetadata{
		ContractType:  "DIGITAL_GOOD",
		PaymentMethod: "DIRECT",
		BuyerPeerID:   "", // anonymous guest buyer
	}

	for _, it := range go_.Items {
		if it.ListingSlug == "" {
			continue
		}
		qty := uint32(it.Quantity)
		if qty == 0 {
			qty = 1
		}
		meta.LineItems = append(meta.LineItems, digital.OrderLineItem{
			ListingSlug: it.ListingSlug,
			// TECHDEBT(TD-099): GuestOrderItem stores VariantHash, not
			// VariantSKU. Resolving SKU requires the listing variant
			// table; getAssetModelsByListing treats "" as "any variant".
			Quantity: qty,
		})
	}

	return meta, nil
}

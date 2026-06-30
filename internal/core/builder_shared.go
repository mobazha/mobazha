package core

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mobazha/mobazha3.0/internal/core/digital"
	guestcore "github.com/mobazha/mobazha3.0/internal/core/guest"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/storage"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	pkgcontracts "github.com/mobazha/mobazha3.0/pkg/contracts"
	pkgdatabase "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
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

// initSupplyAvailabilitySubsystem wires the provider-neutral supply boundary.
// It is intentionally schema-free for SA-1: SKU availability reuses the
// existing InventoryReservation table, and checkout only consumes shadow Quote
// while supplyAvailabilityEnabled remains experimental.
func initSupplyAvailabilitySubsystem(obNode *MobazhaNode) {
	if obNode == nil || obNode.db == nil {
		return
	}
	svc, err := NewSupplyAvailabilityAppService(supplyAvailabilityProvidersForNode(obNode)...)
	if err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Supply availability: failed to initialize: %v", err)
		return
	}
	obNode.supplyAvailabilityService = svc
	logger.LogInfoWithID(log, obNode.nodeID, "Supply availability subsystem initialized")
}

func supplyAvailabilityProvidersForNode(obNode *MobazhaNode) []pkgcontracts.SupplyProvider {
	providers := []pkgcontracts.SupplyProvider{
		NewSkuQuantityProvider(obNode.db),
		digital.NewLicenseKeyPoolProvider(obNode.db),
		digital.NewUnlimitedDigitalProvider(obNode.db),
	}
	if obNode.supplyChainRegistry != nil {
		providers = append(providers, NewExternalSupplyProvider(obNode.db, obNode.supplyChainRegistry))
	}
	return providers
}

// initStorePolicySubsystem initializes the per-node store policy subsystem:
// migrates policy models, creates StorePolicyStore, and wires StorePolicyAppService.
// Shared between full and private_distribution builds (no build tags).
func initStorePolicySubsystem(obNode *MobazhaNode) {
	if err := database.MigrateStorePolicyModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "StorePolicy: failed to migrate models: %v", err)
		return
	}
	store := database.NewGormStorePolicyStore(obNode.db)
	obNode.storePolicyService = NewStorePolicyAppService(store, obNode.eventBus)
	logger.LogInfoWithID(log, obNode.nodeID, "StorePolicy subsystem initialized")
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

// wireDigitalSupplyLineResolver installs the shared digital supply resolver on
// every order service that exposes the capability in the selected distribution.
func wireDigitalSupplyLineResolver(obNode *MobazhaNode, assetSvc *digital.DigitalAssetAppService) {
	if obNode == nil || assetSvc == nil {
		return
	}
	if setter, ok := any(obNode.orderService).(digitalSupplyLineResolverSetter); ok {
		setter.SetDigitalSupplyLineResolver(assetSvc)
	}
	if setter, ok := any(obNode.guestOrderService).(digitalSupplyLineResolverSetter); ok {
		setter.SetDigitalSupplyLineResolver(assetSvc)
	}
}

type digitalSupplyLineResolverSetter interface {
	SetDigitalSupplyLineResolver(pkgcontracts.DigitalSupplyLineResolver)
}

// initDigitalSubsystem initializes the per-node digital goods subsystem. It
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
	assetSvc.SetNodePeerID(obNode.Identity().String())
	assetSvc.SetCoTenantDigitalAssets(obNode.coTenantDigitalAssetsDeferred())

	if obNode.eventBus == nil {
		assetSvc.SetOrderQuerier(&dbOrderQuerier{db: obNode.db})
		obNode.digitalAssetService = assetSvc
		wireDigitalSupplyLineResolver(obNode, assetSvc)
		logger.LogInfoWithID(log, obNode.nodeID, "Digital asset subsystem initialized (entitlement disabled: no event bus)")
		return
	}

	orders := &dbOrderQuerier{db: obNode.db}
	assetSvc.SetOrderQuerier(orders)
	obNode.digitalAssetService = assetSvc
	wireDigitalSupplyLineResolver(obNode, assetSvc)
	digitalCtx := obNode.nodeCtx
	if pkgconfig.TenantIDFromContext(digitalCtx) == "" {
		digitalCtx = pkgconfig.ContextWithTenantID(digitalCtx, pkgdatabase.StandaloneTenantID)
	}
	entitlementSvc := digital.NewDigitalEntitlementAppService(digitalCtx, obNode.db, obNode.featureResolver, assetSvc, orders, obNode.eventBus)
	assetSvc.SetDigitalDeliveryRetrier(entitlementSvc.RetryDigitalDelivery)
	// Wire the shipper so auto-delivery also advances order state. Full nodes
	// use OrderService; private_distribution/guest checkout uses GuestOrderAppService.
	if shipper := newDigitalOrderShipper(obNode.Order(), obNode.guestOrderService); shipper != nil {
		entitlementSvc.SetShipper(shipper)
	}
	if err := entitlementSvc.Start(); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Digital: entitlement start failed: %v", err)
		return
	}
	obNode.digitalEntitlementService = entitlementSvc
	logger.LogInfoWithID(log, obNode.nodeID, "Digital subsystem initialized")
}

type guestDigitalOrderShipper interface {
	ShipGuestOrder(ctx context.Context, token string, tracking, carrier string) error
}

type digitalOrderShipper struct {
	order digital.OrderShipper
	guest guestDigitalOrderShipper
}

func newDigitalOrderShipper(order digital.OrderShipper, guest guestDigitalOrderShipper) digital.OrderShipper {
	if order == nil && guest == nil {
		return nil
	}
	return &digitalOrderShipper{order: order, guest: guest}
}

func (s *digitalOrderShipper) ShipOrder(orderID models.OrderID, shipments []models.Shipment, done chan struct{}) error {
	id := string(orderID)
	if strings.HasPrefix(id, guestcore.OrderTokenPrefix) {
		if s.guest == nil {
			return errors.New("guest order shipper unavailable")
		}
		return s.guest.ShipGuestOrder(context.Background(), id, "", "digital")
	}
	if s.order == nil {
		return errors.New("order shipper unavailable")
	}
	return s.order.ShipOrder(orderID, shipments, done)
}

// TECHDEBT(TD-099): dbOrderQuerier loads the Order GORM row and decodes the
// embedded OrderOpen / PaymentSent protobufs to extract entitlement metadata.
// The mobazha3.0 orders schema does not store contract type / listing slug /
// payment method as flat columns — they only exist inside the serialized
// protobufs. A thin adapter is acceptable as long as we don't expand the
// queried fields; if more callers need this metadata, promote it to
// contracts.OrderRepo and have OrderAppService implement it.
// 清除条件: contracts.OrderRepo exposes GetOrderMetadata or OrderConfirmation
// events carry the resolved (slug, variantSKU) pair directly.
type dbOrderQuerier struct {
	db pkgdatabase.Database
}

func (q *dbOrderQuerier) GetOrderMetadata(orderID string) (*digital.OrderMetadata, error) {
	var ord models.Order
	err := q.db.View(func(tx pkgdatabase.Tx) error {
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

	paymentMethod := ""
	if method, ok := ord.SettlementMethod(); ok {
		paymentMethod = method.String()
	}

	meta := &digital.OrderMetadata{
		ContractType:  ord.ContractType().String(),
		PaymentMethod: paymentMethod,
	}

	if oo, err := ord.OrderOpenMessage(); err == nil && oo != nil {
		if oo.BuyerID != nil {
			meta.BuyerPeerID = oo.BuyerID.PeerID
		}

		// Items and Listings are NOT 1:1 index-aligned. Items reference their
		// listing by ListingHash (== SignedListing.Cid). Build a lookup map.
		cidToListing := make(map[string]*pb.SignedListing, len(oo.Listings))
		for _, sl := range oo.Listings {
			if sl != nil && sl.Listing != nil && sl.Cid != "" {
				cidToListing[sl.Cid] = sl
				if meta.SellerPeerID == "" && sl.Listing.VendorID != nil {
					meta.SellerPeerID = sl.Listing.VendorID.PeerID
				}
			}
		}

		for i, oi := range oo.Items {
			if oi == nil {
				continue
			}
			sl := cidToListing[oi.ListingHash]
			if sl == nil {
				// Item.ListingHash is frozen at purchase time; SignedListing.Cid
				// changes when the seller edits/republishes. Fall back when the
				// order snapshot still carries the listing row.
				if len(oo.Listings) == 1 && oo.Listings[0] != nil && oo.Listings[0].Listing != nil {
					sl = oo.Listings[0]
				} else if i < len(oo.Listings) && oo.Listings[i] != nil && oo.Listings[i].Listing != nil {
					sl = oo.Listings[i]
				}
			}
			if sl == nil || sl.Listing == nil || sl.Listing.Slug == "" {
				continue
			}
			item := digital.OrderLineItem{
				ListingSlug: sl.Listing.Slug,
				VariantSKU:  standardOrderMetadataVariantSKU(sl.Listing, oi.Options),
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
// BuyerPeerID is intentionally empty: anonymous guest buyers don't have a
// peer identity. Buyer Portal reads are protected by the independent
// buyerPortalToken issued with the GuestOrder, not by the order token alone.
func (q *dbOrderQuerier) getGuestOrderMetadata(orderToken string) (*digital.OrderMetadata, error) {
	var go_ models.GuestOrder
	if err := q.db.View(func(tx pkgdatabase.Tx) error {
		return tx.Read().Where("order_token = ?", orderToken).
			Preload("Items").First(&go_).Error
	}); err != nil {
		return nil, err
	}

	meta := &digital.OrderMetadata{
		ContractType:  guestcore.ContractTypeFromItems(go_.Items),
		PaymentMethod: "DIRECT",
		BuyerPeerID:   "", // anonymous guest buyer
	}

	for _, it := range go_.Items {
		if it.ListingSlug == "" {
			continue
		}
		if meta.SellerPeerID == "" {
			meta.SellerPeerID = it.SellerPeerID
		}
		qty := uint32(it.Quantity)
		if qty == 0 {
			qty = 1
		}
		meta.LineItems = append(meta.LineItems, digital.OrderLineItem{
			ListingSlug: it.ListingSlug,
			VariantSKU:  it.VariantSKU,
			Quantity:    qty,
		})
	}
	return meta, nil
}

func standardOrderMetadataVariantSKU(listing *pb.Listing, options []*pb.OrderOpen_Item_Option) string {
	sku, err := standardOrderMetadataSelectedSKU(listing, options)
	if err != nil || sku == nil {
		return ""
	}
	return strings.TrimSpace(sku.GetProductID())
}

func standardOrderMetadataSelectedSKU(listing *pb.Listing, options []*pb.OrderOpen_Item_Option) (*pb.Listing_Item_Sku, error) {
	if listing == nil || listing.Item == nil || len(listing.Item.Options) == 0 {
		return nil, nil
	}
	opts := make(map[string]string)
	for _, option := range options {
		opts[strings.ToLower(strings.TrimSpace(option.Name))] = strings.ToLower(strings.TrimSpace(option.Value))
	}
	for _, sku := range listing.Item.Skus {
		matches := true
		for _, sel := range sku.Selections {
			if opts[strings.ToLower(strings.TrimSpace(sel.Option))] != strings.ToLower(strings.TrimSpace(sel.Variant)) {
				matches = false
				break
			}
		}
		if matches {
			return sku, nil
		}
	}
	return nil, errors.New("selected sku not found in listing")
}

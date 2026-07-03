package digital

import (
	"context"
	"fmt"

	pkgconfig "github.com/mobazha/mobazha/pkg/config"
	pkgcontracts "github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
)

// OrderShipper fulfills the ship-order step, advancing the order state and
// notifying the buyer. Implemented by *order.OrderAppService (and by
// contracts.NodeService, which MobazhaNode satisfies).
// A nil shipper disables auto-ship (grants are still created).
type OrderShipper interface {
	ShipOrder(orderID models.OrderID, shipments []models.Shipment, done chan struct{}) error
}

// OrderLineItem preserves the digital package API while the channel-neutral
// supply contract owns the shared line-item shape.
type OrderLineItem = pkgcontracts.DigitalOrderLineItem

// OrderMetadata holds the subset of order fields needed by the entitlement
// logic. Returned by OrderQuerier.GetOrderMetadata in a single query.
type OrderMetadata struct {
	ContractType  string
	BuyerPeerID   string
	SellerPeerID  string
	PaymentMethod string
	LineItems     []OrderLineItem
}

// OrderQuerier retrieves order metadata needed by entitlement logic.
// Implemented by a thin DB adapter (builder_digital.go) or a mock in tests.
type OrderQuerier interface {
	GetOrderMetadata(orderID string) (*OrderMetadata, error)
}

// DigitalEntitlementAppService listens to order lifecycle events and
// manages digital asset entitlements (grants, license allocation).
//
// Event subscriptions:
//   - OrderConfirmation → create grants + allocate license keys + auto-ShipOrder
//   - DisputeOpen       → freeze grants + suspend licenses
//   - DisputeClose      → restore or revoke based on outcome
//   - Refund            → revoke all grants + licenses
type DigitalEntitlementAppService struct {
	db       database.Database
	features pkgconfig.ResolverInterface
	nodeCtx  context.Context // carries tenantID for SaaS feature flag resolution
	assets   *DigitalAssetAppService
	orders   OrderQuerier
	bus      events.Bus
	shipper  OrderShipper // may be nil: auto-ship disabled, grants still created
}

// NewDigitalEntitlementAppService creates a new entitlement service.
// nodeCtx must carry tenant identity (via pkgconfig.ContextWithTenantID) so
// that feature flags resolve against the correct tenant layer.
func NewDigitalEntitlementAppService(
	nodeCtx context.Context,
	db database.Database,
	features pkgconfig.ResolverInterface,
	assets *DigitalAssetAppService,
	orders OrderQuerier,
	bus events.Bus,
) *DigitalEntitlementAppService {
	return &DigitalEntitlementAppService{
		db:       db,
		features: features,
		nodeCtx:  nodeCtx,
		assets:   assets,
		orders:   orders,
		bus:      bus,
	}
}

// SetShipper wires the auto-ship dependency. Must be called before Start().
// If never called (or called with nil), grants are still created on
// OrderConfirmation but the order state is not advanced to FULFILLED.
func (s *DigitalEntitlementAppService) SetShipper(shipper OrderShipper) {
	s.shipper = shipper
}

// Start subscribes to order events and processes them in background goroutines.
func (s *DigitalEntitlementAppService) Start() error {
	confirmSub, err := s.bus.Subscribe(&events.OrderConfirmation{}, events.BufSize(64))
	if err != nil {
		return fmt.Errorf("subscribe OrderConfirmation: %w", err)
	}

	disputeOpenSub, err := s.bus.Subscribe(&events.DisputeOpen{}, events.BufSize(32))
	if err != nil {
		return fmt.Errorf("subscribe DisputeOpen: %w", err)
	}

	disputeCloseSub, err := s.bus.Subscribe(&events.DisputeClose{}, events.BufSize(32))
	if err != nil {
		return fmt.Errorf("subscribe DisputeClose: %w", err)
	}

	refundSub, err := s.bus.Subscribe(&events.Refund{}, events.BufSize(32))
	if err != nil {
		return fmt.Errorf("subscribe Refund: %w", err)
	}

	// Each listener goroutine exits when the subscription channel is closed,
	// which happens when the EventBus is shut down during node Stop().
	// No separate context/cancel is needed — EventBus.Close → channel close
	// → range loop exits → goroutine terminates.
	go s.listenConfirmations(confirmSub)
	go s.listenDisputeOpen(disputeOpenSub)
	go s.listenDisputeClose(disputeCloseSub)
	go s.listenRefund(refundSub)

	return nil
}

func (s *DigitalEntitlementAppService) listenConfirmations(sub events.Subscription) {
	for evt := range sub.Out() {
		confirm, ok := evt.(*events.OrderConfirmation)
		if !ok {
			continue
		}
		if err := s.handleOrderConfirmation(confirm); err != nil {
			log.Errorf("[digital-entitlement] OrderConfirmation %s error: %v", confirm.OrderID, err)
		}
	}
}

func (s *DigitalEntitlementAppService) handleOrderConfirmation(confirm *events.OrderConfirmation) error {
	return s.deliverDigitalOrder(confirm.OrderID)
}

// RetryDigitalDelivery replays the automatic entitlement creation for a paid
// digital order. It is safe to call repeatedly: download grants and license
// allocations are keyed by order/listing/asset and remain idempotent.
func (s *DigitalEntitlementAppService) RetryDigitalDelivery(orderID string) error {
	return s.deliverDigitalOrder(orderID)
}

func (s *DigitalEntitlementAppService) deliverDigitalOrder(orderID string) error {
	// Fail closed: if featureResolver is not yet wired (nil during early init),
	// treat the feature as disabled. Callers must explicitly enable the beta
	// feature via configuration.
	if s.features == nil || !s.features.IsEnabled(s.nodeCtx, pkgconfig.FeatureDigitalAutoDeliveryEnabled.Key) {
		return nil
	}

	meta, err := s.orders.GetOrderMetadata(orderID)
	if err != nil {
		return fmt.Errorf("get order metadata: %w", err)
	}
	if meta.ContractType != "DIGITAL_GOOD" {
		return nil
	}
	if len(meta.LineItems) == 0 {
		return nil
	}

	grantStatus := determineGrantStatus(meta.PaymentMethod)

	grantsCreated := 0
	deliveredItems := make(map[int]bool)
	for itemIndex, item := range meta.LineItems {
		qty := item.Quantity
		if qty == 0 {
			qty = 1
		}

		assets, err := s.assets.getAssetModelsByListing(item.ListingSlug, item.VariantSKU)
		if err != nil {
			log.Errorf("[digital-entitlement] get assets for %s/%s: %v", item.ListingSlug, item.VariantSKU, err)
			continue
		}
		if len(assets) == 0 {
			log.Errorf("[digital-entitlement] no configured digital assets for order %s item %d listing %s/%s",
				orderID, itemIndex, item.ListingSlug, item.VariantSKU)
			continue
		}

		itemDelivered := true
		for i := range assets {
			asset := &assets[i]

			// Files and links: one grant covers all seats (download is unlimited).
			// License keys: allocate qty keys to match the purchased quantity.
			if asset.AssetType == models.AssetTypeLicenseKey {
				already := s.assets.CountAllocatedKeys(orderID, asset.ListingSlug, asset.VariantSKU)
				remaining := int64(qty) - already
				allocated := int64(0)
				for seat := int64(0); seat < remaining; seat++ {
					_, allocErr := s.assets.AllocateLicenseKey(
						asset.ListingSlug, asset.VariantSKU,
						orderID, meta.BuyerPeerID,
					)
					if allocErr != nil {
						log.Errorf("[digital-entitlement] allocate license for asset %s seat %d/%d: %v",
							asset.ID, already+seat+1, qty, allocErr)
						break
					}
					allocated++
				}
				if already+allocated < int64(qty) {
					itemDelivered = false
					if already+allocated == 0 {
						continue
					}
				}
			}

			_, grantErr := s.assets.CreateDownloadGrant(asset, orderID, meta.BuyerPeerID, grantStatus)
			if grantErr != nil {
				log.Errorf("[digital-entitlement] create grant for asset %s: %v", asset.ID, grantErr)
				itemDelivered = false
				continue
			}
			grantsCreated++
		}
		if itemDelivered {
			deliveredItems[itemIndex] = true
		}
	}

	// Auto-ship: write Buyer Portal entry and advance order state to FULFILLED.
	// The URL points to the seller-side digital assets endpoint; the buyer
	// accesses the same data via GET /v1/orders/{orderID}/digital-assets.
	// Non-fatal: if ShipOrder fails, grants are already live and the buyer
	// can still access downloads via the portal. The order state will remain
	// AWAITING_FULFILLMENT until the seller manually ships or retries.
	if len(deliveredItems) == len(meta.LineItems) && grantsCreated > 0 && s.shipper != nil {
		shipments := make([]models.Shipment, 0, len(deliveredItems))
		for itemIndex := range meta.LineItems {
			if deliveredItems[itemIndex] {
				shipments = append(shipments, models.Shipment{
					ItemIndex: itemIndex,
					DigitalDelivery: &models.DigitalDelivery{
						URL: "/v1/orders/" + orderID + "/digital-assets",
					},
				})
			}
		}
		if err := s.shipper.ShipOrder(models.OrderID(orderID), shipments, nil); err != nil {
			log.Errorf("[digital-entitlement] auto-ShipOrder order %s: %v — grants active, order state not advanced",
				orderID, err)
		} else {
			log.Infof("[digital-entitlement] auto-ShipOrder order %s: %d grants created, order advanced to FULFILLED",
				orderID, grantsCreated)
		}
	}

	return nil
}

// determineGrantStatus maps payment method to initial grant status.
//
//	CANCELABLE / FIAT / DIRECT → "active"
//	MODERATED → "protected"
func determineGrantStatus(paymentMethod string) string {
	switch paymentMethod {
	case "MODERATED":
		return models.GrantStatusProtected
	default:
		return models.GrantStatusActive
	}
}

func (s *DigitalEntitlementAppService) listenDisputeOpen(sub events.Subscription) {
	for evt := range sub.Out() {
		dispute, ok := evt.(*events.DisputeOpen)
		if !ok {
			continue
		}
		if err := s.assets.FreezeGrantsByOrder(dispute.OrderID, "dispute_opened"); err != nil {
			log.Errorf("[digital-entitlement] freeze grants for order %s: %v", dispute.OrderID, err)
		}
		s.suspendLicensesByOrder(dispute.OrderID)
	}
}

func (s *DigitalEntitlementAppService) listenDisputeClose(sub events.Subscription) {
	for evt := range sub.Out() {
		dc, ok := evt.(*events.DisputeClose)
		if !ok {
			continue
		}
		if dc.BuyerRefunded {
			if err := s.assets.RevokeGrantsByOrder(dc.OrderID, "dispute_buyer_won"); err != nil {
				log.Errorf("[digital-entitlement] revoke grants (dispute buyer won) order %s: %v", dc.OrderID, err)
			}
			s.revokeLicensesByOrder(dc.OrderID)
		} else {
			s.restoreGrantsByOrder(dc.OrderID)
			s.restoreLicensesByOrder(dc.OrderID)
		}
	}
}

func (s *DigitalEntitlementAppService) listenRefund(sub events.Subscription) {
	for evt := range sub.Out() {
		refund, ok := evt.(*events.Refund)
		if !ok {
			continue
		}
		if err := s.assets.RevokeGrantsByOrder(refund.OrderID, "refunded"); err != nil {
			log.Errorf("[digital-entitlement] revoke grants for order %s: %v", refund.OrderID, err)
		}
		s.revokeLicensesByOrder(refund.OrderID)
	}
}

func (s *DigitalEntitlementAppService) suspendLicensesByOrder(orderID string) {
	if err := s.db.Update(func(tx database.Tx) error {
		var keys []models.DigitalLicenseKey
		if err := tx.Read().
			Where("order_id = ? AND status = ?", orderID, models.LicenseKeyStatusDispensed).
			Find(&keys).Error; err != nil {
			return err
		}
		for i := range keys {
			keys[i].Status = models.LicenseKeyStatusSuspended
			if err := tx.Save(&keys[i]); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		log.Errorf("[digital-entitlement] suspendLicensesByOrder %s: %v", orderID, err)
	}
}

func (s *DigitalEntitlementAppService) revokeLicensesByOrder(orderID string) {
	if err := s.db.Update(func(tx database.Tx) error {
		var keys []models.DigitalLicenseKey
		if err := tx.Read().
			Where("order_id = ? AND status IN (?, ?)", orderID,
				models.LicenseKeyStatusDispensed, models.LicenseKeyStatusSuspended).
			Find(&keys).Error; err != nil {
			return err
		}
		for i := range keys {
			keys[i].Status = models.LicenseKeyStatusRevoked
			if err := tx.Save(&keys[i]); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		log.Errorf("[digital-entitlement] revokeLicensesByOrder %s: %v", orderID, err)
	}
}

func (s *DigitalEntitlementAppService) restoreLicensesByOrder(orderID string) {
	if err := s.db.Update(func(tx database.Tx) error {
		var keys []models.DigitalLicenseKey
		if err := tx.Read().
			Where("order_id = ? AND status = ?", orderID, models.LicenseKeyStatusSuspended).
			Find(&keys).Error; err != nil {
			return err
		}
		for i := range keys {
			keys[i].Status = models.LicenseKeyStatusDispensed
			if err := tx.Save(&keys[i]); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		log.Errorf("[digital-entitlement] restoreLicensesByOrder %s: %v", orderID, err)
	}
}

func (s *DigitalEntitlementAppService) restoreGrantsByOrder(orderID string) {
	if err := s.db.Update(func(tx database.Tx) error {
		var grants []models.DownloadGrant
		if err := tx.Read().
			Where("order_id = ? AND status = ?", orderID, models.GrantStatusFrozen).
			Find(&grants).Error; err != nil {
			return err
		}
		for i := range grants {
			grants[i].Status = models.GrantStatusActive
			if grants[i].PreviousStatus != "" {
				grants[i].Status = grants[i].PreviousStatus
			}
			grants[i].PreviousStatus = ""
			grants[i].RevokeReason = ""
			if err := tx.Save(&grants[i]); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		log.Errorf("[digital-entitlement] restoreGrantsByOrder %s: %v", orderID, err)
	}
}

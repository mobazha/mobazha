package digital

import (
	"context"
	"fmt"

	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// OrderLineItem represents one purchased item in a multi-line order.
type OrderLineItem struct {
	ListingSlug string
	VariantSKU  string
	Quantity    uint32
}

// OrderMetadata holds the subset of order fields needed by the entitlement
// logic. Returned by OrderQuerier.GetOrderMetadata in a single query.
type OrderMetadata struct {
	ContractType  string
	BuyerPeerID   string
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
//   - OrderConfirmation → create grants + allocate license keys
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
	// Fail closed: if featureResolver is not yet wired (nil during early init),
	// treat the feature as disabled. Callers must explicitly enable the beta
	// feature via configuration.
	if s.features == nil || !s.features.IsEnabled(s.nodeCtx, pkgconfig.FeatureDigitalAutoDeliveryEnabled.Key) {
		return nil
	}

	meta, err := s.orders.GetOrderMetadata(confirm.OrderID)
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

	for _, item := range meta.LineItems {
		qty := item.Quantity
		if qty == 0 {
			qty = 1
		}

		assets, err := s.assets.getAssetModelsByListing(item.ListingSlug, item.VariantSKU)
		if err != nil {
			log.Errorf("[digital-entitlement] get assets for %s/%s: %v", item.ListingSlug, item.VariantSKU, err)
			continue
		}

		for i := range assets {
			asset := &assets[i]

			// Files and links: one grant covers all seats (download is unlimited).
			// License keys: allocate qty keys to match the purchased quantity.
			_, grantErr := s.assets.CreateDownloadGrant(asset, confirm.OrderID, meta.BuyerPeerID, grantStatus)
			if grantErr != nil {
				log.Errorf("[digital-entitlement] create grant for asset %s: %v", asset.ID, grantErr)
				continue
			}

			if asset.AssetType == models.AssetTypeLicenseKey {
				already := s.assets.CountAllocatedKeys(confirm.OrderID, asset.ListingSlug, asset.VariantSKU)
				remaining := int64(qty) - already
				for seat := int64(0); seat < remaining; seat++ {
					_, allocErr := s.assets.AllocateLicenseKey(
						asset.ListingSlug, asset.VariantSKU,
						confirm.OrderID, meta.BuyerPeerID,
					)
					if allocErr != nil {
						log.Errorf("[digital-entitlement] allocate license for asset %s seat %d/%d: %v",
							asset.ID, already+seat+1, qty, allocErr)
						break
					}
				}
			}
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

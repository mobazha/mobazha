package core

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/internal/fulfillment/printful"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/fulfillment"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"golang.org/x/crypto/hkdf"
	"gorm.io/gorm"

	"crypto/sha256"
)

// SupplyChainOrderOps is the subset of OrderAppService needed by the supply chain
// subsystem. Kept narrow to avoid a circular import between App Services.
type SupplyChainOrderOps interface {
	ConfirmOrder(orderID models.OrderID, txid iwallet.TransactionID, payoutAddress string, done chan struct{}) error
	ShipOrder(orderID models.OrderID, shipments []models.Shipment, done chan struct{}) error
	IsOrderConfirmed(orderID models.OrderID) (bool, error)
	IsOrderShipped(orderID models.OrderID) (bool, error)
	GetOrderState(orderID models.OrderID) (models.OrderState, error)
}

// SupplyChainListingOps is the subset of ListingAppService needed by ImportProduct.
// Kept narrow to avoid a circular import between App Services.
type SupplyChainListingOps interface {
	SaveListing(listing *pb.Listing, done chan<- struct{}) error
}

// SupplyChainAppService orchestrates supply chain operations:
// provider management, catalog browsing, product import, and order bridging.
// It implements contracts.SupplyChainService and contracts.SupplyChainChecker.
type SupplyChainAppService struct {
	registry   contracts.FulfillmentProviderRegistry
	db         database.Database
	nodeID     string
	credKey    [32]byte // AES-256-GCM key for encrypting provider credentials at rest

	eventBus       events.Bus
	shutdown       <-chan struct{}
	orderOps       SupplyChainOrderOps
	listingOps     SupplyChainListingOps
	saasMode       bool
	webhookBaseURL string
}

// NewSupplyChainAppService creates the supply chain service skeleton.
// privKeyBytes is the raw bytes of the node's libp2p identity key,
// used to derive a stable encryption key for provider credentials.
// Providers are registered via ConnectProvider or restored via Start().
func NewSupplyChainAppService(
	registry contracts.FulfillmentProviderRegistry,
	db database.Database,
	nodeID string,
	privKeyBytes []byte,
) *SupplyChainAppService {
	svc := &SupplyChainAppService{
		registry: registry,
		db:       db,
		nodeID:   nodeID,
		credKey:  deriveCredentialKey(privKeyBytes),
	}

	fulfillment.SetRebuildFunc(registry, svc.rebuildProviders)

	return svc
}

// deriveCredentialKey derives a deterministic AES-256 key from the node's
// private key material using HKDF-SHA256. The private key is never exposed
// in logs or metadata, making this derivation secure against DB-dump attacks.
func deriveCredentialKey(privKeyBytes []byte) [32]byte {
	var key [32]byte
	r := hkdf.New(sha256.New, privKeyBytes, []byte("mobazha-supply-chain"), []byte("credential-encryption"))
	_, _ = io.ReadFull(r, key[:])
	return key
}

// SetEventBus wires the event bus for OrderFunded subscription.
func (s *SupplyChainAppService) SetEventBus(bus events.Bus, shutdown <-chan struct{}) {
	s.eventBus = bus
	s.shutdown = shutdown
}

// SetOrderOps wires the order operations interface for auto-confirm and auto-ship.
func (s *SupplyChainAppService) SetOrderOps(ops SupplyChainOrderOps) {
	s.orderOps = ops
}

// SetListingOps wires the listing operations interface for ImportProduct.
func (s *SupplyChainAppService) SetListingOps(ops SupplyChainListingOps) {
	s.listingOps = ops
}

// Start restores provider instances from DB into the in-memory registry.
// Called during node initialization / SaaS EnsureNode.
func (s *SupplyChainAppService) Start(ctx context.Context) {
	if err := s.registry.RebuildFromDB(ctx); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: failed to rebuild providers from DB: %v", err)
	}
}

// StartFulfillmentMonitor subscribes to order lifecycle events and automatically
// bridges them to supplier fulfillment operations:
// - OrderFunded  → create supplier fulfillment order
// - OrderCancel  → cancel supplier fulfillment order
// - Refund       → cancel supplier fulfillment order
func (s *SupplyChainAppService) StartFulfillmentMonitor() {
	if s.eventBus == nil {
		return
	}
	go s.subscribeOrderFunded()
	go s.subscribeOrderCancel()
	go s.subscribeRefund()
	go s.subscribeDisputeOpen()
	go s.subscribeDisputeClose()
}

// StartWorkers launches background workers for retry, reconciliation, and cleanup.
// Must be called ONLY when FeatureSupplyChainEnabled is true (gated in builder.go).
func (s *SupplyChainAppService) StartWorkers(ctx context.Context, saasMode bool, webhookBaseURL string) {
	s.saasMode = saasMode
	s.webhookBaseURL = webhookBaseURL
	go s.retryFailedOrdersLoop(ctx)
	go s.reconcileStaleOrdersLoop(ctx)
	go s.cleanupProcessedEventsLoop(ctx)
}

const (
	// Retry worker
	retryWorkerInterval = 30 * time.Second
	maxRetryAttempts    = 3
	retryLeaseDuration  = 5 * time.Minute
	retryBackoffBase    = 5 * time.Minute // attempt N delay = retryBackoffBase * 2^N

	// Reconcile worker
	reconcileIntervalDefault = 5 * time.Minute // SaaS / public-webhook standalone
	reconcileIntervalNAT     = 1 * time.Minute // standalone behind NAT (no webhook)
	reconcileStaleThreshold  = 30 * time.Minute

	// Event cleanup
	eventCleanupInterval = 1 * time.Hour
	eventRetentionTTL    = 7 * 24 * time.Hour

	// FulfillmentOrderMapping.OrderAdvancementStatus values (P1-3 / TD-075).
	// Empty string = order has not yet reached `shipped`. Set as soon as the
	// mapping is marked shipped so reconcile can pick up unfinished advances.
	advancementStatusPending       = "pending"
	advancementStatusDone          = "done"
	advancementStatusPermanentFail = "permanent_fail"
)

func (s *SupplyChainAppService) retryFailedOrdersLoop(ctx context.Context) {
	ticker := time.NewTicker(retryWorkerInterval)
	defer ticker.Stop()
	logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: retry worker started (interval: %s)", retryWorkerInterval)
	for {
		select {
		case <-ctx.Done():
			logger.LogInfoWithID(log, s.nodeID, "SupplyChain: retry worker stopped")
			return
		case <-s.shutdown:
			logger.LogInfoWithID(log, s.nodeID, "SupplyChain: retry worker stopped")
			return
		case <-ticker.C:
			s.retryFailedOrders(ctx)
		}
	}
}

func (s *SupplyChainAppService) retryFailedOrders(ctx context.Context) {
	now := time.Now()

	var candidates []models.FulfillmentOrderMapping
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"failure_reason = ? AND retry_count < ? AND next_retry_at <= ? AND dispute_held = ? AND (retry_locked_until IS NULL OR retry_locked_until < ?)",
			string(contracts.FailureReasonRetryableProviderError), maxRetryAttempts, now, false, now,
		).Select("id, mobazha_order_id, provider_id, retry_count").Find(&candidates).Error
	})

	for _, c := range candidates {
		claimed := s.claimRetryLease(c.ID, now)
		if !claimed {
			continue
		}
		s.processRetry(ctx, c)
	}
}

// claimRetryLease atomically claims a mapping row for exclusive processing by
// setting retry_locked_until = now + retryLeaseDuration. Returns true if this
// caller acquired the lease.
//
// Concurrency model:
//  1. UPDATE ... WHERE retry_locked_until IS NULL OR retry_locked_until < now
//     — at the SQL row-lock level, only one concurrent updater wins.
//  2. After the update, we re-read the row and verify retry_locked_until equals
//     the leaseUntil we wrote. If a faster process already overwrote it
//     (extremely unlikely under SQL row locking but defensive), we treat the
//     claim as lost.
//
// Note on time.Time{} (zero value): GORM stores zero time as '0001-01-01...',
// not NULL. The condition (IS NULL OR < now) covers both freshly created rows
// (NULL by schema default if pointer / explicit) and previously released
// leases (zero time, which is < now).
//
// If processRetry / reconcileSingleOrder panic or are killed mid-flight, the
// lease auto-expires after retryLeaseDuration (5min) and another worker can
// retry — see releaseLease for normal cleanup.
func (s *SupplyChainAppService) claimRetryLease(mappingID string, now time.Time) bool {
	leaseUntil := now.Add(retryLeaseDuration)
	err := s.db.Update(func(tx database.Tx) error {
		return tx.Update(
			"retry_locked_until", leaseUntil,
			map[string]interface{}{
				"id = ?": mappingID,
				"(retry_locked_until IS NULL OR retry_locked_until < ?)": now,
			},
			&models.FulfillmentOrderMapping{},
		)
	})
	if err != nil {
		return false
	}
	var check models.FulfillmentOrderMapping
	if readErr := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ? AND retry_locked_until = ?", mappingID, leaseUntil).First(&check).Error
	}); readErr != nil {
		return false
	}
	return true
}

// processRetry runs one retry attempt for a previously-failed mapping.
// The lease must already be claimed by the caller.
//
// Steps:
//  1. rebuildRetryParams — load order contract, derive recipient + items
//     (no PII stored on the mapping row).
//  2. EstimateShipping + margin gate — supplier prices may have shifted; the
//     gate from FF-2.6 must pass at retry time.
//  3. executeRetryCreate — call provider.CreateFulfillmentOrder, persist
//     either success (clear failure_reason) or failure (bump retry_count,
//     compute next backoff, mark permanently_failed at the limit).
func (s *SupplyChainAppService) processRetry(ctx context.Context, mapping models.FulfillmentOrderMapping) {
	oo, params, reason, msg, ok := s.rebuildRetryParams(mapping)
	if !ok {
		s.markRetryOutcome(mapping.ID, mapping.RetryCount, reason, msg)
		return
	}

	// Re-run the FULL margin gate (cost+price+shipping) — not just shipping —
	// because retail price or supplier cost may have shifted between attempts.
	// Shared with the first-attempt path via evaluateMarginGate.
	if ok, reason, msg := s.evaluateMarginGate(ctx, supplyMarginInputs{
		oo:         oo,
		providerID: mapping.ProviderID,
		recipient:  params.Recipient,
		items:      params.Items,
	}); !ok {
		s.markRetryOutcome(mapping.ID, mapping.RetryCount, reason, msg)
		return
	}

	provider, provErr := s.registry.ForProvider(mapping.ProviderID)
	if provErr != nil {
		s.markRetryOutcome(mapping.ID, mapping.RetryCount, contracts.FailureReasonPermanentlyFailed,
			fmt.Sprintf("provider not found: %v", provErr))
		return
	}

	s.executeRetryCreate(ctx, mapping, params, provider)
}

// rebuildRetryParams reconstructs CreateFulfillmentParams from the order
// contract. Returns (params, reason, msg, ok). When ok is false, reason+msg
// describe why the retry is permanently abandoned (validation/permanent).
//
// TECHDEBT(TD-028): Uses ALL items from the order contract. Correct for
// FF-2 single-supplier orders (1 mapping per order). FF-3 multi-supplier
// orders introduce mapping.ItemIndices and this function must filter by it.
// Clearance: when FulfillmentOrderMapping.ItemIndices becomes non-empty
// (multi-supplier split lands), this helper must honor it.
func (s *SupplyChainAppService) rebuildRetryParams(mapping models.FulfillmentOrderMapping) (
	*pb.OrderOpen, contracts.CreateFulfillmentParams, contracts.FailureReason, string, bool,
) {
	orderID := mapping.MobazhaOrderID

	var order models.Order
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	}); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain retry: cannot fetch order %s: %v", orderID, err)
		return nil, contracts.CreateFulfillmentParams{}, contracts.FailureReasonPermanentlyFailed, "order not found", false
	}

	oo, err := order.OrderOpenMessage()
	if err != nil || oo == nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain retry: cannot decode OrderOpen for %s: %v", orderID, err)
		return nil, contracts.CreateFulfillmentParams{}, contracts.FailureReasonPermanentlyFailed, "cannot decode order", false
	}

	recipient := extractRecipientFromOrder(oo)
	var allItems []contracts.FulfillmentItem
	for i, li := range oo.Listings {
		if li == nil || li.Listing == nil {
			continue
		}
		item := contracts.FulfillmentItem{Quantity: 1}
		if i < len(oo.Items) && oo.Items[i] != nil {
			if q, parseErr := strconv.Atoi(oo.Items[i].Quantity); parseErr == nil && q > 0 {
				item.Quantity = q
			}
			item.CatalogVariantID = resolveVariantID(li.Listing, oo.Items[i])
		}
		if item.CatalogVariantID == "" {
			return nil, contracts.CreateFulfillmentParams{}, contracts.FailureReasonValidationFailed,
				fmt.Sprintf("cannot resolve variant for item %d", i), false
		}
		allItems = append(allItems, item)
	}

	return oo, contracts.CreateFulfillmentParams{
		ExternalOrderID: orderID,
		Recipient:       recipient,
		Items:           allItems,
	}, contracts.FailureReasonNone, "", true
}

// executeRetryCreate calls the supplier and persists the outcome.
//
// TECHDEBT(TD-030): Uses the same ExternalOrderID as the first attempt and
// relies on supplier-side uniqueness. If the first call actually succeeded
// at the supplier but the response was lost, retry returns a 4xx duplicate
// error which ClassifyError marks as validation_failed (not retryable). The
// reconcile worker (5min) recovers status. A future improvement is to call
// GetFulfillmentOrder by external_id before retrying to avoid the duplicate
// path entirely.
func (s *SupplyChainAppService) executeRetryCreate(
	ctx context.Context,
	mapping models.FulfillmentOrderMapping,
	params contracts.CreateFulfillmentParams,
	provider contracts.FulfillmentProvider,
) {
	orderID := mapping.MobazhaOrderID
	providerID := mapping.ProviderID

	start := time.Now()
	fo, createErr := provider.CreateFulfillmentOrder(ctx, params)
	duration := time.Since(start)

	if createErr != nil {
		newRetryCount := mapping.RetryCount + 1
		reason := classifyProviderError(providerID, createErr)
		if newRetryCount >= maxRetryAttempts {
			reason = contracts.FailureReasonPermanentlyFailed
		}
		nextRetry := time.Now().Add(retryBackoffBase * time.Duration(1<<uint(newRetryCount)))
		if err := s.db.Update(func(tx database.Tx) error {
			var m models.FulfillmentOrderMapping
			if readErr := tx.Read().Where("id = ?", mapping.ID).First(&m).Error; readErr != nil {
				return readErr
			}
			m.RetryCount = newRetryCount
			m.FailureReason = string(reason)
			m.ErrorMessage = createErr.Error()
			m.NextRetryAt = nextRetry
			m.RetryLockedUntil = time.Time{}
			return tx.Save(&m)
		}); err != nil {
			logger.LogErrorWithIDf(log, s.nodeID,
				"SupplyChain retry: failed to persist failure outcome mappingID=%s err=%v",
				mapping.ID, err)
		}
		logger.LogWarningWithIDf(log, s.nodeID,
			"SupplyChain retry: orderID=%s providerID=%s attempt=%d/%d failureReason=%s duration=%s err=%v",
			orderID, providerID, newRetryCount, maxRetryAttempts, reason, duration, createErr)
		return
	}

	supplierOrderID := fo.ID
	if supplierOrderID == "" {
		supplierOrderID = fo.ExternalID
	}
	if err := s.db.Update(func(tx database.Tx) error {
		var m models.FulfillmentOrderMapping
		if readErr := tx.Read().Where("id = ?", mapping.ID).First(&m).Error; readErr != nil {
			return readErr
		}
		m.FulfillmentOrderID = supplierOrderID
		m.Status = string(fo.Status)
		m.SupplierCost = costTotal(fo.Costs)
		m.FailureReason = ""
		m.ErrorMessage = ""
		m.RetryLockedUntil = time.Time{}
		return tx.Save(&m)
	}); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain retry: created supplier order but failed to persist mappingID=%s supplierOrderID=%s err=%v",
			mapping.ID, supplierOrderID, err)
		return
	}
	logger.LogInfoWithIDf(log, s.nodeID,
		"SupplyChain retry: orderID=%s providerID=%s fulfillmentOrderID=%s attempt=%d supplierCost=%s duration=%s",
		orderID, providerID, supplierOrderID, mapping.RetryCount+1, costTotal(fo.Costs), duration)
}

// markRetryOutcome records a terminal retry result (validation/permanent
// failures, manual_action_required) and releases the lease.
func (s *SupplyChainAppService) markRetryOutcome(mappingID string, currentRetryCount int, reason contracts.FailureReason, msg string) {
	if err := s.db.Update(func(tx database.Tx) error {
		var m models.FulfillmentOrderMapping
		if readErr := tx.Read().Where("id = ?", mappingID).First(&m).Error; readErr != nil {
			return readErr
		}
		m.FailureReason = string(reason)
		m.ErrorMessage = msg
		m.RetryCount = currentRetryCount + 1
		m.RetryLockedUntil = time.Time{}
		return tx.Save(&m)
	}); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain retry: failed to mark outcome mappingID=%s reason=%s err=%v",
			mappingID, reason, err)
	}
}

func (s *SupplyChainAppService) reconcileStaleOrdersLoop(ctx context.Context) {
	interval := reconcileIntervalDefault
	if !s.saasMode && s.webhookBaseURL == "" {
		interval = reconcileIntervalNAT
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: reconcile worker started (interval: %s)", interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case <-ticker.C:
			s.reconcileStaleOrders(ctx)
		}
	}
}

func (s *SupplyChainAppService) reconcileStaleOrders(ctx context.Context) {
	now := time.Now()
	staleThreshold := now.Add(-reconcileStaleThreshold)

	// Track 1: stale pending/in_process fulfillment orders — re-poll supplier
	// for status updates that may have been missed (no webhook, lost event).
	var stale []models.FulfillmentOrderMapping
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"status IN (?, ?) AND updated_at < ? AND fulfillment_order_id != '' AND dispute_held = ? AND (retry_locked_until IS NULL OR retry_locked_until < ?)",
			string(contracts.FulfillmentStatusPending), string(contracts.FulfillmentStatusInProcess),
			staleThreshold, false, now,
		).Find(&stale).Error
	})
	for _, m := range stale {
		if !s.claimRetryLease(m.ID, now) {
			continue
		}
		s.reconcileSingleOrder(ctx, m)
	}

	// Track 2 (P1-3 / TD-075): mappings already advanced to `shipped` whose
	// AutoConfirmAndShip step never succeeded. Without this, an order that
	// shipped at the supplier but whose chain confirm/ship call failed would
	// leave the Mobazha order stuck in `funded` indefinitely.
	var pendingAdvance []models.FulfillmentOrderMapping
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"status = ? AND order_advancement_status = ? AND updated_at < ? AND dispute_held = ? AND (retry_locked_until IS NULL OR retry_locked_until < ?)",
			string(contracts.FulfillmentStatusShipped), advancementStatusPending,
			staleThreshold, false, now,
		).Find(&pendingAdvance).Error
	})
	for _, m := range pendingAdvance {
		if !s.claimRetryLease(m.ID, now) {
			continue
		}
		s.retryOrderAdvancement(m)
	}
}

// retryOrderAdvancement re-attempts AutoConfirmAndShip for a mapping that
// reached `shipped` but failed to advance the Mobazha order state.
// Reconstructs the FulfillmentShipment from persisted tracking fields.
func (s *SupplyChainAppService) retryOrderAdvancement(mapping models.FulfillmentOrderMapping) {
	defer s.releaseLease(mapping.ID)

	if mapping.TrackingNumber == "" && mapping.Carrier == "" && mapping.TrackingURL == "" {
		// No tracking info to advance with — log and skip; manual intervention.
		logger.LogWarningWithIDf(log, s.nodeID,
			"SupplyChain reconcile: orderID=%s mapping shipped but no tracking info to retry advancement",
			mapping.MobazhaOrderID)
		s.markAdvancementStatus(mapping.ID, advancementStatusPermanentFail)
		return
	}

	shipment := &contracts.FulfillmentShipment{
		Carrier:        mapping.Carrier,
		TrackingNumber: mapping.TrackingNumber,
		TrackingURL:    mapping.TrackingURL,
	}
	if err := s.autoConfirmAndShip(mapping.MobazhaOrderID, shipment); err != nil {
		logger.LogWarningWithIDf(log, s.nodeID,
			"SupplyChain reconcile: autoConfirmAndShip retry failed orderID=%s err=%v",
			mapping.MobazhaOrderID, err)
		// Leave OrderAdvancementStatus = pending; next tick retries.
		return
	}
	logger.LogInfoWithIDf(log, s.nodeID,
		"SupplyChain reconcile: order advancement recovered orderID=%s", mapping.MobazhaOrderID)
	s.markAdvancementStatus(mapping.ID, advancementStatusDone)
}

// markAdvancementStatus updates the order_advancement_status column.
// Failures are logged but not returned — they only affect reconcile efficiency.
func (s *SupplyChainAppService) markAdvancementStatus(mappingID, status string) {
	if err := s.db.Update(func(tx database.Tx) error {
		return tx.Update(
			"order_advancement_status", status,
			map[string]interface{}{"id = ?": mappingID},
			&models.FulfillmentOrderMapping{},
		)
	}); err != nil {
		logger.LogWarningWithIDf(log, s.nodeID,
			"SupplyChain: failed to set order_advancement_status=%s on mapping %s: %v",
			status, mappingID, err)
	}
}

func (s *SupplyChainAppService) reconcileSingleOrder(ctx context.Context, mapping models.FulfillmentOrderMapping) {
	provider, err := s.registry.ForProvider(mapping.ProviderID)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain reconcile: providerID=%s orderID=%s err=provider_not_found",
			mapping.ProviderID, mapping.MobazhaOrderID)
		s.releaseLease(mapping.ID)
		return
	}

	start := time.Now()
	fo, err := provider.GetFulfillmentOrder(ctx, mapping.FulfillmentOrderID)
	duration := time.Since(start)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain reconcile: orderID=%s fulfillmentOrderID=%s duration=%s err=%v",
			mapping.MobazhaOrderID, mapping.FulfillmentOrderID, duration, err)
		s.releaseLease(mapping.ID)
		return
	}

	s.applyFulfillmentStatus(ctx, mapping, fo)
}

// applyFulfillmentStatus is the unified status transition logic shared by webhook
// and reconcile paths. It always releases the retry lease before returning,
// regardless of whether the supplier-side status changed.
func (s *SupplyChainAppService) applyFulfillmentStatus(_ context.Context, mapping models.FulfillmentOrderMapping, fo *contracts.FulfillmentOrder) {
	if fo == nil {
		s.releaseLease(mapping.ID)
		return
	}

	newStatus := string(fo.Status)
	if newStatus == mapping.Status {
		s.releaseLease(mapping.ID)
		return
	}

	var shipment *contracts.FulfillmentShipment
	if err := s.db.Update(func(tx database.Tx) error {
		var m models.FulfillmentOrderMapping
		if readErr := tx.Read().Where("id = ?", mapping.ID).First(&m).Error; readErr != nil {
			return readErr
		}
		m.Status = newStatus
		m.RetryLockedUntil = time.Time{}
		if len(fo.Shipments) > 0 {
			shipment = &fo.Shipments[0]
			m.Carrier = shipment.Carrier
			m.TrackingNumber = shipment.TrackingNumber
			m.TrackingURL = shipment.TrackingURL
		}
		// P1-3: As soon as we mark the mapping `shipped`, flag advancement as
		// pending so the reconcile worker can retry AutoConfirmAndShip if the
		// inline call below fails.
		if fo.Status == contracts.FulfillmentStatusShipped && m.OrderAdvancementStatus == "" {
			m.OrderAdvancementStatus = advancementStatusPending
		}
		return tx.Save(&m)
	}); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain reconcile: failed to persist status orderID=%s fulfillmentOrderID=%s err=%v",
			mapping.MobazhaOrderID, mapping.FulfillmentOrderID, err)
		s.releaseLease(mapping.ID)
		return
	}

	if fo.Status == contracts.FulfillmentStatusShipped && shipment != nil {
		if err := s.autoConfirmAndShip(mapping.MobazhaOrderID, shipment); err != nil {
			logger.LogWarningWithIDf(log, s.nodeID,
				"SupplyChain reconcile: autoConfirmAndShip failed orderID=%s err=%v (will retry via reconcile)",
				mapping.MobazhaOrderID, err)
			// Leave OrderAdvancementStatus = pending so reconcile worker retries.
		} else {
			s.markAdvancementStatus(mapping.ID, advancementStatusDone)
		}
	}

	logger.LogInfoWithIDf(log, s.nodeID,
		"SupplyChain reconcile: orderID=%s fulfillmentOrderID=%s statusOld=%s statusNew=%s",
		mapping.MobazhaOrderID, mapping.FulfillmentOrderID, mapping.Status, newStatus)
}

// releaseLease clears retry_locked_until so subsequent worker ticks can
// re-claim this mapping. Failures are logged but not returned — the lease
// will auto-expire after retryLeaseDuration as a fallback.
func (s *SupplyChainAppService) releaseLease(mappingID string) {
	if err := s.db.Update(func(tx database.Tx) error {
		return tx.Update(
			"retry_locked_until", time.Time{},
			map[string]interface{}{"id = ?": mappingID},
			&models.FulfillmentOrderMapping{},
		)
	}); err != nil {
		logger.LogWarningWithIDf(log, s.nodeID,
			"SupplyChain: failed to release lease on mapping %s: %v (will auto-expire)", mappingID, err)
	}
}

func (s *SupplyChainAppService) cleanupProcessedEventsLoop(ctx context.Context) {
	ticker := time.NewTicker(eventCleanupInterval)
	defer ticker.Stop()
	logger.LogInfoWithID(log, s.nodeID, "SupplyChain: event cleanup worker started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case <-ticker.C:
			s.cleanupProcessedEvents()
		}
	}
}

func (s *SupplyChainAppService) cleanupProcessedEvents() {
	cutoff := time.Now().Add(-eventRetentionTTL)
	if err := s.db.Update(func(tx database.Tx) error {
		return tx.Delete(
			"status", "processed",
			map[string]interface{}{"processed_at < ?": cutoff},
			&models.ProcessedFulfillmentEvent{},
		)
	}); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: event cleanup failed: %v", err)
	}
}

func (s *SupplyChainAppService) subscribeOrderFunded() {
	sub, err := s.eventBus.Subscribe(&events.OrderFunded{})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: failed to subscribe to OrderFunded: %v", err)
		return
	}
	logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: OrderFunded fulfillment monitor started")

	for {
		select {
		case event := <-sub.Out():
			if e, ok := event.(*events.OrderFunded); ok {
				go s.handleOrderFunded(e)
			}
		case <-s.shutdown:
			sub.Close()
			logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: OrderFunded fulfillment monitor stopped")
			return
		}
	}
}

func (s *SupplyChainAppService) subscribeOrderCancel() {
	sub, err := s.eventBus.Subscribe(&events.OrderCancel{})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: failed to subscribe to OrderCancel: %v", err)
		return
	}
	for {
		select {
		case event := <-sub.Out():
			if e, ok := event.(*events.OrderCancel); ok {
				go s.cancelFulfillmentForOrder(e.OrderID)
			}
		case <-s.shutdown:
			sub.Close()
			return
		}
	}
}

func (s *SupplyChainAppService) subscribeRefund() {
	sub, err := s.eventBus.Subscribe(&events.Refund{})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: failed to subscribe to Refund: %v", err)
		return
	}
	for {
		select {
		case event := <-sub.Out():
			if e, ok := event.(*events.Refund); ok {
				go s.handleOrderRefunded(e.OrderID)
			}
		case <-s.shutdown:
			sub.Close()
			return
		}
	}
}

// handleOrderRefunded handles a refund event. If the fulfillment has already shipped,
// the supplier cost is unrecoverable — mark as supplier_loss and notify the seller.
func (s *SupplyChainAppService) handleOrderRefunded(orderID string) {
	var mapping models.FulfillmentOrderMapping
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("mobazha_order_id = ?", orderID).First(&mapping).Error
	})
	if err != nil {
		return
	}

	postShipStatuses := map[string]bool{
		string(contracts.FulfillmentStatusShipped):   true,
		string(contracts.FulfillmentStatusDelivered): true,
	}

	if postShipStatuses[mapping.Status] {
		lossMsg := fmt.Sprintf("Refund issued after fulfillment %s. Supplier cost %s is not recoverable.",
			mapping.Status, mapping.SupplierCost)
		if err := s.db.Update(func(tx database.Tx) error {
			var m models.FulfillmentOrderMapping
			if readErr := tx.Read().Where("id = ?", mapping.ID).First(&m).Error; readErr != nil {
				return readErr
			}
			m.Status = string(contracts.FulfillmentStatusSupplierLoss)
			m.ErrorMessage = lossMsg
			return tx.Save(&m)
		}); err != nil {
			logger.LogErrorWithIDf(log, s.nodeID,
				"SupplyChain: failed to mark supplier_loss for order %s: %v", orderID, err)
		}
		logger.LogWarningWithIDf(log, s.nodeID,
			"SupplyChain: supplier_loss for order %s — %s", orderID, lossMsg)
		return
	}

	s.cancelFulfillmentForOrder(orderID)
}

func (s *SupplyChainAppService) subscribeDisputeOpen() {
	sub, err := s.eventBus.Subscribe(&events.DisputeOpen{})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: failed to subscribe to DisputeOpen: %v", err)
		return
	}
	for {
		select {
		case event := <-sub.Out():
			if e, ok := event.(*events.DisputeOpen); ok {
				go s.handleDisputeOpened(e.OrderID)
			}
		case <-s.shutdown:
			sub.Close()
			return
		}
	}
}

func (s *SupplyChainAppService) subscribeDisputeClose() {
	sub, err := s.eventBus.Subscribe(&events.DisputeClose{})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: failed to subscribe to DisputeClose: %v", err)
		return
	}
	for {
		select {
		case event := <-sub.Out():
			if e, ok := event.(*events.DisputeClose); ok {
				go s.handleDisputeClosed(e.OrderID)
			}
		case <-s.shutdown:
			sub.Close()
			return
		}
	}
}

func (s *SupplyChainAppService) handleDisputeOpened(orderID string) {
	var mappings []models.FulfillmentOrderMapping
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("mobazha_order_id = ? AND status IN (?, ?)",
			orderID, string(contracts.FulfillmentStatusPending), string(contracts.FulfillmentStatusInProcess),
		).Find(&mappings).Error
	})
	if len(mappings) == 0 {
		return
	}

	for _, m := range mappings {
		if err := s.db.Update(func(tx database.Tx) error {
			var mapping models.FulfillmentOrderMapping
			if readErr := tx.Read().Where("id = ?", m.ID).First(&mapping).Error; readErr != nil {
				return readErr
			}
			mapping.DisputeHeld = true
			return tx.Save(&mapping)
		}); err != nil {
			logger.LogErrorWithIDf(log, s.nodeID,
				"SupplyChain: failed to set DisputeHeld on mapping %s: %v", m.ID, err)
		}
	}
	logger.LogInfoWithIDf(log, s.nodeID,
		"SupplyChain: dispute opened for order %s — held %d fulfillment(s)", orderID, len(mappings))
}

func (s *SupplyChainAppService) handleDisputeClosed(orderID string) {
	var mappings []models.FulfillmentOrderMapping
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("mobazha_order_id = ? AND dispute_held = ?", orderID, true).Find(&mappings).Error
	})
	if len(mappings) == 0 {
		return
	}

	// Determine outcome by querying the order's current state
	var buyerWon bool
	if s.orderOps != nil {
		state, err := s.orderOps.GetOrderState(models.OrderID(orderID))
		if err == nil {
			buyerWon = state == models.OrderState_REFUNDED || state == models.OrderState_CANCELED
		} else {
			logger.LogWarningWithIDf(log, s.nodeID,
				"SupplyChain: cannot determine dispute outcome for %s: %v — keeping held", orderID, err)
			return
		}
	}

	// First clear DisputeHeld so workers and cancelFulfillmentForOrder are
	// no longer skipped by the dispute_held = false filter. This must
	// happen BEFORE invoking cancelFulfillmentForOrder to avoid the supplier
	// cancel call racing against a still-held mapping read.
	for _, m := range mappings {
		if updErr := s.db.Update(func(tx database.Tx) error {
			var mapping models.FulfillmentOrderMapping
			if readErr := tx.Read().Where("id = ?", m.ID).First(&mapping).Error; readErr != nil {
				return readErr
			}
			mapping.DisputeHeld = false
			return tx.Save(&mapping)
		}); updErr != nil {
			logger.LogErrorWithIDf(log, s.nodeID,
				"SupplyChain: failed to clear DisputeHeld on mapping %s: %v", m.ID, updErr)
		}
	}

	if buyerWon {
		// Synchronous cancel: cancelFulfillmentForOrder calls the supplier API
		// and writes Status=canceled itself. Running it inline (not `go`) avoids
		// a race against any subsequent worker tick reading a stale mapping.
		s.cancelFulfillmentForOrder(orderID)
	}

	outcome := "seller won — resuming"
	if buyerWon {
		outcome = "buyer won — canceling"
	}
	logger.LogInfoWithIDf(log, s.nodeID,
		"SupplyChain: dispute closed for order %s (%s), %d fulfillment(s) updated", orderID, outcome, len(mappings))
}

// cancelFulfillmentForOrder attempts to cancel the supplier fulfillment order
// associated with a Mobazha order. No-op if no mapping exists (non-supply-chain order).
func (s *SupplyChainAppService) cancelFulfillmentForOrder(orderID string) {
	var mapping models.FulfillmentOrderMapping
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("mobazha_order_id = ?", orderID).First(&mapping).Error
	})
	if err != nil {
		return
	}

	terminalStatuses := map[string]bool{
		string(contracts.FulfillmentStatusShipped):   true,
		string(contracts.FulfillmentStatusDelivered): true,
		string(contracts.FulfillmentStatusCanceled):  true,
	}
	if terminalStatuses[mapping.Status] {
		logger.LogInfoWithIDf(log, s.nodeID,
			"SupplyChain: skipping cancel for order %s — fulfillment already %s", orderID, mapping.Status)
		return
	}

	provider, err := s.registry.ForProvider(mapping.ProviderID)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain: cannot cancel fulfillment for order %s — provider %s not found: %v",
			orderID, mapping.ProviderID, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := provider.CancelFulfillmentOrder(ctx, mapping.FulfillmentOrderID); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain: failed to cancel fulfillment %s for order %s: %v",
			mapping.FulfillmentOrderID, orderID, err)
		if updateErr := s.db.Update(func(tx database.Tx) error {
			return tx.Update("error_message", fmt.Sprintf("cancel failed: %v", err),
				map[string]interface{}{"mobazha_order_id = ?": orderID},
				&models.FulfillmentOrderMapping{})
		}); updateErr != nil {
			logger.LogErrorWithIDf(log, s.nodeID,
				"SupplyChain: failed to update error on mapping for order %s: %v", orderID, updateErr)
		}
		return
	}

	if err := s.db.Update(func(tx database.Tx) error {
		return tx.Update("status", string(contracts.FulfillmentStatusCanceled),
			map[string]interface{}{"mobazha_order_id = ?": orderID},
			&models.FulfillmentOrderMapping{})
	}); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain: failed to update cancel status for order %s: %v", orderID, err)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID,
		"SupplyChain: cancelled fulfillment %s for order %s", mapping.FulfillmentOrderID, orderID)
}

// handleOrderFunded checks whether the funded order contains supply-chain-managed
// listings and, if so, creates a fulfillment order at the supplier.
// It does NOT call ConfirmOrder — that happens later when the supplier confirms shipment.
func (s *SupplyChainAppService) handleOrderFunded(event *events.OrderFunded) {
	ctx := context.Background()
	orderID := event.OrderID

	var order models.Order
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	}); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: cannot fetch order %s: %v", orderID, err)
		return
	}

	oo, err := order.OrderOpenMessage()
	if err != nil || oo == nil {
		logger.LogWarningWithIDf(log, s.nodeID, "SupplyChain: cannot decode OrderOpen for %s: %v", orderID, err)
		return
	}

	// Skip MODERATED orders — they need manual multi-sig confirmation
	if order.PaymentMethod() == pb.PaymentSent_MODERATED {
		logger.LogInfoWithIDf(log, s.nodeID,
			"SupplyChain: skipping auto-fulfillment for MODERATED order %s", orderID)
		return
	}

	// Find which items are supply-chain-managed and group by provider
	type providerItems struct {
		providerID string
		items      []contracts.FulfillmentItem
		itemSlug   string
	}
	var groups []providerItems
	totalListings := 0

	for i, li := range oo.Listings {
		if li == nil || li.Listing == nil {
			continue
		}
		totalListings++
		slug := li.Listing.GetSlug()
		if slug == "" {
			continue
		}
		var mapping models.SyncedProductMapping
		findErr := s.db.View(func(tx database.Tx) error {
			return tx.Read().Where("listing_slug = ?", slug).First(&mapping).Error
		})
		if findErr != nil {
			continue
		}
		item := contracts.FulfillmentItem{
			Quantity: 1,
		}
		if i < len(oo.Items) && oo.Items[i] != nil {
			if q, parseErr := strconv.Atoi(oo.Items[i].Quantity); parseErr == nil && q > 0 {
				item.Quantity = q
			}
			item.CatalogVariantID = resolveVariantID(li.Listing, oo.Items[i])
		}
		if item.CatalogVariantID == "" {
			logger.LogWarningWithIDf(log, s.nodeID,
				"SupplyChain: order %s item %d (%s): could not resolve variant ID from buyer selections — skipping auto-fulfillment (fail closed)",
				orderID, i, slug)
			return
		}
		groups = append(groups, providerItems{
			providerID: mapping.ProviderID,
			items:      []contracts.FulfillmentItem{item},
			itemSlug:   slug,
		})
	}

	if len(groups) == 0 {
		return
	}

	// Safety: reject mixed orders where some items are supply-chain-managed and others are not.
	// ShipOrder applies to ALL physical items, so shipping only the POD items would incorrectly
	// mark manually-fulfilled items as shipped too. FF-3 will add per-item-index shipping.
	if len(groups) < totalListings {
		logger.LogWarningWithIDf(log, s.nodeID,
			"SupplyChain: order %s has %d/%d items managed by suppliers — skipping mixed order (not fully managed)",
			orderID, len(groups), totalListings)
		return
	}

	// Safety: all items must be from the same provider. Multi-provider split is FF-3.
	providerID := groups[0].providerID
	for _, g := range groups[1:] {
		if g.providerID != providerID {
			logger.LogWarningWithIDf(log, s.nodeID,
				"SupplyChain: order %s has items from multiple providers (%s, %s) — skipping until FF-3",
				orderID, providerID, g.providerID)
			return
		}
	}

	recipient := extractRecipientFromOrder(oo)

	var allItems []contracts.FulfillmentItem
	for _, g := range groups {
		allItems = append(allItems, g.items...)
	}

	params := contracts.CreateFulfillmentParams{
		ExternalOrderID: orderID,
		Recipient:       recipient,
		Items:           allItems,
	}

	// --- Margin Protection (FF-2.6) ---
	// Shared with retry worker via evaluateMarginGate (supply_chain_margin.go).
	if ok, reason, msg := s.evaluateMarginGate(ctx, supplyMarginInputs{
		oo:         oo,
		providerID: providerID,
		recipient:  recipient,
		items:      allItems,
	}); !ok {
		s.failFulfillmentWithReason(orderID, providerID, reason, msg)
		return
	}

	fo, err := s.createFulfillmentForItems(ctx, orderID, providerID, params)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain: failed to create fulfillment for order %s: %v", orderID, err)
		return
	}
	logger.LogInfoWithIDf(log, s.nodeID,
		"SupplyChain: created fulfillment order %s for Mobazha order %s (provider: %s)",
		fo.ExternalID, orderID, providerID)
}

// extractRecipientFromOrder builds a FulfillmentRecipient from the order's shipping address.
func extractRecipientFromOrder(oo *pb.OrderOpen) contracts.FulfillmentRecipient {
	r := contracts.FulfillmentRecipient{}
	if oo.Shipping == nil {
		return r
	}
	r.Name = oo.Shipping.ShipTo
	r.Address1 = oo.Shipping.Address
	r.City = oo.Shipping.City
	r.StateCode = oo.Shipping.State
	r.CountryCode = oo.Shipping.Country
	r.ZIP = oo.Shipping.PostalCode
	return r
}

// rebuildProviders scans FulfillmentProviderConfig WHERE status='connected',
// decrypts credentials, instantiates the corresponding provider, and registers it.
func (s *SupplyChainAppService) rebuildProviders(_ context.Context) error {
	var configs []models.FulfillmentProviderConfig
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("status = ?", "connected").Find(&configs).Error
	})
	if err != nil {
		return fmt.Errorf("scan connected providers: %w", err)
	}
	if len(configs) == 0 {
		logger.LogInfoWithID(log, s.nodeID, "SupplyChain: no connected providers to rebuild")
		return nil
	}

	var rebuilt int
	for _, cfg := range configs {
		provider, err := s.instantiateProvider(cfg.ProviderID, cfg.ProviderType, cfg.Credentials, cfg.WebhookSecret)
		if err != nil {
			logger.LogErrorWithIDf(log, s.nodeID,
				"SupplyChain: failed to rebuild provider %q: %v — marking error", cfg.ProviderID, err)
			_ = s.db.Update(func(tx database.Tx) error {
				return tx.Update("status", "error",
					map[string]interface{}{"id = ?": cfg.ID},
					&models.FulfillmentProviderConfig{})
			})
			continue
		}
		if regErr := s.registry.Register(provider); regErr != nil {
			logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: failed to register rebuilt provider %q: %v", cfg.ProviderID, regErr)
			continue
		}
		rebuilt++
	}
	logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: rebuilt %d/%d providers from DB", rebuilt, len(configs))
	return nil
}

// newProviderFromCredentials creates a provider from plaintext credentials (used during ConnectProvider).
func (s *SupplyChainAppService) newProviderFromCredentials(providerID string, creds contracts.ProviderCredentials) (contracts.FulfillmentProvider, error) {
	switch providerID {
	case "printful":
		return printful.NewProvider(creds.APIKey, ""), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", providerID)
	}
}

// instantiateProvider creates the concrete FulfillmentProvider from persisted (encrypted) config.
func (s *SupplyChainAppService) instantiateProvider(providerID, providerType string, credBlob []byte, webhookSecret string) (contracts.FulfillmentProvider, error) {
	plaintext, err := decryptAESGCM(s.credKey[:], credBlob)
	if err != nil {
		return nil, fmt.Errorf("decrypt credentials: %w", err)
	}
	var creds contracts.ProviderCredentials
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return nil, fmt.Errorf("unmarshal credentials: %w", err)
	}

	switch providerID {
	case "printful":
		// Printful v1 API does not support webhook payload signing.
		// Authentication relies on URL secret ({webhookSecret} in path).
		// Pass empty string so ParseWebhook skips HMAC verification.
		return printful.NewProvider(creds.APIKey, ""), nil
	default:
		return nil, fmt.Errorf("unknown provider %q (type %s)", providerID, providerType)
	}
}

// ---------------------------------------------------------------------------
// Provider Management
// ---------------------------------------------------------------------------

func (s *SupplyChainAppService) ConnectProvider(ctx context.Context, params contracts.ConnectProviderParams) (*contracts.ProviderConnection, error) {
	providerID := params.ProviderID
	if providerID == "" {
		return nil, fmt.Errorf("providerId is required")
	}

	provider, err := s.newProviderFromCredentials(providerID, params.Credentials)
	if err != nil {
		return nil, fmt.Errorf("unsupported provider: %s: %w", providerID, err)
	}

	if err := provider.ValidateCredentials(ctx, params.Credentials); err != nil {
		return nil, fmt.Errorf("credential validation failed: %w", err)
	}

	credJSON, err := json.Marshal(params.Credentials)
	if err != nil {
		return nil, fmt.Errorf("marshal credentials: %w", err)
	}
	encryptedCred, err := encryptAESGCM(s.credKey[:], credJSON)
	if err != nil {
		return nil, fmt.Errorf("encrypt credentials: %w", err)
	}

	webhookSecret, err := generateWebhookSecret()
	if err != nil {
		return nil, fmt.Errorf("generate webhook secret: %w", err)
	}

	now := time.Now()
	cfg := &models.FulfillmentProviderConfig{
		ID:            uuid.New().String(),
		ProviderID:    providerID,
		ProviderType:  provider.ProviderType(),
		Credentials:   encryptedCred,
		WebhookSecret: webhookSecret,
		StoreID:       params.Credentials.StoreID,
		Status:        "connected",
		ConnectedAt:   now,
		LastSyncAt:    now,
	}

	if err := s.db.Update(func(tx database.Tx) error {
		var existing models.FulfillmentProviderConfig
		if tx.Read().Where("provider_id = ?", providerID).Select("id").First(&existing).Error == nil {
			cfg.ID = existing.ID
		}
		return tx.Save(cfg)
	}); err != nil {
		return nil, fmt.Errorf("persist provider config: %w", err)
	}

	if regErr := s.registry.Register(provider); regErr != nil {
		return nil, fmt.Errorf("register provider: %w", regErr)
	}

	var webhookURL string
	if params.WebhookBaseURL != "" {
		webhookURL = params.WebhookBaseURL + "/" + webhookSecret
	}
	conn := &contracts.ProviderConnection{
		ProviderID:   providerID,
		ProviderType: provider.ProviderType(),
		ProviderName: providerID,
		Status:       "connected",
		StoreName:    cfg.StoreName,
		WebhookURL:   webhookURL,
		ConnectedAt:  now,
	}
	return conn, nil
}

func (s *SupplyChainAppService) DisconnectProvider(_ context.Context, providerID string) error {
	s.registry.Unregister(providerID)

	return s.db.Update(func(tx database.Tx) error {
		return tx.Update("status", "disconnected",
			map[string]interface{}{"provider_id = ?": providerID},
			&models.FulfillmentProviderConfig{})
	})
}

func (s *SupplyChainAppService) GetProviderStatus(_ context.Context, providerID string) (*contracts.ProviderConnection, error) {
	var cfg models.FulfillmentProviderConfig
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("provider_id = ?", providerID).First(&cfg).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, contracts.ErrFulfillmentProviderNotFound
		}
		return nil, err
	}
	return configToConnection(&cfg), nil
}

func (s *SupplyChainAppService) ListConnections(_ context.Context) ([]contracts.ProviderConnection, error) {
	var configs []models.FulfillmentProviderConfig
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Find(&configs).Error
	})
	if err != nil {
		return nil, err
	}
	conns := make([]contracts.ProviderConnection, len(configs))
	for i := range configs {
		conns[i] = *configToConnection(&configs[i])
	}
	return conns, nil
}

// ---------------------------------------------------------------------------
// Catalog Browsing (delegates to provider)
// ---------------------------------------------------------------------------

func (s *SupplyChainAppService) BrowseCatalog(ctx context.Context, providerID string, query contracts.CatalogQuery) (*contracts.CatalogPage, error) {
	cat, err := s.getCatalogProvider(providerID)
	if err != nil {
		return nil, err
	}
	return cat.ListProducts(ctx, query)
}

func (s *SupplyChainAppService) GetCatalogProduct(ctx context.Context, providerID string, productID string) (*contracts.CatalogProduct, error) {
	cat, err := s.getCatalogProvider(providerID)
	if err != nil {
		return nil, err
	}
	return cat.GetProduct(ctx, productID)
}

func (s *SupplyChainAppService) EstimateShipping(ctx context.Context, providerID string, params contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return nil, err
	}
	return provider.EstimateShipping(ctx, params)
}

func (s *SupplyChainAppService) getCatalogProvider(providerID string) (contracts.FulfillmentCatalogProvider, error) {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return nil, err
	}
	cat, ok := provider.(contracts.FulfillmentCatalogProvider)
	if !ok {
		return nil, contracts.ErrFulfillmentCatalogNotSupported
	}
	return cat, nil
}

// ---------------------------------------------------------------------------
// Product Import & Sync
// ---------------------------------------------------------------------------

func (s *SupplyChainAppService) ImportProduct(ctx context.Context, params contracts.ImportProductParams) (*contracts.ImportResult, error) {
	if s.listingOps == nil {
		return nil, fmt.Errorf("ImportProduct: listing ops not wired")
	}

	cat, err := s.getCatalogProvider(params.ProviderID)
	if err != nil {
		return nil, err
	}

	product, err := cat.GetProduct(ctx, params.ProductID)
	if err != nil {
		return nil, fmt.Errorf("fetch catalog product: %w", err)
	}

	variants := product.Variants
	if len(params.VariantIDs) > 0 {
		variants = filterVariants(product.Variants, params.VariantIDs)
		if len(variants) == 0 {
			return nil, fmt.Errorf("none of the requested variant IDs match the catalog product")
		}
	}
	if len(variants) == 0 {
		return nil, fmt.Errorf("catalog product has no variants")
	}

	markup := params.RetailMarkup
	if markup <= 0 {
		markup = 1.0
	}

	listing, supplierCost, retailPrice, buildErr := s.buildListingFromCatalog(product, variants, markup, params)
	if buildErr != nil {
		return nil, fmt.Errorf("build listing from catalog: %w", buildErr)
	}

	done := make(chan struct{})
	if err := s.listingOps.SaveListing(listing, done); err != nil {
		return nil, fmt.Errorf("save listing draft: %w", err)
	}
	<-done

	variantMeta, metaErr := buildVariantMetadata(variants, markup)
	if metaErr != nil {
		// Should already have failed in buildListingFromCatalog above; defensive.
		return nil, fmt.Errorf("build variant metadata: %w", metaErr)
	}
	metaJSON, _ := json.Marshal(variantMeta)

	mapping := &models.SyncedProductMapping{
		ID:            uuid.NewString(),
		ProviderID:    params.ProviderID,
		ListingSlug:   listing.Slug,
		ExternalID:    product.ID,
		SyncProductID: product.ID,
		SupplierCost:  supplierCost,
		RetailPrice:   retailPrice,
		Status:        "synced",
		LastSyncAt:    time.Now(),
		Metadata:      metaJSON,
	}
	if err := s.db.Update(func(tx database.Tx) error {
		return tx.Save(mapping)
	}); err != nil {
		return nil, fmt.Errorf("save synced product mapping: %w", err)
	}

	return &contracts.ImportResult{
		ListingSlug:   listing.Slug,
		SyncProductID: product.ID,
		VariantsCount: len(variants),
		RetailPrice:   retailPrice,
		SupplierCost:  supplierCost,
	}, nil
}

// buildListingFromCatalog converts a catalog product into a draft protobuf Listing.
// Returns the listing, supplier cost string, and retail price string.
// buildListingFromCatalog renders a draft listing + canonical
// (supplierCost, retailPrice) snapshot from a provider catalog product.
//
// If ANY catalog variant carries an unparseable price string, the function
// fails the whole import — silently writing 0-cost SKUs would (a) create a
// public listing with garbage prices for buyers and (b) confuse the margin
// gate later (cost=0 trivially passes the 80% rule). Reviewer P2 (2nd
// pass): "导入阶段应该对任何选中 variant 的价格解析失败直接返回错误".
func (s *SupplyChainAppService) buildListingFromCatalog(
	product *contracts.CatalogProduct,
	variants []contracts.CatalogVariant,
	markup float64,
	params contracts.ImportProductParams,
) (*pb.Listing, string, string, error) {
	title := product.Title
	if params.Title != "" {
		title = params.Title
	}
	description := product.Description
	if params.Description != "" {
		description = params.Description
	}
	tags := params.Tags
	if len(tags) == 0 {
		tags = []string{"pod", "print-on-demand"}
	}

	currency := product.Currency
	if currency == "" {
		currency = "USD"
	}

	var images []*pb.Image
	for _, url := range product.Images {
		if url != "" {
			images = append(images, &pb.Image{
				Filename: url,
				Large:    url,
				Medium:   url,
				Small:    url,
				Tiny:     url,
			})
		}
	}
	if len(images) == 0 && product.ImageURL != "" {
		images = []*pb.Image{{
			Filename: product.ImageURL,
			Large:    product.ImageURL,
			Medium:   product.ImageURL,
			Small:    product.ImageURL,
			Tiny:     product.ImageURL,
		}}
	}

	attrNames := collectOptionNames(variants)
	var options []*pb.Listing_Item_Option
	for _, attr := range attrNames {
		seen := map[string]bool{}
		var optVariants []*pb.Listing_Item_Option_Variant
		for _, v := range variants {
			val := v.Attributes[attr]
			if val != "" && !seen[val] {
				seen[val] = true
				optVariants = append(optVariants, &pb.Listing_Item_Option_Variant{Name: val})
			}
		}
		options = append(options, &pb.Listing_Item_Option{
			Name:     attr,
			Variants: optVariants,
		})
	}

	// Track cheapest variant in CENTS (not float dollars) so the listing-level
	// snapshot in SyncedProductMapping.SupplierCost is exact.
	var minCostCents uint64
	var skus []*pb.Listing_Item_Sku
	for _, v := range variants {
		costCents, parseOK := parseUSDDollarsToCents(v.Price)
		if !parseOK {
			// Fail closed: reject the whole import. Silently writing a 0-cost
			// SKU would publish bogus pricing to buyers and cause the margin
			// gate to trivially pass (cost=0 always under 80% of retail).
			return nil, "", "", fmt.Errorf("variant %q has unparseable price %q (must be USD decimal like \"4.50\")",
				v.ID, v.Price)
		}
		if costCents == 0 {
			return nil, "", "", fmt.Errorf("variant %q has zero price — refusing to import (would create $0 SKU)", v.ID)
		}
		if minCostCents == 0 || costCents < minCostCents {
			minCostCents = costCents
		}
		retailCents := computeRetailCents(costCents, markup)
		retailStr := strconv.FormatUint(retailCents, 10)

		var selections []*pb.Listing_Item_Sku_Selection
		for _, attr := range attrNames {
			if val := v.Attributes[attr]; val != "" {
				selections = append(selections, &pb.Listing_Item_Sku_Selection{
					Option:  attr,
					Variant: val,
				})
			}
		}

		// SECURITY: do NOT write supplier wholesale cost into CompareAtPrice.
		// CompareAtPrice is a public "original / strike-through price" rendered
		// on the storefront detail page (see Listing.proto and unified
		// useProductDetail). Keep per-variant cost in the private
		// SyncedProductMapping.Metadata blob (loadVariantCostMap).
		sku := &pb.Listing_Item_Sku{
			Selections: selections,
			ProductID:  v.ID,
			Quantity:   "999",
			Price:      retailStr,
		}
		if v.ImageURL != "" {
			sku.Images = []*pb.Image{{
				Filename: v.ImageURL,
				Large:    v.ImageURL,
				Medium:   v.ImageURL,
				Small:    v.ImageURL,
				Tiny:     v.ImageURL,
			}}
		}
		skus = append(skus, sku)
	}

	supplierCost := strconv.FormatUint(minCostCents, 10)
	retailPrice := strconv.FormatUint(computeRetailCents(minCostCents, markup), 10)

	listing := &pb.Listing{
		Slug:   uuid.NewString(),
		Status: models.ListingStatusDraft,
		Item: &pb.Listing_Item{
			Title:       title,
			Description: description,
			Tags:        tags,
			Images:      images,
			Options:     options,
			Skus:        skus,
			Price:       retailPrice,
			ProductType: "physical",
		},
		Metadata: &pb.Listing_Metadata{
			Version:      1,
			ContractType: pb.Listing_Metadata_PHYSICAL_GOOD,
			Format:       pb.Listing_Metadata_FIXED_PRICE,
			PricingCurrency: &pb.Currency{
				Code:         currency,
				Divisibility: 2,
			},
		},
	}

	return listing, supplierCost, retailPrice, nil
}

// filterVariants keeps only variants whose IDs appear in the requested set.
func filterVariants(all []contracts.CatalogVariant, ids []string) []contracts.CatalogVariant {
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	var out []contracts.CatalogVariant
	for _, v := range all {
		if want[v.ID] {
			out = append(out, v)
		}
	}
	return out
}

// resolveVariantID finds the catalog variant ID by matching the buyer's
// selected options against the listing's SKU table. Each SKU stores the
// CatalogVariant.ID in its ProductID field (set during ImportProduct).
func resolveVariantID(listing *pb.Listing, orderItem *pb.OrderOpen_Item) string {
	if listing == nil || listing.Item == nil || orderItem == nil {
		return ""
	}

	buyerSelections := make(map[string]string, len(orderItem.Options))
	for _, opt := range orderItem.Options {
		buyerSelections[opt.Name] = opt.Value
	}
	if len(buyerSelections) == 0 {
		if len(listing.Item.Skus) == 1 {
			return listing.Item.Skus[0].GetProductID()
		}
		return ""
	}

	for _, sku := range listing.Item.Skus {
		if matchesSKUSelections(sku, buyerSelections) {
			return sku.GetProductID()
		}
	}
	return ""
}

func matchesSKUSelections(sku *pb.Listing_Item_Sku, buyerSelections map[string]string) bool {
	if sku == nil || len(sku.Selections) == 0 {
		return false
	}
	for _, sel := range sku.Selections {
		if buyerSelections[sel.Option] != sel.Variant {
			return false
		}
	}
	return true
}

// collectOptionNames extracts the unique option attribute names across all
// variants, preserving first-seen order (e.g. "Size", "Color").
func collectOptionNames(variants []contracts.CatalogVariant) []string {
	seen := map[string]bool{}
	var names []string
	for _, v := range variants {
		for k := range v.Attributes {
			if !seen[k] {
				seen[k] = true
				names = append(names, k)
			}
		}
	}
	return names
}

// parseFloat is a best-effort float parser; returns 0 on error.
func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// variantMetadataEntry stores the mapping between a catalog variant and the
// listing SKU productID, plus PRIVATE economic data the margin gate needs.
//
// SupplierCostCents and RetailCents must NOT leak to public listing fields:
// `Listing.Skus[i].CompareAtPrice` is rendered as a strike-through "original
// price" on the storefront detail page and would expose Printful's wholesale
// cost to buyers. Keep economics in this private metadata only.
type variantMetadataEntry struct {
	CatalogVariantID  string            `json:"catalogVariantId"`
	SKU               string            `json:"sku,omitempty"`
	Attributes        map[string]string `json:"attributes"`
	SupplierCostCents uint64            `json:"supplierCostCents,omitempty"`
	RetailCents       uint64            `json:"retailCents,omitempty"`
}

// buildVariantMetadata captures per-variant supplier cost + computed retail at
// import time. `markup` matches the value used by buildListingFromCatalog so
// the snapshot stays internally consistent.
//
// Inputs are USD decimal strings from the provider catalog ("12.50", "8.29").
// We use parseUSDDollarsToCents for exact integer conversion — using
// `parseFloat(v.Price) * 100` would silently truncate values like "8.29" to
// 828 cents (binary-float representation of 8.29 is 8.28999...). Margin gate
// later trusts these numbers; off-by-one cents on each variant compounds to
// real money on bulk-import catalogs.
//
// Retail = ceil(cost * markup) — rounding UP avoids letting a fractional cent
// slip into the seller's margin and pass the safety gate by accident.
//
// Returns an error when ANY variant price is unparseable. Callers (currently
// only ImportProduct) must surface this to the user — a 0-cent metadata blob
// would let a thin-margin order squeak past the gate later.
func buildVariantMetadata(variants []contracts.CatalogVariant, markup float64) ([]variantMetadataEntry, error) {
	entries := make([]variantMetadataEntry, 0, len(variants))
	for _, v := range variants {
		costCents, parseOK := parseUSDDollarsToCents(v.Price)
		if !parseOK {
			return nil, fmt.Errorf("variant %q has unparseable price %q (must be USD decimal like \"4.50\")",
				v.ID, v.Price)
		}
		retailCents := computeRetailCents(costCents, markup)
		entries = append(entries, variantMetadataEntry{
			CatalogVariantID:  v.ID,
			SKU:               v.SKU,
			Attributes:        v.Attributes,
			SupplierCostCents: costCents,
			RetailCents:       retailCents,
		})
	}
	return entries, nil
}

// computeRetailCents applies a markup factor to a cents amount, rounding UP.
// Markup is provided as float64 (typical values: 1.5, 2.0, 2.5 from seller
// input). math.Ceil guards against tiny float-representation errors causing
// a 1-cent loss on the seller side.
func computeRetailCents(costCents uint64, markup float64) uint64 {
	if costCents == 0 || markup <= 0 {
		return 0
	}
	return uint64(math.Ceil(float64(costCents) * markup))
}

// loadVariantCostMap returns catalogVariantID -> supplierCostCents from the
// private SyncedProductMapping.Metadata blob. Returns nil on missing/invalid
// metadata so callers can fall back to the snapshot for single-SKU listings.
func loadVariantCostMap(spm *models.SyncedProductMapping) map[string]uint64 {
	if spm == nil || len(spm.Metadata) == 0 {
		return nil
	}
	var entries []variantMetadataEntry
	if err := json.Unmarshal(spm.Metadata, &entries); err != nil {
		return nil
	}
	out := make(map[string]uint64, len(entries))
	for _, e := range entries {
		if e.CatalogVariantID != "" && e.SupplierCostCents > 0 {
			out[e.CatalogVariantID] = e.SupplierCostCents
		}
	}
	return out
}

func (s *SupplyChainAppService) SyncProduct(_ context.Context, _ string) (*contracts.SyncStatus, error) {
	return nil, fmt.Errorf("SyncProduct (FF-2.x): %w", contracts.ErrFulfillmentNotImplemented)
}

func (s *SupplyChainAppService) ListSyncedProducts(_ context.Context, providerID string) ([]contracts.SyncedProduct, error) {
	var mappings []models.SyncedProductMapping
	err := s.db.View(func(tx database.Tx) error {
		q := tx.Read()
		if providerID != "" {
			q = q.Where("provider_id = ?", providerID)
		}
		return q.Find(&mappings).Error
	})
	if err != nil {
		return nil, err
	}
	products := make([]contracts.SyncedProduct, len(mappings))
	for i, m := range mappings {
		products[i] = contracts.SyncedProduct{
			ID:            m.ID,
			ProviderID:    m.ProviderID,
			ListingSlug:   m.ListingSlug,
			ExternalID:    m.ExternalID,
			SyncProductID: m.SyncProductID,
			Status:        m.Status,
			LastSyncAt:    m.LastSyncAt,
			SupplierCost:  m.SupplierCost,
			RetailPrice:   m.RetailPrice,
		}
	}
	return products, nil
}

// ---------------------------------------------------------------------------
// Order Fulfillment Bridge
// ---------------------------------------------------------------------------

// TECHDEBT(TD-025): CreateFulfillmentFromOrder 是早期 scaffold，
// handleOrderFunded 已使用 createFulfillmentForItems 替代。
// 此方法保留是因为 contracts.SupplyChainService 接口中定义了签名。
// 清除条件: 评估是否需要保留手动触发路径（如前端"手动重试"按钮），
// 若不需要则从接口和实现中一同删除。
func (s *SupplyChainAppService) CreateFulfillmentFromOrder(ctx context.Context, mobazhaOrderID string) (*contracts.FulfillmentOrder, error) {
	var existing models.FulfillmentOrderMapping
	existsErr := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("mobazha_order_id = ?", mobazhaOrderID).First(&existing).Error
	})
	if existsErr == nil {
		return nil, fmt.Errorf("fulfillment order already exists for order %s (status: %s)", mobazhaOrderID, existing.Status)
	}

	return nil, fmt.Errorf("CreateFulfillmentFromOrder: use handleOrderFunded EventBus path instead")
}

// createFulfillmentForItems is the internal method called by the OrderFunded listener.
// It bridges a Mobazha order to a supplier fulfillment order.
func (s *SupplyChainAppService) createFulfillmentForItems(
	ctx context.Context,
	mobazhaOrderID string,
	providerID string,
	params contracts.CreateFulfillmentParams,
) (*contracts.FulfillmentOrder, error) {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return nil, fmt.Errorf("provider lookup: %w", err)
	}

	// Reserve mapping row BEFORE calling the external provider.
	// This ensures we always have a local record to correlate webhooks/retries,
	// even if the DB write after the provider call were to fail.
	mapping := &models.FulfillmentOrderMapping{
		ID:             uuid.New().String(),
		MobazhaOrderID: mobazhaOrderID,
		ProviderID:     providerID,
		Status:         string(contracts.FulfillmentStatusPending),
	}
	if saveErr := s.db.Update(func(tx database.Tx) error { return tx.Save(mapping) }); saveErr != nil {
		return nil, fmt.Errorf("reserve fulfillment mapping: %w", saveErr)
	}

	start := time.Now()
	fo, err := provider.CreateFulfillmentOrder(ctx, params)
	providerDuration := time.Since(start)
	if err != nil {
		reason := classifyProviderError(providerID, err)
		if persistErr := s.db.Update(func(tx database.Tx) error {
			mapping.Status = string(contracts.FulfillmentStatusFailed)
			mapping.ErrorMessage = err.Error()
			mapping.FailureReason = string(reason)
			if reason.IsRetryable() {
				mapping.RetryCount = 0
				mapping.NextRetryAt = time.Now().Add(retryBackoffBase)
			}
			return tx.Save(mapping)
		}); persistErr != nil {
			logger.LogErrorWithIDf(log, s.nodeID,
				"SupplyChain: failed to persist failure mapping for order %s: %v",
				mobazhaOrderID, persistErr)
		}
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain: orderID=%s providerID=%s failureReason=%s duration=%s err=%v",
			mobazhaOrderID, providerID, reason, providerDuration, err)
		return nil, fmt.Errorf("create fulfillment order: %w", err)
	}

	supplierOrderID := fo.ID
	if supplierOrderID == "" {
		supplierOrderID = fo.ExternalID
	}
	if updateErr := s.db.Update(func(tx database.Tx) error {
		mapping.FulfillmentOrderID = supplierOrderID
		mapping.Status = string(fo.Status)
		mapping.SupplierCost = costTotal(fo.Costs)
		return tx.Save(mapping)
	}); updateErr != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain: orderID=%s fulfillmentOrderID=%s supplierCost=%s duration=%s err=save_mapping_failed: %v",
			mobazhaOrderID, supplierOrderID, costTotal(fo.Costs), providerDuration, updateErr)
		return fo, fmt.Errorf("save fulfillment mapping after provider create: %w", updateErr)
	}

	return fo, nil
}

func (s *SupplyChainAppService) GetFulfillmentStatus(_ context.Context, mobazhaOrderID string) (*contracts.FulfillmentOrder, error) {
	var mapping models.FulfillmentOrderMapping
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("mobazha_order_id = ?", mobazhaOrderID).First(&mapping).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, contracts.ErrFulfillmentOrderNotFound
		}
		return nil, err
	}

	fo := &contracts.FulfillmentOrder{
		ID:         mapping.FulfillmentOrderID,
		ExternalID: mapping.MobazhaOrderID,
		Status:     contracts.FulfillmentStatus(mapping.Status),
		Shipments:  buildShipments(&mapping),
		CreatedAt:  mapping.CreatedAt,
		UpdatedAt:  mapping.UpdatedAt,
	}
	if mapping.ErrorMessage != "" {
		fo.ErrorMessage = mapping.ErrorMessage
	}
	if mapping.FailureReason != "" {
		fo.FailureReason = contracts.FailureReason(mapping.FailureReason)
		// Only surface retry counters when there is an actual failure reason.
		// Bound to uint8 to match DTO; retry_count is stored as int but is
		// always small (<= maxRetryAttempts).
		retryCount := mapping.RetryCount
		if retryCount < 0 {
			retryCount = 0
		} else if retryCount > 255 {
			retryCount = 255
		}
		fo.RetryCount = uint8(retryCount)
		fo.MaxRetries = uint8(maxRetryAttempts)
	}
	if mapping.SupplierCost != "" {
		fo.Costs = &contracts.FulfillmentCosts{Total: mapping.SupplierCost}
	}
	return fo, nil
}

// ---------------------------------------------------------------------------
// Webhook Processing
// ---------------------------------------------------------------------------

func (s *SupplyChainAppService) ValidateWebhookSecret(_ context.Context, providerID string, secret string) bool {
	var cfg models.FulfillmentProviderConfig
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("provider_id = ? AND webhook_secret = ?", providerID, secret).
			Select("id").First(&cfg).Error
	})
	return err == nil
}

func (s *SupplyChainAppService) HandleProviderWebhook(ctx context.Context, providerID string, payload []byte, headers map[string]string) error {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}

	event, err := provider.ParseWebhook(ctx, payload, headers)
	if err != nil {
		return fmt.Errorf("parse webhook: %w", err)
	}

	// Idempotency: atomic reserve → process → mark processed.
	// Step 1: Insert a row with status="processing". The unique index
	//   (tenant_id, provider_id, event_id) blocks concurrent duplicates atomically.
	// Step 2: Process the event.
	// Step 3: On success, update to status="processed".
	//         On failure, delete the reservation so retries can proceed.
	if event.EventID != "" {
		skip, retryable, reserveErr := s.reserveEvent(providerID, event)
		if reserveErr != nil {
			logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: reserve event %s failed: %v", event.EventID, reserveErr)
			return reserveErr
		}
		if skip {
			logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: skipping already-processed event %s", event.EventID)
			return nil
		}
		if retryable {
			logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: event %s is being processed by another handler, please retry", event.EventID)
			return fmt.Errorf("event %s is currently being processed, retry later", event.EventID)
		}
	}

	if err := s.processWebhookEvent(ctx, providerID, event); err != nil {
		// Processing failed — remove the reservation to allow provider retries.
		if event.EventID != "" {
			s.releaseEvent(providerID, event.EventID)
		}
		return err
	}

	// Mark event as successfully processed. On failure return error so the
	// provider retries rather than leaving a stale "processing" row.
	if event.EventID != "" {
		if markErr := s.markEventProcessed(providerID, event.EventID); markErr != nil {
			logger.LogErrorWithIDf(log, s.nodeID,
				"SupplyChain: failed to mark event %s as processed: %v", event.EventID, markErr)
			return fmt.Errorf("mark event processed: %w", markErr)
		}
	}
	return nil
}

func (s *SupplyChainAppService) processWebhookEvent(_ context.Context, providerID string, event *contracts.FulfillmentWebhookEvent) error {
	if event.OrderID == "" && event.ExternalID == "" {
		logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: webhook event %s has no order ID, skipping mapping update", event.Type)
		return nil
	}

	var shipData *contracts.FulfillmentShipment
	var mobazhaOrderID string

	err := s.db.Update(func(tx database.Tx) error {
		var mapping models.FulfillmentOrderMapping
		// Look up by supplier's internal order ID first, then fallback to
		// mobazha_order_id. The fallback covers early-arriving webhooks where
		// the supplier ID hasn't been written to the mapping yet.
		found := false
		if event.ExternalID != "" {
			if err := tx.Read().
				Where("provider_id = ? AND fulfillment_order_id = ?", providerID, event.ExternalID).
				First(&mapping).Error; err == nil {
				found = true
			}
		}
		if !found && event.OrderID != "" {
			if err := tx.Read().
				Where("provider_id = ? AND mobazha_order_id = ?", providerID, event.OrderID).
				First(&mapping).Error; err != nil {
				return err
			}
		} else if !found {
			return gorm.ErrRecordNotFound
		}
		mobazhaOrderID = mapping.MobazhaOrderID
		mapping.LastWebhookEventID = event.EventID

		switch event.Type {
		case contracts.FulfillmentWebhookShipped:
			mapping.Status = string(contracts.FulfillmentStatusShipped)
			shipData = extractShipmentData(event)
			if shipData != nil {
				mapping.TrackingNumber = shipData.TrackingNumber
				mapping.TrackingURL = shipData.TrackingURL
				mapping.Carrier = shipData.Carrier
			}
		case contracts.FulfillmentWebhookOrderUpdated:
			mapping.Status = string(contracts.FulfillmentStatusInProcess)
			// Partial shipment: save tracking info even though we don't trigger auto-confirm yet
			if sd := extractShipmentData(event); sd != nil && sd.TrackingNumber != "" {
				mapping.TrackingNumber = sd.TrackingNumber
				mapping.TrackingURL = sd.TrackingURL
				mapping.Carrier = sd.Carrier
			}
		case contracts.FulfillmentWebhookOrderFailed:
			mapping.Status = string(contracts.FulfillmentStatusFailed)
			if msg := extractErrorMessage(event); msg != "" {
				mapping.ErrorMessage = msg
			}
		case contracts.FulfillmentWebhookOrderCanceled:
			mapping.Status = string(contracts.FulfillmentStatusCanceled)
		default:
			logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: unhandled webhook type %s for order %s", event.Type, mapping.MobazhaOrderID)
			return nil
		}
		return tx.Save(&mapping)
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.LogInfoWithIDf(log, s.nodeID,
				"SupplyChain: webhook for unknown fulfillment order %s (provider %s)", event.OrderID, providerID)
			return nil
		}
		return fmt.Errorf("update mapping: %w", err)
	}

	if event.Type == contracts.FulfillmentWebhookShipped {
		if err := s.autoConfirmAndShip(mobazhaOrderID, shipData); err != nil {
			logger.LogErrorWithIDf(log, s.nodeID,
				"SupplyChain: order-advance failed for %s, returning error for provider retry: %v", mobazhaOrderID, err)
			return fmt.Errorf("order advance: %w", err)
		}
	}
	return nil
}

// reserveEvent atomically inserts a "processing" row.
// Returns: (skip=true) if already processed, (retryable=true) if another
// handler is currently processing, or both false on successful reservation.
// A stale "processing" row (older than staleThreshold) is force-acquired.
func (s *SupplyChainAppService) reserveEvent(providerID string, event *contracts.FulfillmentWebhookEvent) (skip bool, retryable bool, err error) {
	const staleThreshold = 5 * time.Minute

	rec := &models.ProcessedFulfillmentEvent{
		ID:         uuid.New().String(),
		ProviderID: providerID,
		EventID:    event.EventID,
		EventType:  string(event.Type),
		OrderID:    event.OrderID,
		Status:     "processing",
	}
	saveErr := s.db.Update(func(tx database.Tx) error { return tx.Save(rec) })
	if saveErr == nil {
		return false, false, nil
	}
	if !isUniqueConstraintError(saveErr) {
		return false, false, saveErr
	}

	// Unique conflict — check whether the existing row is "processed" or "processing".
	var existing models.ProcessedFulfillmentEvent
	lookupErr := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("provider_id = ? AND event_id = ?", providerID, event.EventID).
			First(&existing).Error
	})
	if lookupErr != nil {
		return false, false, fmt.Errorf("lookup existing event: %w", lookupErr)
	}

	if existing.Status == "processed" {
		return true, false, nil
	}

	// Status is "processing" — another handler owns this event.
	// If the row is older than staleThreshold, force-acquire it (the original
	// handler likely crashed or timed out).
	if time.Since(existing.ProcessedAt) > staleThreshold {
		logger.LogWarningWithIDf(log, s.nodeID,
			"SupplyChain: force-acquiring stale processing reservation for event %s (age: %s)",
			event.EventID, time.Since(existing.ProcessedAt))
		overwriteErr := s.db.Update(func(tx database.Tx) error {
			// Refresh timestamp so subsequent requests see a fresh lock
			if err := tx.Update("processed_at", time.Now(), map[string]interface{}{
				"provider_id = ?": providerID,
				"event_id = ?":    event.EventID,
			}, &models.ProcessedFulfillmentEvent{}); err != nil {
				return err
			}
			return nil
		})
		if overwriteErr != nil {
			return false, false, fmt.Errorf("force-acquire stale event: %w", overwriteErr)
		}
		return false, false, nil
	}

	return false, true, nil
}

// markEventProcessed updates the reservation from "processing" to "processed".
func (s *SupplyChainAppService) markEventProcessed(providerID, eventID string) error {
	return s.db.Update(func(tx database.Tx) error {
		return tx.Update("status", "processed", map[string]interface{}{
			"provider_id = ?": providerID,
			"event_id = ?":    eventID,
		}, &models.ProcessedFulfillmentEvent{})
	})
}

// releaseEvent deletes the reservation row so a retry from the provider can proceed.
func (s *SupplyChainAppService) releaseEvent(providerID, eventID string) {
	err := s.db.Update(func(tx database.Tx) error {
		return tx.Delete("provider_id", providerID, map[string]interface{}{
			"event_id = ?": eventID,
		}, &models.ProcessedFulfillmentEvent{})
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain: failed to release event reservation %s: %v", eventID, err)
	}
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// SQLite: "UNIQUE constraint failed"
	// PostgreSQL: "duplicate key value violates unique constraint"
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key value violates unique constraint")
}

// ---------------------------------------------------------------------------
// Auto ConfirmOrder + ShipOrder on supplier shipment (FF-1.10)
// ---------------------------------------------------------------------------

// autoConfirmAndShip is triggered when the supplier webhook reports "shipped".
// For CANCELABLE orders this releases escrow funds and records the shipment.
// MODERATED orders are skipped (need manual multi-sig).
//
// TECHDEBT(TD-023): This confirms/ships the entire order, which is correct
// for FF-1 (single supplier per order). For FF-3 (multi-supplier split orders),
// this must be changed to confirm only supplier-managed item indices and
// auto-ship/confirm only when ALL fulfillment mappings for the order are shipped.
// Cleanup condition: FF-3 multi-supplier split implementation.
func (s *SupplyChainAppService) autoConfirmAndShip(mobazhaOrderID string, shipData *contracts.FulfillmentShipment) error {
	if s.orderOps == nil {
		return fmt.Errorf("orderOps not wired")
	}

	allShipped, checkErr := s.allFulfillmentsShipped(mobazhaOrderID)
	if checkErr != nil {
		return fmt.Errorf("cannot verify fulfillment status: %w", checkErr)
	}
	if !allShipped {
		logger.LogInfoWithIDf(log, s.nodeID,
			"SupplyChain: not all fulfillments shipped for %s, deferring auto-confirm", mobazhaOrderID)
		return nil
	}

	oid := models.OrderID(mobazhaOrderID)

	// Idempotent: if a previous attempt already confirmed the order (but
	// ShipOrder failed), skip ConfirmOrder on retry to avoid "order is not
	// in a state where it can be confirmed" errors.
	confirmed, err := s.orderOps.IsOrderConfirmed(oid)
	if err != nil {
		return fmt.Errorf("check order confirmed state: %w", err)
	}

	if !confirmed {
		if err := s.orderOps.ConfirmOrder(oid, "", "", nil); err != nil {
			return fmt.Errorf("auto-confirm: %w", err)
		}
		logger.LogInfoWithIDf(log, s.nodeID,
			"SupplyChain: auto-confirmed order %s after supplier shipment", mobazhaOrderID)
	} else {
		logger.LogInfoWithIDf(log, s.nodeID,
			"SupplyChain: order %s already confirmed, skipping (idempotent retry)", mobazhaOrderID)
	}

	shipped, err := s.orderOps.IsOrderShipped(oid)
	if err != nil {
		return fmt.Errorf("check order shipped state: %w", err)
	}
	if shipped {
		logger.LogInfoWithIDf(log, s.nodeID,
			"SupplyChain: order %s already shipped, skipping (idempotent retry)", mobazhaOrderID)
		return nil
	}

	shipments := []models.Shipment{{
		PhysicalDelivery: &models.PhysicalDelivery{},
	}}
	if shipData != nil {
		shipments[0].PhysicalDelivery.TrackingNumber = shipData.TrackingNumber
		shipments[0].PhysicalDelivery.Shipper = shipData.Carrier
	}

	if err := s.orderOps.ShipOrder(oid, shipments, nil); err != nil {
		return fmt.Errorf("auto-ship: %w", err)
	}
	logger.LogInfoWithIDf(log, s.nodeID,
		"SupplyChain: auto-shipped order %s with tracking from supplier", mobazhaOrderID)
	return nil
}

// allFulfillmentsShipped returns true only if every FulfillmentOrderMapping
// for the given Mobazha order has status "shipped".
func (s *SupplyChainAppService) allFulfillmentsShipped(mobazhaOrderID string) (bool, error) {
	var total, shipped int64
	err := s.db.View(func(tx database.Tx) error {
		if err := tx.Read().Model(&models.FulfillmentOrderMapping{}).
			Where("mobazha_order_id = ?", mobazhaOrderID).
			Count(&total).Error; err != nil {
			return err
		}
		return tx.Read().Model(&models.FulfillmentOrderMapping{}).
			Where("mobazha_order_id = ? AND status = ?", mobazhaOrderID, string(contracts.FulfillmentStatusShipped)).
			Count(&shipped).Error
	})
	if err != nil {
		return false, err
	}
	return total > 0 && total == shipped, nil
}

// ---------------------------------------------------------------------------
// contracts.SupplyChainChecker implementation
// ---------------------------------------------------------------------------

// IsOrderAutoFulfillable returns true only when every slug maps to the same
// fulfillment provider. This mirrors the safety checks in handleOrderFunded:
// mixed orders and multi-provider orders are NOT auto-fulfillable.
func (s *SupplyChainAppService) IsOrderAutoFulfillable(slugs []string) bool {
	if len(slugs) == 0 {
		return false
	}
	var providerID string
	for _, slug := range slugs {
		var mapping models.SyncedProductMapping
		err := s.db.View(func(tx database.Tx) error {
			return tx.Read().Where("listing_slug = ?", slug).First(&mapping).Error
		})
		if err != nil {
			return false
		}
		if providerID == "" {
			providerID = mapping.ProviderID
		} else if mapping.ProviderID != providerID {
			return false
		}
	}
	return true
}

// IsListingManagedBySupplier checks if the given listing slug has a SyncedProductMapping,
// indicating it was imported from a fulfillment provider.
func (s *SupplyChainAppService) IsListingManagedBySupplier(listingSlug string) bool {
	var count int64
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Table("synced_product_mappings").
			Where("listing_slug = ?", listingSlug).
			Count(&count).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain: IsListingManagedBySupplier query failed for %q: %v", listingSlug, err)
		return false
	}
	return count > 0
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func configToConnection(cfg *models.FulfillmentProviderConfig) *contracts.ProviderConnection {
	return &contracts.ProviderConnection{
		ProviderID:   cfg.ProviderID,
		ProviderType: cfg.ProviderType,
		ProviderName: cfg.ProviderID,
		Status:       cfg.Status,
		StoreName:    cfg.StoreName,
		ConnectedAt:  cfg.ConnectedAt,
		LastSyncAt:   cfg.LastSyncAt,
	}
}

func generateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func costTotal(c *contracts.FulfillmentCosts) string {
	if c == nil {
		return ""
	}
	return c.Total
}

func buildShipments(m *models.FulfillmentOrderMapping) []contracts.FulfillmentShipment {
	if m.TrackingNumber == "" {
		return nil
	}
	return []contracts.FulfillmentShipment{{
		Carrier:        m.Carrier,
		TrackingNumber: m.TrackingNumber,
		TrackingURL:    m.TrackingURL,
	}}
}

// extractShipmentData extracts tracking info from the webhook event data.
// Printful's ParseWebhook stores a *contracts.FulfillmentOrder in event.Data
// (via convertOrder), where tracking is nested under Shipments[].
func extractShipmentData(event *contracts.FulfillmentWebhookEvent) *contracts.FulfillmentShipment {
	if event.Data == nil {
		return nil
	}
	// Try direct type assertion first (in-process)
	if fo, ok := event.Data.(*contracts.FulfillmentOrder); ok && len(fo.Shipments) > 0 {
		s := fo.Shipments[0]
		return &s
	}
	// Fallback: re-marshal and try FulfillmentOrder shape
	raw, err := json.Marshal(event.Data)
	if err != nil {
		return nil
	}
	var fo contracts.FulfillmentOrder
	if json.Unmarshal(raw, &fo) == nil && len(fo.Shipments) > 0 {
		s := fo.Shipments[0]
		return &s
	}
	// Legacy fallback: top-level FulfillmentShipment
	var ship contracts.FulfillmentShipment
	if json.Unmarshal(raw, &ship) == nil && ship.TrackingNumber != "" {
		return &ship
	}
	return nil
}

func extractErrorMessage(event *contracts.FulfillmentWebhookEvent) string {
	if event.Data == nil {
		return ""
	}
	raw, err := json.Marshal(event.Data)
	if err != nil {
		return ""
	}
	var obj struct {
		Reason  string `json:"reason"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		if obj.Reason != "" {
			return obj.Reason
		}
		return obj.Message
	}
	return ""
}

// ---------------------------------------------------------------------------
// AES-256-GCM credential encryption
// ---------------------------------------------------------------------------

func encryptAESGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptAESGCM(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

// failFulfillmentWithReason creates a failed FulfillmentOrderMapping with the given reason.
// Used by margin protection and validation gates to record why auto-fulfillment was skipped.
func (s *SupplyChainAppService) failFulfillmentWithReason(orderID, providerID string, reason contracts.FailureReason, msg string) {
	logger.LogWarningWithIDf(log, s.nodeID,
		"SupplyChain: order %s: %s — %s", orderID, reason, msg)
	if err := s.db.Update(func(tx database.Tx) error {
		mapping := &models.FulfillmentOrderMapping{
			ID:             uuid.New().String(),
			MobazhaOrderID: orderID,
			ProviderID:     providerID,
			Status:         string(contracts.FulfillmentStatusFailed),
			FailureReason:  string(reason),
			ErrorMessage:   msg,
		}
		return tx.Save(mapping)
	}); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain: failed to record failure mapping for order %s: %v", orderID, err)
	}
}

// classifyProviderError delegates to a provider-specific error classifier.
// For unknown providers, defaults to retryable (transient failures are assumed).
func classifyProviderError(providerID string, err error) contracts.FailureReason {
	switch providerID {
	case "printful":
		re := printful.ClassifyError(err)
		if re != nil {
			return contracts.ClassifyFulfillmentError(re)
		}
		return contracts.FailureReasonRetryableProviderError
	default:
		return contracts.ClassifyFulfillmentError(err)
	}
}

// Compile-time interface checks.
var (
	_ contracts.SupplyChainService = (*SupplyChainAppService)(nil)
	_ contracts.SupplyChainChecker = (*SupplyChainAppService)(nil)
)

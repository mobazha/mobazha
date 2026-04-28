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
}

// SupplyChainAppService orchestrates supply chain operations:
// provider management, catalog browsing, product import, and order bridging.
// It implements contracts.SupplyChainService and contracts.SupplyChainChecker.
type SupplyChainAppService struct {
	registry contracts.FulfillmentProviderRegistry
	db       database.Database
	nodeID   string
	credKey  [32]byte // AES-256-GCM key for encrypting provider credentials at rest

	eventBus events.Bus
	shutdown <-chan struct{}
	orderOps SupplyChainOrderOps
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

// Start restores provider instances from DB into the in-memory registry.
// Called during node initialization / SaaS EnsureNode.
func (s *SupplyChainAppService) Start(ctx context.Context) {
	if err := s.registry.RebuildFromDB(ctx); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: failed to rebuild providers from DB: %v", err)
	}
}

// StartFulfillmentMonitor subscribes to OrderFunded events and automatically
// creates supplier fulfillment orders for supply-chain-managed listings.
// Only starts if eventBus is wired (Feature Flag gating is external).
func (s *SupplyChainAppService) StartFulfillmentMonitor() {
	if s.eventBus == nil {
		return
	}
	go s.subscribeOrderFunded()
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
		slug := li.Listing.GetSlug()
		if slug == "" {
			continue
		}
		totalListings++
		var mapping models.SyncedProductMapping
		findErr := s.db.View(func(tx database.Tx) error {
			return tx.Read().Where("listing_slug = ?", slug).First(&mapping).Error
		})
		if findErr != nil {
			continue
		}
		item := contracts.FulfillmentItem{
			CatalogVariantID: mapping.ExternalID,
			Quantity:         1,
		}
		if i < len(oo.Items) && oo.Items[i] != nil {
			if q, parseErr := strconv.Atoi(oo.Items[i].Quantity); parseErr == nil && q > 0 {
				item.Quantity = q
			}
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

// instantiateProvider creates the concrete FulfillmentProvider from persisted config.
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

	provider, err := s.instantiateProvider(providerID, "pod", nil, "")
	if err != nil && providerID == "printful" {
		provider = printful.NewProvider(params.Credentials.APIKey, "")
	} else if err != nil {
		return nil, fmt.Errorf("unsupported provider: %s", providerID)
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

	realProvider := printful.NewProvider(params.Credentials.APIKey, "")
	if regErr := s.registry.Register(realProvider); regErr != nil {
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
// Product Import & Sync (stub — full implementation in FF-2)
// ---------------------------------------------------------------------------

func (s *SupplyChainAppService) ImportProduct(_ context.Context, _ contracts.ImportProductParams) (*contracts.ImportResult, error) {
	return nil, fmt.Errorf("ImportProduct (FF-2): %w", contracts.ErrFulfillmentNotImplemented)
}

func (s *SupplyChainAppService) SyncProduct(_ context.Context, _ string) (*contracts.SyncStatus, error) {
	return nil, fmt.Errorf("SyncProduct (FF-2): %w", contracts.ErrFulfillmentNotImplemented)
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

func (s *SupplyChainAppService) CreateFulfillmentFromOrder(ctx context.Context, mobazhaOrderID string) (*contracts.FulfillmentOrder, error) {
	var existing models.FulfillmentOrderMapping
	existsErr := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("mobazha_order_id = ?", mobazhaOrderID).First(&existing).Error
	})
	if existsErr == nil {
		return nil, fmt.Errorf("fulfillment order already exists for order %s (status: %s)", mobazhaOrderID, existing.Status)
	}

	// For now, the handler passes a fully-formed mobazhaOrderID.
	// The actual order → supplier item mapping requires reading order data
	// and resolving SyncedProductMappings. This is a scaffold that the handler
	// will feed with a CreateFulfillmentParams-based flow in FF-1.9.
	return nil, fmt.Errorf("CreateFulfillmentFromOrder: full order bridging requires FF-1.9 EventBus integration")
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

	fo, err := provider.CreateFulfillmentOrder(ctx, params)
	if err != nil {
		// Update the reserved mapping to failed state
		_ = s.db.Update(func(tx database.Tx) error {
			mapping.Status = string(contracts.FulfillmentStatusFailed)
			mapping.ErrorMessage = err.Error()
			mapping.RetryCount = 0
			mapping.NextRetryAt = time.Now().Add(5 * time.Minute)
			return tx.Save(mapping)
		})
		return nil, fmt.Errorf("create fulfillment order: %w", err)
	}

	// Update mapping with the supplier's internal order ID and costs.
	// fo.ID is the supplier's own order identifier (e.g. Printful's integer ID),
	// which webhooks reference as event.ExternalID. fo.ExternalID is the Mobazha
	// order ID we originally sent, NOT the supplier's ID.
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
			"SupplyChain: created supplier order %s but failed to save mapping: %v", supplierOrderID, updateErr)
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
		go s.autoConfirmAndShip(mobazhaOrderID, shipData)
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
func (s *SupplyChainAppService) autoConfirmAndShip(mobazhaOrderID string, shipData *contracts.FulfillmentShipment) {
	if s.orderOps == nil {
		logger.LogWarningWithIDf(log, s.nodeID,
			"SupplyChain: orderOps not wired, skipping auto-confirm/ship for %s", mobazhaOrderID)
		return
	}

	// Safety check: only auto-confirm if ALL fulfillment mappings for this order are shipped.
	// This prevents partial shipment from releasing escrow prematurely.
	allShipped, checkErr := s.allFulfillmentsShipped(mobazhaOrderID)
	if checkErr != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain: cannot verify fulfillment status for %s: %v", mobazhaOrderID, checkErr)
		return
	}
	if !allShipped {
		logger.LogInfoWithIDf(log, s.nodeID,
			"SupplyChain: not all fulfillments shipped for %s, deferring auto-confirm", mobazhaOrderID)
		return
	}

	oid := models.OrderID(mobazhaOrderID)

	if err := s.orderOps.ConfirmOrder(oid, "", "", nil); err != nil {
		logger.LogWarningWithIDf(log, s.nodeID,
			"SupplyChain: auto-confirm failed for %s (may be MODERATED or already confirmed): %v", mobazhaOrderID, err)
	} else {
		logger.LogInfoWithIDf(log, s.nodeID,
			"SupplyChain: auto-confirmed order %s after supplier shipment", mobazhaOrderID)
	}

	shipments := []models.Shipment{{
		PhysicalDelivery: &models.PhysicalDelivery{},
	}}
	if shipData != nil {
		shipments[0].PhysicalDelivery.TrackingNumber = shipData.TrackingNumber
		shipments[0].PhysicalDelivery.Shipper = shipData.Carrier
	}

	if err := s.orderOps.ShipOrder(oid, shipments, nil); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"SupplyChain: auto-ship failed for %s: %v", mobazhaOrderID, err)
		return
	}
	logger.LogInfoWithIDf(log, s.nodeID,
		"SupplyChain: auto-shipped order %s with tracking from supplier", mobazhaOrderID)
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

// Compile-time interface checks.
var (
	_ contracts.SupplyChainService = (*SupplyChainAppService)(nil)
	_ contracts.SupplyChainChecker = (*SupplyChainAppService)(nil)
)

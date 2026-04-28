package core

import (
	"context"
	"fmt"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/fulfillment"
)

// SupplyChainAppService orchestrates supply chain operations:
// provider management, catalog browsing, product import, and order bridging.
// It implements contracts.SupplyChainService and contracts.SupplyChainChecker.
type SupplyChainAppService struct {
	registry contracts.FulfillmentProviderRegistry
	db       database.Database
	nodeID   string
}

// NewSupplyChainAppService creates the supply chain service skeleton.
// Providers are registered via ConnectProvider or restored via Start().
func NewSupplyChainAppService(
	registry contracts.FulfillmentProviderRegistry,
	db database.Database,
	nodeID string,
) *SupplyChainAppService {
	svc := &SupplyChainAppService{
		registry: registry,
		db:       db,
		nodeID:   nodeID,
	}

	fulfillment.SetRebuildFunc(registry, svc.rebuildProviders)

	return svc
}

// Start restores provider instances from DB into the in-memory registry.
// Called during node initialization / SaaS EnsureNode.
func (s *SupplyChainAppService) Start(ctx context.Context) {
	if err := s.registry.RebuildFromDB(ctx); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: failed to rebuild providers from DB: %v", err)
	}
}

// rebuildProviders scans FulfillmentProviderConfig WHERE status='connected',
// decrypts credentials, instantiates the corresponding provider, and registers it.
// This is the ProviderFactory injected into the registry.
func (s *SupplyChainAppService) rebuildProviders(_ context.Context) error {
	// FF-1 will implement: scan DB → decrypt credentials → instantiate provider → register.
	// For FF-0 skeleton, this is a no-op since no concrete providers exist yet.
	logger.LogInfoWithID(log, s.nodeID, "SupplyChain: rebuildProviders (no concrete providers in FF-0)")
	return nil
}

// ---------------------------------------------------------------------------
// contracts.SupplyChainService implementation (stubs for FF-0)
// ---------------------------------------------------------------------------

func (s *SupplyChainAppService) ConnectProvider(_ context.Context, _ contracts.ConnectProviderParams) (*contracts.ProviderConnection, error) {
	return nil, fmt.Errorf("supply chain: ConnectProvider not implemented (FF-1)")
}

func (s *SupplyChainAppService) DisconnectProvider(_ context.Context, _ string) error {
	return fmt.Errorf("supply chain: DisconnectProvider not implemented (FF-1)")
}

func (s *SupplyChainAppService) GetProviderStatus(_ context.Context, providerID string) (*contracts.ProviderConnection, error) {
	return nil, fmt.Errorf("supply chain: GetProviderStatus not implemented (FF-1)")
}

func (s *SupplyChainAppService) ListConnections(_ context.Context) ([]contracts.ProviderConnection, error) {
	return nil, nil
}

func (s *SupplyChainAppService) BrowseCatalog(_ context.Context, _ string, _ contracts.CatalogQuery) (*contracts.CatalogPage, error) {
	return nil, fmt.Errorf("supply chain: BrowseCatalog not implemented (FF-1)")
}

func (s *SupplyChainAppService) GetCatalogProduct(_ context.Context, _ string, _ string) (*contracts.CatalogProduct, error) {
	return nil, fmt.Errorf("supply chain: GetCatalogProduct not implemented (FF-1)")
}

func (s *SupplyChainAppService) ImportProduct(_ context.Context, _ contracts.ImportProductParams) (*contracts.ImportResult, error) {
	return nil, fmt.Errorf("supply chain: ImportProduct not implemented (FF-1)")
}

func (s *SupplyChainAppService) SyncProduct(_ context.Context, _ string) (*contracts.SyncStatus, error) {
	return nil, fmt.Errorf("supply chain: SyncProduct not implemented (FF-1)")
}

func (s *SupplyChainAppService) ListSyncedProducts(_ context.Context, _ string) ([]contracts.SyncedProduct, error) {
	return nil, nil
}

func (s *SupplyChainAppService) CreateFulfillmentFromOrder(_ context.Context, _ string) (*contracts.FulfillmentOrder, error) {
	return nil, fmt.Errorf("supply chain: CreateFulfillmentFromOrder not implemented (FF-1)")
}

func (s *SupplyChainAppService) GetFulfillmentStatus(_ context.Context, _ string) (*contracts.FulfillmentOrder, error) {
	return nil, fmt.Errorf("supply chain: GetFulfillmentStatus not implemented (FF-1)")
}

func (s *SupplyChainAppService) HandleProviderWebhook(_ context.Context, _ string, _ []byte, _ map[string]string) error {
	return fmt.Errorf("supply chain: HandleProviderWebhook not implemented (FF-1)")
}

func (s *SupplyChainAppService) EstimateShipping(_ context.Context, _ string, _ contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
	return nil, fmt.Errorf("supply chain: EstimateShipping not implemented (FF-1)")
}

// ---------------------------------------------------------------------------
// contracts.SupplyChainChecker implementation
// ---------------------------------------------------------------------------

// IsListingManagedBySupplier checks if the given listing slug has a SyncedProductMapping,
// indicating it was imported from a fulfillment provider.
// Used by PaymentAppService to suppress auto-confirm for supply-chain-managed orders.
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

// Compile-time interface checks.
var (
	_ contracts.SupplyChainService = (*SupplyChainAppService)(nil)
	_ contracts.SupplyChainChecker = (*SupplyChainAppService)(nil)
)

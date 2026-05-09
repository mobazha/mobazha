package contracts

import (
	"context"
	"errors"
)

// Sentinel errors for typed error handling in fulfillment handlers.
var (
	ErrFulfillmentProviderNotFound    = errors.New("fulfillment: provider not found")
	ErrFulfillmentNotConnected        = errors.New("fulfillment: provider not connected")
	ErrFulfillmentOrderNotFound       = errors.New("fulfillment: order mapping not found")
	ErrFulfillmentCatalogNotSupported = errors.New("fulfillment: provider does not support catalog browsing")
	ErrFulfillmentNotImplemented      = errors.New("fulfillment: not implemented")
)

// FulfillmentProvider is the core interface for external fulfillment services.
// All POD and dropshipping providers implement this interface.
// Design follows the FiatPaymentProvider pattern.
type FulfillmentProvider interface {
	// ProviderID returns the unique identifier (e.g. "printful", "printify").
	ProviderID() string

	// ProviderType returns "pod" or "dropshipping".
	ProviderType() string

	// ValidateCredentials tests the given credentials against the provider API.
	ValidateCredentials(ctx context.Context, creds ProviderCredentials) error

	// CreateFulfillmentOrder submits an order to the external supplier.
	CreateFulfillmentOrder(ctx context.Context, params CreateFulfillmentParams) (*FulfillmentOrder, error)

	// GetFulfillmentOrder retrieves the current status of a supplier order.
	GetFulfillmentOrder(ctx context.Context, orderID string) (*FulfillmentOrder, error)

	// CancelFulfillmentOrder cancels a supplier order (best-effort).
	CancelFulfillmentOrder(ctx context.Context, orderID string) error

	// ParseWebhook validates the webhook signature and parses the payload.
	ParseWebhook(ctx context.Context, payload []byte, headers map[string]string) (*FulfillmentWebhookEvent, error)

	// EstimateShipping returns shipping cost estimates for the given items.
	EstimateShipping(ctx context.Context, params ShippingEstimateParams) ([]ShippingEstimate, error)
}

// FulfillmentCatalogProvider is an optional extension for browsing supplier catalogs.
// Use type assertion: if cat, ok := provider.(FulfillmentCatalogProvider); ok { ... }
type FulfillmentCatalogProvider interface {
	ListCategories(ctx context.Context) ([]CatalogCategory, error)
	ListProducts(ctx context.Context, params CatalogQuery) (*CatalogPage, error)
	GetProduct(ctx context.Context, productID string) (*CatalogProduct, error)
	GetVariant(ctx context.Context, variantID string) (*CatalogVariant, error)
}

// FulfillmentMockupProvider is an optional extension for POD mockup generation.
// Use type assertion: if mp, ok := provider.(FulfillmentMockupProvider); ok { ... }
type FulfillmentMockupProvider interface {
	GenerateMockup(ctx context.Context, params MockupParams) (*MockupResult, error)
	GetMockupResult(ctx context.Context, taskID string) (*MockupResult, error)
}

// FulfillmentSyncProvider is an optional extension for bidirectional product sync.
// Use type assertion: if sp, ok := provider.(FulfillmentSyncProvider); ok { ... }
type FulfillmentSyncProvider interface {
	SyncProduct(ctx context.Context, params SyncProductParams) (*SyncedProduct, error)
	GetSyncStatus(ctx context.Context, syncProductID string) (*SyncStatus, error)
	DeleteSyncProduct(ctx context.Context, syncProductID string) error
}

// FulfillmentStoreSyncProvider is an optional extension for browsing products
// the seller has already designed in the supplier's dashboard (Sync Products).
// Unlike FulfillmentCatalogProvider (generic templates), these products have
// custom designs/mockups applied.
// Use type assertion: if ssp, ok := provider.(FulfillmentStoreSyncProvider); ok { ... }
type FulfillmentStoreSyncProvider interface {
	ListStoreSyncProducts(ctx context.Context, offset, limit int) (*StoreSyncPage, error)
	GetStoreSyncProduct(ctx context.Context, syncProductID string) (*StoreSyncProduct, error)
}

// FulfillmentProviderRegistry manages registered FulfillmentProvider instances.
type FulfillmentProviderRegistry interface {
	Register(provider FulfillmentProvider) error
	ForProvider(providerID string) (FulfillmentProvider, error)
	ListProviders() []FulfillmentProvider
	Unregister(providerID string)

	// RebuildFromDB restores provider instances from DB after node restart
	// or SaaS LRU eviction. Scans FulfillmentProviderConfig where status='connected',
	// decrypts credentials, instantiates providers and registers them.
	RebuildFromDB(ctx context.Context) error
}

// SupplyChainService is the business-level service exposed to handlers.
type SupplyChainService interface {
	// Provider management
	ConnectProvider(ctx context.Context, params ConnectProviderParams) (*ProviderConnection, error)
	DisconnectProvider(ctx context.Context, providerID string) error
	GetProviderStatus(ctx context.Context, providerID string) (*ProviderConnection, error)
	ListConnections(ctx context.Context) ([]ProviderConnection, error)

	// Locations
	ListLocations(ctx context.Context) ([]FulfillmentLocation, error)
	GetLocation(ctx context.Context, locationID string) (*FulfillmentLocation, error)

	// Catalog
	BrowseCatalog(ctx context.Context, providerID string, query CatalogQuery) (*CatalogPage, error)
	GetCatalogProduct(ctx context.Context, providerID string, productID string) (*CatalogProduct, error)

	// Product sync
	ImportProduct(ctx context.Context, params ImportProductParams) (*ImportResult, error)
	SyncProduct(ctx context.Context, listingSlug string) (*SyncStatus, error)
	ListSyncedProducts(ctx context.Context, providerID string) ([]SyncedProduct, error)
	UnlinkSyncedProduct(ctx context.Context, providerID, mappingID string) error

	// Store sync products (designed in supplier dashboard)
	BrowseStoreSyncProducts(ctx context.Context, providerID string, offset, limit int) (*StoreSyncPage, error)
	GetStoreSyncProduct(ctx context.Context, providerID string, syncProductID string) (*StoreSyncProduct, error)

	// Order fulfillment bridge
	CreateFulfillmentFromOrder(ctx context.Context, mobazhaOrderID string) (*FulfillmentOrder, error)
	GetFulfillmentStatus(ctx context.Context, mobazhaOrderID string) (*FulfillmentOrder, error)

	// Inbound webhook
	ValidateWebhookSecret(ctx context.Context, providerID string, secret string) bool
	HandleProviderWebhook(ctx context.Context, providerID string, payload []byte, headers map[string]string) error

	// Shipping
	EstimateShipping(ctx context.Context, providerID string, params ShippingEstimateParams) ([]ShippingEstimate, error)

	// Alerts & Rules (M6 monitoring)
	ListAlerts(ctx context.Context, dismissed bool, limit int) ([]SupplyChainAlert, error)
	DismissAlert(ctx context.Context, alertID string) error
	ListRules(ctx context.Context) ([]AutoActionRule, error)
	CreateRule(ctx context.Context, rule *AutoActionRule) error
	DeleteRule(ctx context.Context, ruleID string) error
}

// SupplyChainChecker is the narrow port that PaymentAppService depends on
// to determine whether an order's listing is managed by a supply chain provider.
// This decouples payment logic from the supply chain module.
type SupplyChainChecker interface {
	IsListingManagedBySupplier(listingSlug string) bool
	// IsOrderAutoFulfillable returns true when ALL slugs are supply-chain-managed
	// AND they all belong to the same provider. Only then will handleOrderFunded
	// actually create a supplier order, so only then should auto-confirm be suppressed.
	IsOrderAutoFulfillable(slugs []string) bool
}

// SupplyChainProvider exposes the supply chain subsystem.
// Handlers obtain this via type assertion on NodeService:
//
//	if sc, ok := nodeService.(SupplyChainProvider); ok { ... }
type SupplyChainProvider interface {
	SupplyChain() SupplyChainService
}

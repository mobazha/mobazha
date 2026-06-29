package core

import (
	"context"
	"time"

	internalapi "github.com/mobazha/mobazha3.0/internal/api"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/internal/core"
	"github.com/mobazha/mobazha3.0/internal/database/dbstore"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/repo"
	"gorm.io/gorm"
)

type MobazhaNode = core.MobazhaNode

type CollectiblePrimarySalePaidSignal = core.CollectiblePrimarySalePaidSignal
type CollectiblePrimarySalePaidHook = core.CollectiblePrimarySalePaidHook
type CollectibleFirstSaleAuthorizationSignal = core.CollectibleFirstSaleAuthorizationSignal
type CollectibleFirstSaleAuthorizationHook = core.CollectibleFirstSaleAuthorizationHook
type CollectibleFirstSaleReservationReleaseSignal = core.CollectibleFirstSaleReservationReleaseSignal
type CollectibleFirstSaleReservationReleaseHook = core.CollectibleFirstSaleReservationReleaseHook
type CollectibleFirstSalePreflightSignal = core.CollectibleFirstSalePreflightSignal
type CollectibleFirstSalePreflightHook = core.CollectibleFirstSalePreflightHook

// APIGateway is a type alias for internal/api.Gateway, enabling hosting
// to reference the concrete Gateway type without importing internal packages.
type APIGateway = internalapi.Gateway

// NodeOption is a functional option for MobazhaNode construction.
// Re-exported from internal/core so that hosting packages can use it
// without importing internal packages.
type NodeOption = core.NodeOption

func NewNode(ctx context.Context, cfg *repo.Config, nodeID string, hostService coreiface.HostService) (*MobazhaNode, error) {
	return core.NewNode(ctx, cfg, nodeID, hostService)
}

// NewNodeWithOptions constructs a MobazhaNode with the given HostService
// and applies functional options before Start().
// Use this instead of NewNode when you need to inject optional dependencies.
func NewNodeWithOptions(ctx context.Context, cfg *repo.Config, nodeID string,
	hs coreiface.HostService, opts ...NodeOption) (*MobazhaNode, error) {
	return core.NewNodeWithOptions(ctx, cfg, nodeID, hs, opts...)
}

// WithPaymentCapabilityProvider injects the distribution's immutable payment
// capability allowlist without exposing a concrete commercial adapter config.
func WithPaymentCapabilityProvider(provider payment.ChainCapabilityProvider) NodeOption {
	return core.WithPaymentCapabilityProvider(provider)
}

// WithPaymentModules installs trusted, statically linked distribution payment
// modules without exposing Open Core internal packages or MobazhaNode.
func WithPaymentModules(modules ...distribution.PaymentModule) NodeOption {
	return core.WithPaymentModules(modules...)
}

// WithAIProfile injects distribution-provided AI routes through the public
// provider-neutral contract.
func WithAIProfile(profile contracts.AIProfile) NodeOption {
	return core.WithAIProfile(profile)
}

// WithSovereignNode selects the local-first single-node composition before
// any runtime resources are created.
func WithSovereignNode(config distribution.SovereignNodeConfig) NodeOption {
	return core.WithSovereignNode(config)
}

// SetAIProfile updates distribution-provided AI routes on a running node.
func SetAIProfile(node *MobazhaNode, profile contracts.AIProfile) {
	if node == nil {
		return
	}
	node.SetAIProfile(profile)
}

// WithCollectiblePrimarySalePaidHook wires an optional first-sale lifecycle
// callback into verified payment handling.
func WithCollectiblePrimarySalePaidHook(hook CollectiblePrimarySalePaidHook) NodeOption {
	return core.WithCollectiblePrimarySalePaidHook(hook)
}

// WithCollectibleFirstSalePreflightHook requires a composed adapter to validate
// source custody before Node provisions payment for a collectible first sale.
func WithCollectibleFirstSalePreflightHook(hook CollectibleFirstSalePreflightHook) NodeOption {
	return core.WithCollectibleFirstSalePreflightHook(hook)
}

func WithCollectibleFirstSaleAuthorizationHook(hook CollectibleFirstSaleAuthorizationHook) NodeOption {
	return core.WithCollectibleFirstSaleAuthorizationHook(hook)
}

func WithCollectibleFirstSaleReservationReleaseHook(hook CollectibleFirstSaleReservationReleaseHook) NodeOption {
	return core.WithCollectibleFirstSaleReservationReleaseHook(hook)
}

// RuntimeAccess exposes the narrow shared-runtime ports needed by a
// distribution composition root.
type RuntimeAccess struct{}

// Runtime returns access to shared runtime ports without exporting internal
// manager implementations.
func Runtime() RuntimeAccess { return RuntimeAccess{} }

// NodeManager returns the runtime node-manager port. It returns a proper nil
// interface before the shared manager is initialized.
func (RuntimeAccess) NodeManager() coreiface.NodeManagerIface {
	if core.SharedManagerInstance == nil {
		return nil
	}
	return core.SharedManagerInstance
}

// NodeRegistry returns the global contracts.NodeRegistry adapter.
// It exposes a race-free snapshot of all active NodeService instances for
// the shared scheduler (Phase AH-3). Returns nil if SharedManagerInstance
// is not yet initialized — callers should treat nil as "no registry available"
// and skip Job registration that depends on NodeFn.
//
// This is intentionally a separate accessor from NodeManager: the
// scheduler only needs read-only iteration, and exposing it via a narrower
// interface (NodeRegistry) keeps the dependency surface minimal.
func (RuntimeAccess) NodeRegistry() contracts.NodeRegistry {
	if core.SharedManagerInstance == nil {
		return nil
	}
	return core.SharedManagerInstance
}

// NewDBPublicData creates a PublicData backed by the shared GORM database,
// scoped to the given tenantID. Used by SaaS hosting to resolve co-tenant
// public data directly from the shared DB.
func NewDBPublicData(db *gorm.DB, tenantID string) pkgdb.PublicData {
	return dbstore.NewDBPublicData(db, tenantID)
}

// AttachHTTPGateway registers a Gateway with the SharedManager so that
// builder.go's NotifyWebsockets integration reaches the correct WS hubs.
// Called from hosting after creating a pkgapi.Router via NewRouter().
func (RuntimeAccess) AttachHTTPGateway(gw *APIGateway) {
	if core.SharedManagerInstance != nil {
		core.SharedManagerInstance.SetHTTPGateway(gw)
	}
}

// GetExchangeRateProviderHealth returns health metrics for all exchange rate
// providers. Returns nil if SharedManager or the provider is not initialized.
func GetExchangeRateProviderHealth() []contracts.ProviderHealthInfo {
	if core.SharedManagerInstance == nil {
		return nil
	}
	if core.SharedManagerInstance.ExchangeRateProvider == nil {
		return nil
	}
	return core.SharedManagerInstance.ExchangeRateProvider.GetProviderHealth()
}

// SetExchangeRateConfig allows hosting to override exchange rate configuration
// from DB-backed runtime config before SharedManager creates the provider.
func SetExchangeRateConfig(apiKey string, enabled *bool, cacheTTLSeconds int) {
	cfg := config.GetGlobalExchangeRateConfig()
	if apiKey != "" {
		cfg.SetCoinGeckoAPIKey(apiKey)
	}
	if enabled != nil {
		cfg.SetCoinGeckoEnabled(*enabled)
	}
	if cacheTTLSeconds > 0 {
		cfg.SetCacheTTL(time.Duration(cacheTTLSeconds) * time.Second)
	}
}

// SetBinanceConfig allows hosting to enable/disable the Binance secondary provider.
func SetBinanceConfig(enabled *bool) {
	if enabled == nil {
		return
	}
	cfg := config.GetGlobalExchangeRateConfig()
	cfg.SetBinanceEnabled(*enabled)
}

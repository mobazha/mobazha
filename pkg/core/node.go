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
	"github.com/mobazha/mobazha3.0/pkg/repo"
	"gorm.io/gorm"
)

type MobazhaNode = core.MobazhaNode

// APIGateway is a type alias for internal/api.Gateway, enabling hosting
// to reference the concrete Gateway type without importing internal packages.
type APIGateway = internalapi.Gateway

func NewNode(ctx context.Context, cfg *repo.Config, nodeID string, hostService coreiface.HostService) (*MobazhaNode, error) {
	return core.NewNode(ctx, cfg, nodeID, hostService)
}

// GetNodeManager returns the global NodeManagerIface instance.
// This allows external packages (e.g., mobazha_hosting) to register
// TenantService or other NodeService implementations with the shared manager.
// Returns a proper nil interface when SharedManagerInstance is not yet initialized,
// avoiding the Go typed-nil-pointer-in-interface pitfall.
func GetNodeManager() coreiface.NodeManagerIface {
	if core.SharedManagerInstance == nil {
		return nil
	}
	return core.SharedManagerInstance
}

// GetNodeRegistry returns the global contracts.NodeRegistry adapter.
// It exposes a race-free snapshot of all active NodeService instances for
// the shared scheduler (Phase AH-3). Returns nil if SharedManagerInstance
// is not yet initialized — callers should treat nil as "no registry available"
// and skip Job registration that depends on NodeFn.
//
// This is intentionally a separate accessor from GetNodeManager(): the
// scheduler only needs read-only iteration, and exposing it via a narrower
// interface (NodeRegistry) keeps the dependency surface minimal.
func GetNodeRegistry() contracts.NodeRegistry {
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

// SetSharedHTTPGateway registers a Gateway with the SharedManager so that
// builder.go's NotifyWebsockets integration reaches the correct WS hubs.
// Called from hosting after creating a pkgapi.Router via NewRouter().
func SetSharedHTTPGateway(gw *APIGateway) {
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

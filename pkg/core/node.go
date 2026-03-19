package core

import (
	"context"
	"time"

	internalapi "github.com/mobazha/mobazha3.0/internal/api"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/internal/core"
	"github.com/mobazha/mobazha3.0/internal/database/dbstore"
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
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

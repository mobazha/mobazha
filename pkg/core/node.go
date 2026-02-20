package core

import (
	"context"

	internalapi "github.com/mobazha/mobazha3.0/internal/api"
	"github.com/mobazha/mobazha3.0/internal/core"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/repo"
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
func GetNodeManager() coreiface.NodeManagerIface {
	return core.SharedManagerInstance
}

// SetSharedHTTPGateway registers a Gateway with the SharedManager so that
// builder.go's NotifyWebsockets integration reaches the correct WS hubs.
// Called from hosting after creating a pkgapi.Router via NewRouter().
func SetSharedHTTPGateway(gw *APIGateway) {
	if core.SharedManagerInstance != nil {
		core.SharedManagerInstance.SetHTTPGateway(gw)
	}
}

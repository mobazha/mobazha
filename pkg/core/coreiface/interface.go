package coreiface

import (
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/mobazha/mobazha/internal/wallet"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
)

// CoreIface enumerates the interface of the MobazhaNode object in the Core package.
// It embeds contracts.NodeService (the aggregate interface suitable for both MobazhaNode
// and TenantService) and adds internal-only methods that depend on internal/ types.
//
// API handlers should prefer depending on contracts.NodeService where possible.
// CoreIface is kept for backward compatibility and for methods that require internal types.
type CoreIface interface {
	// Embed all public aggregate interfaces from pkg/contracts.
	// This ensures MobazhaNode implements contracts.NodeService automatically.
	contracts.NodeService

	// --- Internal-only methods (depend on internal/ types) ---

	Start()
	Stop(force bool) error
	PeerHost() host.Host
	DestroyNode()

	// DB returns the internal database handle.
	// TenantService would use shared DB with tenant scoping instead.
	DB() database.Database

	// Multiwallet returns the WalletOperator interface for wallet operations.
	// Internal callers that need the concrete map type can type-assert.
	Multiwallet() contracts.WalletOperator

	// ExchangeRates returns the internal exchange rate provider.
	ExchangeRates() *wallet.ExchangeRateProvider

	// UsingTorMode returns whether the node is using Tor.
	UsingTorMode() bool
}

// NodeManagerIface manages node instances in the shared manager.
//
// AddNode/GetNode/GetNodes use contracts.NodeService to support both
// MobazhaNode (CoreIface) and TenantService (NodeService only).
// GetDefaultNode returns CoreIface because the default node is always
// a full MobazhaNode — callers needing DB/Multiwallet/PeerHost use this.
type NodeManagerIface interface {
	// GetDefaultNode returns the default node as CoreIface.
	// The default node is always a MobazhaNode (full node).
	GetDefaultNode() CoreIface

	// AddNode registers a node (MobazhaNode or TenantService) by ID.
	AddNode(nodeID string, node contracts.NodeService)
	RemoveNode(nodeID string)

	// GetNodes returns all registered nodes as NodeService.
	GetNodes() map[string]contracts.NodeService

	// GetNode returns a node by ID as NodeService.
	GetNode(nodeID string) (contracts.NodeService, bool)

	// GetExchangeRateService returns the shared exchange rate service.
	// Used by external packages (e.g., hosting) to provide exchange rates
	// to TenantService via SharedInfra.
	GetExchangeRateService() contracts.ExchangeRateService

	// Config methods
	GetMaxImportZipSize() int64
	GetMaxImportVideoSize() int64
}

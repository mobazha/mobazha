package coreiface

import (
	"github.com/libp2p/go-libp2p/core/peer"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/relay"
	"github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// HostService 定义托管服务的接口
type HostService interface {
	// GetUTXOMonitor 获取共享的 UTXO Monitor 服务
	// 如果 HostService 不支持共享 Monitor，返回 nil
	GetUTXOMonitor() utxo.UTXOMonitorService

	// GetEVMRelayService 获取共享的 EVM Relay 服务
	// 如果 HostService 不支持 Relay，返回 nil
	// Hosting 模式下直接调用，省去 HTTP 中转
	GetEVMRelayService() relay.EVMRelayService

	// GetGlobalBlockedIDs 获取全局封禁节点列表（从智能合约查询，启动时缓存）
	// SaaS 节点通过此方法共享默认节点获取的封禁列表，避免 per-tenant 合约查询
	// 返回 nil 表示不支持或尚未初始化
	GetGlobalBlockedIDs() []peer.ID

	// SetGlobalBlockedIDs 缓存全局封禁节点列表
	// 由默认节点的 builder 在查询智能合约后调用，供 SaaS 节点共享
	SetGlobalBlockedIDs(ids []peer.ID)

	// GetEVMChainClient 获取共享的 EVM 链客户端（per-chain，带 RPC 连接）
	// SaaS 节点通过此方法共享 EVM 客户端，避免 per-tenant RPC 连接
	// 返回 nil 表示该链不支持或尚未初始化
	GetEVMChainClient(chain iwallet.ChainType) iwallet.ChainClient

	// GetDiscountAccessForPeer returns the DiscountService and DiscountStore
	// scoped to the vendor identified by peerID. In SaaS mode this crosses
	// tenant boundaries via the NodeManager, enabling buyer-side discount
	// resolution (engine needs both svc + store) and redemption recording.
	// Returns (nil, nil, error) if the vendor node is not found or not running.
	GetDiscountAccessForPeer(peerID peer.ID) (contracts.DiscountService, contracts.DiscountStore, error)

	// GetBlobStore returns the shared BlobStore for media storage (R2 in SaaS,
	// LocalFS in standalone). Returns nil when no BlobStore is configured
	// (legacy mode — media bytes stored in DB).
	GetBlobStore() contracts.BlobStore

	// GetPlatformFeatureProvider returns the platform-global feature flag
	// provider (Scope=platform_global layer of the three-layer resolver).
	//
	// SaaS hosting returns an implementation backed by app.yaml + runtime
	// admin API so platform-wide kill switches propagate to every tenant
	// node. Standalone deployments (no platform control plane) return nil
	// and the node falls back to pkgconfig.AllowAllPlatformProvider.
	//
	// Returning nil is explicitly allowed — the node core treats it as
	// "no platform override available" and does NOT fail initialization.
	GetPlatformFeatureProvider() pkgconfig.PlatformGlobalProvider
}

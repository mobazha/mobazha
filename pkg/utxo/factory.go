package utxo

import (
	"context"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// MonitorFactory creates and configures UTXO monitors
// This interface allows external packages (like hosting) to create monitors
// without directly importing internal packages
type MonitorFactory interface {
	// CreateMonitor creates a new monitor with default sources for supported chains
	// testnet: whether to use testnet servers
	// Returns a fully configured and started monitor
	CreateMonitor(ctx context.Context, testnet bool) (*Monitor, error)
}

// DefaultFactory is the default monitor factory implementation
// It should be set by the internal package during initialization
var DefaultFactory MonitorFactory

// NewMonitorWithDefaultSources creates a monitor with default Electrum/Mempool sources
// This is a convenience function that uses the DefaultFactory
// If DefaultFactory is not set, it returns an error
func NewMonitorWithDefaultSources(ctx context.Context, testnet bool) (*Monitor, error) {
	if DefaultFactory == nil {
		// Return a basic monitor without sources if factory is not set
		return NewMonitor(DefaultMonitorConfig()), nil
	}
	return DefaultFactory.CreateMonitor(ctx, testnet)
}

// SourceConfig holds configuration for creating payment sources
type SourceConfig struct {
	Chain   iwallet.ChainType
	Testnet bool
}

// ChainOperations defines the interface for chain operations used by ChainClient
// Both *Monitor and UTXOMonitorService implement this interface
type ChainOperations interface {
	GetTransaction(chainType iwallet.ChainType, txid string) (*iwallet.Transaction, error)
	GetFeeEstimate(chainType iwallet.ChainType, targetBlocks int) uint64
	BroadcastTransaction(chainType iwallet.ChainType, txHex string) (string, error)
	GetAddressTransactions(chainType iwallet.ChainType, address string, scriptPubKey []byte) ([]iwallet.Transaction, error)
	IsHealthy(chainType iwallet.ChainType) bool
}

// UTXOMonitorService 定义 UTXO 监控服务的统一接口
// 支持 standalone 模式（节点自己管理）和 shared 模式（HostService 管理）
type UTXOMonitorService interface {
	// Start 启动监控服务
	Start()

	// Stop 停止监控服务
	// 如果 Monitor.isShared=true，则为空操作（由 HostService 通过 ForceStop 管理）
	Stop()

	// WatchAddress 开始监控一个支付地址
	WatchAddress(wa *WatchedAddress) error

	// UnwatchAddress 停止监控一个地址
	UnwatchAddress(address string) error

	// GetAddressTransactions 获取地址的所有交易（一次性查询）
	GetAddressTransactions(chainType iwallet.ChainType, address string, scriptPubKey []byte) ([]iwallet.Transaction, error)

	// GetWatchedAddress 返回指定地址的 WatchedAddress
	GetWatchedAddress(address string) *WatchedAddress

	// RegisterNodeCallback 注册节点的交易回调
	RegisterNodeCallback(nodeID string, callback func(tx iwallet.Transaction, wa *WatchedAddress)) error

	// UnregisterNode 注销节点，移除该节点的所有监控和回调
	UnregisterNode(nodeID string)

	// GetTransaction 获取指定交易
	GetTransaction(chainType iwallet.ChainType, txid string) (*iwallet.Transaction, error)

	// GetFeeEstimate 获取费率估算 (sat/vbyte)
	GetFeeEstimate(chainType iwallet.ChainType, targetBlocks int) uint64

	// BroadcastTransaction 广播交易
	BroadcastTransaction(chainType iwallet.ChainType, txHex string) (string, error)

	// IsHealthy 检查 Monitor 是否健康
	IsHealthy(chainType iwallet.ChainType) bool
}

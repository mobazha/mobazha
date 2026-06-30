package relay

import (
	"context"
	"math/big"
)

// ── EVM Relay ───────────────────────────────────────────────────────────

// EVMRelayRequest 中继请求
type EVMRelayRequest struct {
	ChainType string // eth, bsc, polygon, etc.
	To        string // 合约地址
	Data      string // 调用数据（hex格式）
	OrderID   string // Mobazha 订单 ID（用于日志追踪与会话对齐）

	// SettlementAction echoes confirm|cancel|complete|dispute_release|relay_submit — hosting logs。
	SettlementAction string

	// ClientActionID correlates hosting ↔ node projections (poll key before tx lands).
	ClientActionID string
}

// EVMRelayResponse 中继响应
type EVMRelayResponse struct {
	TxHash string

	// TaskID is allocated at Execute start on the platform relay (HTTP or in-process)
	// for log correlation and projections.
	TaskID string
}

// EVMGasWalletStatus reports operational health of the relay's gas
// wallet on a specific chain.
type EVMGasWalletStatus struct {
	Address string   // hex-encoded gas wallet EOA
	Balance *big.Int // native-asset balance in wei
	Healthy bool     // false when balance < threshold or RPC degraded
	Reason  string   // non-empty when Healthy is false
}

// EVMRelayService 定义 EVM 交易中继服务接口
// 支持两种模式：
// - Hosting 模式：HostService 提供实现，直接调用
// - 独立节点模式：通过 HTTP 调用平台 Relay API
type EVMRelayService interface {
	// Execute 执行交易中继，代付 gas 并广播交易
	Execute(ctx context.Context, req *EVMRelayRequest) (*EVMRelayResponse, error)

	// GetSupportedChains 获取支持的链列表
	GetSupportedChains() []string

	// IsAvailable 检查服务是否可用
	IsAvailable() bool

	// GetGasWalletAddress returns the hex-encoded gas wallet EOA for
	// the given EVM chainID. ManagedEscrowAdapter calls this before signing a
	// ManagedEscrowTx to lock the refundReceiver field.
	GetGasWalletAddress(ctx context.Context, chainID uint64) (string, error)

	// GetGasWalletStatus returns the operational health of the gas
	// wallet on the given chain. Adapters consult it before quoting
	// an order to decline orders on chains where relay cannot
	// reliably broadcast.
	GetGasWalletStatus(ctx context.Context, chainID uint64) (*EVMGasWalletStatus, error)

	// ChainTypeForID resolves a numeric EVM chainID (e.g. 56) to the
	// relay-service config key (e.g. "bsc"). Adapters that build an
	// EVMRelayRequest from a ManagedEscrowTx (which carries chainID) use this
	// to avoid hard-coding a parallel chainID→chainType map that would
	// drift from hosting's RelayConfig over time.
	// The HTTP standalone implementation prefers GET /platform/v1/relay/status
	// evmChains when available, then falls back to a static map.
	ChainTypeForID(chainID uint64) (string, error)
}

package relay

import "context"

// EVMRelayRequest 中继请求
type EVMRelayRequest struct {
	ChainType string // eth, bsc, polygon, etc.
	To        string // 合约地址
	Data      string // 调用数据（hex格式）
	OrderID   string // 订单ID（用于日志追踪）
}

// EVMRelayResponse 中继响应
type EVMRelayResponse struct {
	TxHash string
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
}

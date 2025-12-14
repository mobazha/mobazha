package coreiface

import (
	"github.com/mobazha/mobazha3.0/pkg/utxo"
)

// HostService 定义托管服务的接口
type HostService interface {
	// RegisterStripeAccount 注册 Stripe 账户
	RegisterStripeAccount(userID, stripeAccountID string) error

	// GetStripeAccountIDByUserID 通过用户ID获取 Stripe 账户ID
	GetStripeAccountIDByUserID(userID string) (string, error)

	// GetUserIDByStripeAccountID 通过 Stripe 账户ID获取用户ID
	GetUserIDByStripeAccountID(stripeAccountID string) (string, error)

	// GetStripeConfig 获取 Stripe 配置
	GetStripeConfig() (publicKey, secretKey, webhookKey string, err error)

	// GetUTXOMonitor 获取共享的 UTXO Monitor 服务
	// 如果 HostService 不支持共享 Monitor，返回 nil
	GetUTXOMonitor() utxo.UTXOMonitorService
}

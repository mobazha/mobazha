package config

import (
	"sync"
)

// ExchangeRateConfig 汇率配置
type ExchangeRateConfig struct {
	mu sync.RWMutex

	// Chainlink配置
	ChainlinkRPCURL  string
	ChainlinkEnabled bool

	// 传统API配置
	TraditionalAPIEnabled bool
	TraditionalAPISources []string

	// 缓存配置
	CacheTimeoutMinutes int
	MaxRetries          int
}

// DefaultExchangeRateConfig 返回默认的汇率配置
func DefaultExchangeRateConfig() *ExchangeRateConfig {
	return &ExchangeRateConfig{
		ChainlinkRPCURL:       "https://polygon-rpc.com",
		ChainlinkEnabled:      false, // 禁用 Chainlink，只使用 ticker API
		TraditionalAPIEnabled: true,
		TraditionalAPISources: []string{"https://info.mobazha.org/api/ticker"},
		CacheTimeoutMinutes:   1,
		MaxRetries:            3,
	}
}

// GetChainlinkRPCURL 获取Chainlink RPC URL
func (c *ExchangeRateConfig) GetChainlinkRPCURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ChainlinkRPCURL
}

// SetChainlinkRPCURL 设置Chainlink RPC URL
func (c *ExchangeRateConfig) SetChainlinkRPCURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ChainlinkRPCURL = url
}

// IsChainlinkEnabled 检查Chainlink是否启用
func (c *ExchangeRateConfig) IsChainlinkEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ChainlinkEnabled
}

// SetChainlinkEnabled 设置Chainlink启用状态
func (c *ExchangeRateConfig) SetChainlinkEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ChainlinkEnabled = enabled
}

// IsTraditionalAPIEnabled 检查传统API是否启用
func (c *ExchangeRateConfig) IsTraditionalAPIEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.TraditionalAPIEnabled
}

// SetTraditionalAPIEnabled 设置传统API启用状态
func (c *ExchangeRateConfig) SetTraditionalAPIEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.TraditionalAPIEnabled = enabled
}

// GetTraditionalAPISources 获取传统API源列表
func (c *ExchangeRateConfig) GetTraditionalAPISources() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]string{}, c.TraditionalAPISources...)
}

// SetTraditionalAPISources 设置传统API源列表
func (c *ExchangeRateConfig) SetTraditionalAPISources(sources []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.TraditionalAPISources = append([]string{}, sources...)
}

// GetCacheTimeoutMinutes 获取缓存超时时间（分钟）
func (c *ExchangeRateConfig) GetCacheTimeoutMinutes() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.CacheTimeoutMinutes
}

// SetCacheTimeoutMinutes 设置缓存超时时间（分钟）
func (c *ExchangeRateConfig) SetCacheTimeoutMinutes(minutes int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CacheTimeoutMinutes = minutes
}

// GetMaxRetries 获取最大重试次数
func (c *ExchangeRateConfig) GetMaxRetries() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.MaxRetries
}

// SetMaxRetries 设置最大重试次数
func (c *ExchangeRateConfig) SetMaxRetries(retries int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.MaxRetries = retries
}

// 全局配置实例
var (
	globalExchangeRateConfig *ExchangeRateConfig
	configOnce               sync.Once
)

// GetGlobalExchangeRateConfig 获取全局汇率配置实例
func GetGlobalExchangeRateConfig() *ExchangeRateConfig {
	configOnce.Do(func() {
		globalExchangeRateConfig = DefaultExchangeRateConfig()
	})
	return globalExchangeRateConfig
}

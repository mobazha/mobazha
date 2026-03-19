package config

import (
	"sync"
	"time"
)

// ExchangeRateConfig holds configuration for the exchange rate system.
type ExchangeRateConfig struct {
	mu sync.RWMutex

	// CoinGecko configuration (primary provider)
	CoinGeckoAPIKey  string
	CoinGeckoBaseURL string
	CoinGeckoEnabled bool

	// Chainlink configuration (secondary validation source)
	ChainlinkRPCURL  string
	ChainlinkEnabled bool

	// Legacy API configuration (deprecated, will be removed after EXR-1)
	TraditionalAPIEnabled bool
	TraditionalAPISources []string

	// Cache configuration
	CacheTTL   time.Duration
	MaxRetries int
}

// DefaultExchangeRateConfig returns the default exchange rate config.
// CoinGecko is the primary provider, Chainlink is disabled by default.
func DefaultExchangeRateConfig() *ExchangeRateConfig {
	return &ExchangeRateConfig{
		CoinGeckoAPIKey:       "",
		CoinGeckoBaseURL:      "https://api.coingecko.com/api/v3",
		CoinGeckoEnabled:      true,
		ChainlinkRPCURL:       "https://polygon-rpc.com",
		ChainlinkEnabled:      false,
		TraditionalAPIEnabled: false,
		TraditionalAPISources: nil,
		CacheTTL:              30 * time.Second,
		MaxRetries:            3,
	}
}

// GetCoinGeckoAPIKey returns the CoinGecko API key.
func (c *ExchangeRateConfig) GetCoinGeckoAPIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.CoinGeckoAPIKey
}

// SetCoinGeckoAPIKey sets the CoinGecko API key.
func (c *ExchangeRateConfig) SetCoinGeckoAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CoinGeckoAPIKey = key
}

// GetCoinGeckoBaseURL returns the CoinGecko base URL.
func (c *ExchangeRateConfig) GetCoinGeckoBaseURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.CoinGeckoBaseURL == "" {
		return "https://api.coingecko.com/api/v3"
	}
	return c.CoinGeckoBaseURL
}

// SetCoinGeckoBaseURL sets the CoinGecko base URL.
func (c *ExchangeRateConfig) SetCoinGeckoBaseURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CoinGeckoBaseURL = url
}

// IsCoinGeckoEnabled checks if CoinGecko provider is enabled.
func (c *ExchangeRateConfig) IsCoinGeckoEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.CoinGeckoEnabled
}

// SetCoinGeckoEnabled sets CoinGecko enabled state.
func (c *ExchangeRateConfig) SetCoinGeckoEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CoinGeckoEnabled = enabled
}

// GetChainlinkRPCURL returns the Chainlink RPC URL.
func (c *ExchangeRateConfig) GetChainlinkRPCURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ChainlinkRPCURL
}

// SetChainlinkRPCURL sets the Chainlink RPC URL.
func (c *ExchangeRateConfig) SetChainlinkRPCURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ChainlinkRPCURL = url
}

// IsChainlinkEnabled checks if Chainlink is enabled.
func (c *ExchangeRateConfig) IsChainlinkEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ChainlinkEnabled
}

// SetChainlinkEnabled sets Chainlink enabled state.
func (c *ExchangeRateConfig) SetChainlinkEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ChainlinkEnabled = enabled
}

// IsTraditionalAPIEnabled checks if legacy API is enabled.
func (c *ExchangeRateConfig) IsTraditionalAPIEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.TraditionalAPIEnabled
}

// SetTraditionalAPIEnabled sets legacy API enabled state.
func (c *ExchangeRateConfig) SetTraditionalAPIEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.TraditionalAPIEnabled = enabled
}

// GetTraditionalAPISources returns legacy API source URLs.
func (c *ExchangeRateConfig) GetTraditionalAPISources() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]string{}, c.TraditionalAPISources...)
}

// SetTraditionalAPISources sets legacy API source URLs.
func (c *ExchangeRateConfig) SetTraditionalAPISources(sources []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.TraditionalAPISources = append([]string{}, sources...)
}

// GetCacheTTL returns the cache TTL duration.
func (c *ExchangeRateConfig) GetCacheTTL() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.CacheTTL == 0 {
		return 30 * time.Second
	}
	return c.CacheTTL
}

// SetCacheTTL sets the cache TTL duration.
func (c *ExchangeRateConfig) SetCacheTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CacheTTL = ttl
}

// GetCacheTimeoutMinutes returns the cache timeout in minutes (backward compat).
// Deprecated: Use GetCacheTTL instead.
func (c *ExchangeRateConfig) GetCacheTimeoutMinutes() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return int(c.CacheTTL.Minutes())
}

// SetCacheTimeoutMinutes sets the cache timeout in minutes (backward compat).
// Deprecated: Use SetCacheTTL instead.
func (c *ExchangeRateConfig) SetCacheTimeoutMinutes(minutes int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CacheTTL = time.Duration(minutes) * time.Minute
}

// GetMaxRetries returns the maximum number of retries.
func (c *ExchangeRateConfig) GetMaxRetries() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.MaxRetries
}

// SetMaxRetries sets the maximum number of retries.
func (c *ExchangeRateConfig) SetMaxRetries(retries int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.MaxRetries = retries
}

var (
	globalExchangeRateConfig *ExchangeRateConfig
	configOnce               sync.Once
)

// GetGlobalExchangeRateConfig returns the global exchange rate config singleton.
func GetGlobalExchangeRateConfig() *ExchangeRateConfig {
	configOnce.Do(func() {
		globalExchangeRateConfig = DefaultExchangeRateConfig()
	})
	return globalExchangeRateConfig
}

package config

import (
	"strconv"
	"sync"

	"github.com/go-resty/resty/v2"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type NetConfig struct {
	BootstrapAddrs         []string `json:"bootstrapAddrs,omitempty"`
	StoreAndForwardServers []string `json:"snfServers,omitempty"`

	// ExchangeRateProviders API URL to use for exchange rates. Must conform to the BitcoinAverage format.
	ExchangeRateProviders []string `json:"exchangeRateProviders,omitempty"`

	PlatformAddrs     map[iwallet.ChainType]string `json:"platformAddrs,omitempty"`
	platformAddrMutex sync.RWMutex                 `json:"-"`

	dataMutex sync.RWMutex      `json:"-"`
	Data      map[string]string `json:"data,omitempty"`

	Testnet bool `json:"-"`
}

var defaultMainnetPlatformAddrs = map[iwallet.ChainType]string{
	iwallet.ChainBitcoin:     "bc1qq0qpv5l54v2a3vvwq2ygrrdn54pnv3x4rjxpxhlryghrxlmrzr0qds28qd",
	iwallet.ChainEthereum:    "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainBitcoinCash: "ppaz03a9gc9r339wq9ctggf5st79zkjfxgle6qvuss",
	iwallet.ChainLitecoin:    "MTRuWRh99NfdsyRL4oMUaB2NzMqKVKKRkK",
	iwallet.ChainZCash:       "t1VNBTzKypFaAJH8A6uj4Fq67xyMcKhkmyf",
	iwallet.ChainPolygon:     "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainConflux:     "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
}

var defaultTestnetPlatformAddrs = map[iwallet.ChainType]string{}

func DefaultNetConfig() *NetConfig {
	return &NetConfig{
		dataMutex: sync.RWMutex{},
		Data:      make(map[string]string),

		platformAddrMutex: sync.RWMutex{},
		Testnet:           true,
	}
}

func LoadNetConfig(endpoint string) (*NetConfig, error) {
	var config NetConfig
	_, err := resty.New().R().ForceContentType("application/json").SetResult(&config).Get(endpoint)
	if err != nil {
		return DefaultNetConfig(), err
	}
	if config.Data == nil {
		config.Data = make(map[string]string)
	}
	config.dataMutex = sync.RWMutex{}
	config.platformAddrMutex = sync.RWMutex{}

	return &config, nil
}

func (config *NetConfig) GetExchangeRateProviders() []string {
	if len(config.ExchangeRateProviders) == 0 {
		return []string{"https://info.mobazha.org/api/ticker"}
	}
	return config.ExchangeRateProviders
}

// GetNetDBEndpoint The endpoint to use for the network database, which is used
// to backup and query profile and listing data here.
func (config *NetConfig) GetNetDBEndpoint() string {
	val, _ := config.GetConfig("netDBEndpoint")
	if len(val) > 0 {
		return val
	}
	return "https://info.mobazha.org/search/v1/netdb"
}

// GetAIProviders returns the remote AI provider presets JSON string from
// the nodeConfig data map. Returns empty string if not configured.
func (config *NetConfig) GetAIProviders() string {
	val, ok := config.GetConfig("aiProviders")
	if ok && len(val) > 0 {
		return val
	}
	return ""
}

func (config *NetConfig) GetConfig(key string) (string, bool) {
	config.dataMutex.RLock()
	defer config.dataMutex.RUnlock()

	val, ok := config.Data[key]
	return val, ok
}

// GetCommission If not set or fail to parse, use default 1(1%)
func (config *NetConfig) GetCommission() float64 {
	val, ok := config.GetConfig("commission")
	if !ok || len(val) == 0 {
		// Not set, default to 1%
		return 1
	}

	commission, err := strconv.ParseFloat(val, 64)
	if err != nil || commission < 0 || commission > 100 {
		return 1
	}
	return commission
}

func (config *NetConfig) GetPlatformAddr(chainType iwallet.ChainType) string {
	config.platformAddrMutex.RLock()
	defer config.platformAddrMutex.RUnlock()

	if len(config.PlatformAddrs) == 0 {
		if config.Testnet {
			return defaultTestnetPlatformAddrs[chainType]
		}
		return defaultMainnetPlatformAddrs[chainType]
	}
	return config.PlatformAddrs[chainType]
}

func (config *NetConfig) GetCommissionInfo(coinType iwallet.CoinType) (iwallet.Address, float64) {
	coinInfo, _ := iwallet.CoinInfoFromCoinType(coinType)
	addr := config.GetPlatformAddr(coinInfo.Chain)
	commission := config.GetCommission()

	if len(addr) == 0 {
		return iwallet.Address{}, 0
	}

	return iwallet.NewAddress(addr, coinType), commission
}

// GetVerifiedModEndpoint API URL to get verified moderator IDs.
func (config *NetConfig) GetVerifiedModEndpoint() string {
	val, _ := config.GetConfig("verifiedModEndpoint")
	if len(val) > 0 {
		return val
	}
	return "https://info.mobazha.org/search/v1/moderators/verified"
}

// GetMaxImportZipSize returns the maximum size for batch import ZIP files in bytes.
// Default is 300MB (314572800 bytes).
func (config *NetConfig) GetMaxImportZipSize() int64 {
	val, ok := config.GetConfig("maxImportZipSize")
	if ok && len(val) > 0 {
		size, err := strconv.ParseInt(val, 10, 64)
		if err == nil && size > 0 {
			return size
		}
	}
	return 300 << 20 // 300MB default
}

// GetMaxImportVideoSize returns the maximum size for individual video files in batch import.
// Default is 15MB (15728640 bytes).
func (config *NetConfig) GetMaxImportVideoSize() int64 {
	val, ok := config.GetConfig("maxImportVideoSize")
	if ok && len(val) > 0 {
		size, err := strconv.ParseInt(val, 10, 64)
		if err == nil && size > 0 {
			return size
		}
	}
	return 15 << 20 // 15MB default
}

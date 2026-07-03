package config

import (
	"strconv"
	"sync"

	"github.com/go-resty/resty/v2"

	pkgconfig "github.com/mobazha/mobazha/pkg/config"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type NetConfig struct {
	BootstrapAddrs         []string `json:"bootstrapAddrs,omitempty"`
	StoreAndForwardServers []string `json:"snfServers,omitempty"`

	PlatformAddrs     map[iwallet.ChainType]string `json:"platformAddrs,omitempty"`
	platformAddrMutex sync.RWMutex                 `json:"-"`

	dataMutex sync.RWMutex      `json:"-"`
	Data      map[string]string `json:"data,omitempty"`

	// Matrix homeserver configuration (injected by hosting in SaaS mode)
	MatrixInternalURL        string `json:"matrixInternalURL,omitempty"`
	MatrixServerName         string `json:"matrixServerName,omitempty"`
	MatrixRegistrationSecret string `json:"matrixRegistrationSecret,omitempty"`
	MatrixSDKLogLevel        string `json:"matrixSDKLogLevel,omitempty"` // off|info|debug

	Testnet bool `json:"-"`
}

var defaultMainnetPlatformAddrs = map[iwallet.ChainType]string{
	iwallet.ChainBitcoin:     "bc1qq0qpv5l54v2a3vvwq2ygrrdn54pnv3x4rjxpxhlryghrxlmrzr0qds28qd",
	iwallet.ChainEthereum:    "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainBSC:         "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainBitcoinCash: "ppaz03a9gc9r339wq9ctggf5st79zkjfxgle6qvuss",
	iwallet.ChainLitecoin:    "MTRuWRh99NfdsyRL4oMUaB2NzMqKVKKRkK",
	iwallet.ChainZCash:       "t1VNBTzKypFaAJH8A6uj4Fq67xyMcKhkmyf",
	iwallet.ChainPolygon:     "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainBase:        "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainArbitrum:    "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainOptimism:    "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainAvalanche:   "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainGnosis:      "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainCelo:        "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainMantle:      "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainScroll:      "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainLinea:       "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainConflux:     "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
}

var defaultTestnetPlatformAddrs = map[iwallet.ChainType]string{
	iwallet.ChainEthereum:  "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainBSC:       "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainPolygon:   "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainBase:      "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainArbitrum:  "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainOptimism:  "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainAvalanche: "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainGnosis:    "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainCelo:      "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainMantle:    "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainScroll:    "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.ChainLinea:     "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
}

func DefaultNetConfig() *NetConfig {
	return &NetConfig{
		dataMutex: sync.RWMutex{},
		Data:      make(map[string]string),

		platformAddrMutex: sync.RWMutex{},
		Testnet:           true,
	}
}

func LoadNetConfig(endpoint string) (*NetConfig, error) {
	var envelope struct {
		Data *NetConfig `json:"data"`
	}
	_, err := resty.New().R().ForceContentType("application/json").SetResult(&envelope).Get(endpoint)
	if err != nil {
		return DefaultNetConfig(), err
	}
	if envelope.Data == nil {
		// API returned null or omitted "data"; behave like an empty unmarshaled object.
		envelope.Data = &NetConfig{}
	}
	config := envelope.Data
	if config.Data == nil {
		config.Data = make(map[string]string)
	}
	return config, nil
}

// GetNetDBEndpoint returns the endpoint for search index sync (NetDB).
// Testnet nodes return empty by default to prevent accidental data leakage
// to the production search index. Use --netdbendpoint to override explicitly.
func (config *NetConfig) GetNetDBEndpoint() string {
	val, _ := config.GetConfig("netDBEndpoint")
	if len(val) > 0 {
		return val
	}
	if config.Testnet {
		return ""
	}
	return "https://app.mobazha.org/search/v1/netdb"
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

func (config *NetConfig) SetConfig(key, value string) {
	config.dataMutex.Lock()
	defer config.dataMutex.Unlock()

	if config.Data == nil {
		config.Data = make(map[string]string)
	}
	if value == "" {
		delete(config.Data, key)
		return
	}
	config.Data[key] = value
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

	if addr := config.PlatformAddrs[chainType]; addr != "" {
		return addr
	}
	return config.defaultPlatformAddr(chainType)
}

func (config *NetConfig) SetPlatformAddr(chainType iwallet.ChainType, addr string) {
	config.platformAddrMutex.Lock()
	defer config.platformAddrMutex.Unlock()

	if config.PlatformAddrs == nil {
		config.PlatformAddrs = make(map[iwallet.ChainType]string)
	}
	if addr == "" {
		delete(config.PlatformAddrs, chainType)
		return
	}
	config.PlatformAddrs[chainType] = addr
}

func (config *NetConfig) defaultPlatformAddr(chainType iwallet.ChainType) string {
	if config.Testnet {
		return defaultTestnetPlatformAddrs[chainType]
	}
	return defaultMainnetPlatformAddrs[chainType]
}

func ManagedEscrowReleaseFeeUSDCentsKey(chainType iwallet.ChainType) string {
	return pkgconfig.ManagedEscrowReleaseFeeUSDCentsKey(chainType)
}

func (config *NetConfig) GetManagedEscrowReleaseFeeUSDCents(chainType iwallet.ChainType) (uint64, bool) {
	if config == nil {
		return 0, false
	}
	raw, ok := config.GetConfig(ManagedEscrowReleaseFeeUSDCentsKey(chainType))
	if !ok || raw == "" {
		return 0, false
	}
	fee, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return fee, true
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
	return "https://app.mobazha.org/search/v1/moderators/verified"
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

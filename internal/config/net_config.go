package config

import (
	"fmt"
	"net/http"
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

	// ExtraFeesPerByte 从测试看，从一些fee provider拿到的fee比实际的fee要低，所以需要额外加一些fee
	ExtraFeesPerByte      map[iwallet.ChainType]int32 `json:"extraFeesPerByte,omitempty"`
	extraFeesPerByteMutex sync.RWMutex                `json:"-"`

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

var defaultExtraFeesPerByte = map[iwallet.ChainType]int32{
	iwallet.ChainBitcoin:     0,
	iwallet.ChainEthereum:    0,
	iwallet.ChainBitcoinCash: 3,
	iwallet.ChainLitecoin:    3,
	iwallet.ChainZCash:       0,
}

func DefaultNetConfig() *NetConfig {
	return &NetConfig{
		dataMutex: sync.RWMutex{},
		Data:      make(map[string]string),

		platformAddrMutex: sync.RWMutex{},
		Testnet:           true,

		extraFeesPerByteMutex: sync.RWMutex{},
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
	config.extraFeesPerByteMutex = sync.RWMutex{}

	return &config, nil
}

func (config *NetConfig) GetExchangeRateProviders() []string {
	if len(config.ExchangeRateProviders) == 0 {
		return []string{"https://info.mobazha.org/api/ticker"}
	}
	return config.ExchangeRateProviders
}

// GetIPNSResolver If a url is provided the node will resolve IPNS records by
// querying this server instead of using the peer-to-peer network.
func (config *NetConfig) GetIPNSResolver() string {
	val, _ := config.GetConfig("ipnsResolver")
	if len(val) > 0 {
		return val
	}
	return "https://store.mobazha.org/api/ipns"
}

// GetNetDBEndpoint The endpoint to use for the network database, which is used
// to backup and query profile and listing data here.
func (config *NetConfig) GetNetDBEndpoint() string {
	val, _ := config.GetConfig("netDBEndpoint")
	if len(val) > 0 {
		return val
	}
	return "https://info.mobazha.org/api/netdb"
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

func (config *NetConfig) GetExtraFeesPerByte(chainType iwallet.ChainType) iwallet.Amount {
	config.extraFeesPerByteMutex.RLock()
	defer config.extraFeesPerByteMutex.RUnlock()

	val, ok := config.ExtraFeesPerByte[chainType]
	if ok {
		return iwallet.NewAmount(val)
	}
	return iwallet.NewAmount(defaultExtraFeesPerByte[chainType])
}

// GetVerifiedModEndpoint API URL to get verified moderator IDs.
func (config *NetConfig) GetVerifiedModEndpoint() string {
	val, _ := config.GetConfig("verifiedModEndpoint")
	if len(val) > 0 {
		return val
	}
	return "https://info.mobazha.org/api/search/filters/moderators"
}

func (config *NetConfig) GetFeeUrl(coinType iwallet.ChainType) string {
	protocol := "bitcoin"
	switch coinType {
	case iwallet.ChainBitcoin:
		protocol = "bitcoin"
	case iwallet.ChainLitecoin:
		protocol = "litecoin"
	case iwallet.ChainBitcoinCash:
		protocol = "bitcoincash"
	case iwallet.ChainZCash:
		protocol = "zcash"
	}

	url, _ := config.GetConfig("feeUrl")
	if len(url) == 0 {
		url = "https://mobazha.info/api/ticker/fees"
	}
	return fmt.Sprintf(url+"?protocol=%s", protocol)
}

func (config *NetConfig) GetBlockbookWebsocketHeader() http.Header {
	origin, _ := config.GetConfig("blockbookOrigin")
	if len(origin) == 0 {
		origin = "https://node.trezor.io"
	}
	userAgent, _ := config.GetConfig("blockbookUserAgent")
	if len(userAgent) == 0 {
		userAgent = "Trezor Suite v24.9.2"
	}
	return http.Header{
		"Origin":     []string{origin},
		"User-Agent": []string{userAgent},
	}
}

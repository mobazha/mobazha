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

	PlatformAddrs     map[iwallet.CoinType]string `json:"platformAddrs,omitempty"`
	platformAddrMutex sync.RWMutex                `json:"-"`

	ExtraFeesPerByte      map[iwallet.CoinType]int32 `json:"extraFeesPerByte,omitempty"`
	extraFeesPerByteMutex sync.RWMutex               `json:"-"`

	dataMutex sync.RWMutex      `json:"-"`
	Data      map[string]string `json:"data,omitempty"`

	Testnet bool `json:"-"`
}

var defaultMainnetPlatformAddrs = map[iwallet.CoinType]string{
	iwallet.CtBitcoin:     "bc1qq0qpv5l54v2a3vvwq2ygrrdn54pnv3x4rjxpxhlryghrxlmrzr0qds28qd",
	iwallet.CtEthereum:    "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.CtBitcoinCash: "ppaz03a9gc9r339wq9ctggf5st79zkjfxgle6qvuss",
	iwallet.CtLitecoin:    "MTRuWRh99NfdsyRL4oMUaB2NzMqKVKKRkK",
	iwallet.CtZCash:       "t1VNBTzKypFaAJH8A6uj4Fq67xyMcKhkmyf",
	iwallet.CtMATIC:       "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
	iwallet.CtCFX:         "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
}

var defaultTestnetPlatformAddrs = map[iwallet.CoinType]string{}

var defaultExtraFeesPerByte = map[iwallet.CoinType]int32{
	iwallet.CtBitcoin:     0,
	iwallet.CtEthereum:    0,
	iwallet.CtBitcoinCash: 3,
	iwallet.CtLitecoin:    3,
	iwallet.CtZCash:       0,
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
	return "https://store.mobazha.org/netdb"
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

func (config *NetConfig) GetPlatformAddr(coinType iwallet.CoinType) string {
	config.platformAddrMutex.RLock()
	defer config.platformAddrMutex.RUnlock()

	if coinType.IsERC20Token() {
		coinType = coinType.ChainCoinType()
	}

	if len(config.PlatformAddrs) == 0 {
		if config.Testnet {
			return defaultTestnetPlatformAddrs[coinType]
		}
		return defaultMainnetPlatformAddrs[coinType]
	}
	return config.PlatformAddrs[coinType]
}

func (config *NetConfig) GetCommissionInfo(coinType iwallet.CoinType) (iwallet.Address, float64) {
	addr := config.GetPlatformAddr(coinType)
	commission := config.GetCommission()

	if len(addr) == 0 {
		return iwallet.Address{}, 0
	}

	return iwallet.NewAddress(addr, coinType), commission
}

func (config *NetConfig) GetExtraFeesPerByte(coinType iwallet.CoinType) iwallet.Amount {
	config.extraFeesPerByteMutex.RLock()
	defer config.extraFeesPerByteMutex.RUnlock()
	val, ok := config.ExtraFeesPerByte[coinType]
	if ok {
		return iwallet.NewAmount(val)
	}
	return iwallet.NewAmount(defaultExtraFeesPerByte[coinType])
}

// GetVerifiedModEndpoint API URL to get verified moderator IDs.
func (config *NetConfig) GetVerifiedModEndpoint() string {
	val, _ := config.GetConfig("verifiedModEndpoint")
	if len(val) > 0 {
		return val
	}
	return "https://info.mobazha.org/api/search/filters/moderators"
}

func (config *NetConfig) GetFeeUrl(coinType iwallet.CoinType) string {
	protocol := "bitcoin"
	switch coinType {
	case iwallet.CtBitcoin:
		protocol = "bitcoin"
	case iwallet.CtLitecoin:
		protocol = "litecoin"
	case iwallet.CtBitcoinCash:
		protocol = "bitcoincash"
	case iwallet.CtZCash:
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

package wallet

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ChainlinkProvider 实现provider接口，通过Chainlink预言机获取汇率
type ChainlinkProvider struct {
	rpcURL     string
	client     *ethclient.Client
	httpClient *http.Client
	feeds      map[string]string // 币种到合约地址的映射
}

// ChainlinkPriceFeed ABI 的简化版本，只包含我们需要的方法
const chainlinkABI = `[
	{
		"inputs": [],
		"name": "decimals",
		"outputs": [{"internalType": "uint8", "name": "", "type": "uint8"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "latestRoundData",
		"outputs": [
			{"internalType": "uint80", "name": "roundId", "type": "uint80"},
			{"internalType": "int256", "name": "answer", "type": "int256"},
			{"internalType": "uint256", "name": "startedAt", "type": "uint256"},
			{"internalType": "uint256", "name": "updatedAt", "type": "uint256"},
			{"internalType": "uint80", "name": "answeredInRound", "type": "uint80"}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

// 价格源配置 - 使用Polygon网络的Chainlink预言机
var priceFeeds = map[string]string{
	"USDT":  "0x0A6513e40db6EB1b165753AD52E80663aeA50545",
	"USDC":  "0xfE4A8cc5b5B2366C1B58Bea3858e81843581b2F7",
	"SOL":   "0x10C8264C0935b3B9870013e057f330Ff3e9C56dC",
	"BNB":   "0x82a6c4AF830caa6c97bb504425f6A66165C2c26e",
	"MATIC": "0xAB594600376Ec9fD91F8e885dADF0CE036862dE0",
	"BTC":   "0xc907E116054Ad103354f2D350FD2514433D57F6f",
	"ETH":   "0xF9680D99D6C9589e2a93a78A04A279e509205945",
	"BCH":   "0x327d9822e9932996f55b39F557AEC838313da8b7",
	"LTC":   "0xEB99F173cf7d9a6dC4D889C2Ad7103e8383b6Efa",
	"ZEC":   "0xBC08c639e579a391C4228F20d0C29d0690092DF0",
	"EXTERNAL_PAYMENT":   "0xBE6FB0AB6302B693368D0E9001fAF77ecc6571db",
}

// NewChainlinkProvider 创建新的Chainlink预言机provider
func NewChainlinkProvider(rpcURL string) (*ChainlinkProvider, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client: %w", err)
	}

	httpClient := &http.Client{
		Timeout: time.Minute,
	}

	return &ChainlinkProvider{
		rpcURL:     rpcURL,
		client:     client,
		httpClient: httpClient,
		feeds:      priceFeeds,
	}, nil
}

// fetchRates implements the provider interface, returning rates from Chainlink oracles.
// All Chainlink price feeds return crypto/USD prices. The function normalizes rates
// so that the result is: 1 unit of base = X smallest-units of target.
func (c *ChainlinkProvider) fetchRates(base models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error) {
	_, ok := models.CurrencyDefinitions[base.String()]
	if !ok {
		return nil, fmt.Errorf("base currency %s is not supported", base.String())
	}

	// rates stores crypto → USD price (e.g., BTC → 65000.0, ETH → 3500.0)
	rates := make(map[models.CurrencyCode]*big.Float)

	for symbol, address := range c.feeds {
		if c.isStablecoin(symbol) {
			rates[models.CurrencyCode(symbol)] = new(big.Float).SetFloat64(1.0)
			continue
		}

		rate, err := c.getPriceFromChainlink(address)
		if err != nil {
			fmt.Printf("Failed to get price for %s: %v\n", symbol, err)
			continue
		}

		if rate > 0 {
			rates[models.CurrencyCode(symbol)] = new(big.Float).SetFloat64(rate)
		}
	}

	if len(rates) == 0 {
		return nil, fmt.Errorf("no rates available from Chainlink")
	}

	// Handle USD base: 1 USD = (1 / cryptoUSD) units of each crypto
	if base.String() == "USD" {
		result := make(map[models.CurrencyCode]iwallet.Amount)
		for currency, usdPrice := range rates {
			usdToCurrency := new(big.Float).SetPrec(256).Quo(
				new(big.Float).SetPrec(256).SetFloat64(1.0),
				usdPrice,
			)

			def := models.CurrencyDefinitions[currency.String()]
			divisibility := new(big.Float).SetPrec(256).SetFloat64(math.Pow10(int(def.Divisibility)))
			convertedInt, _ := new(big.Float).SetPrec(256).Mul(usdToCurrency, divisibility).Int(nil)
			result[currency] = iwallet.NewAmount(convertedInt)
		}
		return result, nil
	}

	// For any other base (crypto): 1 base = (baseUSD / targetUSD) units of target
	baseFloat, ok := rates[base]
	if !ok {
		return nil, fmt.Errorf("base currency %s not found in Chainlink feeds", base.String())
	}

	result := make(map[models.CurrencyCode]iwallet.Amount)
	for currency, usdPrice := range rates {
		baseToCurrency := new(big.Float).SetPrec(256).Quo(baseFloat, usdPrice)

		def := models.CurrencyDefinitions[currency.String()]
		divisibility := new(big.Float).SetPrec(256).SetFloat64(math.Pow10(int(def.Divisibility)))
		convertedInt, _ := new(big.Float).SetPrec(256).Mul(baseToCurrency, divisibility).Int(nil)
		result[currency] = iwallet.NewAmount(convertedInt)
	}

	return result, nil
}

// getPriceFromChainlink 从Chainlink预言机获取价格
func (c *ChainlinkProvider) getPriceFromChainlink(feedAddress string) (float64, error) {
	// 创建合约调用
	address := common.HexToAddress(feedAddress)

	// 调用latestRoundData方法
	data, err := c.client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &address,
		Data: common.FromHex("0xfeaf968c"), // latestRoundData() 方法的选择器
	}, nil)

	if err != nil {
		return 0, fmt.Errorf("failed to call latestRoundData: %w", err)
	}

	// 解析返回数据
	if len(data) < 32 {
		return 0, fmt.Errorf("invalid response length")
	}

	// 获取answer字段（第二个返回值）
	answerBytes := data[32:64]
	answer := new(big.Int).SetBytes(answerBytes)

	// 获取decimals
	decimalsData, err := c.client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &address,
		Data: common.FromHex("0x313ce567"), // decimals() 方法的选择器
	}, nil)

	if err != nil {
		return 0, fmt.Errorf("failed to call decimals: %w", err)
	}

	if len(decimalsData) < 32 {
		return 0, fmt.Errorf("invalid decimals response")
	}

	decimalsBytes := decimalsData[31:32] // decimals是uint8，只取最后一个字节
	decimals := int(decimalsBytes[0])

	// 转换为float64
	price := new(big.Float).SetInt(answer)
	divisor := new(big.Float).SetFloat64(math.Pow10(decimals))
	priceFloat, _ := new(big.Float).Quo(price, divisor).Float64()

	return priceFloat, nil
}

// isStablecoin 检查是否为稳定币
func (c *ChainlinkProvider) isStablecoin(symbol string) bool {
	stablecoins := []string{"USDT", "USDC"}
	for _, stablecoin := range stablecoins {
		if symbol == stablecoin {
			return true
		}
	}
	return false
}

// Close 关闭provider连接
func (c *ChainlinkProvider) Close() error {
	if c.client != nil {
		c.client.Close()
	}
	return nil
}

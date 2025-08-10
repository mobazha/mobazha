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

// fetchRates 实现provider接口，获取汇率数据
func (c *ChainlinkProvider) fetchRates(base models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error) {
	// 验证基础货币是否支持
	_, ok := models.CurrencyDefinitions[base.String()]
	if !ok {
		return nil, fmt.Errorf("base currency %s is not supported", base.String())
	}

	// 获取所有支持的币种对USD的汇率
	rates := make(map[models.CurrencyCode]*big.Float)

	for symbol, address := range c.feeds {
		// 对于稳定币，直接设置为1
		if c.isStablecoin(symbol) {
			rates[models.CurrencyCode(symbol)] = new(big.Float).SetFloat64(1.0)
			continue
		}

		rate, err := c.getPriceFromChainlink(address)
		if err != nil {
			// 记录错误但继续处理其他币种
			fmt.Printf("Failed to get price for %s: %v\n", symbol, err)
			continue
		}

		if rate > 0 {
			rates[models.CurrencyCode(symbol)] = new(big.Float).SetFloat64(rate)
		}
	}

	// 添加额外的币种映射
	c.addAdditionalCurrenciesRates(rates)

	// 手动添加USD汇率（USD对USD = 1）
	rates[models.CurrencyCode("USD")] = new(big.Float).SetFloat64(1.0)

	// 如果基础货币是USDT，直接返回所有汇率
	if base.String() == ReserveCurrency.String() {
		result := make(map[models.CurrencyCode]iwallet.Amount)
		for currency, val := range rates {
			def := models.CurrencyDefinitions[currency.String()]
			divisibility := new(big.Float).SetFloat64(math.Pow10(int(def.Divisibility)))
			convertedInt, _ := new(big.Float).Mul(val, divisibility).Int(nil)
			result[currency] = iwallet.NewAmount(convertedInt)
		}
		return result, nil
	}

	// 如果基础货币不是USDT，需要进行转换
	baseMap := make(map[models.CurrencyCode]iwallet.Amount)

	// 获取基础货币对USD的汇率
	baseFloat, ok := rates[base]
	if !ok {
		return nil, fmt.Errorf("base currency %s not found in Chainlink feeds", base.String())
	}

	// 对于非USDT基础货币，我们需要计算其他货币相对于基础货币的汇率
	// Chainlink返回的是所有货币对USD的价格，我们需要转换为对基础货币的价格
	for currency, rate := range rates {
		// 计算 currency/base 的汇率
		// 如果 currency 对 USD 的价格是 rate，base 对 USD 的价格是 baseFloat
		// 那么 currency/base = rate/baseFloat
		// 但是对于 USD，我们需要特殊处理：USD/BTC = 1/BTC_USD_price
		var convertedFloat *big.Float
		if currency.String() == "USD" {
			// 当请求 BTC/USD 汇率时，我们需要返回 BTC 对 USD 的价格
			// 即 1 BTC = baseFloat USD
			convertedFloat = baseFloat
		} else {
			// 其他货币：currency/BTC = currency_USD_price / BTC_USD_price
			convertedFloat = new(big.Float).Quo(rate, baseFloat)
		}
		def := models.CurrencyDefinitions[currency.String()]
		divisibility := new(big.Float).SetFloat64(math.Pow10(int(def.Divisibility)))
		convertedInt, _ := new(big.Float).Mul(convertedFloat, divisibility).Int(nil)
		baseMap[currency] = iwallet.NewAmount(convertedInt)
	}

	return baseMap, nil
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
	stablecoins := []string{"USD", "USDT", "USDC"}
	for _, stablecoin := range stablecoins {
		if symbol == stablecoin {
			return true
		}
	}
	return false
}

// addAdditionalCurrenciesRates 添加额外的币种汇率映射
func (c *ChainlinkProvider) addAdditionalCurrenciesRates(rateMap map[models.CurrencyCode]*big.Float) {
	// 添加USDT的BEP20版本映射
	if rate, ok := rateMap["USDT"]; ok {
		rateMap[models.CurrencyCode(iwallet.CtBEP20USDT.CurrencyCode())] = rate
	}

	// 添加USDC的BEP20版本映射
	if rate, ok := rateMap["USDC"]; ok {
		rateMap[models.CurrencyCode(iwallet.CtBEP20USDC.CurrencyCode())] = rate
	}
}

// Close 关闭provider连接
func (c *ChainlinkProvider) Close() error {
	if c.client != nil {
		c.client.Close()
	}
	return nil
}

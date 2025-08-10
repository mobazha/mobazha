package wallet

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/libs/proxyclient"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ReserveCurrency is the currency used to calculate the exchange rates
// for all other currencies. In this case it's USDT. If you want to know
// the USD price of BCH we first get the USD price of USDT, then get the
// ratio of USDT/BCH and use it to calculate the BCH USD price.
const ReserveCurrency = models.CurrencyCode("USDT")

// ExchangeRateProvider provides exchange rate data to be used by OpenBazaar.
// It gives the exchange rate from any listed cryptocurrency into any other
// currency.
type ExchangeRateProvider struct {
	cache       map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount
	lastQueried map[models.CurrencyCode]time.Time
	mtx         sync.Mutex
	providers   []provider
}

// NewExchangeRateProvider returns a new ExchangeRateProvider. If proxy is
// not nil the http connection to the API server will use the proxy. The
// provided sources must conform to the BitcoinAverage API specification.
func NewExchangeRateProvider(sources []string) *ExchangeRateProvider {
	e := ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		mtx:         sync.Mutex{},
	}

	// 获取配置
	cfg := config.GetGlobalExchangeRateConfig()
	client := proxyclient.NewHttpClient()
	client.Timeout = time.Duration(cfg.GetCacheTimeoutMinutes()) * time.Minute

	// 如果启用Chainlink，添加Chainlink预言机provider作为主要数据源
	if cfg.IsChainlinkEnabled() {
		chainlinkProvider, err := NewChainlinkProvider(cfg.GetChainlinkRPCURL())
		if err == nil {
			e.providers = append(e.providers, chainlinkProvider)
			fmt.Printf("Chainlink provider initialized successfully\n")
		} else {
			fmt.Printf("Failed to initialize Chainlink provider: %v\n", err)
		}
	}

	// 如果启用传统API，添加传统的API providers作为补充数据源
	if cfg.IsTraditionalAPIEnabled() {
		// 使用配置中的源，如果没有则使用传入的sources
		apiSources := cfg.GetTraditionalAPISources()
		if len(apiSources) == 0 {
			apiSources = sources
		}

		for _, src := range apiSources {
			e.providers = append(e.providers, &openBazaarAPI{src, client})
		}
	}

	return &e
}

// GetRate returns the rate for a given currency converting from the provided base currency.
func (e *ExchangeRateProvider) GetRate(base models.CurrencyCode, to models.CurrencyCode, breakCache bool) (iwallet.Amount, error) {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	base = models.CurrencyCode(strings.TrimPrefix(strings.ToUpper(base.String()), "T"))
	lastQueried := e.lastQueried[base]

	rates, ok := e.cache[base]
	if breakCache || !ok || lastQueried.Add(time.Minute*10).Before(time.Now()) {
		var err error
		rates, err = e.fetchRatesFromProviders(base)
		if err != nil {
			return iwallet.NewAmount(0), err
		}
		e.cache[base] = rates
		e.lastQueried[base] = time.Now()
	}
	amount, ok := rates[to]
	if !ok {
		return amount, errors.New("rate not found")
	}
	return amount, nil
}

// GetUSDRate returns the USD exchange rate for the given coin.
func (e *ExchangeRateProvider) GetUSDRate(coinType iwallet.CoinType) (iwallet.Amount, error) {
	return e.GetRate(models.CurrencyCode(coinType.CurrencyCode()), models.CurrencyCode("USD"), false)
}

// GetAllRates returns a map of all exchange rates for the provided base currency.
func (e *ExchangeRateProvider) GetAllRates(base models.CurrencyCode, breakCache bool) (map[models.CurrencyCode]iwallet.Amount, error) {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	lastQueried := e.lastQueried[base]

	rates, ok := e.cache[base]
	if breakCache || !ok || lastQueried.Add(time.Minute*10).Before(time.Now()) {
		var err error
		rates, err = e.fetchRatesFromProviders(base)
		if err != nil {
			return nil, err
		}
		e.cache[base] = rates
		e.lastQueried[base] = time.Now()
	}
	return rates, nil
}

// fetchRatesFromProviders queries all exchange rate sources and combines the results.
// For the same currency, Chainlink data takes priority.
func (e *ExchangeRateProvider) fetchRatesFromProviders(base models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error) {
	var combinedRates map[models.CurrencyCode]iwallet.Amount
	var chainlinkRates map[models.CurrencyCode]iwallet.Amount

	for i, provider := range e.providers {
		rates, err := provider.fetchRates(base)
		if err != nil {
			fmt.Printf("fetch rate failed for provider %T: %v\n", provider, err)
			continue
		}

		// 检查是否是Chainlink provider（第一个provider）
		if i == 0 && len(e.providers) > 0 {
			// 保存Chainlink的数据
			chainlinkRates = rates
			combinedRates = make(map[models.CurrencyCode]iwallet.Amount)
			// 复制Chainlink的所有数据
			for currency, rate := range rates {
				combinedRates[currency] = rate
			}
		} else {
			// 对于其他provider，只添加Chainlink中没有的币种
			if combinedRates == nil {
				combinedRates = make(map[models.CurrencyCode]iwallet.Amount)
			}

			for currency, rate := range rates {
				// 如果Chainlink中没有这个币种，则添加
				if chainlinkRates == nil || chainlinkRates[currency].Int64() == 0 {
					combinedRates[currency] = rate
				}
			}
		}
	}

	if combinedRates == nil {
		return nil, errors.New("all exchange rate providers failed")
	}

	return combinedRates, nil
}

// provider is an interface to a specific exchange rate API.
type provider interface {
	fetchRates(baseCurrency models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error)
}

// openBazaarAPI is an implementation of the provider interface which connects to the openbazaar.org API.
type openBazaarAPI struct {
	url    string
	client *http.Client
}

type apiRate struct {
	Last float64 `json:"last"`
}

type exchangeRateAPIResponse struct {
	Rates map[string]float64 `json:"rates"`
}

// fetchRates returns a rate map for the given base currency as does the conversion from the
// reserve currency as necessary.
func (b *openBazaarAPI) fetchRates(base models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error) {
	_, ok := models.CurrencyDefinitions[ReserveCurrency.String()]
	if !ok {
		return nil, fmt.Errorf("reserve currency %s is not in map", ReserveCurrency.String())
	}

	_, ok = models.CurrencyDefinitions[base.String()]
	if !ok {
		return nil, fmt.Errorf("base currency %s is not in map", base.String())
	}

	// 检查URL是否包含exchangerate-api.com，如果是则使用新的格式
	if strings.Contains(b.url, "exchangerate-api.com") {
		return b.fetchRatesFromExchangeRateAPI(base)
	} else {
		return b.fetchRatesFromLegacyAPI(base)
	}
}

// fetchRatesFromExchangeRateAPI 处理新的汇率API格式
func (b *openBazaarAPI) fetchRatesFromExchangeRateAPI(base models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error) {
	var response exchangeRateAPIResponse
	resp, err := b.client.Get(b.url)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}

	reserveMap := make(map[models.CurrencyCode]*big.Float)
	for cc, rate := range response.Rates {
		_, ok := models.CurrencyDefinitions[cc]
		if !ok {
			continue
		}

		if rate <= 0 {
			continue
		}

		reserveMap[models.CurrencyCode(cc)] = new(big.Float).SetFloat64(rate)
	}
	b.addAdditionalCurrenciesRates(reserveMap)

	if base.String() == ReserveCurrency.String() {
		result := map[models.CurrencyCode]iwallet.Amount{}
		for currency, val := range reserveMap {
			def := models.CurrencyDefinitions[currency.String()]

			divisity := new(big.Float).SetFloat64(math.Pow10(int(def.Divisibility)))
			convertedInt, _ := new(big.Float).Mul(val, divisity).Int(nil)

			result[currency] = iwallet.NewAmount(convertedInt)
		}
		return result, nil
	}

	baseMap := make(map[models.CurrencyCode]iwallet.Amount)

	// 获取基础货币对USD的汇率
	baseFloat, ok := reserveMap[base]
	if !ok {
		return nil, errors.New("base currency not found in API rates")
	}

	// 对于非USD基础货币，我们需要计算其他货币相对于基础货币的汇率
	for currency, rate := range reserveMap {
		// 计算 currency/base 的汇率
		var convertedFloat *big.Float
		if currency.String() == "USD" {
			// 当请求 base/USD 汇率时，我们需要返回 base 对 USD 的价格
			convertedFloat = baseFloat
		} else {
			// 其他货币：currency/base = currency_USD_price / base_USD_price
			convertedFloat = new(big.Float).Quo(rate, baseFloat)
		}

		def := models.CurrencyDefinitions[currency.String()]
		divisity := new(big.Float).SetFloat64(math.Pow10(int(def.Divisibility)))
		convertedInt, _ := new(big.Float).Mul(convertedFloat, divisity).Int(nil)

		baseMap[currency] = iwallet.NewAmount(convertedInt)
	}

	return baseMap, nil
}

// fetchRatesFromLegacyAPI 处理原有的API格式
func (b *openBazaarAPI) fetchRatesFromLegacyAPI(base models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error) {
	rates := make(map[string]apiRate)

	resp, err := b.client.Get(b.url)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&rates); err != nil {
		return nil, err
	}

	reserveMap := make(map[models.CurrencyCode]*big.Float)
	for cc, rate := range rates {
		_, ok := models.CurrencyDefinitions[cc]
		if !ok {
			continue
		}

		if rate.Last <= 0 {
			continue
		}

		reserveMap[models.CurrencyCode(cc)] = new(big.Float).SetFloat64(rate.Last)
	}
	b.addAdditionalCurrenciesRates(reserveMap)

	// 计算汇率：所有货币都相对于USDT
	// 由于API返回的是以某种基础单位表示的价格，我们需要计算相对汇率
	if base.String() == ReserveCurrency.String() {
		result := map[models.CurrencyCode]iwallet.Amount{}

		// 获取USDT的价格作为基准
		usdtPrice, ok := reserveMap[models.CurrencyCode("USDT")]
		if !ok {
			return nil, errors.New("USDT price not found in API rates")
		}

		for currency, price := range reserveMap {
			// 计算 currency/USDT 的汇率
			// 如果 currency 的价格是 price，USDT 的价格是 usdtPrice
			// 那么 currency/USDT = price/usdtPrice
			var rate *big.Float
			if currency.String() == "USDT" {
				// USDT对USDT的汇率应该是1
				rate = new(big.Float).SetFloat64(1.0)
			} else {
				rate = new(big.Float).Quo(price, usdtPrice)
			}

			def := models.CurrencyDefinitions[currency.String()]
			divisity := new(big.Float).SetFloat64(math.Pow10(int(def.Divisibility)))
			convertedInt, _ := new(big.Float).Mul(rate, divisity).Int(nil)

			result[currency] = iwallet.NewAmount(convertedInt)
		}
		return result, nil
	}

	baseMap := make(map[models.CurrencyCode]iwallet.Amount)

	// 获取基础货币的价格
	basePrice, ok := reserveMap[base]
	if !ok {
		return nil, errors.New("base currency not found in API rates")
	}

	// 获取USDT的价格作为基准
	usdtPrice, ok := reserveMap[models.CurrencyCode("USDT")]
	if !ok {
		return nil, errors.New("USDT price not found in API rates")
	}

	// 计算基础货币对USDT的汇率
	baseToUsdtRate := new(big.Float).Quo(basePrice, usdtPrice)

	// 对于非USDT基础货币，我们需要计算其他货币相对于基础货币的汇率
	for currency, price := range reserveMap {
		// 计算 currency/base 的汇率
		// 如果 currency 的价格是 price，base 的价格是 basePrice
		// 那么 currency/base = price/basePrice
		// 但是我们需要转换为 currency/USDT 和 base/USDT 的比率
		currencyToUsdtRate := new(big.Float).Quo(price, usdtPrice)
		convertedFloat := new(big.Float).Quo(currencyToUsdtRate, baseToUsdtRate)

		def := models.CurrencyDefinitions[currency.String()]
		divisity := new(big.Float).SetFloat64(math.Pow10(int(def.Divisibility)))
		convertedInt, _ := new(big.Float).Mul(convertedFloat, divisity).Int(nil)

		baseMap[currency] = iwallet.NewAmount(convertedInt)
	}

	return baseMap, nil
}

func (b *openBazaarAPI) addAdditionalCurrenciesRates(rateMap map[models.CurrencyCode]*big.Float) {
	if rate, ok := rateMap["USDT"]; ok {
		rateMap[models.CurrencyCode(iwallet.CtBEP20USDT.CurrencyCode())] = rate
	}

	if rate, ok := rateMap["USDC"]; ok {
		rateMap[models.CurrencyCode(iwallet.CtBEP20USDC.CurrencyCode())] = rate
	}
}

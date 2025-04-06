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

	"github.com/mobazha/mobazha3.0/libs/proxyclient"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ReserveCurrency is the currency used to calculate the exchange rates
// for all other currencies. In this case it's BTC. If you want to know
// the USD price of BCH we first get the USD price of BTC, then get the
// ratio of BTC/BCH and use it to calculate the BCH USD price.
const ReserveCurrency = models.CurrencyCode("BTC")

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

	client := proxyclient.NewHttpClient()
	client.Timeout = time.Minute

	for _, src := range sources {
		e.providers = append(e.providers, &openBazaarAPI{src, client})
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

// fetchRatesFromProviders queries the exchange rate sources serially until it gets a response back.
func (e *ExchangeRateProvider) fetchRatesFromProviders(base models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error) {
	for _, provider := range e.providers {
		rates, err := provider.fetchRates(base)
		if err == nil {
			return rates, nil
		} else {
			fmt.Printf("fetch rate failed, %v", err)
		}
	}
	return nil, errors.New("all exchange rate providers failed")
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

	reserveFloat := new(big.Float).SetInt64(1)
	baseFloat, ok := reserveMap[base]
	if !ok {
		return nil, errors.New("base currency not found in API rates")
	}
	conversionFloat := new(big.Float).Quo(reserveFloat, baseFloat)

	fmt.Printf("baseFloat:%s, reserveFloat: %s, conversionFloat: %s\n", baseFloat, reserveFloat, conversionFloat)

	for currency, rate := range reserveMap {
		convertedFloat := new(big.Float).Mul(rate, conversionFloat)

		def := models.CurrencyDefinitions[currency.String()]
		divisity := new(big.Float).SetFloat64(math.Pow10(int(def.Divisibility)))
		convertedInt, _ := new(big.Float).Mul(convertedFloat, divisity).Int(nil)

		baseMap[currency] = iwallet.NewAmount(convertedInt)
	}

	return baseMap, nil
}

func (b *openBazaarAPI) addAdditionalCurrenciesRates(rateMap map[models.CurrencyCode]*big.Float) {
	if rate, ok := rateMap["USDT"]; ok {
		rateMap[iwallet.CtBNBUSDT] = rate
		rateMap[iwallet.CtMATICUSDT] = rate
		rateMap[iwallet.CtCFXUSDT] = rate

		rateMap[iwallet.CtBNBMBZ] = rate
		rateMap[iwallet.CtMATICMBZ] = rate
		rateMap[iwallet.CtCFXMBZ] = rate
	}

	if rate, ok := rateMap["USDC"]; ok {
		rateMap[iwallet.CtBNBUSDC] = rate
		rateMap[iwallet.CtMATICUSDC] = rate
		rateMap[iwallet.CtCFXUSDC] = rate
	}
}

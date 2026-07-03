package wallet

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mobazha/mobazha/pkg/assetid"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const (
	// testMockCurrencyCode is kept for internal mock-chain flows.
	testMockCurrencyCode = "MCK"
	testMockCurrencyUSD  = 25.72
)

// usdPeggedCurrencyCodes lists currencies that should be treated as USD-pegged
// by secondary providers (for example Binance USDT quotes).
// Built from PricingMeta in assetid registry.
var usdPeggedCurrencyCodes = buildUSDPeggedCurrencyCodes()

// coinGeckoIDMap maps CoinGecko coin IDs to our internal currency codes.
// It is fully sourced from assetid registry metadata.
var coinGeckoIDMap = buildCoinGeckoIDMap()

func buildCoinGeckoIDMap() map[string][]string {
	byID := make(map[string]map[string]struct{})

	add := func(cgID, code string) {
		id := strings.TrimSpace(strings.ToLower(cgID))
		normalizedCode := strings.TrimSpace(strings.ToUpper(code))
		if id == "" || normalizedCode == "" {
			return
		}
		if _, ok := byID[id]; !ok {
			byID[id] = make(map[string]struct{})
		}
		byID[id][normalizedCode] = struct{}{}
	}

	for _, def := range assetid.DefaultRegistry().List() {
		priceID, ok := def.PriceIDForProvider("coingecko")
		if !ok {
			continue
		}
		add(priceID, def.Code)
	}
	// Keep test currency MCK available in mock exchange-rate flows.
	add("mockcoin", testMockCurrencyCode)

	result := make(map[string][]string, len(byID))
	for cgID, set := range byID {
		codes := make([]string, 0, len(set))
		for code := range set {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		result[cgID] = codes
	}

	return result
}

func buildUSDPeggedCurrencyCodes() map[string]struct{} {
	result := make(map[string]struct{})
	add := func(code string) {
		normalizedCode := strings.TrimSpace(strings.ToUpper(code))
		if normalizedCode == "" {
			return
		}
		result[normalizedCode] = struct{}{}
	}

	for _, def := range assetid.DefaultRegistry().List() {
		if !strings.EqualFold(def.Pricing.PeggedTo, "USD") {
			continue
		}
		add(def.Code)
	}

	return result
}

// coinGeckoVsCurrencies are the fiat currencies to request from CoinGecko.
// Only includes currencies that CoinGecko supports as vs_currencies AND
// that exist in our CurrencyDefinitions.
var coinGeckoVsCurrencies = []string{
	"usd", "aed", "ars", "aud", "bdt", "bhd", "bmd", "brl",
	"cad", "chf", "clp", "cny", "czk", "dkk", "eur", "gbp",
	"hkd", "huf", "idr", "ils", "inr", "jpy", "krw", "kwd",
	"lkr", "mmk", "mxn", "myr", "ngn", "nok", "nzd", "php",
	"pkr", "pln", "rub", "sar", "sek", "sgd", "thb", "try",
	"twd", "uah", "vnd", "zar",
}

// coinGeckoProvider implements the provider interface using CoinGecko API.
// It fetches all crypto prices in a single API call and caches the raw response.
// Cross-rates are derived from the USD-normalized internal data on each fetchRates call.
type coinGeckoProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client

	rawPrices map[string]map[string]float64
	lastFetch time.Time
	cacheTTL  time.Duration
	mu        sync.RWMutex
}

func newCoinGeckoProvider(baseURL, apiKey string, client *http.Client, cacheTTL time.Duration) *coinGeckoProvider {
	if cacheTTL <= 0 {
		cacheTTL = 30 * time.Second
	}
	return &coinGeckoProvider{
		baseURL:   baseURL,
		apiKey:    apiKey,
		client:    client,
		rawPrices: make(map[string]map[string]float64),
		cacheTTL:  cacheTTL,
	}
}

// fetchRates implements the provider interface.
// Returns map of target currency code → amount in smallest units for 1 unit of base.
func (c *coinGeckoProvider) fetchRates(base models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error) {
	if err := c.refreshIfNeeded(); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	usdPrices := c.buildUSDPriceMap()

	baseStr := base.String()
	baseUSD, ok := usdPrices[baseStr]
	if !ok {
		return nil, fmt.Errorf("base currency %s not found in exchange rate data", baseStr)
	}

	result := make(map[models.CurrencyCode]iwallet.Amount)
	for currency, targetUSD := range usdPrices {
		if targetUSD <= 0 {
			continue
		}

		def, exists := models.CurrencyDefinitions[currency]
		if !exists {
			continue
		}

		// Use 256-bit precision to avoid truncation for high-divisibility coins (ETH: 10^18)
		rate := new(big.Float).SetPrec(256).Quo(
			new(big.Float).SetPrec(256).SetFloat64(baseUSD),
			new(big.Float).SetPrec(256).SetFloat64(targetUSD),
		)

		divisibility := new(big.Float).SetPrec(256).SetFloat64(math.Pow10(int(def.Divisibility)))
		convertedInt, _ := new(big.Float).SetPrec(256).Mul(rate, divisibility).Int(nil)

		result[models.CurrencyCode(currency)] = iwallet.NewAmount(convertedInt)
	}

	return result, nil
}

// buildUSDPriceMap returns a map of our CurrencyCode → price in USD.
// For cryptos: sourced directly from CoinGecko data.
// For fiat: derived from BTC cross-rates (btcUSD / btcFiat).
// For USD: always 1.0.
// Must be called with c.mu held for reading.
func (c *coinGeckoProvider) buildUSDPriceMap() map[string]float64 {
	prices := make(map[string]float64)
	prices["USD"] = 1.0

	var btcFiatPrices map[string]float64
	if btcData, ok := c.rawPrices["bitcoin"]; ok {
		btcFiatPrices = btcData
	}

	for cgID, fiatPrices := range c.rawPrices {
		usdPrice, hasUSD := fiatPrices["usd"]
		if !hasUSD || usdPrice <= 0 {
			continue
		}

		currCodes, ok := coinGeckoIDMap[cgID]
		if !ok {
			continue
		}

		for _, code := range currCodes {
			prices[code] = usdPrice
		}
	}

	if btcFiatPrices != nil {
		btcUSD, hasBTCUSD := btcFiatPrices["usd"]
		if hasBTCUSD && btcUSD > 0 {
			for _, fiatCode := range coinGeckoVsCurrencies {
				if fiatCode == "usd" {
					continue
				}
				btcFiat, ok := btcFiatPrices[fiatCode]
				if !ok || btcFiat <= 0 {
					continue
				}
				upperCode := strings.ToUpper(fiatCode)
				if _, exists := prices[upperCode]; !exists {
					prices[upperCode] = btcUSD / btcFiat
				}
			}
		}
	}

	if _, exists := prices[testMockCurrencyCode]; !exists {
		if _, defined := models.CurrencyDefinitions[testMockCurrencyCode]; defined {
			prices[testMockCurrencyCode] = testMockCurrencyUSD
		}
	}

	return prices
}

func (c *coinGeckoProvider) refreshIfNeeded() error {
	c.mu.RLock()
	fresh := len(c.rawPrices) > 0 && time.Since(c.lastFetch) < c.cacheTTL
	c.mu.RUnlock()

	if fresh {
		return nil
	}
	return c.fetchFromCoinGecko()
}

func (c *coinGeckoProvider) fetchFromCoinGecko() error {
	var coinIDs []string
	for id := range coinGeckoIDMap {
		coinIDs = append(coinIDs, id)
	}
	sort.Strings(coinIDs)

	vsCurrencies := strings.Join(coinGeckoVsCurrencies, ",")

	url := fmt.Sprintf("%s/simple/price?ids=%s&vs_currencies=%s",
		c.baseURL,
		strings.Join(coinIDs, ","),
		vsCurrencies,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create CoinGecko request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("x-cg-demo-api-key", c.apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("CoinGecko request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("CoinGecko rate limited (429)")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CoinGecko returned status %d", resp.StatusCode)
	}

	var result map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode CoinGecko response: %w", err)
	}

	if len(result) == 0 {
		return fmt.Errorf("CoinGecko returned empty response")
	}

	c.mu.Lock()
	c.rawPrices = result
	c.lastFetch = time.Now()
	c.mu.Unlock()

	return nil
}

package wallet

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/assetid"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// binanceSymbolMap maps our internal currency codes to Binance USDT trading pair symbols.
// It is fully sourced from pkg/assetid Pricing.Sources(provider=binance).
var binanceSymbolMap = buildBinanceSymbolMap()

func buildBinanceSymbolMap() map[string]string {
	result := make(map[string]string)
	add := func(code, symbol string) {
		normalizedCode := strings.TrimSpace(strings.ToUpper(code))
		normalizedSymbol := strings.TrimSpace(strings.ToUpper(symbol))
		if normalizedCode == "" || normalizedSymbol == "" {
			return
		}
		result[normalizedCode] = normalizedSymbol
	}

	for _, def := range assetid.DefaultRegistry().List() {
		symbol, ok := def.PriceIDForProvider("binance")
		if !ok {
			continue
		}
		add(def.Code, symbol)
	}

	return result
}

// binanceProvider implements the provider interface using the Binance public ticker API.
// Fetches crypto/USDT prices (treated as ≈ USD) in a single HTTP call.
// Designed as a lightweight secondary provider for divergence detection
// against the primary CoinGecko provider.
type binanceProvider struct {
	baseURL  string
	client   *http.Client
	cacheTTL time.Duration

	mu        sync.RWMutex
	usdPrices map[string]float64
	lastFetch time.Time
}

func newBinanceProvider(baseURL string, client *http.Client, cacheTTL time.Duration) *binanceProvider {
	if baseURL == "" {
		baseURL = "https://api.binance.com"
	}
	if cacheTTL <= 0 {
		cacheTTL = 30 * time.Second
	}
	return &binanceProvider{
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		client:    client,
		usdPrices: make(map[string]float64),
		cacheTTL:  cacheTTL,
	}
}

type binanceTickerPrice struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

// fetchRates implements the provider interface.
// Returns map of target currency code → amount in smallest units for 1 unit of base.
func (b *binanceProvider) fetchRates(base models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error) {
	if err := b.refreshIfNeeded(); err != nil {
		return nil, err
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	baseStr := base.String()
	baseUSD, ok := b.usdPrices[baseStr]
	if !ok {
		return nil, fmt.Errorf("base currency %s not found in Binance data", baseStr)
	}

	result := make(map[models.CurrencyCode]iwallet.Amount)
	for currency, targetUSD := range b.usdPrices {
		if targetUSD <= 0 {
			continue
		}

		def, exists := models.CurrencyDefinitions[currency]
		if !exists {
			continue
		}

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

func (b *binanceProvider) refreshIfNeeded() error {
	b.mu.RLock()
	fresh := len(b.usdPrices) > 0 && time.Since(b.lastFetch) < b.cacheTTL
	b.mu.RUnlock()

	if fresh {
		return nil
	}
	return b.fetchFromBinance()
}

func (b *binanceProvider) fetchFromBinance() error {
	symbolSet := make(map[string]struct{})
	for _, sym := range binanceSymbolMap {
		symbolSet[sym] = struct{}{}
	}

	symbolList := make([]string, 0, len(symbolSet))
	for sym := range symbolSet {
		symbolList = append(symbolList, sym)
	}
	sort.Strings(symbolList)

	symbolsJSON, err := json.Marshal(symbolList)
	if err != nil {
		return fmt.Errorf("marshal symbols: %w", err)
	}

	reqURL := fmt.Sprintf("%s/api/v3/ticker/price?symbols=%s",
		b.baseURL, url.QueryEscape(string(symbolsJSON)))

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create Binance request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("Binance request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 418 {
		return fmt.Errorf("Binance rate limited (%d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Binance returned status %d", resp.StatusCode)
	}

	var tickers []binanceTickerPrice
	if err := json.NewDecoder(resp.Body).Decode(&tickers); err != nil {
		return fmt.Errorf("decode Binance response: %w", err)
	}

	symbolToCodes := make(map[string][]string, len(binanceSymbolMap))
	for code, sym := range binanceSymbolMap {
		symbolToCodes[sym] = append(symbolToCodes[sym], code)
	}

	prices := make(map[string]float64)
	prices["USD"] = 1.0

	for _, ticker := range tickers {
		codes, ok := symbolToCodes[ticker.Symbol]
		if !ok || len(codes) == 0 {
			continue
		}
		price, _, err := new(big.Float).SetPrec(256).Parse(ticker.Price, 10)
		if err != nil || price.Sign() <= 0 {
			continue
		}
		priceF64, _ := price.Float64()
		for _, code := range codes {
			prices[code] = priceF64
		}
	}

	// Stablecoins: Binance quotes in USDT, so USD-pegged codes should be
	// treated as $1 even when Binance has no direct ticker for that code.
	for code := range usdPeggedCurrencyCodes {
		if _, exists := prices[code]; !exists {
			prices[code] = 1.0
		}
	}

	cryptoCount := 0
	for code := range prices {
		if code != "USD" {
			cryptoCount++
		}
	}
	if cryptoCount == 0 {
		return fmt.Errorf("Binance returned no valid crypto prices")
	}

	b.mu.Lock()
	b.usdPrices = prices
	b.lastFetch = time.Now()
	b.mu.Unlock()

	return nil
}

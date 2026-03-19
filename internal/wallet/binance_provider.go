package wallet

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// binanceSymbolMap maps our internal currency codes to Binance USDT trading pair symbols.
// Only includes cryptos that Binance actively lists as XXX/USDT pairs.
var binanceSymbolMap = map[string]string{
	"BTC":   "BTCUSDT",
	"ETH":   "ETHUSDT",
	"BCH":   "BCHUSDT",
	"LTC":   "LTCUSDT",
	"ZEC":   "ZECUSDT",
	"EXTERNAL_PAYMENT":   "EXTERNAL_PAYMENTUSDT",
	"SOL":   "SOLUSDT",
	"BNB":   "BNBUSDT",
	"MATIC": "MATICUSDT",
	"DASH":  "DASHUSDT",
	"XRP":   "XRPUSDT",
	"EOS":   "EOSUSDT",
	"XLM":   "XLMUSDT",
	"ADA":   "ADAUSDT",
	"TRX":   "TRXUSDT",
	"LINK":  "LINKUSDT",
	"XTZ":   "XTZUSDT",
	"NEO":   "NEOUSDT",
	"ETC":   "ETCUSDT",
	"CFX":   "CFXUSDT",
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
	var symbolList []string
	for _, sym := range binanceSymbolMap {
		symbolList = append(symbolList, sym)
	}

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

	symbolToCode := make(map[string]string, len(binanceSymbolMap))
	for code, sym := range binanceSymbolMap {
		symbolToCode[sym] = code
	}

	prices := make(map[string]float64)
	prices["USD"] = 1.0

	for _, ticker := range tickers {
		code, ok := symbolToCode[ticker.Symbol]
		if !ok {
			continue
		}
		price, _, err := new(big.Float).SetPrec(256).Parse(ticker.Price, 10)
		if err != nil || price.Sign() <= 0 {
			continue
		}
		priceF64, _ := price.Float64()
		prices[code] = priceF64
	}

	// Stablecoins: Binance USDT pairs price assets in USDT ≈ USD.
	// Assign $1.0 to stablecoins that share the USDT/USDC peg.
	for _, codes := range coinGeckoIDMap {
		if len(codes) <= 1 {
			continue
		}
		baseCode := codes[0]
		if basePrice, ok := prices[baseCode]; ok {
			for _, alias := range codes[1:] {
				if _, exists := prices[alias]; !exists {
					prices[alias] = basePrice
				}
			}
		}
	}

	// Chain-native token aliases (BASE=ETH, BSC=BNB)
	for alias, source := range chainNativeAliases {
		if sp, ok := prices[source]; ok {
			if _, exists := prices[alias]; !exists {
				prices[alias] = sp
			}
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

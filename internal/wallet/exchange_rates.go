package wallet

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/mobazha/mobazha/internal/config"
	"github.com/mobazha/mobazha/libs/proxyclient"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ReserveCurrency is the internal base currency for all exchange rate calculations.
// All rates are normalized to USD internally. To get the price of any crypto or fiat
// currency, we derive it from its USD price.
const ReserveCurrency = models.CurrencyCode("USD")

var _ contracts.ExchangeRateService = (*ExchangeRateProvider)(nil)

// DefaultMaxStaleRateAge bounds stale-while-revalidate for payment-critical
// exchange rates. A temporary provider outage may use a recent known value,
// but an indefinitely old quote must fail closed.
const DefaultMaxStaleRateAge = 15 * time.Minute

// ExchangeRateProvider provides exchange rate data to be used by OpenBazaar.
// It gives the exchange rate from any listed cryptocurrency into any other
// currency.
type ExchangeRateProvider struct {
	cache       map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount
	lastQueried map[models.CurrencyCode]time.Time
	mtx         sync.Mutex
	providers   []provider
	cacheTTL    time.Duration
	maxStaleAge time.Duration

	providerHealth []providerHealth
}

type providerHealth struct {
	name         string
	lastSuccess  time.Time
	lastError    string
	lastErrorAt  time.Time
	successCount int64
	errorCount   int64
}

// NewExchangeRateProvider returns a new ExchangeRateProvider.
// The sources parameter is deprecated and ignored; CoinGecko is the primary data source.
func NewExchangeRateProvider(sources []string) *ExchangeRateProvider {
	cfg := config.GetGlobalExchangeRateConfig()

	e := ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		mtx:         sync.Mutex{},
		cacheTTL:    cfg.GetCacheTTL(),
		maxStaleAge: DefaultMaxStaleRateAge,
	}

	client := proxyclient.NewHttpClient()
	client.Timeout = 15 * time.Second

	if cfg.GetRemoteSaaSURL() != "" {
		rp := NewRemoteProvider(cfg.GetRemoteSaaSURL(), client, cfg.GetCacheTTL())
		e.providers = append(e.providers, rp)
		e.providerHealth = append(e.providerHealth, providerHealth{name: "remote_saas"})
		fmt.Printf("Remote exchange rate provider initialized (SaaS URL: %s)\n", cfg.GetRemoteSaaSURL())
	}

	if cfg.IsCoinGeckoEnabled() {
		cgProvider := newCoinGeckoProvider(
			cfg.GetCoinGeckoBaseURL(),
			cfg.GetCoinGeckoAPIKey(),
			client,
			cfg.GetCacheTTL(),
		)
		e.providers = append(e.providers, cgProvider)
		e.providerHealth = append(e.providerHealth, providerHealth{name: "coingecko"})
	}

	if cfg.IsBinanceEnabled() {
		bp := newBinanceProvider(cfg.GetBinanceBaseURL(), client, cfg.GetCacheTTL())
		e.providers = append(e.providers, bp)
		e.providerHealth = append(e.providerHealth, providerHealth{name: "binance"})
		fmt.Printf("Binance provider initialized (URL: %s)\n", cfg.GetBinanceBaseURL())
	}

	if cfg.IsChainlinkEnabled() {
		chainlinkProvider, err := NewChainlinkProvider(cfg.GetChainlinkRPCURL())
		if err == nil {
			e.providers = append(e.providers, chainlinkProvider)
			e.providerHealth = append(e.providerHealth, providerHealth{name: "chainlink"})
			fmt.Printf("Chainlink provider initialized successfully\n")
		} else {
			fmt.Printf("Failed to initialize Chainlink provider: %v\n", err)
		}
	}

	return &e
}

// NewFixedRateProvider returns an ExchangeRateProvider pre-loaded with the
// supplied rates for a single base currency. Intended for deterministic
// unit/integration tests — no network calls.
func NewFixedRateProvider(base models.CurrencyCode, rates map[models.CurrencyCode]iwallet.Amount) *ExchangeRateProvider {
	cache := map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount{
		base: rates,
	}
	return &ExchangeRateProvider{
		cache:       cache,
		lastQueried: map[models.CurrencyCode]time.Time{base: time.Now().Add(time.Hour)},
		cacheTTL:    time.Hour,
		maxStaleAge: DefaultMaxStaleRateAge,
	}
}

func (e *ExchangeRateProvider) effectiveMaxStaleAge() time.Duration {
	if e.maxStaleAge <= 0 {
		return DefaultMaxStaleRateAge
	}
	return e.maxStaleAge
}

func (e *ExchangeRateProvider) canUseStale(lastUpdated time.Time) bool {
	return !lastUpdated.IsZero() && time.Since(lastUpdated) <= e.effectiveMaxStaleAge()
}

func normalizeBaseForRateQuery(base models.CurrencyCode) models.CurrencyCode {
	rawBase := strings.TrimSpace(string(base))
	if rawBase == "" {
		return models.CurrencyCode("")
	}

	if pricingCode, err := iwallet.CoinType(rawBase).PricingCurrencyCode(); err == nil {
		return models.CurrencyCode(strings.ToUpper(strings.TrimSpace(pricingCode)))
	}

	return models.CurrencyCode(strings.ToUpper(rawBase))
}

// GetRate returns the rate for a given currency converting from the provided base currency.
// Uses stale-while-revalidate: if providers fail, returns the last known rate from cache.
func (e *ExchangeRateProvider) GetRate(base models.CurrencyCode, to models.CurrencyCode, breakCache bool) (iwallet.Amount, error) {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	baseForQuery := normalizeBaseForRateQuery(base)
	to = models.CurrencyCode(strings.ToUpper(strings.TrimSpace(string(to))))

	lastQueried := e.lastQueried[baseForQuery]
	cachedRates, hasCached := e.cache[baseForQuery]

	if breakCache || !hasCached || lastQueried.Add(e.cacheTTL).Before(time.Now()) {
		freshRates, err := e.fetchRatesFromProviders(baseForQuery)
		if err != nil {
			if hasCached && e.canUseStale(lastQueried) {
				staleness := time.Since(lastQueried)
				fmt.Printf("exchange rate provider failed, using stale cache (age %s) for %s: %v\n", staleness.Round(time.Second), baseForQuery, err)
				amount, ok := cachedRates[to]
				if !ok {
					return amount, errors.New("rate not found")
				}
				return amount, nil
			}
			return iwallet.NewAmount(0), err
		}
		e.cache[baseForQuery] = freshRates
		e.lastQueried[baseForQuery] = time.Now()
		cachedRates = freshRates
	}
	amount, ok := cachedRates[to]
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
// Uses stale-while-revalidate: if providers fail, returns the last known rates from cache.
func (e *ExchangeRateProvider) GetAllRates(base models.CurrencyCode, breakCache bool) (map[models.CurrencyCode]iwallet.Amount, error) {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	baseForQuery := normalizeBaseForRateQuery(base)

	lastQueried := e.lastQueried[baseForQuery]
	cachedRates, hasCached := e.cache[baseForQuery]

	if breakCache || !hasCached || lastQueried.Add(e.cacheTTL).Before(time.Now()) {
		freshRates, err := e.fetchRatesFromProviders(baseForQuery)
		if err != nil {
			if hasCached && e.canUseStale(lastQueried) {
				staleness := time.Since(lastQueried)
				fmt.Printf("exchange rate provider failed, using stale cache (age %s) for %s: %v\n", staleness.Round(time.Second), baseForQuery, err)
				return cachedRates, nil
			}
			return nil, err
		}
		e.cache[baseForQuery] = freshRates
		e.lastQueried[baseForQuery] = time.Now()
		cachedRates = freshRates
	}
	return cachedRates, nil
}

// LastUpdated returns the last successful provider refresh time for a base
// currency. Callers use it to preserve source freshness when forwarding a
// rate snapshot to another runtime.
func (e *ExchangeRateProvider) LastUpdated(base models.CurrencyCode) time.Time {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	return e.lastQueried[normalizeBaseForRateQuery(base)]
}

// DivergenceThreshold is the maximum acceptable percentage difference between
// primary and secondary exchange rate providers before a warning is logged.
const DivergenceThreshold = 0.05 // 5%

// fetchRatesFromProviders queries the exchange rate sources serially until it gets a response back.
// The first provider (CoinGecko) is treated as primary; subsequent providers fill in
// currencies that the primary didn't cover. When both primary and secondary providers
// return rates for the same currency, a divergence check (>5%) is performed.
func (e *ExchangeRateProvider) fetchRatesFromProviders(base models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error) {
	var combinedRates map[models.CurrencyCode]iwallet.Amount
	var primaryRates map[models.CurrencyCode]iwallet.Amount

	for i, provider := range e.providers {
		rates, err := provider.fetchRates(base)
		if err != nil {
			fmt.Printf("fetch rate failed for provider %T: %v\n", provider, err)
			if i < len(e.providerHealth) {
				e.providerHealth[i].lastError = err.Error()
				e.providerHealth[i].lastErrorAt = time.Now()
				e.providerHealth[i].errorCount++
			}
			continue
		}
		if i < len(e.providerHealth) {
			e.providerHealth[i].lastSuccess = time.Now()
			e.providerHealth[i].successCount++
		}

		if i == 0 {
			primaryRates = rates
			combinedRates = make(map[models.CurrencyCode]iwallet.Amount)
			for currency, rate := range rates {
				combinedRates[currency] = rate
			}
		} else {
			if combinedRates == nil {
				combinedRates = make(map[models.CurrencyCode]iwallet.Amount)
			}
			if primaryRates != nil {
				checkRateDivergence(base, primaryRates, rates, provider)
			}
			for currency, rate := range rates {
				if primaryRates == nil || primaryRates[currency].Int64() == 0 {
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

// checkRateDivergence compares overlapping currency rates between primary and secondary
// providers. Logs a warning for each currency where the divergence exceeds DivergenceThreshold.
func checkRateDivergence(base models.CurrencyCode, primary, secondary map[models.CurrencyCode]iwallet.Amount, secondaryProvider provider) {
	for currency, primaryRate := range primary {
		secondaryRate, ok := secondary[currency]
		if !ok {
			continue
		}
		pVal := primaryRate.Int64()
		sVal := secondaryRate.Int64()
		if pVal == 0 || sVal == 0 {
			continue
		}
		diff := math.Abs(float64(pVal - sVal))
		pct := diff / float64(pVal)
		if pct > DivergenceThreshold {
			fmt.Printf("WARN: exchange rate divergence %s/%s: primary=%d secondary(%T)=%d (%.1f%%)\n",
				base, currency, pVal, secondaryProvider, sVal, pct*100)
		}
	}
}

// GetProviderHealth returns health metrics for all configured providers.
func (e *ExchangeRateProvider) GetProviderHealth() []contracts.ProviderHealthInfo {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	result := make([]contracts.ProviderHealthInfo, len(e.providerHealth))
	for i, h := range e.providerHealth {
		result[i] = contracts.ProviderHealthInfo{
			Name:         h.name,
			LastSuccess:  h.lastSuccess,
			LastError:    h.lastError,
			LastErrorAt:  h.lastErrorAt,
			SuccessCount: h.successCount,
			ErrorCount:   h.errorCount,
		}
	}
	return result
}

// provider is an interface to a specific exchange rate API.
type provider interface {
	fetchRates(baseCurrency models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error)
}

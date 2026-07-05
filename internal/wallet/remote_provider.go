package wallet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const remoteRateFetchTimeout = 10 * time.Second

// remoteExchangeRateResponse mirrors the SaaS endpoint response structure:
// {"data": {"rates": {"BTC": "65000", ...}, "reserve": "USD", "updatedAt": "..."}}
type remoteExchangeRateResponse struct {
	Data struct {
		Rates     map[string]string `json:"rates"`
		Reserve   string            `json:"reserve"`
		UpdatedAt string            `json:"updatedAt"`
	} `json:"data"`
}

// remoteProvider implements the provider interface by fetching pre-computed
// exchange rates from the SaaS platform. Used by standalone stores in the
// Hub-and-Spoke distribution model so they need no CoinGecko API key.
//
// Pattern follows internal/net/cert_fetcher.go (FetchCasdoorCertificate).
type remoteProvider struct {
	saasURL  string
	client   *http.Client
	cacheTTL time.Duration

	mu            sync.RWMutex
	cached        map[models.CurrencyCode]iwallet.Amount
	cachedReserve models.CurrencyCode
	lastFetch     time.Time
}

// NewRemoteProvider creates a provider that fetches rates from a SaaS endpoint.
// saasURL must be an HTTPS URL (or HTTP for localhost/testing).
func NewRemoteProvider(saasURL string, client *http.Client, cacheTTL time.Duration) *remoteProvider {
	if client == nil {
		client = &http.Client{Timeout: remoteRateFetchTimeout}
	}
	if cacheTTL <= 0 {
		cacheTTL = 30 * time.Second
	}
	saasURL = strings.TrimSuffix(saasURL, "/")
	return &remoteProvider{
		saasURL:  saasURL,
		client:   client,
		cacheTTL: cacheTTL,
	}
}

// fetchRates implements the provider interface.
// It returns cached rates if still fresh, otherwise fetches from SaaS.
// On fetch failure, returns stale cache (stale-while-revalidate).
func (p *remoteProvider) fetchRates(base models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error) {
	p.mu.RLock()
	fresh := p.cached != nil && time.Since(p.lastFetch) < p.cacheTTL
	stale := p.cached
	staleReserve := p.cachedReserve
	p.mu.RUnlock()

	if fresh {
		return rebaseRemoteRates(stale, staleReserve, base)
	}

	rates, reserve, err := p.fetchFromSaaS()
	if err != nil {
		if stale != nil {
			staleness := time.Since(p.lastFetch)
			fmt.Printf("remote exchange rate fetch failed, using stale cache (age %s): %v\n",
				staleness.Round(time.Second), err)
			return rebaseRemoteRates(stale, staleReserve, base)
		}
		return nil, fmt.Errorf("remote exchange rate fetch failed (no cache): %w", err)
	}

	p.mu.Lock()
	p.cached = rates
	p.cachedReserve = reserve
	p.lastFetch = time.Now()
	p.mu.Unlock()

	return rebaseRemoteRates(rates, reserve, base)
}

func (p *remoteProvider) fetchFromSaaS() (map[models.CurrencyCode]iwallet.Amount, models.CurrencyCode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), remoteRateFetchTimeout)
	defer cancel()

	u, err := url.Parse(p.saasURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid saas URL %q: %w", p.saasURL, err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, "", fmt.Errorf("saas URL must use http(s) scheme, got %q", u.Scheme)
	}
	u.Path = u.Path + "/platform/v1/exchange-rates"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch exchange rates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var result remoteExchangeRateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("decode response: %w", err)
	}

	if len(result.Data.Rates) == 0 {
		return nil, "", fmt.Errorf("empty rates in response")
	}
	reserve := models.CurrencyCode(strings.ToUpper(strings.TrimSpace(result.Data.Reserve)))
	if reserve == "" {
		return nil, "", fmt.Errorf("empty reserve currency in response")
	}

	rates := make(map[models.CurrencyCode]iwallet.Amount, len(result.Data.Rates))
	for code, amountStr := range result.Data.Rates {
		val, ok := new(big.Int).SetString(amountStr, 10)
		if !ok || val.Sign() < 0 {
			fmt.Printf("remote provider: skipping invalid rate for %s: %q\n", code, amountStr)
			continue
		}
		rates[models.CurrencyCode(strings.ToUpper(strings.TrimSpace(code)))] = iwallet.NewAmount(val)
	}

	if len(rates) == 0 {
		return nil, "", fmt.Errorf("no valid rates parsed from response")
	}

	return rates, reserve, nil
}

// rebaseRemoteRates converts the SaaS reserve-currency matrix into the base
// requested by ExchangeRateProvider. The platform endpoint deliberately sends
// one USD-reserve snapshot; provider.fetchRates, however, must return a matrix
// for its base argument. Returning the reserve snapshot unchanged makes
// ETH/USD look like USD/USD and can overprice fixed USD fees by several orders
// of magnitude.
func rebaseRemoteRates(
	rates map[models.CurrencyCode]iwallet.Amount,
	reserve models.CurrencyCode,
	base models.CurrencyCode,
) (map[models.CurrencyCode]iwallet.Amount, error) {
	reserve = models.CurrencyCode(strings.ToUpper(strings.TrimSpace(string(reserve))))
	base = models.CurrencyCode(strings.ToUpper(strings.TrimSpace(string(base))))
	if reserve == "" || base == "" {
		return nil, fmt.Errorf("remote exchange rates require reserve and base currencies")
	}
	if base == reserve {
		return cloneRemoteRates(rates), nil
	}

	baseRate, ok := rates[base]
	if !ok {
		return nil, fmt.Errorf("remote exchange rate base %s is missing from %s snapshot", base, reserve)
	}
	baseRateInt := new(big.Int).Set((*big.Int)(&baseRate))
	if baseRateInt.Sign() <= 0 {
		return nil, fmt.Errorf("remote exchange rate base %s must be positive", base)
	}
	baseCurrency, err := models.CurrencyDefinitions.Lookup(string(base))
	if err != nil {
		return nil, fmt.Errorf("remote exchange rate base %s: %w", base, err)
	}
	baseScale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(baseCurrency.Divisibility)), nil)

	rebased := make(map[models.CurrencyCode]iwallet.Amount, len(rates))
	for code, rate := range rates {
		rateInt := new(big.Int).Set((*big.Int)(&rate))
		if rateInt.Sign() < 0 {
			continue
		}
		rateInt.Mul(rateInt, baseScale)
		rateInt.Quo(rateInt, baseRateInt)
		rebased[code] = iwallet.NewAmount(rateInt)
	}
	return rebased, nil
}

func cloneRemoteRates(rates map[models.CurrencyCode]iwallet.Amount) map[models.CurrencyCode]iwallet.Amount {
	cloned := make(map[models.CurrencyCode]iwallet.Amount, len(rates))
	for code, rate := range rates {
		cloned[code] = iwallet.NewAmount(new(big.Int).Set((*big.Int)(&rate)))
	}
	return cloned
}

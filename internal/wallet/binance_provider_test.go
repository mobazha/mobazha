package wallet

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestBinanceProvider_FetchRates_BTC(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tickers := []binanceTickerPrice{
			{Symbol: "BTCUSDT", Price: "65000.00000000"},
			{Symbol: "ETHUSDT", Price: "3500.00000000"},
			{Symbol: "SOLUSDT", Price: "150.00000000"},
			{Symbol: "BNBUSDT", Price: "600.00000000"},
			{Symbol: "LTCUSDT", Price: "80.00000000"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tickers)
	}))
	defer server.Close()

	p := newBinanceProvider(server.URL, server.Client(), 30*time.Second)
	rates, err := p.fetchRates(models.CurrencyCode("BTC"))
	if err != nil {
		t.Fatalf("fetchRates BTC failed: %v", err)
	}

	if len(rates) == 0 {
		t.Fatal("no rates returned")
	}

	// BTC→BTC should be 1 BTC = 10^8 satoshis
	btcRate, ok := rates[models.CurrencyCode("BTC")]
	if !ok {
		t.Fatal("BTC rate missing")
	}
	expected := big.NewInt(100000000)
	if btcRate.String() != expected.String() {
		t.Errorf("BTC→BTC rate: want %s, got %s", expected, btcRate)
	}

	// BTC→ETH: 65000/3500 * 10^18 ≈ 18571428571428571428
	ethRate, ok := rates[models.CurrencyCode("ETH")]
	if !ok {
		t.Fatal("ETH rate missing")
	}
	if ethRate.Cmp(iwallet.NewAmount(0)) <= 0 {
		t.Errorf("ETH rate should be positive, got %s", ethRate)
	}
	t.Logf("BTC→ETH rate: %s (wei)", ethRate)

	// BTC→SOL: 65000/150 * 10^9
	solRate, ok := rates[models.CurrencyCode("SOL")]
	if !ok {
		t.Fatal("SOL rate missing")
	}
	if solRate.Cmp(iwallet.NewAmount(0)) <= 0 {
		t.Errorf("SOL rate should be positive, got %s", solRate)
	}
	t.Logf("BTC→SOL rate: %s", solRate)
}

func TestBinanceProvider_FetchRates_ETH(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tickers := []binanceTickerPrice{
			{Symbol: "BTCUSDT", Price: "65000.00"},
			{Symbol: "ETHUSDT", Price: "3500.00"},
			{Symbol: "SOLUSDT", Price: "150.00"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tickers)
	}))
	defer server.Close()

	p := newBinanceProvider(server.URL, server.Client(), 30*time.Second)
	rates, err := p.fetchRates(models.CurrencyCode("ETH"))
	if err != nil {
		t.Fatalf("fetchRates ETH failed: %v", err)
	}

	// ETH→ETH should be 10^18 wei
	ethRate, ok := rates[models.CurrencyCode("ETH")]
	if !ok {
		t.Fatal("ETH rate missing")
	}
	expected := new(big.Int).SetUint64(1000000000000000000)
	if ethRate.String() != iwallet.NewAmount(expected).String() {
		t.Errorf("ETH→ETH rate: want %s, got %s", expected, ethRate)
	}
}

func TestBinanceProvider_ChainNativeAliases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tickers := []binanceTickerPrice{
			{Symbol: "BTCUSDT", Price: "65000.00"},
			{Symbol: "ETHUSDT", Price: "3500.00"},
			{Symbol: "BNBUSDT", Price: "600.00"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tickers)
	}))
	defer server.Close()

	p := newBinanceProvider(server.URL, server.Client(), 30*time.Second)
	rates, err := p.fetchRates(models.CurrencyCode("BTC"))
	if err != nil {
		t.Fatalf("fetchRates failed: %v", err)
	}

	// BASE should have ETH's price (via chainNativeAliases)
	baseRate, ok := rates[models.CurrencyCode("BASE")]
	if !ok {
		t.Skip("BASE not in CurrencyDefinitions, skipping alias check")
	}
	ethRate := rates[models.CurrencyCode("ETH")]
	if baseRate.String() != ethRate.String() {
		t.Errorf("BASE rate (%s) should equal ETH rate (%s)", baseRate, ethRate)
	}
}

func TestBinanceProvider_CacheHit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		tickers := []binanceTickerPrice{
			{Symbol: "BTCUSDT", Price: "65000.00"},
			{Symbol: "ETHUSDT", Price: "3500.00"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tickers)
	}))
	defer server.Close()

	p := newBinanceProvider(server.URL, server.Client(), 5*time.Second)

	_, err := p.fetchRates(models.CurrencyCode("BTC"))
	if err != nil {
		t.Fatalf("first fetchRates failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	_, err = p.fetchRates(models.CurrencyCode("BTC"))
	if err != nil {
		t.Fatalf("second fetchRates failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("cache should prevent second call, got %d calls", callCount)
	}
}

func TestBinanceProvider_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	p := newBinanceProvider(server.URL, server.Client(), 30*time.Second)
	_, err := p.fetchRates(models.CurrencyCode("BTC"))
	if err == nil {
		t.Fatal("expected error on rate limit")
	}
	t.Logf("rate limit error: %v", err)
}

func TestBinanceProvider_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]binanceTickerPrice{})
	}))
	defer server.Close()

	p := newBinanceProvider(server.URL, server.Client(), 30*time.Second)
	_, err := p.fetchRates(models.CurrencyCode("BTC"))
	if err == nil {
		t.Fatal("expected error on empty response")
	}
}

func TestBinanceProvider_InvalidBase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tickers := []binanceTickerPrice{
			{Symbol: "BTCUSDT", Price: "65000.00"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tickers)
	}))
	defer server.Close()

	p := newBinanceProvider(server.URL, server.Client(), 30*time.Second)
	_, err := p.fetchRates(models.CurrencyCode("INVALID"))
	if err == nil {
		t.Fatal("expected error for invalid base currency")
	}
}

func TestBinanceProvider_InvalidPriceSkipped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tickers := []binanceTickerPrice{
			{Symbol: "BTCUSDT", Price: "65000.00"},
			{Symbol: "ETHUSDT", Price: "invalid_price"},
			{Symbol: "SOLUSDT", Price: "-100"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tickers)
	}))
	defer server.Close()

	p := newBinanceProvider(server.URL, server.Client(), 30*time.Second)
	rates, err := p.fetchRates(models.CurrencyCode("BTC"))
	if err != nil {
		t.Fatalf("fetchRates failed: %v", err)
	}

	if _, ok := rates[models.CurrencyCode("ETH")]; ok {
		t.Error("ETH with invalid price should be skipped")
	}
	if _, ok := rates[models.CurrencyCode("SOL")]; ok {
		t.Error("SOL with negative price should be skipped")
	}
}

// TestBinanceProviderIntegration tests against the real Binance API.
// Skipped in CI or when network is unavailable.
func TestBinanceProviderIntegration(t *testing.T) {
	client := &http.Client{Timeout: 15 * time.Second}

	p := newBinanceProvider("https://api.binance.com", client, 30*time.Second)
	rates, err := p.fetchRates(models.CurrencyCode("BTC"))
	if err != nil {
		t.Skipf("Binance API unavailable, skipping integration test: %v", err)
	}

	if len(rates) == 0 {
		t.Fatal("no rates returned from Binance")
	}

	requiredCurrencies := []string{"ETH", "SOL", "BNB", "LTC"}
	for _, c := range requiredCurrencies {
		rate, ok := rates[models.CurrencyCode(c)]
		if !ok {
			t.Errorf("missing rate for %s", c)
			continue
		}
		if rate.Cmp(iwallet.NewAmount(0)) <= 0 {
			t.Errorf("%s rate should be positive, got %s", c, rate)
		}
		t.Logf("BTC→%s rate: %s", c, rate)
	}

	btcRate, ok := rates[models.CurrencyCode("BTC")]
	if !ok {
		t.Fatal("BTC self-rate missing")
	}
	expected := big.NewInt(100000000)
	if btcRate.String() != expected.String() {
		t.Errorf("BTC→BTC: want %s, got %s", expected, btcRate)
	}
}

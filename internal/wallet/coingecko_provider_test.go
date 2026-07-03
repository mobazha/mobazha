package wallet

import (
	"math"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/mobazha/mobazha/pkg/models"
)

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func newTestCoinGeckoProvider(client *http.Client) *coinGeckoProvider {
	return newCoinGeckoProvider(
		"https://api.coingecko.com/api/v3",
		"",
		client,
		30*time.Second,
	)
}

func setupMockCoinGecko(t *testing.T) (*http.Client, func()) {
	t.Helper()
	client := &http.Client{}
	httpmock.ActivateNonDefault(client)

	httpmock.RegisterResponder(http.MethodGet, `=~^https://api\.coingecko\.com/api/v3/simple/price`,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(http.StatusOK, MockCoinGeckoResponse)
		},
	)

	return client, func() { httpmock.DeactivateAndReset() }
}

func TestCoinGeckoProvider_FetchRates_BTCBase(t *testing.T) {
	client, cleanup := setupMockCoinGecko(t)
	defer cleanup()

	p := newTestCoinGeckoProvider(client)
	rates, err := p.fetchRates("BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	usdRate, ok := rates["USD"]
	if !ok {
		t.Fatal("USD rate not found")
	}

	// 1 BTC = 65000 USD. USD divisibility=2, so 65000 * 100 = 6500000
	expected := int64(6500000)
	if usdRate.Int64() != expected {
		t.Errorf("expected USD rate %d, got %d", expected, usdRate.Int64())
	}

	ethRate, ok := rates["ETH"]
	if !ok {
		t.Fatal("ETH rate not found")
	}

	// 1 BTC ≈ 18.571 ETH. ETH divisibility=18 → value exceeds int64 range.
	// Use big.Int comparison: expected ≈ 18571428571428571428 (> MaxInt64)
	expectedETH := new(big.Float).SetPrec(256).Mul(
		new(big.Float).SetPrec(256).Quo(
			new(big.Float).SetPrec(256).SetFloat64(65000),
			new(big.Float).SetPrec(256).SetFloat64(3500),
		),
		new(big.Float).SetPrec(256).SetFloat64(math.Pow10(18)),
	)
	expectedInt, _ := expectedETH.Int(nil)

	ethBig := big.Int(ethRate)
	diff := new(big.Int).Sub(&ethBig, expectedInt)
	diff.Abs(diff)

	// Allow 0.1% tolerance
	toleranceBig, _ := new(big.Float).SetPrec(256).Mul(
		new(big.Float).SetPrec(256).SetInt(expectedInt),
		new(big.Float).SetPrec(256).SetFloat64(0.001),
	).Int(nil)

	if diff.Cmp(toleranceBig) > 0 {
		t.Errorf("ETH rate outside tolerance: got %s, expected ~%s", ethBig.String(), expectedInt.String())
	}
}

func TestCoinGeckoProvider_FetchRates_USDBase(t *testing.T) {
	client, cleanup := setupMockCoinGecko(t)
	defer cleanup()

	p := newTestCoinGeckoProvider(client)
	rates, err := p.fetchRates("USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	btcRate, ok := rates["BTC"]
	if !ok {
		t.Fatal("BTC rate not found")
	}

	// 1 USD = 1/65000 BTC. BTC divisibility=8, so (1/65000)*10^8 ≈ 1538
	expected := int64(1538)
	if btcRate.Int64() != expected {
		t.Errorf("expected BTC rate %d, got %d", expected, btcRate.Int64())
	}

	// 1 USD = 1 USD
	usdRate, ok := rates["USD"]
	if !ok {
		t.Fatal("USD rate not found")
	}
	// USD divisibility=2, 1.0 * 100 = 100
	if usdRate.Int64() != 100 {
		t.Errorf("expected USD self-rate 100, got %d", usdRate.Int64())
	}
}

func TestCoinGeckoProvider_FetchRates_ETHBase(t *testing.T) {
	client, cleanup := setupMockCoinGecko(t)
	defer cleanup()

	p := newTestCoinGeckoProvider(client)
	rates, err := p.fetchRates("ETH")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	usdRate, ok := rates["USD"]
	if !ok {
		t.Fatal("USD rate not found")
	}

	// 1 ETH = 3500 USD. USD divisibility=2 → 3500 * 100 = 350000
	expected := int64(350000)
	if usdRate.Int64() != expected {
		t.Errorf("expected USD rate %d, got %d", expected, usdRate.Int64())
	}
}

func TestCoinGeckoProvider_FetchRates_FiatBase(t *testing.T) {
	client, cleanup := setupMockCoinGecko(t)
	defer cleanup()

	p := newTestCoinGeckoProvider(client)
	rates, err := p.fetchRates("EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	usdRate, ok := rates["USD"]
	if !ok {
		t.Fatal("USD rate not found")
	}

	// EUR_USD derived from BTC: btcUSD / btcEUR = 65000 / 60000 ≈ 1.0833
	// 1 EUR = 1.0833 USD → in cents: ~108
	if usdRate.Int64() < 105 || usdRate.Int64() > 112 {
		t.Errorf("EUR→USD rate outside expected range: got %d", usdRate.Int64())
	}
}

func TestCoinGeckoProvider_StablecoinMapping(t *testing.T) {
	client, cleanup := setupMockCoinGecko(t)
	defer cleanup()

	p := newTestCoinGeckoProvider(client)
	rates, err := p.fetchRates("USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, code := range []string{"ETHUSDT", "BSCUSDT", "BASEUSDT", "MATICUSDT", "SOLUSDT", "TRXUSDT"} {
		rate, ok := rates[models.CurrencyCode(code)]
		if !ok {
			t.Errorf("%s rate not found", code)
			continue
		}
		def, err := models.CurrencyDefinitions.Lookup(code)
		if err != nil {
			t.Fatalf("lookup %s: %v", code, err)
		}
		// 1 USD pegged stablecoin: rate=1.0 in smallest units = 10^divisibility.
		expected := int64(math.Pow10(int(def.Divisibility)))
		if rate.Int64() != expected {
			t.Errorf("%s expected rate %d, got %d", code, expected, rate.Int64())
		}
	}

	if _, ok := rates[models.CurrencyCode("USDT")]; ok {
		t.Fatal("legacy USDT alias should not be present in catalog-driven rates")
	}
}

func TestCoinGeckoProvider_CatalogDrivenMapping(t *testing.T) {
	tetherCodes, ok := coinGeckoIDMap["tether"]
	if !ok {
		t.Fatal("tether mapping not found")
	}
	for _, code := range []string{"ETHUSDT", "BSCUSDT", "BASEUSDT", "MATICUSDT", "SOLUSDT", "TRXUSDT"} {
		if !containsString(tetherCodes, code) {
			t.Fatalf("expected tether mapping to include %s", code)
		}
	}
	if containsString(tetherCodes, "USDT") {
		t.Fatal("legacy USDT alias should not be present in tether mapping")
	}

	usdCoinCodes, ok := coinGeckoIDMap["usd-coin"]
	if !ok {
		t.Fatal("usd-coin mapping not found")
	}
	for _, code := range []string{"ETHUSDC", "BSCUSDC", "BASEUSDC", "MATICUSDC", "SOLUSDC"} {
		if !containsString(usdCoinCodes, code) {
			t.Fatalf("expected usd-coin mapping to include %s", code)
		}
	}
	if containsString(usdCoinCodes, "USDC") {
		t.Fatal("legacy USDC alias should not be present in usd-coin mapping")
	}

	bitcoinCashCodes, ok := coinGeckoIDMap["bitcoin-cash"]
	if !ok {
		t.Fatal("bitcoin-cash mapping not found")
	}
	if !containsString(bitcoinCashCodes, "BCH") {
		t.Fatal("expected bitcoin-cash mapping to include BCH")
	}

	zcashCodes, ok := coinGeckoIDMap["zcash"]
	if !ok {
		t.Fatal("zcash mapping not found")
	}
	if !containsString(zcashCodes, "ZEC") {
		t.Fatal("expected zcash mapping to include ZEC")
	}
	confluxCodes, ok := coinGeckoIDMap["conflux-token"]
	if !ok {
		t.Fatal("conflux-token mapping not found")
	}
	if !containsString(confluxCodes, "CFX") {
		t.Fatal("expected conflux-token mapping to include CFX")
	}
}

func TestCoinGeckoProvider_Cache(t *testing.T) {
	client, cleanup := setupMockCoinGecko(t)
	defer cleanup()

	p := newCoinGeckoProvider("https://api.coingecko.com/api/v3", "", client, 1*time.Hour)

	_, err := p.fetchRates("BTC")
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}

	callCount := httpmock.GetTotalCallCount()
	if callCount != 1 {
		t.Fatalf("expected 1 API call, got %d", callCount)
	}

	// Second call should use cache
	_, err = p.fetchRates("ETH")
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}

	if httpmock.GetTotalCallCount() != 1 {
		t.Errorf("expected cache hit (still 1 call), got %d", httpmock.GetTotalCallCount())
	}
}

func TestCoinGeckoProvider_CacheExpiry(t *testing.T) {
	client, cleanup := setupMockCoinGecko(t)
	defer cleanup()

	p := newCoinGeckoProvider("https://api.coingecko.com/api/v3", "", client, 50*time.Millisecond)

	_, err := p.fetchRates("BTC")
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	_, err = p.fetchRates("BTC")
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}

	if httpmock.GetTotalCallCount() != 2 {
		t.Errorf("expected 2 API calls after cache expiry, got %d", httpmock.GetTotalCallCount())
	}
}

func TestCoinGeckoProvider_UnknownBaseCurrency(t *testing.T) {
	client, cleanup := setupMockCoinGecko(t)
	defer cleanup()

	p := newTestCoinGeckoProvider(client)
	_, err := p.fetchRates("UNKNOWN")
	if err == nil {
		t.Fatal("expected error for unknown base currency")
	}
}

func TestCoinGeckoProvider_APIError(t *testing.T) {
	client := &http.Client{}
	httpmock.ActivateNonDefault(client)
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet, `=~^https://api\.coingecko\.com`,
		httpmock.NewStringResponder(500, "internal server error"),
	)

	p := newTestCoinGeckoProvider(client)
	_, err := p.fetchRates("BTC")
	if err == nil {
		t.Fatal("expected error on API failure")
	}
}

func TestCoinGeckoProvider_RateLimited(t *testing.T) {
	client := &http.Client{}
	httpmock.ActivateNonDefault(client)
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet, `=~^https://api\.coingecko\.com`,
		httpmock.NewStringResponder(429, "rate limited"),
	)

	p := newTestCoinGeckoProvider(client)
	_, err := p.fetchRates("BTC")
	if err == nil {
		t.Fatal("expected error on rate limit")
	}
}

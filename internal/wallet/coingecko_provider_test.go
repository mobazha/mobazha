package wallet

import (
	"math"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

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

	// USDT should be available (mapped from "tether")
	usdtRate, ok := rates["USDT"]
	if !ok {
		t.Fatal("USDT rate not found")
	}

	// 1 USD = 1/1.0 USDT. USDT divisibility=6 → 1 * 10^6 = 1000000
	if usdtRate.Int64() != 1000000 {
		t.Errorf("expected USDT rate 1000000, got %d", usdtRate.Int64())
	}

	// Chain-specific USDT variants should also work
	for _, code := range []string{"ETHUSDT", "BSCUSDT", "BASEUSDT", "SOLUSDT"} {
		if _, ok := rates[models.CurrencyCode(code)]; !ok {
			t.Errorf("%s rate not found", code)
		}
	}
}

func TestCoinGeckoProvider_ChainNativeAliases(t *testing.T) {
	client, cleanup := setupMockCoinGecko(t)
	defer cleanup()

	p := newTestCoinGeckoProvider(client)
	rates, err := p.fetchRates("USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// BASE should have same price as ETH
	baseRate, hasBase := rates["BASE"]
	ethRate, hasETH := rates["ETH"]
	if !hasBase {
		t.Fatal("BASE rate not found")
	}
	if !hasETH {
		t.Fatal("ETH rate not found")
	}
	if baseRate.Int64() != ethRate.Int64() {
		t.Errorf("BASE rate (%d) should equal ETH rate (%d)", baseRate.Int64(), ethRate.Int64())
	}

	// BSC should have same price as BNB
	bscRate, hasBSC := rates["BSC"]
	bnbRate, hasBNB := rates["BNB"]
	if !hasBSC {
		t.Fatal("BSC rate not found")
	}
	if !hasBNB {
		t.Fatal("BNB rate not found")
	}
	if bscRate.Int64() != bnbRate.Int64() {
		t.Errorf("BSC rate (%d) should equal BNB rate (%d)", bscRate.Int64(), bnbRate.Int64())
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

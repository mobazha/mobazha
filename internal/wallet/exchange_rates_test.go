package wallet

import (
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestReserveCurrencyIsUSD(t *testing.T) {
	if ReserveCurrency.String() != "USD" {
		t.Fatalf("expected ReserveCurrency to be USD, got %s", ReserveCurrency)
	}
}

func TestNewExchangeRateProvider(t *testing.T) {
	cfg := config.GetGlobalExchangeRateConfig()
	cfg.SetCoinGeckoEnabled(true)
	cfg.SetChainlinkEnabled(false)

	provider := NewExchangeRateProvider(nil)

	if provider == nil {
		t.Fatal("ExchangeRateProvider should not be nil")
	}

	if len(provider.providers) == 0 {
		t.Fatal("Should have at least one provider")
	}

	if _, ok := provider.providers[0].(*coinGeckoProvider); !ok {
		t.Error("First provider should be coinGeckoProvider")
	}

	t.Logf("Initialized %d providers", len(provider.providers))
}

func TestExchangeRateProviderGetRate_Mock(t *testing.T) {
	erp, err := NewMockExchangeRates()
	if err != nil {
		t.Fatalf("failed to create mock exchange rates: %v", err)
	}

	rate, err := erp.GetRate("BTC", "USD", false)
	if err != nil {
		t.Fatalf("failed to get BTC/USD rate: %v", err)
	}

	// BTC = 65000 USD, USD divisibility=2 → 6500000
	if rate.Int64() != 6500000 {
		t.Errorf("expected BTC/USD rate 6500000, got %d", rate.Int64())
	}
}

func TestExchangeRateProviderGetUSDRate_Mock(t *testing.T) {
	erp, err := NewMockExchangeRates()
	if err != nil {
		t.Fatalf("failed to create mock exchange rates: %v", err)
	}

	rate, err := erp.GetUSDRate(iwallet.CtBitcoin)
	if err != nil {
		t.Fatalf("failed to get BTC USD rate: %v", err)
	}

	if rate.Int64() != 6500000 {
		t.Errorf("expected USD rate 6500000, got %d", rate.Int64())
	}
}

func TestExchangeRateProviderGetAllRates_Mock(t *testing.T) {
	erp, err := NewMockExchangeRates()
	if err != nil {
		t.Fatalf("failed to create mock exchange rates: %v", err)
	}

	rates, err := erp.GetAllRates("BTC", false)
	if err != nil {
		t.Fatalf("failed to get all BTC rates: %v", err)
	}

	if rates == nil {
		t.Fatal("Rates should not be nil")
	}

	expected := []string{"USD", "ETH", "EUR"}
	for _, currency := range expected {
		if rate, exists := rates[models.CurrencyCode(currency)]; exists {
			t.Logf("%s rate: %v", currency, rate)
		} else {
			t.Errorf("%s rate not found", currency)
		}
	}
}

func TestExchangeRateProviderCache(t *testing.T) {
	erp, err := NewMockExchangeRates()
	if err != nil {
		t.Fatalf("failed to create mock exchange rates: %v", err)
	}

	rate1, err := erp.GetRate("BTC", "USD", false)
	if err != nil {
		t.Fatalf("failed to get first BTC/USD rate: %v", err)
	}

	rate2, err := erp.GetRate("BTC", "USD", false)
	if err != nil {
		t.Fatal("Failed to get cached rate")
	}

	if rate1.Int64() != rate2.Int64() {
		t.Errorf("Cached rate should be the same: rate1=%v, rate2=%v", rate1, rate2)
	}
}

func TestExchangeRateProviderBreakCache_Mock(t *testing.T) {
	erp, err := NewMockExchangeRates()
	if err != nil {
		t.Fatalf("failed to create mock exchange rates: %v", err)
	}

	rate1, err := erp.GetRate("BTC", "USD", false)
	if err != nil {
		t.Fatalf("failed to get first BTC/USD rate: %v", err)
	}

	rate2, err := erp.GetRate("BTC", "USD", true)
	if err != nil {
		t.Fatal("Failed to get refreshed rate")
	}

	t.Logf("Break cache test: rate1=%v, rate2=%v", rate1, rate2)
}

// mockProvider is used for testing stale-while-revalidate behavior.
type mockProvider struct {
	rates map[models.CurrencyCode]iwallet.Amount
	err   error
}

func (m *mockProvider) fetchRates(_ models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error) {
	return m.rates, m.err
}

func TestGetRate_StaleFallback(t *testing.T) {
	goodRates := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6500000),
	}
	mock := &mockProvider{rates: goodRates}

	e := &ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		providers:   []provider{mock},
		cacheTTL:    30 * time.Second,
	}

	rate, err := e.GetRate("BTC", "USD", false)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if rate.Int64() != 6500000 {
		t.Fatalf("expected 6500000, got %d", rate.Int64())
	}

	mock.rates = nil
	mock.err = errors.New("provider down")
	e.lastQueried["BTC"] = time.Now().Add(-5 * time.Minute)

	rate, err = e.GetRate("BTC", "USD", false)
	if err != nil {
		t.Fatalf("expected stale fallback, got error: %v", err)
	}
	if rate.Int64() != 6500000 {
		t.Fatalf("expected stale value 6500000, got %d", rate.Int64())
	}
}

func TestGetRate_NoCacheFails(t *testing.T) {
	mock := &mockProvider{err: errors.New("provider down")}
	e := &ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		providers:   []provider{mock},
		cacheTTL:    30 * time.Second,
	}

	_, err := e.GetRate("BTC", "USD", false)
	if err == nil {
		t.Fatal("expected error when no cache and provider fails")
	}
}

func TestGetRate_BreakCacheStaleFallback(t *testing.T) {
	goodRates := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6500000),
	}
	mock := &mockProvider{rates: goodRates}

	e := &ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		providers:   []provider{mock},
		cacheTTL:    30 * time.Second,
	}

	_, err := e.GetRate("BTC", "USD", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mock.rates = nil
	mock.err = errors.New("provider down")

	rate, err := e.GetRate("BTC", "USD", true)
	if err != nil {
		t.Fatalf("breakCache=true should still fallback to stale, got error: %v", err)
	}
	if rate.Int64() != 6500000 {
		t.Fatalf("expected stale value 6500000, got %d", rate.Int64())
	}
}

func TestGetAllRates_BreakCacheStaleFallback(t *testing.T) {
	goodRates := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6500000),
	}
	mock := &mockProvider{rates: goodRates}

	e := &ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		providers:   []provider{mock},
		cacheTTL:    30 * time.Second,
	}

	_, err := e.GetAllRates("BTC", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mock.rates = nil
	mock.err = errors.New("provider down")

	rates, err := e.GetAllRates("BTC", true)
	if err != nil {
		t.Fatalf("breakCache=true should still fallback to stale, got error: %v", err)
	}
	if len(rates) != 1 {
		t.Fatalf("expected 1 stale rate, got %d", len(rates))
	}
}

func TestGetAllRates_StaleFallback(t *testing.T) {
	goodRates := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6500000),
		"EUR": iwallet.NewAmount(6000000),
	}
	mock := &mockProvider{rates: goodRates}

	e := &ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		providers:   []provider{mock},
		cacheTTL:    30 * time.Second,
	}

	rates, err := e.GetAllRates("BTC", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rates) != 2 {
		t.Fatalf("expected 2 rates, got %d", len(rates))
	}

	mock.rates = nil
	mock.err = errors.New("provider down")
	e.lastQueried["BTC"] = time.Now().Add(-5 * time.Minute)

	rates, err = e.GetAllRates("BTC", false)
	if err != nil {
		t.Fatalf("expected stale fallback, got error: %v", err)
	}
	if len(rates) != 2 {
		t.Fatalf("expected 2 stale rates, got %d", len(rates))
	}
}

func TestFetchRates_DivergenceDetection(t *testing.T) {
	primaryRates := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6500000),
		"EUR": iwallet.NewAmount(6000000),
	}
	secondaryRates := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6500000),
		"EUR": iwallet.NewAmount(6000000),
	}
	primary := &mockProvider{rates: primaryRates}
	secondary := &mockProvider{rates: secondaryRates}

	e := &ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		providers:   []provider{primary, secondary},
		cacheTTL:    30 * time.Second,
	}

	rates, err := e.GetRate("BTC", "USD", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rates.Int64() != 6500000 {
		t.Fatalf("expected 6500000, got %d", rates.Int64())
	}
}

func TestFetchRates_DivergenceAboveThreshold(t *testing.T) {
	primaryRates := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6500000),
	}
	// 10% divergence
	secondaryRates := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(5850000),
	}
	primary := &mockProvider{rates: primaryRates}
	secondary := &mockProvider{rates: secondaryRates}

	e := &ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		providers:   []provider{primary, secondary},
		cacheTTL:    30 * time.Second,
	}

	rates, err := e.GetRate("BTC", "USD", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Primary rate should be used even when divergence is detected
	if rates.Int64() != 6500000 {
		t.Fatalf("expected primary rate 6500000, got %d", rates.Int64())
	}
}

func TestCheckRateDivergence_NoWarningWithinThreshold(t *testing.T) {
	primary := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6500000),
	}
	// 2% divergence — within 5% threshold
	secondary := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6370000),
	}
	// Should not panic or produce unexpected behavior
	checkRateDivergence("BTC", primary, secondary, &mockProvider{})
}

func TestCheckRateDivergence_WarningAboveThreshold(t *testing.T) {
	primary := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6500000),
	}
	// 8% divergence — above 5% threshold
	secondary := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(5980000),
	}
	// Should not panic; the warning is logged to stdout
	checkRateDivergence("BTC", primary, secondary, &mockProvider{})
}

func TestCheckRateDivergence_SkipsZeroValues(t *testing.T) {
	primary := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(0),
	}
	secondary := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6500000),
	}
	// Should skip zero-value primary rates without panicking
	checkRateDivergence("BTC", primary, secondary, &mockProvider{})
}

func TestCheckRateDivergence_SkipsMissingCurrencies(t *testing.T) {
	primary := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6500000),
		"EUR": iwallet.NewAmount(6000000),
	}
	secondary := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6500000),
	}
	// EUR missing in secondary — should skip without error
	checkRateDivergence("BTC", primary, secondary, &mockProvider{})
}

func TestFetchRates_SecondaryFillsMissingCurrencies(t *testing.T) {
	primaryRates := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6500000),
	}
	secondaryRates := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6490000),
		"EUR": iwallet.NewAmount(6000000),
	}
	primary := &mockProvider{rates: primaryRates}
	secondary := &mockProvider{rates: secondaryRates}

	e := &ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		providers:   []provider{primary, secondary},
		cacheTTL:    30 * time.Second,
	}

	rates, err := e.GetAllRates("BTC", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Primary USD should be used
	if rates["USD"].Int64() != 6500000 {
		t.Errorf("expected primary USD rate 6500000, got %d", rates["USD"].Int64())
	}
	// EUR should come from secondary
	if rates["EUR"].Int64() != 6000000 {
		t.Errorf("expected secondary EUR rate 6000000, got %d", rates["EUR"].Int64())
	}
}

func TestCacheTTLFromConfig(t *testing.T) {
	cfg := config.GetGlobalExchangeRateConfig()
	cfg.SetCacheTTL(45 * time.Second)
	cfg.SetCoinGeckoEnabled(true)
	cfg.SetChainlinkEnabled(false)

	provider := NewExchangeRateProvider(nil)
	if provider.cacheTTL != 45*time.Second {
		t.Errorf("expected cacheTTL 45s, got %s", provider.cacheTTL)
	}

	// Reset for other tests
	cfg.SetCacheTTL(30 * time.Second)
}

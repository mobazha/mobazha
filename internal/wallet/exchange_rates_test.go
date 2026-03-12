package wallet

import (
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestNewExchangeRateProvider(t *testing.T) {
	// 设置测试配置
	cfg := config.GetGlobalExchangeRateConfig()
	cfg.SetChainlinkEnabled(true)
	cfg.SetTraditionalAPIEnabled(true)
	cfg.SetChainlinkRPCURL("https://polygon-rpc.com")
	cfg.SetTraditionalAPISources([]string{"https://info.mobazha.org/api/ticker"})

	// 创建汇率提供者
	provider := NewExchangeRateProvider([]string{})

	if provider == nil {
		t.Fatal("ExchangeRateProvider should not be nil")
	}

	if len(provider.providers) == 0 {
		t.Fatal("Should have at least one provider")
	}

	t.Logf("Initialized %d providers", len(provider.providers))
}

func TestChainlinkProvider(t *testing.T) {
	// 创建Chainlink provider
	provider, err := NewChainlinkProvider("https://polygon-rpc.com")
	if err != nil {
		t.Skipf("Skipping Chainlink test due to connection error: %v", err)
	}

	if provider == nil {
		t.Fatal("ChainlinkProvider should not be nil")
	}

	// 测试获取BTC汇率
	rates, err := provider.fetchRates(models.CurrencyCode("BTC"))
	if err != nil {
		t.Logf("Failed to fetch BTC rates: %v", err)
		// 不失败，因为可能是网络问题
		return
	}

	if rates == nil {
		t.Fatal("Rates should not be nil")
	}

	t.Logf("Successfully fetched %d rates for BTC", len(rates))

	// 检查是否包含USD汇率
	if usdRate, exists := rates[models.CurrencyCode("USD")]; exists {
		t.Logf("USD rate: %v", usdRate)
	} else {
		t.Log("USD rate not found")
	}
}

func TestExchangeRateProviderGetRate(t *testing.T) {
	// 设置测试配置
	cfg := config.GetGlobalExchangeRateConfig()
	cfg.SetChainlinkEnabled(true)
	cfg.SetTraditionalAPIEnabled(true)

	// 创建汇率提供者
	provider := NewExchangeRateProvider([]string{})

	// 测试获取BTC对USD的汇率
	rate, err := provider.GetRate(models.CurrencyCode("BTC"), models.CurrencyCode("USD"), false)
	if err != nil {
		t.Logf("Failed to get BTC/USD rate: %v", err)
		// 不失败，因为可能是网络问题
		return
	}

	if rate.Int64() <= 0 {
		t.Fatal("Rate should be greater than 0")
	}

	t.Logf("BTC/USD rate: %v", rate)
}

func TestExchangeRateProviderGetAllRates(t *testing.T) {
	// 设置测试配置
	cfg := config.GetGlobalExchangeRateConfig()
	cfg.SetChainlinkEnabled(true)
	cfg.SetTraditionalAPIEnabled(true)

	// 创建汇率提供者
	provider := NewExchangeRateProvider([]string{})

	// 测试获取所有BTC汇率
	rates, err := provider.GetAllRates(models.CurrencyCode("USDT"), false)
	if err != nil {
		t.Logf("Failed to get all BTC rates: %v", err)
		// 不失败，因为可能是网络问题
		return
	}

	if rates == nil {
		t.Fatal("Rates should not be nil")
	}

	t.Logf("Successfully fetched %d rates for BTC", len(rates))

	// 检查一些关键汇率
	currencies := []string{"USD", "ETH", "BNB", "BTC"}
	for _, currency := range currencies {
		if rate, exists := rates[models.CurrencyCode(currency)]; exists {
			t.Logf("%s rate: %v", currency, rate)
		} else {
			t.Logf("%s rate not found", currency)
		}
	}
}

func TestExchangeRateProviderCache(t *testing.T) {
	// 设置测试配置
	cfg := config.GetGlobalExchangeRateConfig()
	cfg.SetChainlinkEnabled(true)
	cfg.SetTraditionalAPIEnabled(true)
	cfg.SetCacheTimeoutMinutes(1) // 设置1分钟缓存

	// 创建汇率提供者
	provider := NewExchangeRateProvider([]string{})

	// 第一次获取汇率
	rate1, err := provider.GetRate(models.CurrencyCode("BTC"), models.CurrencyCode("USD"), false)
	if err != nil {
		t.Logf("Failed to get first BTC/USD rate: %v", err)
		return
	}

	// 立即再次获取（应该从缓存）
	rate2, err := provider.GetRate(models.CurrencyCode("BTC"), models.CurrencyCode("USD"), false)
	if err != nil {
		t.Fatal("Failed to get cached rate")
	}

	// 缓存的值应该相同
	if rate1.Int64() != rate2.Int64() {
		t.Fatal("Cached rate should be the same")
	}

	t.Logf("Cache test passed: rate1=%v, rate2=%v", rate1, rate2)
}

func TestExchangeRateProviderBreakCache(t *testing.T) {
	// 设置测试配置
	cfg := config.GetGlobalExchangeRateConfig()
	cfg.SetChainlinkEnabled(true)
	cfg.SetTraditionalAPIEnabled(true)

	// 创建汇率提供者
	provider := NewExchangeRateProvider([]string{})

	// 第一次获取汇率
	rate1, err := provider.GetRate(models.CurrencyCode("BTC"), models.CurrencyCode("USD"), false)
	if err != nil {
		t.Logf("Failed to get first BTC/USD rate: %v", err)
		return
	}

	// 强制刷新缓存
	rate2, err := provider.GetRate(models.CurrencyCode("BTC"), models.CurrencyCode("USD"), true)
	if err != nil {
		t.Fatal("Failed to get refreshed rate")
	}

	t.Logf("Break cache test: rate1=%v, rate2=%v", rate1, rate2)
}

func TestExchangeRateProviderGetUSDRate(t *testing.T) {
	// 设置测试配置
	cfg := config.GetGlobalExchangeRateConfig()
	cfg.SetChainlinkEnabled(true)
	cfg.SetTraditionalAPIEnabled(true)

	// 创建汇率提供者
	provider := NewExchangeRateProvider([]string{})

	// 测试获取BTC的USD汇率
	rate, err := provider.GetUSDRate(iwallet.CtBitcoin)
	if err != nil {
		t.Logf("Failed to get BTC USD rate: %v", err)
		return
	}

	if rate.Int64() <= 0 {
		t.Fatal("USD rate should be greater than 0")
	}

	t.Logf("BTC USD rate: %v", rate)
}

func TestProviderFallback(t *testing.T) {
	// 设置测试配置 - 只启用Chainlink
	cfg := config.GetGlobalExchangeRateConfig()
	cfg.SetChainlinkEnabled(true)
	cfg.SetTraditionalAPIEnabled(false)

	// 创建汇率提供者
	provider := NewExchangeRateProvider([]string{})

	if len(provider.providers) == 0 {
		t.Fatal("Should have at least one provider (Chainlink)")
	}

	t.Logf("Provider fallback test: %d providers initialized", len(provider.providers))
}

func TestChainlinkProviderClose(t *testing.T) {
	// 创建Chainlink provider
	provider, err := NewChainlinkProvider("https://polygon-rpc.com")
	if err != nil {
		t.Skipf("Skipping Chainlink close test due to connection error: %v", err)
	}

	// 测试关闭
	err = provider.Close()
	if err != nil {
		t.Fatal("Failed to close Chainlink provider")
	}

	t.Log("Chainlink provider closed successfully")
}

type mockProvider struct {
	rates map[models.CurrencyCode]iwallet.Amount
	err   error
}

func (m *mockProvider) fetchRates(_ models.CurrencyCode) (map[models.CurrencyCode]iwallet.Amount, error) {
	return m.rates, m.err
}

func TestGetRate_StaleFallback(t *testing.T) {
	goodRates := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6000000),
	}
	mock := &mockProvider{rates: goodRates}

	e := &ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		providers:   []provider{mock},
	}

	rate, err := e.GetRate("BTC", "USD", false)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if rate.Int64() != 6000000 {
		t.Fatalf("expected 6000000, got %d", rate.Int64())
	}

	mock.rates = nil
	mock.err = errors.New("provider down")

	e.lastQueried["BTC"] = time.Now().Add(-20 * time.Minute)

	rate, err = e.GetRate("BTC", "USD", false)
	if err != nil {
		t.Fatalf("expected stale fallback, got error: %v", err)
	}
	if rate.Int64() != 6000000 {
		t.Fatalf("expected stale value 6000000, got %d", rate.Int64())
	}
}

func TestGetRate_NoCacheFails(t *testing.T) {
	mock := &mockProvider{err: errors.New("provider down")}
	e := &ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		providers:   []provider{mock},
	}

	_, err := e.GetRate("BTC", "USD", false)
	if err == nil {
		t.Fatal("expected error when no cache and provider fails")
	}
}

func TestGetRate_BreakCacheStaleFallback(t *testing.T) {
	goodRates := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6000000),
	}
	mock := &mockProvider{rates: goodRates}

	e := &ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		providers:   []provider{mock},
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
	if rate.Int64() != 6000000 {
		t.Fatalf("expected stale value 6000000, got %d", rate.Int64())
	}
}

func TestGetAllRates_BreakCacheStaleFallback(t *testing.T) {
	goodRates := map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(6000000),
	}
	mock := &mockProvider{rates: goodRates}

	e := &ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		providers:   []provider{mock},
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
		"USD": iwallet.NewAmount(6000000),
		"EUR": iwallet.NewAmount(5500000),
	}
	mock := &mockProvider{rates: goodRates}

	e := &ExchangeRateProvider{
		cache:       make(map[models.CurrencyCode]map[models.CurrencyCode]iwallet.Amount),
		lastQueried: make(map[models.CurrencyCode]time.Time),
		providers:   []provider{mock},
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
	e.lastQueried["BTC"] = time.Now().Add(-20 * time.Minute)

	rates, err = e.GetAllRates("BTC", false)
	if err != nil {
		t.Fatalf("expected stale fallback, got error: %v", err)
	}
	if len(rates) != 2 {
		t.Fatalf("expected 2 stale rates, got %d", len(rates))
	}
}

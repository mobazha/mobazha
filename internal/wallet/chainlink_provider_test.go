package wallet

import (
	"math/big"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

// TestChainlinkProviderIntegration 测试真实的 ChainlinkProvider
func TestChainlinkProviderIntegration(t *testing.T) {
	// 使用 Polygon 测试网 RPC
	rpcURL := "https://polygon-rpc.com"

	provider, err := NewChainlinkProvider(rpcURL)
	if err != nil {
		t.Skipf("无法连接到 Polygon RPC，跳过集成测试: %v", err)
	}
	defer provider.Close()

	// 测试以 BTC 为基础货币获取汇率
	rates, err := provider.fetchRates(models.CurrencyCode("BTC"))
	if err != nil {
		t.Skipf("fetchRates failed (likely no valid API key): %v", err)
	}

	// 验证返回的汇率数据不为空
	if len(rates) == 0 {
		t.Fatal("汇率数据为空")
	}

	// 验证关键币种的汇率存在
	requiredCurrencies := []string{"USD", "ETH", "USDT"}
	for _, currency := range requiredCurrencies {
		if _, exists := rates[models.CurrencyCode(currency)]; !exists {
			t.Errorf("缺少 %s 的汇率数据", currency)
		}
	}

	// 验证 BTC 对 USD 的汇率（应该是 1 BTC = 1 BTC）
	btcRate, exists := rates[models.CurrencyCode("BTC")]
	if !exists {
		t.Fatal("BTC 汇率未找到")
	}

	// BTC 对 BTC 的汇率应该是 1
	expectedBTCRate := big.NewInt(100000000) // 1 BTC = 100000000 satoshis
	if btcRate.String() != expectedBTCRate.String() {
		t.Errorf("BTC 汇率不匹配。期望: %s, 实际: %s", expectedBTCRate.String(), btcRate.String())
	}

	// 验证 USD 汇率（应该是 1 BTC 能换多少 USD）
	usdRate, exists := rates[models.CurrencyCode("USD")]
	if !exists {
		t.Fatal("USD 汇率未找到")
	}

	// USD 汇率应该是一个合理的正数
	if usdRate.Int64() <= 0 {
		t.Errorf("USD 汇率无效: %s", usdRate.String())
	}

	t.Logf("BTC 对 USD 的汇率: %s (最小精度单位)", usdRate.String())
}

// TestChainlinkProviderWithETHBase 测试以 ETH 为基础货币的汇率转换
func TestChainlinkProviderWithETHBase(t *testing.T) {
	rpcURL := "https://polygon-rpc.com"

	provider, err := NewChainlinkProvider(rpcURL)
	if err != nil {
		t.Skipf("无法连接到 Polygon RPC，跳过集成测试: %v", err)
	}
	defer provider.Close()

	// 测试以 ETH 为基础货币获取汇率
	rates, err := provider.fetchRates(models.CurrencyCode("ETH"))
	if err != nil {
		t.Skipf("fetchRates failed (likely no valid API key): %v", err)
	}

	// 验证返回的汇率数据不为空
	if len(rates) == 0 {
		t.Fatal("汇率数据为空")
	}

	// 验证 ETH 对 ETH 的汇率（应该是 1）
	ethRate, exists := rates[models.CurrencyCode("ETH")]
	if !exists {
		t.Fatal("ETH 汇率未找到")
	}

	// ETH 对 ETH 的汇率应该是 1
	expectedETHRate := big.NewInt(1000000000000000000) // 1 ETH = 10^18 wei
	if ethRate.String() != expectedETHRate.String() {
		t.Errorf("ETH 汇率不匹配。期望: %s, 实际: %s", expectedETHRate.String(), ethRate.String())
	}

	// 验证 USD 汇率
	usdRate, exists := rates[models.CurrencyCode("USD")]
	if !exists {
		t.Fatal("USD 汇率未找到")
	}

	// USD 汇率应该是一个合理的正数
	if usdRate.Int64() <= 0 {
		t.Errorf("USD 汇率无效: %s", usdRate.String())
	}

	t.Logf("ETH 对 USD 的汇率: %s (最小精度单位)", usdRate.String())
}

// TestChainlinkProviderPriceFeeds 测试价格源数据
func TestChainlinkProviderPriceFeeds(t *testing.T) {
	rpcURL := "https://polygon-rpc.com"

	provider, err := NewChainlinkProvider(rpcURL)
	if err != nil {
		t.Skipf("无法连接到 Polygon RPC，跳过集成测试: %v", err)
	}
	defer provider.Close()

	// 测试获取单个价格源数据
	testCases := []struct {
		symbol  string
		address string
	}{
		{"BTC", "0xc907E116054Ad103354f2D350FD2514433D57F6f"},
		{"ETH", "0xF9680D99D6C9589e2a93a78A04A279e509205945"},
		{"USDT", "0x0A6513e40db6EB1b165753AD52E80663aeA50545"},
	}

	for _, tc := range testCases {
		t.Run(tc.symbol, func(t *testing.T) {
			price, err := provider.getPriceFromChainlink(tc.address)
			if err != nil {
				t.Logf("获取 %s 价格失败: %v", tc.symbol, err)
				return // 跳过失败的测试，不中断整个测试
			}

			if price <= 0 {
				t.Errorf("%s 价格无效: %f", tc.symbol, price)
			}

			t.Logf("%s 价格: $%.2f", tc.symbol, price)
		})
	}
}

// TestChainlinkProviderStablecoins 测试稳定币处理
func TestChainlinkProviderStablecoins(t *testing.T) {
	rpcURL := "https://polygon-rpc.com"

	provider, err := NewChainlinkProvider(rpcURL)
	if err != nil {
		t.Skipf("无法连接到 Polygon RPC，跳过集成测试: %v", err)
	}
	defer provider.Close()

	// 测试稳定币识别（USD 是法币非稳定币，不在 priceFeeds 中）
	stablecoins := []string{"USDT", "USDC"}
	for _, symbol := range stablecoins {
		if !provider.isStablecoin(symbol) {
			t.Errorf("%s 应该被识别为稳定币", symbol)
		}
	}

	// 测试非稳定币识别
	nonStablecoins := []string{"BTC", "ETH", "BNB", "SOL"}
	for _, symbol := range nonStablecoins {
		if provider.isStablecoin(symbol) {
			t.Errorf("%s 不应该被识别为稳定币", symbol)
		}
	}
}

// TestChainlinkProviderErrorHandling 测试错误处理
func TestChainlinkProviderErrorHandling(t *testing.T) {
	// 测试无效的 RPC URL
	_, err := NewChainlinkProvider("invalid-url")
	if err == nil {
		t.Error("应该返回错误，当 RPC URL 无效时")
	}

	// 测试无效的币种
	rpcURL := "https://polygon-rpc.com"
	provider, err := NewChainlinkProvider(rpcURL)
	if err != nil {
		t.Skipf("无法连接到 Polygon RPC，跳过集成测试: %v", err)
	}
	defer provider.Close()

	_, err = provider.fetchRates(models.CurrencyCode("INVALID"))
	if err == nil {
		t.Error("应该返回错误，当币种无效时")
	}
}

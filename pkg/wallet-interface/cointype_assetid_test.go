package wallet_interface

import (
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/edition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoinInfoFromCoinType_CanonicalAssetID(t *testing.T) {
	coin := CoinType("crypto:eip155:56:erc20:0x55d398326f99059ff775485246999027b3197955")

	info, err := CoinInfoFromCoinType(coin)
	if err != nil {
		t.Fatalf("CoinInfoFromCoinType(%s): %v", coin, err)
	}

	if info.Chain != ChainBSC {
		t.Fatalf("chain = %s, want %s", info.Chain, ChainBSC)
	}
	if info.IsNative {
		t.Fatalf("IsNative = true, want false")
	}
	if info.Symbol != "USDT" {
		t.Fatalf("symbol = %s, want USDT", info.Symbol)
	}
	if info.Decimals != 18 {
		t.Fatalf("decimals = %d, want 18", info.Decimals)
	}
	if !strings.EqualFold(info.Contract, "0x55d398326f99059ff775485246999027b3197955") {
		t.Fatalf("contract = %s", info.Contract)
	}
}

func TestCoinInfoFromCoinType_CanonicalAssetID_NativeBCHAndZEC(t *testing.T) {
	tests := []struct {
		coin      CoinType
		wantChain ChainType
		wantCode  string
	}{
		{coin: "crypto:bitcoincash:mainnet:native", wantChain: ChainBitcoinCash, wantCode: "BCH"},
		{coin: "crypto:zcash:mainnet:native", wantChain: ChainZCash, wantCode: "ZEC"},
	}

	for _, tt := range tests {
		info, err := CoinInfoFromCoinType(tt.coin)
		if err != nil {
			t.Fatalf("CoinInfoFromCoinType(%s): %v", tt.coin, err)
		}
		if info.Chain != tt.wantChain {
			t.Fatalf("chain = %s, want %s", info.Chain, tt.wantChain)
		}
		if info.Symbol != tt.wantCode {
			t.Fatalf("symbol = %s, want %s", info.Symbol, tt.wantCode)
		}
		if !info.IsNative {
			t.Fatalf("IsNative = false, want true")
		}
	}
}

func TestCoinInfoFromCoinType_InvalidCanonicalAssetID(t *testing.T) {
	coin := CoinType("crypto:eip155:56:erc20:not-an-address")
	if _, err := CoinInfoFromCoinType(coin); err == nil {
		t.Fatalf("expected error for invalid canonical asset id")
	}
}

func TestCoinInfoFromCoinType_RuntimeCanonicalAssets(t *testing.T) {
	const splMint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	tests := []struct {
		name         string
		coin         CoinType
		wantChain    ChainType
		wantNative   bool
		wantContract string
		wantDecimals uint8
	}{
		{
			name:         "erc20",
			coin:         "crypto:eip155:1:erc20:0x9fe46736679d2d9a65f0992f2272de9f3c7fa6e0",
			wantChain:    ChainEthereum,
			wantContract: "0x9fE46736679d2D9a65F0992F2272dE9f3c7fa6e0",
			wantDecimals: 18,
		},
		{
			name:         "solana native devnet",
			coin:         "crypto:solana:devnet:native",
			wantChain:    ChainSolana,
			wantNative:   true,
			wantDecimals: 9,
		},
		{
			name:         "spl devnet",
			coin:         CoinType("crypto:solana:devnet:spl:" + splMint),
			wantChain:    ChainSolana,
			wantContract: splMint,
			wantDecimals: 9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := CoinInfoFromCoinType(tt.coin)
			if err != nil {
				t.Fatalf("CoinInfoFromCoinType(%s): %v", tt.coin, err)
			}
			if info.Chain != tt.wantChain {
				t.Fatalf("chain = %s, want %s", info.Chain, tt.wantChain)
			}
			if info.IsNative != tt.wantNative {
				t.Fatalf("IsNative = %v, want %v", info.IsNative, tt.wantNative)
			}
			if info.Contract != tt.wantContract {
				t.Fatalf("contract = %s, want %s", info.Contract, tt.wantContract)
			}
			if info.TestnetContract != tt.wantContract && !tt.wantNative {
				t.Fatalf("testnet contract = %s, want %s", info.TestnetContract, tt.wantContract)
			}
			if info.Decimals != tt.wantDecimals {
				t.Fatalf("decimals = %d, want %d", info.Decimals, tt.wantDecimals)
			}
		})
	}
}

func TestCoinType_PricingCurrencyCode(t *testing.T) {
	tests := []struct {
		coin     CoinType
		expected string
	}{
		{"crypto:eip155:1:native", "ETH"},
		{"crypto:bitcoincash:mainnet:native", "BCH"},
		{"crypto:zcash:mainnet:native", "ZEC"},
		{"crypto:eip155:56:erc20:0x55d398326f99059ff775485246999027b3197955", "BSCUSDT"},
		{"fiat:stripe:usd", "USD"},
		{CtMock, "MCK"},
	}

	for _, tt := range tests {
		got, err := tt.coin.PricingCurrencyCode()
		if err != nil {
			t.Fatalf("PricingCurrencyCode(%s): %v", tt.coin, err)
		}
		if got != tt.expected {
			t.Fatalf("PricingCurrencyCode(%s) = %s, want %s", tt.coin, got, tt.expected)
		}
	}
}

func TestCoinType_PricingCurrencyCode_RejectsLegacyCoinCode(t *testing.T) {
	if _, err := CoinType("ETHUSDT").PricingCurrencyCode(); err == nil {
		t.Fatalf("expected error for legacy coin code")
	}
}

func TestCoinType_PricingCurrencyCode_RejectsNonCanonicalFiat(t *testing.T) {
	if _, err := CoinType("fiat:USD").PricingCurrencyCode(); err == nil {
		t.Fatalf("expected error for non-canonical fiat coin")
	}
}

func TestCoinType_IsCanonicalCryptoAssetID(t *testing.T) {
	if !CoinType("crypto:eip155:1:native").IsCanonicalCryptoAssetID() {
		t.Fatal("canonical asset id should be true")
	}
	if CoinType("ETHUSDT").IsCanonicalCryptoAssetID() {
		t.Fatal("legacy coin code should be false")
	}
}

func TestCoinType_IsCanonicalPaymentCoin(t *testing.T) {
	tests := []struct {
		coin     CoinType
		expected bool
	}{
		{"crypto:eip155:1:native", true},
		{"fiat:stripe:USD", true},
		{"fiat:USD", false},
		{CtMock, true},
		{CtTestCoin, true},
		{"ETHUSDT", false},
	}
	for _, tt := range tests {
		got := tt.coin.IsCanonicalPaymentCoin()
		if got != tt.expected {
			t.Fatalf("IsCanonicalPaymentCoin(%s)=%v, want %v", tt.coin, got, tt.expected)
		}
	}
}

func TestCoinType_ValidateCanonicalPaymentCoin(t *testing.T) {
	if err := CoinType("crypto:tron:mainnet:trc20:TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t").ValidateCanonicalPaymentCoin(); err != nil {
		t.Fatalf("unexpected error for canonical coin: %v", err)
	}
	if err := CoinType("fiat:USD").ValidateCanonicalPaymentCoin(); err == nil {
		t.Fatalf("expected error for non-canonical fiat coin")
	}
	if err := CoinType("TRXUSDT").ValidateCanonicalPaymentCoin(); err == nil {
		t.Fatalf("expected error for legacy payment coin")
	}
}

func TestIsPaymentCoinEnabled(t *testing.T) {
	require.NoError(t, edition.ConfigureCurrentPolicy(edition.CommunityName))
	t.Cleanup(func() { require.NoError(t, edition.ConfigureCurrentPolicy(edition.FullName)) })

	tests := []struct {
		coin     string
		expected bool
	}{
		{"", true},
		{"BTC", true},
		{"BCH", true},
		{"crypto:bitcoincash:mainnet:native", true},
		{"LTC", true},
		{"ZEC", false},
		{"crypto:zcash:mainnet:native", false},
		{" crypto:ZCASH:mainnet:native ", false},
		{"ETH", false},
		{"SOL", false},
		{"EXTERNAL_PAYMENT", false},
		{"fiat:stripe:USD", false},
	}
	for _, tt := range tests {
		t.Run(tt.coin, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsPaymentCoinEnabled(tt.coin))
		})
	}
}

func TestIsPaymentCoinEnabledForPolicy_LegacyCoinUsesCapabilityPolicy(t *testing.T) {
	full, err := edition.ResolvePolicy(edition.FullName)
	require.NoError(t, err)
	assert.True(t, IsPaymentCoinEnabledForPolicy(string(CtMock), full))

	manifest, err := edition.CommunityManifest()
	require.NoError(t, err)
	manifest.Edition = "self-hosted"
	restrictive, err := edition.NewPolicy(manifest)
	require.NoError(t, err)
	assert.False(t, IsPaymentCoinEnabledForPolicy(string(CtMock), restrictive))
}

func TestCanonicalNativeCoinType(t *testing.T) {
	tests := []struct {
		chain    ChainType
		expected CoinType
		ok       bool
	}{
		{chain: ChainBitcoin, expected: "crypto:bip122:000000000019d6689c085ae165831e93:native", ok: true},
		{chain: ChainEthereum, expected: "crypto:eip155:1:native", ok: true},
		{chain: ChainBSC, expected: "crypto:eip155:56:native", ok: true},
		{chain: ChainPolygon, expected: "crypto:eip155:137:native", ok: true},
		{chain: ChainBase, expected: "crypto:eip155:8453:native", ok: true},
		{chain: ChainConflux, expected: "crypto:eip155:1030:native", ok: true},
		{chain: ChainSolana, expected: "crypto:solana:mainnet:native", ok: true},
		{chain: ChainTRON, expected: "crypto:tron:mainnet:native", ok: true},
	}

	for _, tt := range tests {
		got, ok := CanonicalNativeCoinType(tt.chain)
		if ok != tt.ok {
			t.Fatalf("CanonicalNativeCoinType(%s) ok=%v, want %v", tt.chain, ok, tt.ok)
		}
		if ok && got != tt.expected {
			t.Fatalf("CanonicalNativeCoinType(%s)=%s, want %s", tt.chain, got, tt.expected)
		}
	}
}

func TestRequireCanonicalNativeCoinType(t *testing.T) {
	eth, err := RequireCanonicalNativeCoinType(ChainEthereum)
	if err != nil {
		t.Fatalf("RequireCanonicalNativeCoinType(ETH): %v", err)
	}
	if eth != CoinType("crypto:eip155:1:native") {
		t.Fatalf("RequireCanonicalNativeCoinType(ETH)=%s, want crypto:eip155:1:native", eth)
	}

	if _, err := RequireCanonicalNativeCoinType(ChainFiat); err == nil {
		t.Fatal("expected error for non-canonical chain Fiat")
	}
}

func TestCoinInfo_NativeCoinType_PrefersCanonical(t *testing.T) {
	info := CoinInfo{Chain: ChainBSC, IsNative: true}
	if got := info.NativeCoinType(); got != CoinType("crypto:eip155:56:native") {
		t.Fatalf("NativeCoinType()=%s, want crypto:eip155:56:native", got)
	}

	conflux := CoinInfo{Chain: ChainConflux, IsNative: true}
	if got := conflux.NativeCoinType(); got != CoinType("crypto:eip155:1030:native") {
		t.Fatalf("NativeCoinType()=%s, want crypto:eip155:1030:native", got)
	}

	unknown := CoinInfo{Chain: ChainFiat, IsNative: true}
	if got := unknown.NativeCoinType(); got != CoinType(ChainFiat) {
		t.Fatalf("NativeCoinType()=%s, want %s", got, ChainFiat)
	}

	unsupported := CoinInfo{Chain: ChainType("UNKNOWN"), IsNative: true}
	if got := unsupported.NativeCoinType(); got != "" {
		t.Fatalf("NativeCoinType()=%s, want empty", got)
	}
}

func TestCanonicalBlockInterval(t *testing.T) {
	interval, ok := CanonicalBlockInterval(ChainEthereum)
	if !ok {
		t.Fatal("expected canonical block interval for ETH")
	}
	if interval != 12*time.Second {
		t.Fatalf("canonical ETH interval=%s, want 12s", interval)
	}

	tronInterval, tronOk := CanonicalBlockInterval(ChainTRON)
	if !tronOk {
		t.Fatal("expected canonical block interval for TRX")
	}
	if tronInterval != 3*time.Second {
		t.Fatalf("canonical TRX interval=%s, want 3s", tronInterval)
	}

	cfxInterval, cfxOk := CanonicalBlockInterval(ChainConflux)
	if !cfxOk {
		t.Fatal("expected canonical block interval for CFX")
	}
	if cfxInterval != 3*time.Second {
		t.Fatalf("canonical CFX interval=%s, want 3s", cfxInterval)
	}
}

func TestCoinInfo_BlockInterval_PrefersCanonical(t *testing.T) {
	info := CoinInfo{Chain: ChainEthereum}
	if got := info.BlockInterval(); got != 12*time.Second {
		t.Fatalf("ETH BlockInterval()=%s, want 12s", got)
	}
}

func TestCoinInfo_BlockInterval_NoFallbackForNonCanonical(t *testing.T) {
	mockInfo := CoinInfo{Chain: ChainMock}
	if got := mockInfo.BlockInterval(); got != 0 {
		t.Fatalf("MCK BlockInterval()=%s, want 0", got)
	}
}

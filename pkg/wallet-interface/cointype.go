package wallet_interface

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/assetid"
)

// ChainType 表示区块链网络类型
type ChainType string

const (
	ChainMock        ChainType = "MCK"
	ChainBitcoin     ChainType = "BTC"
	ChainBitcoinCash ChainType = "BCH"
	ChainLitecoin    ChainType = "LTC"
	ChainZCash       ChainType = "ZEC"
	ChainEthereum    ChainType = "ETH"
	ChainBSC         ChainType = "BSC"
	ChainPolygon     ChainType = "MATIC"
	ChainBase        ChainType = "BASE"
	ChainConflux     ChainType = "CFX"
	// Phase EVM-ManagedEscrow v0.3.0 Sprint 1 D8 — promoted from chainMatrix
	// not-ready bucket. These chains route through the V2 ManagedEscrowAdapter
	// only; their V1 ContractManager Registry is intentionally absent
	// (see pkg/evm/defaults.go zero-address sentinel + EVM client
	// guard in internal/chains/evm/client.go). Order creation MUST
	// fail closed on V1 paths until that registry is deployed.
	ChainArbitrum  ChainType = "ARB"
	ChainOptimism  ChainType = "OP"
	ChainAvalanche ChainType = "AVAX"
	ChainGnosis    ChainType = "XDAI"
	ChainCelo      ChainType = "CELO"
	ChainMantle    ChainType = "MNT"
	ChainZkSyncEra ChainType = "ZKSYNC"
	ChainScroll    ChainType = "SCRL"
	ChainLinea     ChainType = "LINEA"
	ChainSolana    ChainType = "SOL"
	ChainTRON      ChainType = "TRX"
	ChainExternalPayment    ChainType = "EXTERNAL_PAYMENT"

	ChainFiat ChainType = "Fiat"
)

func (chaintype ChainType) String() string {
	return string(chaintype)
}

func GetAllSupportedChainTypes() []ChainType {
	return []ChainType{
		ChainBitcoin,
		ChainBitcoinCash,
		ChainLitecoin,
		ChainZCash,
		ChainEthereum,
		ChainBSC,
		ChainPolygon,
		ChainBase,
		ChainConflux,
		ChainArbitrum,
		ChainOptimism,
		ChainAvalanche,
		ChainGnosis,
		ChainCelo,
		ChainMantle,
		ChainZkSyncEra,
		ChainScroll,
		ChainLinea,
		ChainSolana,
		ChainTRON,
		ChainExternalPayment,
		ChainFiat,
	}
}

// CoinType 表示所有支持的币种名称
type CoinType string

const (
	CtMock   CoinType = CoinType(ChainMock)
	CtFiat CoinType = CoinType(ChainFiat)

	// 测试用的 CoinType
	CtTestCoin CoinType = "TESTCOIN"
)

func (ct CoinType) String() string {
	return string(ct)
}

// CurrencyCode returns the coins currency code.
func (ct CoinType) CurrencyCode() string {
	return ct.String()
}

func (ct CoinType) CoinInfo() (CoinInfo, error) {
	return CoinInfoFromCoinType(ct)
}

// IsFiatPayment returns true if the CoinType represents a fiat payment
// (e.g. "fiat:stripe:USD", "fiat:paypal:EUR"). Fiat payments are processed through
// FiatPaymentAppService, not blockchain wallets.
func (ct CoinType) IsFiatPayment() bool {
	return strings.HasPrefix(strings.ToLower(string(ct)), "fiat:")
}

// FiatProviderID extracts the payment provider identifier from a canonical
// fiat coin type. Requires the three-part format "fiat:{provider}:{currency}".
// "fiat:stripe:USD" → "stripe", "fiat:paypal:EUR" → "paypal".
// Returns "" for non-fiat coins or malformed strings.
func (ct CoinType) FiatProviderID() string {
	if !ct.IsFiatPayment() {
		return ""
	}
	parts := strings.Split(string(ct), ":")
	if len(parts) >= 3 && parts[1] != "" {
		return strings.ToLower(parts[1])
	}
	return ""
}

// FiatBaseCurrency extracts the ISO currency code from fiat coin strings.
// "fiat:stripe:USD" → "USD", "fiat:paypal:EUR" → "EUR", "USD" → "USD".
func (ct CoinType) FiatBaseCurrency() string {
	parts := strings.Split(string(ct), ":")
	return parts[len(parts)-1]
}

const NATIVE_SYMBOL = "NATIVE"

// CoinInfo 表示完整的币种信息
type CoinInfo struct {
	Chain           ChainType // 所属链
	Symbol          string    // 代币符号
	IsNative        bool      // 是否为原生代币
	Contract        string    // 主网合约地址（非原生代币）
	TestnetContract string    // 测试网合约地址（非原生代币）
	Decimals        uint8     // 精度
	Description     string    // 描述
}

// 常用测试币定义
var (
	CtMockInfo = CoinInfo{
		Chain:       ChainMock,
		Symbol:      "MCK",
		IsNative:    true,
		Decimals:    0,
		Description: "Mock",
	}

	// 测试用的 CoinInfo
	CtTestCoinInfo = CoinInfo{
		Chain:       ChainMock,
		Symbol:      "TESTCOIN",
		IsNative:    true,
		Decimals:    18,
		Description: "Test Coin for Testing",
	}
)

// NewCoinInfo 创建新的币种
func NewCoinInfo(chain string, token string) (CoinInfo, error) {
	normalizedChain := strings.ToUpper(strings.TrimSpace(chain))
	if normalizedChain == "" {
		return CoinInfo{}, fmt.Errorf("chain is empty")
	}
	if strings.TrimSpace(token) != "" {
		return CoinInfo{}, fmt.Errorf("legacy chain+token coin type is no longer supported; use canonical crypto:* asset id")
	}

	canonicalNative, err := RequireCanonicalNativeCoinType(ChainType(normalizedChain))
	if err != nil {
		return CoinInfo{}, fmt.Errorf("unsupported chain %q for canonical native asset lookup", normalizedChain)
	}
	return CoinInfoFromCoinType(canonicalNative)
}

func (ct CoinInfo) CoinType() CoinType {
	return CoinType(ct.String())
}

// CurrencyCode 返回币种的货币代码
func (ct CoinInfo) CurrencyCode() string {
	return ct.String()
}

// String 返回币种的字符串表示
func (ct CoinInfo) String() string {
	if ct.IsNative {
		return string(ct.Chain)
	}
	return string(ct.Chain) + ct.Symbol
}

// ContractAddress 返回代币合约地址
func (ct CoinInfo) ContractAddress(testnet bool) string {
	if testnet {
		return ct.TestnetContract
	}
	return ct.Contract
}

// BlockInterval 返回链的出块间隔
func (ct CoinInfo) BlockInterval() time.Duration {
	if interval, ok := CanonicalBlockInterval(ct.Chain); ok {
		return interval
	}
	return 0
}

func (ct CoinInfo) IsEthTypeChain() bool {
	ethTypeChains := []ChainType{
		ChainEthereum, ChainBSC, ChainBase, ChainPolygon, ChainConflux,
		// Phase EVM-ManagedEscrow v0.3.0 Sprint 1 D8 — promoted EVM L2 set.
		// All use SchemeEVMCreate2 except ChainZkSyncEra which uses
		// SchemeZkSyncCreate2 (see pkg/managedescrow/address.go).
		ChainArbitrum, ChainOptimism, ChainAvalanche, ChainGnosis,
		ChainCelo, ChainMantle, ChainZkSyncEra, ChainScroll, ChainLinea,
	}
	return slices.Contains(ethTypeChains, ct.Chain)
}

// NativeCoinType 返回该链的原生币种 CoinType，用于获取对应的钱包实例
func (ct CoinInfo) NativeCoinType() CoinType {
	if canonicalNative, err := RequireCanonicalNativeCoinType(ct.Chain); err == nil {
		return canonicalNative
	}

	// Non-crypto chains/test chains are intentionally non-canonical.
	if ct.Chain == ChainFiat || ct.Chain == ChainMock {
		return CoinType(ct.Chain)
	}

	// No silent fallback for unsupported crypto chains.
	return ""
}

// CoinInfoFromCoinType 从字符串构造 CoinInfo。
// 仅支持 canonical payment coin（crypto:* / fiat:*）以及测试/Mock coin。
func CoinInfoFromCoinType(coinType CoinType) (CoinInfo, error) {
	s := strings.TrimSpace(string(coinType))
	if s == "" {
		return CoinInfo{}, fmt.Errorf("invalid coin type string: %s", s)
	}
	normalized := CoinType(s)

	// 检查是否为测试币种
	if normalized == CtTestCoin {
		return CtTestCoinInfo, nil
	}

	// MCK 是 CurrencyDefinitions 中 Mock 币种的代码，映射到 ChainMock 钱包
	if normalized == CtMock {
		return CtMockInfo, nil
	}

	if normalized == CtFiat {
		return CoinInfo{
			Chain:       ChainFiat,
			Symbol:      "Fiat",
			IsNative:    true,
			Decimals:    0,
			Description: "Fiat",
		}, nil
	}

	if normalized.IsFiatPayment() {
		return CoinInfo{
			Chain:       ChainFiat,
			Symbol:      strings.ToUpper(normalized.FiatBaseCurrency()),
			IsNative:    true,
			Decimals:    0,
			Description: "Fiat Payment",
		}, nil
	}

	if strings.HasPrefix(strings.ToLower(s), "crypto:") {
		return coinInfoFromCanonicalAssetID(normalized)
	}

	return CoinInfo{}, fmt.Errorf("invalid coin type string: %s", s)
}

func IsValidCoinType(coinType CoinType) bool {
	_, err := CoinInfoFromCoinType(coinType)
	return err == nil
}

func IsSPLTokenCoinType(coinType CoinType) bool {
	coinInfo, err := CoinInfoFromCoinType(coinType)
	return err == nil && coinInfo.Chain == ChainSolana && !coinInfo.IsNative
}

func GetAllSupportedCoinTypes() []CoinType {
	defs := assetid.DefaultRegistry().List()
	coins := make([]CoinType, 0, len(defs))
	for _, def := range defs {
		coins = append(coins, CoinType(def.AssetID))
	}
	sort.Slice(coins, func(i, j int) bool { return coins[i] < coins[j] })
	return coins
}

func GetAllSupportedCurrencyCodes() []string {
	defs := assetid.DefaultRegistry().List()
	currencyCodes := make([]string, 0, len(defs))
	for _, def := range defs {
		currencyCodes = append(currencyCodes, strings.ToUpper(strings.TrimSpace(def.Code)))
	}
	sort.Strings(currencyCodes)
	return currencyCodes
}

// IsUTXOChain returns true if the chain uses UTXO model (Bitcoin-like)
// Note: ChainMock is included for testing purposes
func (c ChainType) IsUTXOChain() bool {
	utxoChains := []ChainType{ChainBitcoin, ChainLitecoin, ChainBitcoinCash, ChainZCash, ChainMock}
	return slices.Contains(utxoChains, c)
}

// IsValid reports whether the receiver is one of the chains the platform
// recognises. Useful when callers parse a string of unknown provenance into
// a ChainType — IsValid lets them distinguish "BTC" (a real chain) from
// arbitrary strings like a canonical CoinType ("crypto:bip122:...").
func (c ChainType) IsValid() bool {
	return slices.Contains(GetAllSupportedChainTypes(), c) || c == ChainMock
}

// SupportsHDDerivation returns true if the chain supports HD key derivation (BIP32/44)
func (c ChainType) SupportsHDDerivation() bool {
	// All UTXO chains support HD derivation
	return c.IsUTXOChain()
}

// DerivationType represents the address derivation type for UTXO chains
type DerivationType string

const (
	DerivationNativeSegwit DerivationType = "native_segwit" // BIP84 - bc1q... (BTC), ltc1q... (LTC)
	DerivationSegwit       DerivationType = "segwit"        // BIP49 - 3... (BTC), M... (LTC)
	DerivationLegacy       DerivationType = "legacy"        // BIP44 - 1... (BTC), L... (LTC)
	DerivationCashAddr     DerivationType = "cashaddr"      // BCH CashAddr format
	DerivationTransparent  DerivationType = "transparent"   // ZEC transparent addresses (t1...)
)

// GetDefaultDerivationType returns the default derivation type for a chain
func (c ChainType) GetDefaultDerivationType() DerivationType {
	switch c {
	case ChainBitcoin, ChainLitecoin:
		return DerivationNativeSegwit
	case ChainBitcoinCash:
		return DerivationCashAddr
	case ChainZCash:
		return DerivationTransparent
	default:
		return DerivationLegacy
	}
}

// AvgBlockTimeSec returns the average block interval in seconds for the chain.
// Callers use this with confirmation counts to estimate remaining wait time.
// Returns 0 for chains that do not use block-based confirmation polling
// (EVM balance checks, Solana reference-key checks, TRON).
func (c ChainType) AvgBlockTimeSec() uint32 {
	switch c {
	case ChainBitcoin:
		return 600
	case ChainLitecoin:
		return 150
	case ChainBitcoinCash:
		return 600
	case ChainZCash:
		return 75
	case ChainExternalPayment:
		return 120
	default:
		return 0
	}
}

// GetSupportedDerivationTypes returns all supported derivation types for a chain
func (c ChainType) GetSupportedDerivationTypes() []DerivationType {
	switch c {
	case ChainBitcoin:
		return []DerivationType{DerivationNativeSegwit, DerivationSegwit, DerivationLegacy}
	case ChainLitecoin:
		return []DerivationType{DerivationNativeSegwit, DerivationSegwit, DerivationLegacy}
	case ChainBitcoinCash:
		return []DerivationType{DerivationCashAddr, DerivationLegacy}
	case ChainZCash:
		return []DerivationType{DerivationTransparent}
	default:
		return []DerivationType{}
	}
}

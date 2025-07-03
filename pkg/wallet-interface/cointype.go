package wallet_interface

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// ChainType 表示区块链网络类型
type ChainType string

const (
	ChainMock        ChainType = "MOCK"
	ChainBitcoin     ChainType = "BTC"
	ChainBitcoinCash ChainType = "BCH"
	ChainLitecoin    ChainType = "LTC"
	ChainZCash       ChainType = "ZEC"
	ChainEthereum    ChainType = "ETH"
	ChainBNB         ChainType = "BNB"
	ChainPolygon     ChainType = "MATIC"
	ChainBase        ChainType = "BASE"
	ChainConflux     ChainType = "CFX"
	ChainSolana      ChainType = "SOL"
	ChainExternalPayment      ChainType = "EXTERNAL_PAYMENT"
	ChainDash        ChainType = "DASH"

	ChainStripe ChainType = "Stripe"
)

func (chaintype ChainType) String() string {
	return string(chaintype)
}

func GetAllSupportedChainTypes() []ChainType {
	return []ChainType{
		ChainSolana, ChainStripe, ChainEthereum, ChainBNB,
	}
}

// CoinType 表示所有支持的币种名称
type CoinType string

const (
	CtMock        CoinType = CoinType(ChainMock)
	CtBitcoin     CoinType = CoinType(ChainBitcoin)
	CtBitcoinCash CoinType = CoinType(ChainBitcoinCash)
	CtLitecoin    CoinType = CoinType(ChainLitecoin)
	CtZCash       CoinType = CoinType(ChainZCash)
	CtEthereum    CoinType = CoinType(ChainEthereum)
	CtBNB         CoinType = CoinType(ChainBNB)
	CtPolygon     CoinType = CoinType(ChainPolygon)
	CtBase        CoinType = CoinType(ChainBase)
	CtConflux     CoinType = CoinType(ChainConflux)
	CtSolana      CoinType = CoinType(ChainSolana)
	CtExternalPayment      CoinType = CoinType(ChainExternalPayment)
	CtDash        CoinType = CoinType(ChainDash)

	CtStripe CoinType = CoinType(ChainStripe)

	CtBEP20USDT CoinType = "BNBUSDT"
	CtBEP20USDC CoinType = "BNBUSDC"
	CtMBZ       CoinType = "MBZ"

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

func (ct CoinType) IsStripeChain() bool {
	return strings.HasPrefix(strings.ToUpper(string(ct)), "STRIPE")
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

// 常用代币定义
var (
	CtMockInfo = CoinInfo{
		Chain:       ChainMock,
		Symbol:      "MOCK",
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

	// 原生代币
	CtBitcoinInfo = CoinInfo{
		Chain:       ChainBitcoin,
		Symbol:      "BTC",
		IsNative:    true,
		Decimals:    8,
		Description: "Bitcoin",
	}

	CtBNBInfo = CoinInfo{
		Chain:       ChainBNB,
		Symbol:      "BNB",
		IsNative:    true,
		Decimals:    18,
		Description: "Binance Coin",
	}

	CtBaseInfo = CoinInfo{
		Chain:       ChainBase,
		Symbol:      "ETH",
		IsNative:    true,
		Decimals:    18,
		Description: "Ethereum on Base",
	}

	CtBitcoinCashInfo = CoinInfo{
		Chain:       ChainBitcoinCash,
		Symbol:      "BCH",
		IsNative:    true,
		Decimals:    8,
		Description: "Bitcoin Cash",
	}

	CtLitecoinInfo = CoinInfo{
		Chain:       ChainLitecoin,
		Symbol:      "LTC",
		IsNative:    true,
		Decimals:    8,
		Description: "Litecoin",
	}

	CtZCashInfo = CoinInfo{
		Chain:       ChainZCash,
		Symbol:      "ZEC",
		IsNative:    true,
		Decimals:    8,
		Description: "Zcash",
	}

	CtEthereumInfo = CoinInfo{
		Chain:       ChainEthereum,
		Symbol:      "ETH",
		IsNative:    true,
		Decimals:    18,
		Description: "Ethereum",
	}

	CtPolygonInfo = CoinInfo{
		Chain:       ChainPolygon,
		Symbol:      "MATIC",
		IsNative:    true,
		Decimals:    18,
		Description: "Polygon",
	}

	CtConfluxInfo = CoinInfo{
		Chain:       ChainConflux,
		Symbol:      "CFX",
		IsNative:    true,
		Decimals:    18,
		Description: "Conflux",
	}

	CtSolanaInfo = CoinInfo{
		Chain:       ChainSolana,
		Symbol:      "SOL",
		IsNative:    true,
		Decimals:    9,
		Description: "Solana",
	}

	CtStripeInfo = CoinInfo{
		Chain:       ChainStripe,
		Symbol:      "Stripe",
		IsNative:    true,
		Decimals:    0,
		Description: "Stripe",
	}

	// ERC20代币
	ERC20USDTInfo = CoinInfo{
		Chain:           ChainEthereum,
		Symbol:          "USDT",
		IsNative:        false,
		Contract:        "0xF36BFeE8fd7F1950c0129714Faf6d1e1F94a66AA",
		TestnetContract: "0xF36BFeE8fd7F1950c0129714Faf6d1e1F94a66AA", // Goerli USDT
		Decimals:        6,
		Description:     "Tether USD",
	}

	ERC20USDCInfo = CoinInfo{
		Chain:           ChainEthereum,
		Symbol:          "USDC",
		IsNative:        false,
		Contract:        "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		TestnetContract: "0x79C950C7446B234a6Ad53B908fBF342b01c4d446", // Goerli USDC
	}

	// BEP20代币
	BEP20USDTInfo = CoinInfo{
		Chain:           ChainBNB,
		Symbol:          "USDT",
		IsNative:        false,
		Contract:        "0x55d398326f99059ff775485246999027b3197955",
		TestnetContract: "0x337610d27c682E347C9cD60BD4b3b107C9d34dDd", // BSC Testnet USDT
		Decimals:        18,
		Description:     "Tether USD on BSC",
	}

	BEP20USDCInfo = CoinInfo{
		Chain:           ChainBNB,
		Symbol:          "USDC",
		IsNative:        false,
		Contract:        "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
		TestnetContract: "0x337610d27c682E347C9cD60BD4b3b107C9d34dDd", // BSC Testnet USDT
	}

	// SPL代币
	SPLUSDTInfo = CoinInfo{
		Chain:           ChainSolana,
		Symbol:          "USDT",
		IsNative:        false,
		Contract:        "68DyGgw3jp9wH1WhEN4NaBFNgzDbWYM8TFM8XeFZTKU4",
		TestnetContract: "68DyGgw3jp9wH1WhEN4NaBFNgzDbWYM8TFM8XeFZTKU4", // Solana Devnet USDT
		Decimals:        6,
		Description:     "Tether USD on Solana",
	}

	SPLUSDCInfo = CoinInfo{
		Chain:           ChainSolana,
		Symbol:          "USDC",
		IsNative:        false,
		Contract:        "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
		TestnetContract: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU", // Solana Devnet USDT
	}
)

// 链配置
var ChainConfigs = map[ChainType]struct {
	BlockInterval time.Duration
	Description   string
}{
	ChainBitcoin:     {time.Minute * 10, "Bitcoin"},
	ChainBitcoinCash: {time.Minute * 10, "Bitcoin Cash"},
	ChainLitecoin:    {time.Second * 150, "Litecoin"},
	ChainZCash:       {time.Second * 150, "Zcash"},
	ChainEthereum:    {time.Second * 3, "Ethereum"},
	ChainBNB:         {time.Second * 3, "Binance Smart Chain"},
	ChainBase:        {time.Second * 3, "Base"},
	ChainPolygon:     {time.Second * 2, "Polygon"},
	ChainConflux:     {time.Second * 1, "Conflux"},
	ChainSolana:      {time.Second * 1, "Solana"},
}

// NewCoinInfo 创建新的币种
func NewCoinInfo(chain string, token string) (CoinInfo, error) {
	return CoinInfoFromCoinType(CoinType(chain + token))
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
func (ct CoinInfo) ContractAddress(mainnet bool) string {
	if !mainnet && ct.TestnetContract != "" {
		return ct.TestnetContract
	}
	return ct.Contract
}

// BlockInterval 返回链的出块间隔
func (ct CoinInfo) BlockInterval() time.Duration {
	if config, ok := ChainConfigs[ct.Chain]; ok {
		return config.BlockInterval
	}
	return time.Minute // 默认值
}

func (ct CoinInfo) IsEthTypeChain() bool {
	ethTypeChains := []ChainType{ChainEthereum, ChainBNB, ChainBase, ChainPolygon, ChainConflux}
	return slices.Contains(ethTypeChains, ct.Chain)
}

// CoinInfoFromCoinType 从字符串构造 CoinInfo
// 格式为 "CHAIN" 或 "CHAINTOKEN"，例如 "BTC" 或 "ETHUSDT"
func CoinInfoFromCoinType(coinType CoinType) (CoinInfo, error) {
	s := string(coinType)

	// 检查是否为测试币种
	if s == string(CtTestCoin) {
		return CtTestCoinInfo, nil
	}

	if coinType.IsStripeChain() {
		return CtStripeInfo, nil
	}

	// 检查是否为原生代币
	for chain := range ChainConfigs {
		if string(chain) == s {
			// 找到对应的原生代币
			for _, token := range []CoinInfo{
				CtBitcoinInfo, CtBitcoinCashInfo, CtLitecoinInfo, CtZCashInfo,
				CtEthereumInfo, CtBNBInfo, CtBaseInfo, CtPolygonInfo, CtConfluxInfo,
				CtSolanaInfo,
			} {
				if token.Chain == chain && token.IsNative {
					return token, nil
				}
			}
			return CoinInfo{}, fmt.Errorf("no native token found for chain %s", s)
		}
	}

	// 检查是否为合约代币
	for chain := range ChainConfigs {
		if strings.HasPrefix(s, string(chain)) {
			tokenSymbol := s[len(chain):]
			// 查找对应的合约代币
			for _, token := range []CoinInfo{
				ERC20USDTInfo, ERC20USDCInfo,
				BEP20USDTInfo, BEP20USDCInfo,
				SPLUSDTInfo, SPLUSDCInfo,
			} {
				if token.Chain == chain && token.Symbol == tokenSymbol {
					return token, nil
				}
			}
			continue
		}
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
	return []CoinType{
		CtBitcoin, CtEthereum, CtSolana, CtBNB, CtPolygon, CtConflux, CtSolana, CtExternalPayment, CtDash,
		CtBEP20USDT, CtBEP20USDC, CtMBZ,
	}
}

func GetAllSupportedCurrencyCodes() []string {
	coinTypes := GetAllSupportedCoinTypes()
	currencyCodes := make([]string, len(coinTypes))
	for i, coinType := range coinTypes {
		currencyCodes[i] = coinType.CurrencyCode()
	}
	return currencyCodes
}

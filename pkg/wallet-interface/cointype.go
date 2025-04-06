package wallet_interface

import (
	"strings"
	"time"
)

// CoinType represents a cryptocurrency that has been
// implemented the wallet interface.
type CoinType string

// CurrencyCode returns the coins currency code.
func (ct CoinType) CurrencyCode() string {
	return strings.ToUpper(string(ct))
}

const (
	// Mainnet
	CtMock        = "MCK"
	CtBitcoin     = "BTC"
	CtBitcoinCash = "BCH"
	CtLitecoin    = "LTC"
	CtZCash       = "ZEC"
	CtEthereum    = "ETH"
	CtExternalPayment      = "EXTERNAL_PAYMENT"
	CtDash        = "DASH"

	CtBNB     = "BNB"
	CtBNBUSDT = "BNBUSDT"
	CtBNBUSDC = "BNBUSDC"
	CtBNBMBZ  = "BNBMBZ"

	CtMATIC     = "MATIC"
	CtMATICUSDT = "MATICUSDT"
	CtMATICUSDC = "MATICUSDC"
	CtMATICMBZ  = "MATICMBZ"

	CtCFX     = "CFX"
	CtCFXUSDT = "CFXUSDT"
	CtCFXUSDC = "CFXUSDC"
	CtCFXMBZ  = "CFXMBZ"

	CtSolana     CoinType = "SOL"
	CtSolanaUSDT CoinType = "SOLUSDT"
	CtSolanaUSDC CoinType = "SOLUSDC"
	CtSolanaMBZ  CoinType = "SOLMBZ"
)

var codeMap = map[string]CoinType{
	"MCK":       CtMock,
	"BTC":       CtBitcoin,
	"BCH":       CtBitcoinCash,
	"LTC":       CtLitecoin,
	"ZEC":       CtZCash,
	"ETH":       CtEthereum,
	"EXTERNAL_PAYMENT":       CtExternalPayment,
	"DASH":      CtDash,
	"BNB":       CtBNB,
	"BNBUSDT":   CtBNBUSDT,
	"BNBUSDC":   CtBNBUSDC,
	"BNBMBZ":    CtBNBMBZ,
	"MATIC":     CtMATIC,
	"MATICUSDT": CtMATICUSDT,
	"MATICUSDC": CtMATICUSDC,
	"MATICMBZ":  CtMATICMBZ,

	"CFX":     CtCFX,
	"CFXUSDT": CtCFXUSDT,
	"CFXUSDC": CtCFXUSDC,
	"CFXMBZ":  CtCFXMBZ,

	"SOL":     CtSolana,
	"SOLUSDT": CtSolanaUSDT,
	"SOLUSDC": CtSolanaUSDC,
	"SOLMBZ":  CtSolanaMBZ,
}

var BlockIntervalDictionary = map[CoinType]time.Duration{
	"BTC":   time.Minute * 10,
	"BCH":   time.Minute * 10,
	"LTC":   time.Second * 150,
	"BNB":   time.Second * 3,
	"MATIC": time.Second * 2,
	"CFX":   time.Second * 1, // actually around 0.44s
}

var erc20TokenMap = map[CoinType]bool{
	CtBNBUSDT: true,
	CtBNBUSDC: true,
	CtBNBMBZ:  true,

	CtMATICUSDT: true,
	CtMATICUSDC: true,
	CtMATICMBZ:  true,

	CtCFXUSDT: true,
	CtCFXUSDC: true,
	CtCFXMBZ:  true,
}

func (ct CoinType) IsERC20Token() bool {
	_, ok := erc20TokenMap[ct]

	return ok
}

var coinChainMap = map[CoinType]CoinType{
	CtBNB:     CtBNB,
	CtBNBUSDT: CtBNB,
	CtBNBUSDC: CtBNB,
	CtBNBMBZ:  CtBNB,

	CtMATIC:     CtMATIC,
	CtMATICUSDT: CtMATIC,
	CtMATICUSDC: CtMATIC,
	CtMATICMBZ:  CtMATIC,

	CtCFX:     CtCFX,
	CtCFXUSDT: CtCFX,
	CtCFXUSDC: CtCFX,
	CtCFXMBZ:  CtCFX,
}

func (ct CoinType) ChainCoinType() CoinType {
	return coinChainMap[ct]
}

var mainnetContractAddresses = map[CoinType]string{
	CtBNBUSDT: "0x55d398326f99059ff775485246999027b3197955",
	CtBNBUSDC: "0x8ac76a51cc950d9822d68b83fe1ad97b32cd580d",
	CtBNBMBZ:  "0xBAD8470f50575Ac41d4FE1C31039554112d31E89",

	CtMATICUSDT: "0xc2132D05D31c914a87C6611C10748AEb04B58e8F",
	CtMATICUSDC: "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174",
	CtMATICMBZ:  "0x4c1A1b21c4471CA57145EE08404Cbaf9C8B83991",

	CtCFXUSDT: "0xfe97E85d13ABD9c1c33384E796F10B73905637cE",
	CtCFXUSDC: "0x6963EfED0aB40F6C3d7BdA44A05dcf1437C44372",
	CtCFXMBZ:  "0x4c1A1b21c4471CA57145EE08404Cbaf9C8B83991",
}

var testnetContractAddresses = map[CoinType]string{
	CtBNBUSDT: "0x3DAe8BD5972D7D83A9661E13becd0C2dA9177F3B",
	CtBNBUSDC: "0xaB1a4d4f1D656d2450692D237fdD6C7f9146e814",
	CtBNBMBZ:  "0xBAD8470f50575Ac41d4FE1C31039554112d31E89",

	CtMATICUSDT: "0x393b2FEfA82aB9ddFd7AF920C24A9dB0B27388c7",
	CtMATICUSDC: "0x3E8966Dbd540D9a762A3e976Fb906407ee1b1D79",
	CtMATICMBZ:  "0x4c1A1b21c4471CA57145EE08404Cbaf9C8B83991",

	CtCFXUSDT: "0xFc95C7112d16905691d74C80D796ABff93be7d71",
	CtCFXUSDC: "0x8EC3ec712fd24b11c2Ce369a04249e7C439CB339",
	CtCFXMBZ:  "0x4c1A1b21c4471CA57145EE08404Cbaf9C8B83991",
}

func (ct CoinType) ERC20ContractAddress(mainnet bool) string {
	if mainnet {
		return mainnetContractAddresses[ct]
	}

	return testnetContractAddresses[ct]
}

// SPLTokenMintAddress 返回SPL代币的铸币地址
func (c CoinType) SPLTokenMintAddress(mainnet bool) string {
	switch c {
	case CtSolanaUSDT:
		if mainnet {
			return "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB" // 主网USDT铸币地址
		}
		return "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU" // 测试网USDT铸币地址
	case CtSolanaUSDC:
		if mainnet {
			return "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" // 主网USDC铸币地址
		}
		return "Gh9ZwEmdLJ8DscKNTkTqPbNwLNNBjuSzaG9Vp2KGtKJr" // 测试网USDC铸币地址
	case CtSolanaMBZ:
		if mainnet {
			return "MBZCTPCWBpGY5XESJAciZAYXyK1J9tLbS38reJW2DebF" // 主网MBZ铸币地址
		}
		return "MBZ12CgCp3KjqpjMBBwYJwsGra4xKy4B32XGiYJvUDY" // 测试网MBZ铸币地址
	default:
		return ""
	}
}

package payment

import (
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mobazha/mobazha3.0/pkg/assetid"
)

// BuildFundingQRPayload builds a wallet-scannable payment URI for the given
// funding target. Amount is a human-readable decimal string (same unit as
// FundingTargetView.Amount / FormatSessionAmount output).
//
// Supported formats:
//   - UTXO (BIP-21 / CIP-1): bitcoin:addr?amount=0.001
//   - EVM native (EIP-681): ethereum:0xRecipient@chainId?value=<wei_number>
//   - EVM ERC-20 (EIP-681): ethereum:0xToken@chainId/transfer?address=0xRecipient&uint256=<atomic_units>
//   - Solana Pay: solana:recipient?amount=0.5[&spl-token=<mint>]
func BuildFundingQRPayload(paymentCoin, address, decimalAmount string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}

	coin := strings.TrimSpace(paymentCoin)
	if normalized, ok := NormalizeSettlementPaymentCoin(coin); ok {
		coin = string(normalized)
	}

	if id, err := assetid.Parse(coin); err == nil {
		switch id.Namespace {
		case assetid.NamespaceEIP155:
			return buildEVMQRPayload(coin, id, address, decimalAmount)
		case assetid.NamespaceSolana:
			return buildSolanaQRPayload(id, address, decimalAmount)
		case assetid.NamespaceBIP122, assetid.NamespaceBitcoinCash, assetid.NamespaceZCash:
			if scheme := utxoPaymentURIScheme(coin); scheme != "" {
				return appendURIAmountQuery(normalizeURIAddress(address, scheme), decimalAmount, "amount")
			}
		}
	}

	if scheme := utxoPaymentURIScheme(coin); scheme != "" {
		return appendURIAmountQuery(normalizeURIAddress(address, scheme), decimalAmount, "amount")
	}

	return ""
}

func buildEVMQRPayload(paymentCoin string, id assetid.ID, recipient, decimalAmount string) string {
	recipient = stripURISchemePrefix(recipient, "ethereum")
	if common.IsHexAddress(recipient) {
		recipient = common.HexToAddress(recipient).Hex()
	} else if recipient != "" && !strings.HasPrefix(recipient, "0x") {
		recipient = "0x" + recipient
	}

	chainID := strings.TrimSpace(id.ChainRef)
	if chainID == "" {
		return ""
	}

	switch id.Standard {
	case assetid.StandardERC20:
		token := strings.TrimSpace(id.AssetRef)
		if token == "" {
			return ""
		}
		if common.IsHexAddress(token) {
			token = common.HexToAddress(token).Hex()
		} else if !strings.HasPrefix(token, "0x") {
			token = "0x" + token
		}

		base := fmt.Sprintf("ethereum:%s@%s/transfer", token, chainID)
		if recipient == "" {
			return base
		}

		query := url.Values{}
		query.Set("address", recipient)
		if amountParam := eip681AtomicAmountParam(decimalAmount, paymentCoin); amountParam != "" {
			query.Set("uint256", amountParam)
		}
		return base + "?" + query.Encode()

	default:
		if recipient == "" {
			return ""
		}
		base := fmt.Sprintf("ethereum:%s@%s", recipient, chainID)
		if valueParam := eip681NativeValueParam(decimalAmount, paymentCoin); valueParam != "" {
			return base + "?value=" + valueParam
		}
		return base
	}
}

// eip681NativeValueParam formats native-token value per ERC-681 (decimal/scientific
// number in wei, not hex). Example: 0.011 ETH -> value=0.011e18
func eip681NativeValueParam(decimalAmount, paymentCoin string) string {
	decimalAmount = strings.TrimSpace(decimalAmount)
	if !isPositiveDecimalAmount(decimalAmount) {
		return ""
	}
	decimals, ok := SessionAmountDecimals(paymentCoin)
	if ok && decimals > 0 {
		return decimalAmount + "e" + strconv.Itoa(decimals)
	}
	if smallest, ok := decimalAmountToSmallestUnit(decimalAmount, paymentCoin); ok {
		return smallest.String()
	}
	return decimalAmount
}

// eip681AtomicAmountParam formats ERC-20 transfer uint256 as a decimal integer
// in the token's smallest unit (ERC-681 number, not hex).
func eip681AtomicAmountParam(decimalAmount, paymentCoin string) string {
	if smallest, ok := decimalAmountToSmallestUnit(decimalAmount, paymentCoin); ok {
		return smallest.String()
	}
	return ""
}

func buildSolanaQRPayload(id assetid.ID, recipient, decimalAmount string) string {
	recipient = stripURISchemePrefix(recipient, "solana")
	if recipient == "" {
		return ""
	}

	base := "solana:" + recipient
	params := url.Values{}
	if isPositiveDecimalAmount(decimalAmount) {
		params.Set("amount", strings.TrimSpace(decimalAmount))
	}
	if id.Standard == assetid.StandardSPL {
		if mint := strings.TrimSpace(id.AssetRef); mint != "" {
			params.Set("spl-token", mint)
		}
	}
	if len(params) == 0 {
		return base
	}
	return base + "?" + params.Encode()
}

func appendURIAmountQuery(payload, decimalAmount, paramName string) string {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return ""
	}
	if !isPositiveDecimalAmount(decimalAmount) {
		return payload
	}
	sep := "?"
	if strings.Contains(payload, "?") {
		sep = "&"
	}
	return payload + sep + paramName + "=" + strings.TrimSpace(decimalAmount)
}

func normalizeURIAddress(address, scheme string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	lower := strings.ToLower(address)
	prefix := scheme + ":"
	if strings.HasPrefix(lower, prefix) {
		return address
	}
	return scheme + ":" + address
}

func stripURISchemePrefix(address, scheme string) string {
	address = strings.TrimSpace(address)
	prefix := scheme + ":"
	if strings.HasPrefix(strings.ToLower(address), prefix) {
		return address[len(prefix):]
	}
	return address
}

func utxoPaymentURIScheme(paymentCoin string) string {
	switch coin := strings.ToLower(strings.TrimSpace(paymentCoin)); {
	case coin == "btc", strings.HasPrefix(coin, "crypto:bip122:000000000019d6689c085ae165831e93:"), strings.HasPrefix(coin, "crypto:bitcoin:"):
		return "bitcoin"
	case coin == "ltc", strings.HasPrefix(coin, "crypto:bip122:12a765e31ffd4059bada1e25190f6e98:"), strings.HasPrefix(coin, "crypto:litecoin:"):
		return "litecoin"
	case coin == "bch", strings.HasPrefix(coin, "crypto:bitcoincash:"):
		return "bitcoincash"
	case coin == "zec", strings.HasPrefix(coin, "crypto:zcash:"):
		return "zcash"
	default:
		return ""
	}
}

func isPositiveDecimalAmount(amount string) bool {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return false
	}
	v, ok := new(big.Rat).SetString(amount)
	return ok && v.Sign() > 0
}

func decimalAmountToSmallestUnit(decimalAmount, paymentCoin string) (*big.Int, bool) {
	decimals, ok := SessionAmountDecimals(paymentCoin)
	if !ok || decimals < 0 {
		return nil, false
	}
	return parseDecimalToSmallestUnit(decimalAmount, decimals)
}

func parseDecimalToSmallestUnit(decimalAmount string, decimals int) (*big.Int, bool) {
	decimalAmount = strings.TrimSpace(decimalAmount)
	if decimalAmount == "" || decimals < 0 {
		return nil, false
	}

	negative := false
	if strings.HasPrefix(decimalAmount, "-") {
		negative = true
		decimalAmount = decimalAmount[1:]
	}

	intPart := decimalAmount
	fracPart := ""
	if dot := strings.Index(decimalAmount, "."); dot >= 0 {
		intPart = decimalAmount[:dot]
		fracPart = decimalAmount[dot+1:]
	}
	if intPart == "" {
		intPart = "0"
	}

	intVal := new(big.Int)
	if _, ok := intVal.SetString(intPart, 10); !ok {
		return nil, false
	}

	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	total := new(big.Int).Mul(intVal, scale)

	if fracPart != "" {
		if len(fracPart) > decimals {
			fracPart = fracPart[:decimals]
		}
		for len(fracPart) < decimals {
			fracPart += "0"
		}
		fracVal := new(big.Int)
		if _, ok := fracVal.SetString(fracPart, 10); !ok {
			return nil, false
		}
		total.Add(total, fracVal)
	}

	if negative {
		total.Neg(total)
	}
	if total.Sign() <= 0 {
		return nil, false
	}
	return total, true
}

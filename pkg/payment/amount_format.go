package payment

import (
	"math/big"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/assetid"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// FormatSessionAmount formats a persisted smallest-unit amount for the
// PaymentSession API view. Internal order/payment state continues to use
// smallest units; PaymentSession amount fields are human-readable decimal
// strings by contract.
func FormatSessionAmount(rawAmount, paymentCoin string) string {
	trimmed := strings.TrimSpace(rawAmount)
	if trimmed == "" {
		return ""
	}

	decimals, ok := SessionAmountDecimals(paymentCoin)
	if !ok || decimals <= 0 {
		return trimmed
	}

	amount := new(big.Int)
	if _, ok := amount.SetString(trimmed, 10); !ok {
		return trimmed
	}
	if amount.Sign() == 0 {
		return "0"
	}

	negative := amount.Sign() < 0
	if negative {
		amount.Abs(amount)
	}

	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	intPart := new(big.Int).Quo(new(big.Int).Set(amount), scale)
	fracPart := new(big.Int).Mod(amount, scale)
	if fracPart.Sign() == 0 {
		if negative {
			return "-" + intPart.String()
		}
		return intPart.String()
	}

	frac := fracPart.String()
	if len(frac) < decimals {
		frac = strings.Repeat("0", decimals-len(frac)) + frac
	}
	frac = strings.TrimRight(frac, "0")
	if frac == "" {
		if negative {
			return "-" + intPart.String()
		}
		return intPart.String()
	}

	if negative {
		return "-" + intPart.String() + "." + frac
	}
	return intPart.String() + "." + frac
}

// SessionAmountDecimals resolves the divisibility used when projecting
// smallest-unit amounts into PaymentSession decimal strings.
func SessionAmountDecimals(paymentCoin string) (int, bool) {
	trimmed := strings.TrimSpace(paymentCoin)
	if trimmed == "" {
		return 0, false
	}

	if strings.HasPrefix(strings.ToLower(trimmed), "fiat:") {
		parts := strings.Split(trimmed, ":")
		if len(parts) > 0 {
			code := strings.ToUpper(strings.TrimSpace(parts[len(parts)-1]))
			if def, ok := models.CurrencyDefinitions[code]; ok && def != nil {
				return int(def.Divisibility), true
			}
		}
	}

	if coin, ok := NormalizeSettlementPaymentCoin(trimmed); ok {
		if def, err := assetid.DefaultRegistry().Lookup(string(coin)); err == nil {
			return int(def.Decimals), true
		}
	}

	code := strings.ToUpper(trimmed)
	if def, ok := models.CurrencyDefinitions[code]; ok && def != nil {
		return int(def.Divisibility), true
	}

	return 0, false
}

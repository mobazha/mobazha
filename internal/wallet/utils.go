package wallet

import (
	"errors"
	"math"
	"math/big"

	"github.com/mobazha/mobazha3.0/internal/models"
	iwallet "github.com/mobazha/mobazha3.0/internal/multiwallet/wallet-interface"
)

// ConvertCurrencyAmount converts the value of one currency into another using the exchange rate.
func ConvertCurrencyAmount(value *models.CurrencyValue, paymentCurrency *models.Currency, erp *ExchangeRateProvider) (iwallet.Amount, error) {
	// If both currency types are the same then just return the value.
	if value.Currency.Equal(paymentCurrency) {
		return value.Amount, nil
	}

	if paymentCurrency.CurrencyType != models.CurrencyTypeCrypto {
		return value.Amount, errors.New("payment currency is not type crypto")
	}

	rate, err := erp.GetRate(paymentCurrency.Code, value.Currency.Code, true)
	if err != nil {
		return value.Amount, err
	}

	rateFloat, ok := new(big.Float).SetString(rate.String())
	if !ok {
		return value.Amount, errors.New("error converting exchange rate to float")
	}

	div := new(big.Float).Quo(rateFloat, big.NewFloat(math.Pow10(int(value.Currency.Divisibility))))
	div.Quo(big.NewFloat(1), div)

	v, _ := div.Float64()

	converted, err := value.ConvertTo(paymentCurrency, v)
	if err != nil {
		return value.Amount, err
	}
	return converted.Amount, nil
}

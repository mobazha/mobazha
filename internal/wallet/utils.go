package wallet

import (
	"errors"
	"math"
	"math/big"

	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ConvertCurrencyAmount converts the value of one currency into another using the exchange rate.
// Supports crypto-to-fiat, fiat-to-crypto, and fiat-to-fiat conversions.
func ConvertCurrencyAmount(value *models.CurrencyValue, paymentCurrency *models.Currency, erp *ExchangeRateProvider) (iwallet.Amount, error) {
	if value.Currency.Equal(paymentCurrency) {
		return value.Amount, nil
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

// ConvertFiatAmount converts a fiat amount from one currency to another.
// amount is in smallest currency units (e.g. cents). Returns in smallest units of target.
func ConvertFiatAmount(amount int64, from, to string, erp *ExchangeRateProvider) (int64, error) {
	if from == to {
		return amount, nil
	}

	fromDef, err := models.CurrencyDefinitions.Lookup(from)
	if err != nil {
		return 0, err
	}
	toDef, err := models.CurrencyDefinitions.Lookup(to)
	if err != nil {
		return 0, err
	}

	value := models.CurrencyValue{
		Amount:   iwallet.NewAmount(amount),
		Currency: fromDef,
	}

	result, err := ConvertCurrencyAmount(&value, toDef, erp)
	if err != nil {
		return 0, err
	}
	return result.Int64(), nil
}

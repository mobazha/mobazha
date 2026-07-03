package wallet

import (
	"errors"
	"math/big"

	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ExchangeRateQuerier is the minimal interface needed by currency conversion
// helpers. Both *ExchangeRateProvider and contracts.ExchangeRateService
// satisfy it, allowing callers in shared (build-neutral) code to avoid a
// hard dependency on the concrete provider.
type ExchangeRateQuerier interface {
	GetRate(base models.CurrencyCode, to models.CurrencyCode, breakCache bool) (iwallet.Amount, error)
}

// ConvertCurrencyAmount converts the value of one currency into another using the exchange rate.
// Supports crypto-to-fiat, fiat-to-crypto, and fiat-to-fiat conversions.
func ConvertCurrencyAmount(value *models.CurrencyValue, paymentCurrency *models.Currency, erp ExchangeRateQuerier) (iwallet.Amount, error) {
	if value.Currency.Equal(paymentCurrency) {
		return value.Amount, nil
	}

	rate, err := erp.GetRate(paymentCurrency.Code, value.Currency.Code, true)
	if err != nil {
		return value.Amount, err
	}

	rateInt := big.Int(rate)
	if rateInt.Sign() <= 0 {
		return value.Amount, errors.New("exchange rate must be positive")
	}

	amount := big.Int(value.Amount)
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(paymentCurrency.Divisibility)), nil)
	converted := new(big.Int).Mul(&amount, scale)
	converted.Quo(converted, &rateInt)
	return iwallet.NewAmount(converted), nil
}

// ConvertFiatAmount converts a fiat amount from one currency to another.
// amount is in smallest currency units (e.g. cents). Returns in smallest units of target.
func ConvertFiatAmount(amount int64, from, to string, erp ExchangeRateQuerier) (int64, error) {
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

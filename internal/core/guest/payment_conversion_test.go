// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package guest

import (
	"math/big"
	"testing"

	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type changingGuestExchangeRates struct {
	calls int
	rates []iwallet.Amount
}

func (r *changingGuestExchangeRates) GetRate(models.CurrencyCode, models.CurrencyCode, bool) (iwallet.Amount, error) {
	rate := r.rates[r.calls]
	r.calls++
	return rate, nil
}

func TestPaymentCoinConverter_FreezesOneRateForTotalAndAffiliateLines(t *testing.T) {
	rates := &changingGuestExchangeRates{rates: []iwallet.Amount{
		iwallet.NewAmount(100),
		iwallet.NewAmount(200),
	}}
	service := &GuestOrderAppService{exchangeRates: rates}
	btc, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)

	convert, err := service.paymentCoinConverter("USD", btc)
	require.NoError(t, err)

	total, err := convert(big.NewInt(1000))
	require.NoError(t, err)
	line, err := convert(big.NewInt(500))
	require.NoError(t, err)

	assert.Equal(t, "1000000000", total)
	assert.Equal(t, "500000000", line)
	assert.Equal(t, 1, rates.calls, "one checkout must use one exchange-rate snapshot")
}

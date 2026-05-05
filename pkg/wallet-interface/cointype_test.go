package wallet_interface

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoinType_FiatProviderID_RequiresCanonicalFormat(t *testing.T) {
	assert.Equal(t, "", CoinType("fiat:USD").FiatProviderID())
	assert.Equal(t, "", CoinType("STRIPE_USD").FiatProviderID())
	assert.Equal(t, "", CoinType("FIAT_STRIPE").FiatProviderID())
	assert.Equal(t, "", CoinType("PAYPAL_USD").FiatProviderID())
	assert.Equal(t, "stripe", CoinType("fiat:stripe:USD").FiatProviderID())
	assert.Equal(t, "paypal", CoinType("fiat:paypal:EUR").FiatProviderID())
}

func TestCoinType_FiatBaseCurrency(t *testing.T) {
	assert.Equal(t, "USD", CoinType("fiat:stripe:USD").FiatBaseCurrency())
	assert.Equal(t, "EUR", CoinType("fiat:paypal:EUR").FiatBaseCurrency())
	assert.Equal(t, "USD", CoinType("USD").FiatBaseCurrency())
}

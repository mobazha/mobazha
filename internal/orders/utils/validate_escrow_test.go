package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePaymentAmount_ExactMatch(t *testing.T) {
	err := validatePaymentAmount("1000000000000000000", "1000000000000000000")
	require.NoError(t, err)
}

func TestValidatePaymentAmount_Overpay(t *testing.T) {
	err := validatePaymentAmount("1000000000000000000", "2000000000000000000")
	require.NoError(t, err)
}

func TestValidatePaymentAmount_Underpay(t *testing.T) {
	err := validatePaymentAmount("1000000000000000000", "999999999999999999")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPaymentAmountInsufficient)
}

func TestValidatePaymentAmount_ZeroPaid(t *testing.T) {
	err := validatePaymentAmount("1000000000000000000", "0")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPaymentAmountInsufficient)
}

func TestValidatePaymentAmount_InvalidOrderAmount(t *testing.T) {
	err := validatePaymentAmount("not-a-number", "1000")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid order amount")
}

func TestValidatePaymentAmount_ZeroOrderAmount(t *testing.T) {
	err := validatePaymentAmount("0", "1000")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid order amount")
}

func TestValidatePaymentAmount_NegativeOrderAmount(t *testing.T) {
	err := validatePaymentAmount("-100", "1000")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid order amount")
}

func TestValidatePaymentAmount_InvalidPaidAmount(t *testing.T) {
	err := validatePaymentAmount("1000", "abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payment amount")
}

func TestValidatePaymentAmount_NegativePaidAmount(t *testing.T) {
	err := validatePaymentAmount("1000", "-500")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payment amount")
}

func TestValidatePaymentAmount_LargeWeiValues(t *testing.T) {
	// 10 ETH in wei
	order := "10000000000000000000"
	// 10.5 ETH in wei
	paid := "10500000000000000000"
	err := validatePaymentAmount(order, paid)
	require.NoError(t, err)
}

func TestValidatePaymentAmount_LargeWeiUnderpay(t *testing.T) {
	// 10 ETH in wei
	order := "10000000000000000000"
	// 9.99 ETH in wei
	paid := "9990000000000000000"
	err := validatePaymentAmount(order, paid)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPaymentAmountInsufficient)
}

func TestValidatePaymentAmount_SmallAmounts(t *testing.T) {
	err := validatePaymentAmount("1", "1")
	require.NoError(t, err)
}

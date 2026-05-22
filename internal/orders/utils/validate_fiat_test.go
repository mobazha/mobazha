package utils

import (
	"testing"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeOrder(amount, coin string) *pb.OrderOpen {
	return &pb.OrderOpen{
		Amount:      amount,
		PricingCoin: coin,
	}
}

func makePayment(amount, coin, txID string) *pb.PaymentSent {
	return &pb.PaymentSent{
		Amount:         amount,
		Coin:           coin,
		TransactionID:  txID,
		SettlementSpec: payment.NewFiatSpec().ToPaymentSent(),
	}
}

func TestValidateFiatPayment_ExactMatch(t *testing.T) {
	order := makeOrder("5000", "fiat:USD")
	payment := makePayment("5000", "fiat:USD", "pi_123")
	err := validateFiatPayment(order, payment)
	require.NoError(t, err)
}

func TestValidateFiatPayment_SlightOverpay(t *testing.T) {
	order := makeOrder("10000", "fiat:USD")
	payment := makePayment("10050", "fiat:USD", "pi_123")
	err := validateFiatPayment(order, payment)
	require.NoError(t, err)
}

func TestValidateFiatPayment_AtMinBoundary(t *testing.T) {
	order := makeOrder("10000", "fiat:USD")
	payment := makePayment("9900", "fiat:USD", "pi_123")
	err := validateFiatPayment(order, payment)
	require.NoError(t, err)
}

func TestValidateFiatPayment_AtMaxBoundary(t *testing.T) {
	order := makeOrder("10000", "fiat:USD")
	payment := makePayment("10100", "fiat:USD", "pi_123")
	err := validateFiatPayment(order, payment)
	require.NoError(t, err)
}

func TestValidateFiatPayment_AmountTooLow(t *testing.T) {
	order := makeOrder("10000", "fiat:USD")
	payment := makePayment("9800", "fiat:USD", "pi_123")
	err := validateFiatPayment(order, payment)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFiatAmountTooLow)
}

func TestValidateFiatPayment_AmountTooHigh(t *testing.T) {
	order := makeOrder("10000", "fiat:USD")
	payment := makePayment("10200", "fiat:USD", "pi_123")
	err := validateFiatPayment(order, payment)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFiatAmountTooHigh)
}

func TestValidateFiatPayment_CurrencyMatch(t *testing.T) {
	order := makeOrder("5000", "fiat:usd")
	payment := makePayment("5000", "fiat:USD", "pi_123")
	err := validateFiatPayment(order, payment)
	require.NoError(t, err)
}

func TestValidateFiatPayment_CurrencyMatch_ProviderPrefix(t *testing.T) {
	order := makeOrder("5000", "fiat:stripe:USD")
	payment := makePayment("5000", "fiat:USD", "pi_123")
	require.NoError(t, validateFiatPayment(order, payment))

	order2 := makeOrder("5000", "fiat:USD")
	payment2 := makePayment("5000", "fiat:stripe:USD", "pi_123")
	require.NoError(t, validateFiatPayment(order2, payment2))

	order3 := makeOrder("5000", "fiat:stripe:USD")
	payment3 := makePayment("5000", "fiat:stripe:USD", "pi_123")
	require.NoError(t, validateFiatPayment(order3, payment3))
}

func TestValidateFiatPayment_CurrencyMismatch(t *testing.T) {
	order := makeOrder("5000", "fiat:USD")
	payment := makePayment("5000", "fiat:EUR", "pi_123")
	err := validateFiatPayment(order, payment)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFiatCurrencyMismatch)
}

func TestValidateFiatPayment_NoTransactionID(t *testing.T) {
	order := makeOrder("5000", "fiat:USD")
	payment := makePayment("5000", "fiat:USD", "")
	err := validateFiatPayment(order, payment)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFiatNoTransactionID)
}

func TestValidateFiatPayment_CrossCurrencyRejected(t *testing.T) {
	order := makeOrder("10000", "fiat:USD")
	payment := makePayment("10000", "fiat:EUR", "pi_123")
	err := validateFiatPayment(order, payment)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFiatCurrencyMismatch)
}

func TestValidateFiatPayment_ZeroOrderAmount(t *testing.T) {
	order := makeOrder("0", "fiat:USD")
	payment := makePayment("5000", "fiat:USD", "pi_123")
	err := validateFiatPayment(order, payment)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid order amount")
}

func TestValidateFiatPayment_InvalidOrderAmount(t *testing.T) {
	order := makeOrder("not-a-number", "fiat:USD")
	payment := makePayment("5000", "fiat:USD", "pi_123")
	err := validateFiatPayment(order, payment)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid order amount")
}

func TestValidateFiatPayment_PayPal(t *testing.T) {
	order := makeOrder("2500", "fiat:USD")
	payment := makePayment("2500", "fiat:USD", "PAYID-123")
	err := validateFiatPayment(order, payment)
	require.NoError(t, err)
}

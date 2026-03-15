package utils

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

var (
	ErrFiatAmountTooLow     = errors.New("fiat payment amount below acceptable threshold")
	ErrFiatAmountTooHigh    = errors.New("fiat payment amount above acceptable threshold")
	ErrFiatCurrencyMismatch = errors.New("fiat payment currency does not match order currency")
	ErrFiatNoTransactionID  = errors.New("fiat payment transaction ID is empty")
)

// validateFiatPayment checks that the fiat PaymentSent matches the OrderOpen:
// amount within tolerance, currency match, and non-empty transaction ID.
func validateFiatPayment(order *pb.OrderOpen, paymentSent *pb.PaymentSent) error {
	if paymentSent.TransactionID == "" {
		return ErrFiatNoTransactionID
	}

	orderCoin := strings.ToUpper(order.PricingCoin)
	paidCoin := strings.ToUpper(paymentSent.Coin)

	if orderCoin != paidCoin &&
		iwallet.CoinType(orderCoin).FiatBaseCurrency() != iwallet.CoinType(paidCoin).FiatBaseCurrency() {
		return fmt.Errorf("%w: order=%q paid=%q", ErrFiatCurrencyMismatch, order.PricingCoin, paymentSent.Coin)
	}

	expected, ok := new(big.Float).SetString(order.Amount)
	if !ok || expected.Sign() <= 0 {
		return fmt.Errorf("invalid order amount: %q", order.Amount)
	}

	paid, ok := new(big.Float).SetString(paymentSent.Amount)
	if !ok || paid.Sign() < 0 {
		return fmt.Errorf("invalid payment amount: %q", paymentSent.Amount)
	}

	minRatio := fiatSameCurrencyMinRatio
	maxRatio := fiatSameCurrencyMaxRatio

	minAmount := new(big.Float).Mul(expected, big.NewFloat(minRatio))
	maxAmount := new(big.Float).Mul(expected, big.NewFloat(maxRatio))

	if paid.Cmp(minAmount) < 0 {
		return fmt.Errorf("%w: paid %s < min %s (%.0f%% of %s)",
			ErrFiatAmountTooLow, paymentSent.Amount,
			minAmount.Text('f', 0), minRatio*100, order.Amount)
	}
	if paid.Cmp(maxAmount) > 0 {
		return fmt.Errorf("%w: paid %s > max %s (%.0f%% of %s)",
			ErrFiatAmountTooHigh, paymentSent.Amount,
			maxAmount.Text('f', 0), maxRatio*100, order.Amount)
	}

	return nil
}

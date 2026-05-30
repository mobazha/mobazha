package adapters

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func assertPaymentMessageParams(params payment.PaymentMessageParams) (*pb.OrderOpen, *pb.PaymentSent, error) {
	if params.OrderOpen == nil {
		return nil, nil, errors.New("PaymentMessageParams.OrderOpen is required")
	}
	if params.PaymentSent == nil {
		return nil, nil, errors.New("PaymentMessageParams.PaymentSent is required")
	}
	return params.OrderOpen, params.PaymentSent, nil
}

// validatePaymentMessageAmount validates PaymentSent.Amount against a single
// deterministic source. Address-monitored cross-currency routes must provide the
// locked payment-intent amount; same-currency direct routes may compare against
// OrderOpen.Amount because both values are in the same smallest unit.
func validatePaymentMessageAmount(params payment.PaymentMessageParams) error {
	order, paymentSent, err := assertPaymentMessageParams(params)
	if err != nil {
		return err
	}

	pricingCoin := normalizedPricingCurrencyCode(order.PricingCoin)
	if pricingCoin == "" {
		return errors.New("order pricing coin is required")
	}

	expectedAmount := strings.TrimSpace(params.ExpectedPaymentAmount)
	expectedCoin := strings.TrimSpace(params.ExpectedPaymentCoin)
	if expectedAmount != "" {
		if err := validateExpectedPaymentCoin(expectedCoin, paymentSent.Coin); err != nil {
			return err
		}
		return utils.ValidatePaymentAmount(expectedAmount, paymentSent.Amount)
	}

	paymentCoin, err := iwallet.CoinType(paymentSent.Coin).PricingCurrencyCode()
	if err != nil {
		return fmt.Errorf("invalid payment coin: %w", err)
	}
	if pricingCoin == paymentCoin {
		return utils.ValidatePaymentAmount(order.Amount, paymentSent.Amount)
	}
	return fmt.Errorf("locked expected payment amount is required for cross-currency payment: pricing %s paid in %s", pricingCoin, paymentCoin)
}

func normalizedPricingCurrencyCode(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if normalized, ok := payment.NormalizeSettlementPaymentCoin(trimmed); ok {
		if code, err := iwallet.CoinType(normalized).PricingCurrencyCode(); err == nil {
			return code
		}
	}
	return strings.ToUpper(trimmed)
}

func validateExpectedPaymentCoin(expectedCoin, actualCoin string) error {
	if expectedCoin == "" {
		return errors.New("expected payment coin is required")
	}
	expected, ok := payment.NormalizeSettlementPaymentCoin(expectedCoin)
	if !ok {
		return fmt.Errorf("invalid expected payment coin %q", expectedCoin)
	}
	actual, ok := payment.NormalizeSettlementPaymentCoin(actualCoin)
	if !ok {
		return fmt.Errorf("invalid payment coin %q", actualCoin)
	}
	if expected != actual {
		return fmt.Errorf("payment coin mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

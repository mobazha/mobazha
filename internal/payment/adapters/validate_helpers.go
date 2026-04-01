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
	orderOpen, ok := params.OrderOpen.(*pb.OrderOpen)
	if !ok || orderOpen == nil {
		return nil, nil, errors.New("PaymentMessageParams.OrderOpen must be *pb.OrderOpen")
	}
	paymentSent, ok := params.PaymentSent.(*pb.PaymentSent)
	if !ok || paymentSent == nil {
		return nil, nil, errors.New("PaymentMessageParams.PaymentSent must be *pb.PaymentSent")
	}
	return orderOpen, paymentSent, nil
}

// validatePaymentAmountCrossCurrency validates the payment amount against the
// order amount, but only when both are in the same currency. For cross-currency
// payments (e.g. USD-priced order paid in ETH), the amounts are in different
// units and cannot be compared directly — the escrow contract enforces the
// locked amount on-chain.
func validatePaymentAmountCrossCurrency(order *pb.OrderOpen, paymentSent *pb.PaymentSent) error {
	pricingCoin := strings.ToUpper(strings.TrimSpace(order.PricingCoin))
	paymentCoin, err := iwallet.CoinType(paymentSent.Coin).PricingCurrencyCode()
	if err != nil {
		return fmt.Errorf("invalid payment coin: %w", err)
	}
	if pricingCoin == "" || pricingCoin == paymentCoin {
		return utils.ValidatePaymentAmount(order.Amount, paymentSent.Amount)
	}
	return nil
}

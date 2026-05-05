package wallet_interface

import (
	"fmt"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

// CanonicalPaymentCoinFromPaymentSent validates and returns the canonical
// payment coin from a PaymentSent protobuf message.
func CanonicalPaymentCoinFromPaymentSent(paymentSent *pb.PaymentSent) (CoinType, error) {
	if paymentSent == nil {
		return "", fmt.Errorf("payment sent message is nil")
	}

	coinType := CoinType(paymentSent.Coin)
	if err := coinType.ValidateCanonicalPaymentCoin(); err != nil {
		return "", fmt.Errorf("invalid payment coin %q: %w", paymentSent.Coin, err)
	}

	return coinType, nil
}

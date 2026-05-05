package contracts

import (
	"context"
	"errors"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ErrPaymentAddressMismatch indicates that a payment's on-chain address does
// not match the expected address stored in the order.
var ErrPaymentAddressMismatch = errors.New("payment address mismatch")

// Deprecated: Use ErrPaymentAddressMismatch. Kept for backward compatibility during migration.
var ErrPaymentAddressMiss = ErrPaymentAddressMismatch

// VerifiedPayment carries the result of a successful fetch+verify cycle.
type VerifiedPayment struct {
	Transaction iwallet.Transaction
	CoinType    iwallet.CoinType
}

// PaymentVerifier abstracts payment message validation and on-chain
// transaction verification. OrderAppService depends on this interface
// (not the concrete PaymentVerificationService) to break the circular
// dependency between order/ and payment/ sub-packages.
type PaymentVerifier interface {
	ValidateMessage(
		coinType iwallet.CoinType,
		orderOpen *pb.OrderOpen,
		paymentSent *pb.PaymentSent,
		escrowTimeoutHours uint32,
	) error

	FetchTransaction(
		ctx context.Context,
		coinType iwallet.CoinType,
		txID string,
		providerID string,
	) (*iwallet.Transaction, error)

	FetchAndVerify(
		ctx context.Context,
		orderOpen *pb.OrderOpen,
		paymentSent *pb.PaymentSent,
		paymentAddress string,
	) (*VerifiedPayment, error)
}

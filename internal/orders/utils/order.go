package utils

import (
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// GetOrderEscrowInfo is the internal compatibility wrapper around the public
// shared projection used by first-party distribution modules.
func GetOrderEscrowInfo(orderOpen *pb.OrderOpen, paymentSent *pb.PaymentSent, testnet bool) (iwallet.EscrowInfo, error) {
	return payment.OrderEscrowInfo(orderOpen, paymentSent, testnet)
}

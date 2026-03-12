package adapters

import (
	"errors"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
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

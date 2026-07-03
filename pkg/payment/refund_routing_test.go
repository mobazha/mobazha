package payment

import (
	"testing"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestBuyerDeclaredRefundAddress_OrderWinsOverPaymentSent(t *testing.T) {
	order := &models.Order{RefundAddress: "0xorder"}
	ps := &pb.PaymentSent{RefundAddress: "0xlegacy"}
	require.Equal(t, "0xorder", BuyerDeclaredRefundAddress(order, ps))
}

func TestBuyerDeclaredRefundAddress_FallsBackToPaymentSent(t *testing.T) {
	order := &models.Order{}
	ps := &pb.PaymentSent{RefundAddress: "0xdeclared"}
	require.Equal(t, "0xdeclared", BuyerDeclaredRefundAddress(order, ps))
}

func TestBuyerDeclaredRefundAddress_EmptyWhenUnset(t *testing.T) {
	require.Equal(t, "", BuyerDeclaredRefundAddress(&models.Order{}, &pb.PaymentSent{}))
}

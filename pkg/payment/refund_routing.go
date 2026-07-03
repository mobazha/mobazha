package payment

import (
	"strings"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/models"
)

// BuyerDeclaredRefundAddress returns the buyer-controlled refund destination.
// Order-local RefundAddress wins over PaymentSent so post-payment API updates
// override stale inferred values in legacy PaymentSent envelopes.
func BuyerDeclaredRefundAddress(order *models.Order, paymentSent *pb.PaymentSent) string {
	if order != nil {
		if addr := strings.TrimSpace(order.RefundAddress); addr != "" {
			return addr
		}
	}
	if paymentSent != nil {
		if addr := strings.TrimSpace(paymentSent.RefundAddress); addr != "" {
			return addr
		}
	}
	return ""
}

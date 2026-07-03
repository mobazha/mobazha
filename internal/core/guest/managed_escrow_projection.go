package guest

import (
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
)

// ManagedEscrowGuestProjection validates and copies Core guest-order state for
// a provider projector without exposing the mutable model.
func ManagedEscrowGuestProjection(order *models.GuestOrder) (distribution.ManagedEscrowGuestProjection, error) {
	if order == nil {
		return distribution.ManagedEscrowGuestProjection{}, fmt.Errorf("managed escrow guest projection: order is nil")
	}
	orderID := strings.TrimSpace(order.OrderToken)
	paymentCoin := strings.TrimSpace(order.PaymentCoin)
	paymentAmount := strings.TrimSpace(order.PaymentAmount)
	paymentAddress := strings.TrimSpace(order.PaymentAddress)
	metadata := order.ManagedEscrowGuestMetadata()
	if orderID == "" || paymentCoin == "" || paymentAmount == "" || paymentAddress == "" || len(metadata) == 0 {
		return distribution.ManagedEscrowGuestProjection{}, fmt.Errorf("managed escrow guest projection: complete order, coin, amount, address, and metadata are required")
	}
	return distribution.ManagedEscrowGuestProjection{
		OrderID: orderID, PaymentCoin: paymentCoin, PaymentAmount: paymentAmount,
		PaymentAddress: paymentAddress, Metadata: metadata, ExpiresAt: order.ExpiresAt,
	}, nil
}

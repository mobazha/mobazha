package payment

import (
	"math/big"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

// CancelableAutoConfirmReady reports whether a verified CANCELABLE payment has
// the settlement inputs required for auto-confirm (e.g. UTXO release needs
// PaymentSent funding facts). ManagedEscrow and Solana monitored paths do not require
// UTXO-style funding facts.
func CancelableAutoConfirmReady(order *models.Order, paymentSent *pb.PaymentSent) bool {
	if order == nil || paymentSent == nil {
		return false
	}
	if UsesUTXOScriptEscrow(order, paymentSent) {
		return CountUsableUTXOFundingFacts(paymentSent) > 0
	}
	return true
}

// CancelablePaymentReadyEvent builds a CancelablePaymentReady event when the
// vendor order is verified and settlement inputs are ready. Returns nil when
// emission should be skipped.
func CancelablePaymentReadyEvent(order *models.Order, ps *pb.PaymentSent, total *big.Int) *events.CancelablePaymentReady {
	if order == nil || ps == nil || order.Role() != models.RoleVendor || !order.IsPaymentVerified() {
		return nil
	}
	method, ok := ResolvedPaymentMethod(order, ps)
	if !ok || !MethodIsCancelable(method) || !CancelableAutoConfirmReady(order, ps) {
		return nil
	}

	amount := cancelableReadyAmount(ps, total)
	return &events.CancelablePaymentReady{
		TenantID:      order.TenantID,
		OrderID:       order.ID.String(),
		TransactionID: ps.TransactionID,
		Coin:          ps.Coin,
		Amount:        amount,
	}
}

func cancelableReadyAmount(ps *pb.PaymentSent, total *big.Int) string {
	if total != nil {
		return total.String()
	}
	if ps != nil {
		if amount := strings.TrimSpace(ps.Amount); amount != "" {
			return amount
		}
	}
	return "0"
}

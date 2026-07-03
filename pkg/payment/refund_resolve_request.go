package payment

import (
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// RefundResolveRequest bundles the evidence needed to resolve a buyer refund
// destination on the local node. LocalRefundPreferences must be loaded by the
// caller from UserPreferences; it is only applied when the order role is buyer.
type RefundResolveRequest struct {
	Order                  *models.Order
	PaymentSent            *pb.PaymentSent
	Coin                   iwallet.CoinType
	Observations           []models.PaymentObservation
	PayFromCustodial       bool
	LocalRefundPreferences map[string]string
}

// AccountRefundAddressesForBuyerRole returns buyer account defaults only when
// the local node is acting in buyer role for the order. Seller/moderator nodes
// must not apply locally stored preferences (they belong to the wrong party).
func AccountRefundAddressesForBuyerRole(order *models.Order, prefs map[string]string) map[string]string {
	if order == nil || order.Role() != models.RoleBuyer || len(prefs) == 0 {
		return nil
	}
	return prefs
}

// Resolve runs the canonical buyer refund resolver with account defaults gated
// by order role.
func (r RefundResolveRequest) Resolve() RefundResolveResult {
	return ResolveBuyerRefundAddress(ResolveBuyerRefundAddressParams{
		Order:                  r.Order,
		PaymentSent:            r.PaymentSent,
		Coin:                   r.Coin,
		Observations:           r.Observations,
		PayFromCustodial:       r.PayFromCustodial,
		AccountRefundAddresses: AccountRefundAddressesForBuyerRole(r.Order, r.LocalRefundPreferences),
	})
}

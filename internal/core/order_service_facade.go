package core

import (
	"context"
	"strings"

	"github.com/mobazha/mobazha3.0/internal/core/order"
	corepayment "github.com/mobazha/mobazha3.0/internal/core/payment"
	"github.com/mobazha/mobazha3.0/internal/core/settlement"
	"github.com/mobazha/mobazha3.0/pkg/models"
	paypkg "github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// orderServiceFacade composes OrderAppService + PaymentAppService + SettlementService
// to satisfy contracts.OrderService. Most methods are promoted from the embedded
// OrderAppService; bridged methods are defined explicitly below.
type orderServiceFacade struct {
	*order.OrderAppService
	payment    *corepayment.PaymentAppService
	settlement *settlement.SettlementService
}

func (f *orderServiceFacade) GetOrderInfo(orderID models.OrderID, coinType iwallet.CoinType) (*models.OrderInfo, error) {
	return f.payment.GetOrderInfo(orderID, coinType)
}

// GetConfirmOrderInstructions bridges the legacy client-signed confirm flow.
// ManagedEscrow-backed EVM callers should use ExecuteSettlementAction instead.
func (f *orderServiceFacade) GetConfirmOrderInstructions(orderID models.OrderID, initiatorAddress string, payoutAddress string) (coinType iwallet.CoinType, instructions any, err error) {
	return f.settlement.GetConfirmOrderInstructions(orderID, initiatorAddress, payoutAddress)
}

// ExecuteSettlementAction is the default settlement surface for backend-
// submitted routes such as ManagedEscrow-backed EVM.
func (f *orderServiceFacade) ExecuteSettlementAction(ctx context.Context, action string, orderID models.OrderID, payoutAddr string) (*paypkg.ActionResult, iwallet.CoinType, error) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "complete":
		return f.OrderAppService.ExecuteSettlementCompleteAction(ctx, orderID)
	case "dispute_release":
		return f.OrderAppService.ExecuteSettlementDisputeReleaseAction(ctx, orderID)
	default:
		return f.settlement.ExecuteSettlementAction(ctx, action, orderID, payoutAddr)
	}
}

func (f *orderServiceFacade) ExecuteCollectiblePrimarySaleRelease(ctx context.Context, orderID models.OrderID, payoutAddr string) (*paypkg.ActionResult, iwallet.CoinType, error) {
	return f.settlement.ExecuteCollectiblePrimarySaleRelease(ctx, orderID, payoutAddr)
}

func (f *orderServiceFacade) GetSettlementActionStatus(ctx context.Context, action string, orderID models.OrderID, actionID string) (*paypkg.ActionStatus, iwallet.CoinType, error) {
	return f.settlement.GetSettlementActionStatus(ctx, action, orderID, actionID)
}

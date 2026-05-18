//go:build !private_distribution

package core

import (
	"context"

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

func (f *orderServiceFacade) GetConfirmOrderInstructions(orderID models.OrderID, initiatorAddress string, payoutAddress string) (coinType iwallet.CoinType, instructions any, err error) {
	return f.settlement.GetConfirmOrderInstructions(orderID, initiatorAddress, payoutAddress)
}

func (f *orderServiceFacade) ExecuteSettlementAction(ctx context.Context, action string, orderID models.OrderID, payoutAddr string) (*paypkg.ActionResult, iwallet.CoinType, error) {
	return f.settlement.ExecuteSettlementAction(ctx, action, orderID, payoutAddr)
}

func (f *orderServiceFacade) GetSettlementActionStatus(ctx context.Context, action string, orderID models.OrderID, actionID string) (*paypkg.ActionStatus, iwallet.CoinType, error) {
	return f.settlement.GetSettlementActionStatus(ctx, action, orderID, actionID)
}

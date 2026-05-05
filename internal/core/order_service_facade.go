//go:build !private_distribution

package core

import (
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// orderServiceFacade composes OrderAppService + PaymentAppService + SettlementService
// to satisfy contracts.OrderService. Most methods are promoted from the embedded
// OrderAppService; bridged methods are defined explicitly below.
type orderServiceFacade struct {
	*OrderAppService
	payment    *PaymentAppService
	settlement *SettlementService
}

func (f *orderServiceFacade) GetOrderInfo(orderID models.OrderID, coinType iwallet.CoinType) (*models.OrderInfo, error) {
	return f.payment.GetOrderInfo(orderID, coinType)
}

func (f *orderServiceFacade) GetConfirmOrderInstructions(orderID models.OrderID, initiatorAddress string, payoutAddress string) (coinType iwallet.CoinType, instructions any, err error) {
	return f.settlement.GetConfirmOrderInstructions(orderID, initiatorAddress, payoutAddress)
}

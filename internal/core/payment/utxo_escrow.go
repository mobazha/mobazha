package payment

import "errors"

// CancelPartialPayment delegates to SettlementService.
// Part of contracts.WalletService interface.
func (s *PaymentAppService) CancelPartialPayment(orderID string) (txid string, refundedAmount uint64, err error) {
	if s.escrowOps == nil {
		return "", 0, errors.New("settlement service not initialized")
	}
	return s.escrowOps.CancelPartialPayment(orderID)
}

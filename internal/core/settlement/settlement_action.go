//go:build !private_distribution

package settlement

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ExecuteSettlementAction runs a chain escrow V2 lifecycle action for crypto orders.
//
// Supported actions (Phase PS minimal surface — unified payment architecture §7.2):
//   - "confirm" — cancelable payout / buyer acceptance path (delegates to ChainEscrowV2.Confirm).
//   - "cancel" — cancel / refund-before-ship path (delegates to ChainEscrowV2.Cancel).
//
// Fiat orders return ErrBadRequest — refunds remain on fiat provider APIs.
func (s *SettlementService) ExecuteSettlementAction(
	ctx context.Context,
	action string,
	orderID models.OrderID,
	payoutAddr string,
) (*payment.ActionResult, iwallet.CoinType, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "confirm" && action != "cancel" {
		return nil, "", fmt.Errorf("%w: unsupported settlement action %q (supported: confirm, cancel)",
			coreiface.ErrBadRequest, action)
	}

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return nil, "", err
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, "", fmt.Errorf("%w: payment not recorded for this order", coreiface.ErrBadRequest)
	}

	spec := paymentSent.GetSettlementSpec()
	if spec == nil {
		return nil, "", fmt.Errorf("%w: payment settlement spec is missing", coreiface.ErrBadRequest)
	}
	method := spec.GetMethod()
	if method == pb.PaymentSent_FIAT || iwallet.CoinType(paymentSent.Coin).IsFiatPayment() {
		return nil, "", fmt.Errorf("%w: fiat orders use provider-specific refund APIs", coreiface.ErrBadRequest)
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, "", err
	}

	if s.paymentRegistry == nil {
		return nil, coinType, fmt.Errorf("payment registry not initialized")
	}

	strategy, err := s.paymentRegistry.ForCoinV2(coinType)
	if err != nil {
		return nil, coinType, fmt.Errorf("no chain escrow for coin %s: %w", coinType, err)
	}

	params := payment.ActionParams{
		OrderID:       orderID.String(),
		PaymentCoin:   coinType.String(),
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     &order,
	}

	switch action {
	case "confirm":
		if !order.CanConfirm() {
			return nil, coinType, fmt.Errorf("%w: order cannot be confirmed in its current state",
				coreiface.ErrBadRequest)
		}
		if method != pb.PaymentSent_CANCELABLE {
			return nil, coinType, fmt.Errorf("%w: confirm settlement applies only to cancelable payments",
				coreiface.ErrBadRequest)
		}
		out := payoutAddr
		if out == "" {
			toAddress, gerr := s.GetPayoutAddress(coinType.String())
			if gerr != nil {
				return nil, coinType, fmt.Errorf("failed to get payout address: %w", gerr)
			}
			out = toAddress.String()
		}
		params.PayoutAddr = out
		result, cerr := strategy.Confirm(ctx, params)
		return s.normalizeSettlementActionResult(result, coinType, cerr)

	case "cancel":
		if !order.CanCancel() && !order.CanRefund() {
			return nil, coinType, fmt.Errorf("%w: order cannot be cancelled or refunded in its current state",
				coreiface.ErrBadRequest)
		}
		out := payoutAddr
		if out == "" {
			out = paymentSent.PayerAddress
		}
		params.PayoutAddr = out
		result, cerr := strategy.Cancel(ctx, params)
		return s.normalizeSettlementActionResult(result, coinType, cerr)

	default:
		return nil, coinType, fmt.Errorf("%w: unsupported settlement action", coreiface.ErrBadRequest)
	}
}

func (s *SettlementService) normalizeSettlementActionResult(
	result *payment.ActionResult,
	coinType iwallet.CoinType,
	err error,
) (*payment.ActionResult, iwallet.CoinType, error) {
	if err != nil {
		return nil, coinType, err
	}
	if result == nil {
		return nil, coinType, nil
	}
	if result.Mode == payment.ActionModeInstructionsRequired || result.Instructions != nil {
		return nil, coinType, fmt.Errorf("%w: settlement-actions only support backend-submitted flows for coin %s; use legacy instruction endpoints for client-signed chains",
			coreiface.ErrBadRequest, coinType)
	}
	return result, coinType, nil
}

// GetSettlementActionStatus returns the latest known state for a previously
// issued settlement action. actionID is the opaque poll key returned by
// ExecuteSettlementAction / ActionResult.ActionID.
func (s *SettlementService) GetSettlementActionStatus(
	ctx context.Context,
	action string,
	orderID models.OrderID,
	actionID string,
) (*payment.ActionStatus, iwallet.CoinType, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "confirm" && action != "cancel" && action != "complete" && action != "dispute_release" {
		return nil, "", fmt.Errorf("%w: unsupported settlement action %q (supported: confirm, cancel, complete, dispute_release)",
			coreiface.ErrBadRequest, action)
	}
	actionID = strings.TrimSpace(actionID)
	if actionID == "" {
		return nil, "", fmt.Errorf("%w: actionId is required", coreiface.ErrBadRequest)
	}

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return nil, "", err
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, "", fmt.Errorf("%w: payment not recorded for this order", coreiface.ErrBadRequest)
	}

	spec := paymentSent.GetSettlementSpec()
	if spec == nil {
		return nil, "", fmt.Errorf("%w: payment settlement spec is missing", coreiface.ErrBadRequest)
	}
	method := spec.GetMethod()
	if method == pb.PaymentSent_FIAT || iwallet.CoinType(paymentSent.Coin).IsFiatPayment() {
		return nil, "", fmt.Errorf("%w: fiat orders use provider-specific refund APIs", coreiface.ErrBadRequest)
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, "", err
	}
	if s.paymentRegistry == nil {
		return nil, coinType, fmt.Errorf("payment registry not initialized")
	}

	strategy, err := s.paymentRegistry.ForCoinV2(coinType)
	if err != nil {
		return nil, coinType, fmt.Errorf("no chain escrow for coin %s: %w", coinType, err)
	}
	status, err := strategy.GetActionStatus(ctx, actionID)
	if err != nil {
		if errors.Is(err, payment.ErrActionNotFound) {
			return nil, coinType, fmt.Errorf("%w: settlement action not found", coreiface.ErrNotFound)
		}
		if errors.Is(err, payment.ErrUnsupportedAction) {
			return nil, coinType, fmt.Errorf("%w: settlement action status unsupported for %s", coreiface.ErrBadRequest, coinType)
		}
		return nil, coinType, err
	}
	if status == nil {
		return nil, coinType, fmt.Errorf("%w: settlement action not found", coreiface.ErrNotFound)
	}
	if status.OrderID != "" && status.OrderID != orderID.String() {
		return nil, coinType, fmt.Errorf("%w: settlement action does not belong to order %s", coreiface.ErrBadRequest, orderID)
	}
	if status.SettlementAction != "" && status.SettlementAction != action {
		return nil, coinType, fmt.Errorf("%w: settlement action %s does not match requested action %s",
			coreiface.ErrBadRequest, status.SettlementAction, action)
	}
	return status, coinType, nil
}

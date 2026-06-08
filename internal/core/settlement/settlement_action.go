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
	"gorm.io/gorm"
)

// ExecuteSettlementAction runs a chain escrow V2 lifecycle action for crypto orders.
//
// Supported actions on this SettlementService surface:
//   - "confirm" — cancelable payout / buyer acceptance (ManagedEscrow/Solana relay).
//   - "cancel" — cancelable buyer cancel or pre-confirm moderated seller refund
//     (ManagedEscrow/Solana relay).
//   - "seller_decline_refund" — seller-authorized refund for chains whose
//     on-chain program separates seller decline from buyer cancel.
//
// MODERATED complete and dispute_release are handled by OrderAppService
// POST /v1/orders/{id}/settlement-actions/{complete|dispute-release}, not here.
// UTXO cancelable confirm/cancel still uses ConfirmOrder inline escrow release.
//
// Fiat orders return ErrBadRequest — refunds remain on fiat provider APIs.
func (s *SettlementService) ExecuteSettlementAction(
	ctx context.Context,
	action string,
	orderID models.OrderID,
	payoutAddr string,
) (*payment.ActionResult, iwallet.CoinType, error) {
	normalizedAction, err := normalizeSettlementAction(action)
	if err != nil {
		return nil, "", err
	}

	var order models.Order
	err = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return nil, "", err
	}

	return s.executeSettlementActionForOrder(ctx, normalizedAction, &order, payoutAddr)
}

func normalizeSettlementAction(action string) (string, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != payment.SettlementActionConfirm &&
		action != payment.SettlementActionCancel &&
		action != payment.SettlementActionSellerDeclineRefund {
		return "", fmt.Errorf("%w: unsupported settlement action %q (supported: confirm, cancel, seller_decline_refund)",
			coreiface.ErrBadRequest, action)
	}
	return action, nil
}

func (s *SettlementService) executeSettlementActionForOrder(
	ctx context.Context,
	action string,
	order *models.Order,
	payoutAddr string,
) (*payment.ActionResult, iwallet.CoinType, error) {
	normalizedAction, err := normalizeSettlementAction(action)
	if err != nil {
		return nil, "", err
	}
	action = normalizedAction

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

	params := payment.ActionParams{
		OrderID:       order.ID.String(),
		PaymentCoin:   coinType.String(),
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     order,
	}

	switch action {
	case "confirm":
		if !order.CanConfirm() {
			return nil, coinType, fmt.Errorf("%w: order cannot be confirmed in its current state",
				coreiface.ErrBadRequest)
		}
		if method != pb.PaymentSent_CANCELABLE {
			return &payment.ActionResult{Mode: payment.ActionModeCompleted}, coinType, nil
		}
		strategy, err := s.settlementActionStrategy(coinType)
		if err != nil {
			return nil, coinType, err
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
		if method != pb.PaymentSent_CANCELABLE && method != pb.PaymentSent_MODERATED {
			return &payment.ActionResult{Mode: payment.ActionModeCompleted}, coinType, nil
		}
		if method == pb.PaymentSent_MODERATED && order.SerializedOrderConfirmation != nil {
			return nil, coinType, fmt.Errorf("%w: moderated orders can only be cancelled before seller confirmation",
				coreiface.ErrBadRequest)
		}
		strategy, err := s.settlementActionStrategy(coinType)
		if err != nil {
			return nil, coinType, err
		}
		out := payoutAddr
		if out == "" {
			refundResult := payment.ResolveBuyerRefundAddress(payment.ResolveBuyerRefundAddressParams{
				Order:       order,
				PaymentSent: paymentSent,
				Coin:        coinType,
			})
			if !refundResult.Found() {
				return nil, coinType, fmt.Errorf("%w: %w: no buyer refund address available for cancel settlement (%s)",
					coreiface.ErrBadRequest, models.ErrRefundAddressRequired, refundResult.Reason)
			}
			out = refundResult.Address
		}
		params.PayoutAddr = out
		result, cerr := strategy.Cancel(ctx, params)
		return s.normalizeSettlementActionResult(result, coinType, cerr)

	case payment.SettlementActionSellerDeclineRefund:
		if order.Role() != models.RoleVendor {
			return nil, coinType, fmt.Errorf("%w: seller_decline_refund requires the seller node",
				coreiface.ErrBadRequest)
		}
		if !order.CanCancel() && !order.CanRefund() {
			return nil, coinType, fmt.Errorf("%w: order cannot be seller-declined and refunded in its current state",
				coreiface.ErrBadRequest)
		}
		if method != pb.PaymentSent_CANCELABLE && method != pb.PaymentSent_MODERATED {
			return &payment.ActionResult{Mode: payment.ActionModeCompleted}, coinType, nil
		}
		if method == pb.PaymentSent_MODERATED && order.SerializedOrderConfirmation != nil {
			return nil, coinType, fmt.Errorf("%w: moderated orders can only be seller-declined before seller confirmation",
				coreiface.ErrBadRequest)
		}
		strategy, err := s.settlementActionStrategy(coinType)
		if err != nil {
			return nil, coinType, err
		}
		refunder, ok := strategy.(payment.SellerDeclineRefunder)
		if !ok {
			return nil, coinType, fmt.Errorf("%w: coin %s does not support seller_decline_refund settlement action",
				coreiface.ErrBadRequest, coinType)
		}
		out := payoutAddr
		if out == "" {
			refundResult := payment.ResolveBuyerRefundAddress(payment.ResolveBuyerRefundAddressParams{
				Order:       order,
				PaymentSent: paymentSent,
				Coin:        coinType,
			})
			if !refundResult.Found() {
				return nil, coinType, fmt.Errorf("%w: %w: no buyer refund address available for seller_decline_refund settlement (%s)",
					coreiface.ErrBadRequest, models.ErrRefundAddressRequired, refundResult.Reason)
			}
			out = refundResult.Address
		}
		params.PayoutAddr = out
		result, rerr := refunder.SellerDeclineRefund(ctx, params)
		return s.normalizeSettlementActionResult(result, coinType, rerr)

	default:
		return nil, coinType, fmt.Errorf("%w: unsupported settlement action", coreiface.ErrBadRequest)
	}
}

func (s *SettlementService) settlementActionStrategy(coinType iwallet.CoinType) (payment.ChainEscrowV2, error) {
	if s.paymentRegistry == nil {
		return nil, fmt.Errorf("payment registry not initialized")
	}
	strategy, err := s.paymentRegistry.ForCoinV2(coinType)
	if err != nil {
		return nil, fmt.Errorf("no chain escrow for coin %s: %w", coinType, err)
	}
	return strategy, nil
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
	if action != payment.SettlementActionConfirm &&
		action != payment.SettlementActionCancel &&
		action != payment.SettlementActionSellerDeclineRefund &&
		action != payment.SettlementActionComplete &&
		action != payment.SettlementActionDisputeRelease {
		return nil, "", fmt.Errorf("%w: unsupported settlement action %q (supported: confirm, cancel, seller_decline_refund, complete, dispute_release)",
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
		if errors.Is(err, payment.ErrUnsupportedAction) {
			var storeErr error
			status, storeErr = s.lookupSettlementActionStatusFromStore(actionID)
			if storeErr != nil {
				if errors.Is(storeErr, gorm.ErrRecordNotFound) {
					return nil, coinType, fmt.Errorf("%w: settlement action not found", coreiface.ErrNotFound)
				}
				return nil, coinType, storeErr
			}
		} else if errors.Is(err, payment.ErrActionNotFound) {
			return nil, coinType, fmt.Errorf("%w: settlement action not found", coreiface.ErrNotFound)
		} else {
			return nil, coinType, err
		}
	}
	if status == nil {
		// V1-backed adapters (e.g. UTXO) have no action ledger; sync actions
		// are persisted in settlement_actions by OrderAppService.
		var storeErr error
		status, storeErr = s.lookupSettlementActionStatusFromStore(actionID)
		if storeErr != nil {
			if errors.Is(storeErr, gorm.ErrRecordNotFound) {
				return nil, coinType, fmt.Errorf("%w: settlement action not found", coreiface.ErrNotFound)
			}
			return nil, coinType, storeErr
		}
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

func (s *SettlementService) lookupSettlementActionStatusFromStore(actionID string) (*payment.ActionStatus, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var row models.SettlementAction
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", actionID).First(&row).Error
	})
	if err != nil {
		return nil, err
	}
	snap := row.Snapshot()
	return &payment.ActionStatus{
		State:            snap.State,
		TxHash:           snap.TxHash,
		Confirmations:    snap.Confirmations,
		LastError:        snap.LastError,
		OrderID:          row.OrderID,
		SettlementAction: snap.SettlementAction,
		RelayTaskID:      snap.RelayTaskID,
		SettlementCoin:   snap.SettlementCoin,
		GrossAmount:      snap.GrossAmount,
		PlannedLines:     snap.PlannedLines,
		ObservedLines:    snap.ObservedLines,
	}, nil
}

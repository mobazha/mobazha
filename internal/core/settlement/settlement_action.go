package settlement

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	nodepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"gorm.io/gorm"
)

// ExecuteSettlementAction runs a chain escrow V2 lifecycle action for crypto orders.
//
// Supported actions on this SettlementService surface:
//   - "confirm" — cancelable payout / buyer acceptance (managed EVM/Solana relay).
//   - "cancel" — cancelable buyer cancel or pre-confirm moderated seller refund
//     (managed EVM/Solana relay).
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
	// Explicit API actions and event-driven auto-confirm share this execution
	// path. Serialize the durable intent check and backend submission so the
	// same managed escrow nonce cannot be signed and relayed twice concurrently.
	s.settlementActionMu.Lock()
	defer s.settlementActionMu.Unlock()
	if order == nil {
		return nil, "", fmt.Errorf("%w: order is required", coreiface.ErrBadRequest)
	}
	var current models.Order
	if err := s.db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("id = ?", order.ID.String()).First(&current).Error; err != nil {
			return err
		}
		if action != payment.SettlementActionConfirm {
			return nil
		}
		requiresAttestation, err := orderextensions.RequiresAttestedSettlementTx(tx, order.ID.String())
		if err != nil {
			return err
		}
		if requiresAttestation {
			return fmt.Errorf("%w: extension-attested order requires conditional settlement", coreiface.ErrBadRequest)
		}
		return nil
	}); err != nil {
		return nil, "", err
	}
	return s.executeSettlementActionForOrderLocked(ctx, action, &current, payoutAddr)
}

func (s *SettlementService) executeSettlementActionForOrderLocked(
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
	if existing, err := s.existingSettlementActionResult(order.ID.String(), action, coinType); err != nil {
		return nil, coinType, err
	} else if existing != nil {
		return existing, coinType, nil
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
		affiliatePayout, payoutErr := s.sellerAffiliateSettlementPayout(ctx, order.ID, coinType)
		if payoutErr != nil {
			return nil, coinType, fmt.Errorf("resolve affiliate settlement payout: %w", payoutErr)
		}
		params.AffiliatePayout = affiliatePayout
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
		if err := s.applyFrozenSettlementAttemptActionParams(ctx, strategy, order, coinType, action, &params); err != nil {
			return nil, coinType, err
		}
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
			refundResult := nodepayment.ResolveBuyerRefundForLocalNode(s.db, order, paymentSent, coinType, nil, false)
			if !refundResult.Found() {
				return nil, coinType, fmt.Errorf("%w: %w: no buyer refund address available for cancel settlement (%s)",
					coreiface.ErrBadRequest, models.ErrRefundAddressRequired, refundResult.Reason)
			}
			out = refundResult.Address
		}
		params.PayoutAddr = out
		if err := s.applyFrozenSettlementAttemptActionParams(ctx, strategy, order, coinType, action, &params); err != nil {
			return nil, coinType, err
		}
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
		out := payoutAddr
		if out == "" {
			refundResult := nodepayment.ResolveBuyerRefundForLocalNode(s.db, order, paymentSent, coinType, nil, false)
			if !refundResult.Found() {
				return nil, coinType, fmt.Errorf("%w: %w: no buyer refund address available for seller_decline_refund settlement (%s)",
					coreiface.ErrBadRequest, models.ErrRefundAddressRequired, refundResult.Reason)
			}
			out = refundResult.Address
		}
		params.PayoutAddr = out
		if err := s.applyFrozenSettlementAttemptActionParams(ctx, strategy, order, coinType, action, &params); err != nil {
			return nil, coinType, err
		}
		var result *payment.ActionResult
		var rerr error
		if ok {
			// Some programs distinguish a seller-decline instruction from buyer
			// cancel at the contract level (for example managed Solana).
			result, rerr = refunder.SellerDeclineRefund(ctx, params)
		} else {
			// Managed EVM and other threshold-1 escrows use the same on-chain
			// refund transaction for both intents. Authorization is enforced
			// above by requiring the local vendor order role.
			result, rerr = strategy.Cancel(ctx, params)
		}
		return s.normalizeSettlementActionResult(result, coinType, rerr)

	default:
		return nil, coinType, fmt.Errorf("%w: unsupported settlement action", coreiface.ErrBadRequest)
	}
}

func (s *SettlementService) applyFrozenSettlementAttemptActionParams(
	ctx context.Context,
	strategy payment.ChainEscrowV2,
	order *models.Order,
	coinType iwallet.CoinType,
	action string,
	params *payment.ActionParams,
) error {
	if _, ok := strategy.(payment.AttemptSettlementActionAuthorizer); !ok || s == nil || s.db == nil || order == nil || params == nil {
		return nil
	}
	tenantID := strings.TrimSpace(order.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(s.nodeID)
	}
	var attempts []models.PaymentAttempt
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"tenant_id = ? AND order_id = ? AND kind = ? AND state = ?",
			tenantID, order.ID.String(), models.PaymentAttemptKindCryptoFundingTarget,
			models.PaymentAttemptFundingTargetReady,
		).Find(&attempts).Error
	}); err != nil {
		return err
	}
	if len(attempts) == 0 {
		return nil
	}
	if len(attempts) != 1 {
		return fmt.Errorf("%w: expected one ready funding attempt, found %d", models.ErrPaymentAttemptSettlementTermsConflict, len(attempts))
	}
	if attempts[0].Currency != coinType.String() {
		return fmt.Errorf("%w: ready funding attempt rail does not match settlement action", models.ErrPaymentAttemptSettlementTermsConflict)
	}
	if s.settlementSigner == nil {
		return fmt.Errorf("%w: settlement signer is unavailable", models.ErrPaymentAttemptSettlementTermsConflict)
	}
	attempt := attempts[0]
	terms, err := attempt.GetSettlementTerms()
	if err != nil || terms == nil {
		return fmt.Errorf("%w: frozen settlement terms are unavailable", models.ErrPaymentAttemptSettlementTermsConflict)
	}
	target, err := attempt.GetFundingTarget()
	if err != nil || target == nil {
		return fmt.Errorf("%w: frozen funding target is unavailable", models.ErrPaymentAttemptSettlementTermsConflict)
	}
	bundle, err := attempt.GetAuthorizationBundle()
	if err != nil || bundle == nil {
		return fmt.Errorf("%w: frozen authorization bundle is unavailable", models.ErrPaymentAttemptSettlementTermsConflict)
	}
	authorization := models.PaymentAttemptSettlementAuthorization{
		Version: models.SettlementAuthorizationVersion,
		Terms:   *terms, Target: *target, Authorization: *bundle,
	}
	if err := authorization.Validate(); err != nil {
		return err
	}
	var localRole models.SettlementParticipantRole
	var expectedPeerID string
	switch order.Role() {
	case models.RoleBuyer:
		localRole = models.SettlementParticipantBuyer
		expectedPeerID = terms.BuyerPeerID
	case models.RoleVendor:
		localRole = models.SettlementParticipantSeller
		expectedPeerID = terms.SellerPeerID
	default:
		return fmt.Errorf("%w: local order role is not a settlement participant", models.ErrPaymentAttemptSettlementTermsConflict)
	}
	var localOffer *models.SettlementKeyOffer
	for i := range bundle.Offers {
		if bundle.Offers[i].ParticipantRole == localRole && bundle.Offers[i].ParticipantPeerID == expectedPeerID {
			localOffer = &bundle.Offers[i]
			break
		}
	}
	if localOffer == nil {
		return fmt.Errorf("%w: local participant offer is unavailable", models.ErrPaymentAttemptSettlementTermsConflict)
	}
	keyRef := contracts.SettlementKeyRef{
		TenantID: attempt.TenantID, RailID: attempt.Currency,
		Purpose:     contracts.StandardOrderSettlementKeyPurpose + ":" + string(localRole),
		ReferenceID: attempt.AuthorizationContextID,
	}
	publicKey, _, err := paymentintent.SettlementPublicKeyForRail(ctx, s.settlementSigner, keyRef)
	if err != nil {
		return err
	}
	if !bytes.Equal(publicKey, localOffer.PublicKey) {
		return fmt.Errorf("%w: local settlement key does not match the frozen participant offer", models.ErrPaymentAttemptSettlementTermsConflict)
	}
	if err := applyFrozenAttemptEconomicParams(action, *terms, params); err != nil {
		return err
	}
	params.AttemptAuthorization = &authorization
	params.AttemptTenantID = attempt.TenantID
	params.AttemptLocalRole = localRole
	params.AttemptSequence = 1
	return nil
}

func applyFrozenAttemptEconomicParams(action string, terms models.PaymentAttemptSettlementTerms, params *payment.ActionParams) error {
	if params == nil {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	switch action {
	case payment.SettlementActionConfirm:
		if strings.TrimSpace(terms.SellerAddress) == "" {
			return models.ErrPaymentAttemptSettlementTermsConflict
		}
		params.PayoutAddr = terms.SellerAddress
		params.AffiliatePayout = nil
		if terms.Affiliate != nil && terms.Affiliate.Amount != "0" {
			params.AffiliatePayout = &models.AffiliateSettlementPayout{
				Address: terms.Affiliate.Address,
				Amount:  terms.Affiliate.Amount,
			}
		}
	case payment.SettlementActionCancel, payment.SettlementActionSellerDeclineRefund:
		// New attempts bind this address in every participant offer. Preserve
		// compatibility with already-ready non-Solana version-1 attempts, which
		// legitimately omitted it and must continue using the order-derived
		// refund destination already present in params.
		if frozenRefund := strings.TrimSpace(terms.BuyerRefundAddress); frozenRefund != "" {
			params.PayoutAddr = frozenRefund
		} else if strings.TrimSpace(params.PayoutAddr) == "" {
			return models.ErrPaymentAttemptSettlementTermsConflict
		}
		params.AffiliatePayout = nil
	}
	return nil
}

func (s *SettlementService) sellerAffiliateSettlementPayout(ctx context.Context, orderID models.OrderID, coinType iwallet.CoinType) (*models.AffiliateSettlementPayout, error) {
	if s == nil || s.sellerAffiliate == nil {
		return nil, nil
	}
	return s.sellerAffiliate.SettlementPayout(ctx, orderID.String(), coinType.String())
}

// existingSettlementActionResult makes backend settlement submission
// idempotent across API retries and automatic workers. Failed actions are not
// reused, so an operator or retry loop can recover after a genuine backend
// failure.
func (s *SettlementService) existingSettlementActionResult(orderID, action string, coinType iwallet.CoinType) (*payment.ActionResult, error) {
	var row models.SettlementAction
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("order_id = ? AND action_kind = ? AND state IN ?", orderID, action, []string{"submitting", "submitted", "confirmed"}).
			Order("created_at DESC").
			Limit(1).
			Find(&row).Error
	})
	if err != nil {
		return nil, fmt.Errorf("lookup existing settlement action for order %s action %s: %w", orderID, action, err)
	}
	if row.ActionID == "" {
		return nil, nil
	}
	return &payment.ActionResult{
		Mode:            payment.ActionModeSubmitted,
		ActionID:        row.ActionID,
		SubmittedTxHash: row.TxHash,
		SettlementCoin:  coinType.String(),
		GrossAmount:     row.GrossAmount,
		PlannedLines:    models.DecodeSettlementPayoutLines(row.PlannedLines),
	}, nil
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
	if status.SettlementAction != "" && !settlementActionStatusMatches(strategy, status.SettlementAction, action) {
		return nil, coinType, fmt.Errorf("%w: settlement action %s does not match requested action %s",
			coreiface.ErrBadRequest, status.SettlementAction, action)
	}
	return status, coinType, nil
}

func settlementActionStatusMatches(strategy payment.ChainEscrowV2, recordedAction, requestedAction string) bool {
	if recordedAction == requestedAction {
		return true
	}
	if requestedAction != payment.SettlementActionSellerDeclineRefund || recordedAction != payment.SettlementActionCancel {
		return false
	}
	// Managed EVM expresses a seller-authorized default refund with the same
	// on-chain cancel operation as a buyer cancellation. Chains exposing a
	// dedicated seller-decline instruction must retain the distinct action.
	_, hasDedicatedSellerDecline := strategy.(payment.SellerDeclineRefunder)
	return !hasDedicatedSellerDecline
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

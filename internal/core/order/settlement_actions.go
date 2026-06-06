//go:build !private_distribution

package order

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

const syncSettlementActionStaleAfter = 2 * time.Minute

func (s *OrderAppService) v2StrategyForCoin(coinType iwallet.CoinType) (payment.ChainEscrowV2, error) {
	if s.paymentRegistry == nil {
		return nil, fmt.Errorf("payment registry not initialized")
	}
	strategy, err := s.paymentRegistry.ForCoinV2(coinType)
	if err != nil {
		return nil, err
	}
	return strategy, nil
}

func (s *OrderAppService) signSettlementActionRelease(ctx context.Context, coinType iwallet.CoinType, action string, params payment.ActionParams) ([]*pb.Signature, bool, error) {
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return nil, false, err
	}
	actionSigner, ok := strategy.(payment.ActionSigner)
	if !ok {
		return nil, false, nil
	}
	ownerSigs, err := actionSigner.SignAction(ctx, action, params)
	if err != nil {
		return nil, true, err
	}
	out := make([]*pb.Signature, 0, len(ownerSigs))
	for _, sig := range ownerSigs {
		out = append(out, &pb.Signature{
			From:      []byte(sig.From),
			Signature: append([]byte(nil), sig.Signature...),
			Index:     sig.Index,
		})
	}
	return out, true, nil
}

func cloneEscrowRelease(release *pb.EscrowRelease) *pb.EscrowRelease {
	if release == nil {
		return nil
	}
	cloned, ok := proto.Clone(release).(*pb.EscrowRelease)
	if !ok {
		return nil
	}
	return cloned
}

func cloneDisputeRelease(release *pb.DisputeClose_ModeratedEscrowRelease) *pb.DisputeClose_ModeratedEscrowRelease {
	if release == nil {
		return nil
	}
	cloned, ok := proto.Clone(release).(*pb.DisputeClose_ModeratedEscrowRelease)
	if !ok {
		return nil
	}
	return cloned
}

func orderDataWithPaymentSent(orderID models.OrderID, paymentSent *pb.PaymentSent) (*models.Order, error) {
	if paymentSent == nil {
		return nil, fmt.Errorf("payment sent message is nil")
	}
	order := &models.Order{ID: orderID}
	if err := order.SetPaymentSent(paymentSent); err != nil {
		return nil, err
	}
	return order, nil
}

func orderDataWithContract(orderID models.OrderID, orderOpen *pb.OrderOpen, paymentSent *pb.PaymentSent) (*models.Order, error) {
	if orderOpen == nil {
		return nil, fmt.Errorf("order open message is nil")
	}
	order, err := orderDataWithPaymentSent(orderID, paymentSent)
	if err != nil {
		return nil, err
	}
	raw, err := (protojson.MarshalOptions{}).Marshal(orderOpen)
	if err != nil {
		return nil, err
	}
	order.SerializedOrderOpen = raw
	return order, nil
}

func actionStatusTxHash(ctx context.Context, strategy payment.ChainEscrowV2, actionID string) string {
	if strategy == nil || actionID == "" {
		return ""
	}
	status, err := strategy.GetActionStatus(ctx, actionID)
	if err != nil || status == nil {
		return ""
	}
	return status.TxHash
}

// actionRelayTxHash prefers the hash returned synchronously from relay
// submit, then falls back to GetActionStatus for recently recorded actions.
func actionRelayTxHash(ctx context.Context, strategy payment.ChainEscrowV2, result *payment.ActionResult) string {
	if result != nil && result.SubmittedTxHash != "" {
		return result.SubmittedTxHash
	}
	if result != nil && result.ActionID != "" {
		if h := actionStatusTxHash(ctx, strategy, result.ActionID); h != "" {
			return h
		}
	}
	return ""
}

// orderRequiresMonitoredSettlementActions reports moderated orders whose escrow
// release/complete must go through settlement-actions before domain handlers run.
func orderRequiresMonitoredSettlementActions(
	order *models.Order,
	paymentSent *pb.PaymentSent,
	coinType iwallet.CoinType,
	registry *payment.Registry,
) bool {
	if order == nil || paymentSent == nil || registry == nil {
		return false
	}
	method, ok := payment.ResolvedPaymentMethod(order, paymentSent)
	if !ok || !payment.MethodIsModerated(method) {
		return false
	}
	strategy, err := registry.ForCoinV2(coinType)
	if err != nil || strategy.Model() != payment.PaymentModelMonitored {
		return false
	}
	return true
}

func requireBackendSubmittedSettlementSpec(order *models.Order, paymentSent *pb.PaymentSent) (payment.SettlementSpec, error) {
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	if !ok {
		return payment.SettlementSpec{}, fmt.Errorf("%w: payment settlement spec is required", coreiface.ErrBadRequest)
	}
	if !orderEscrowUsesBackendSubmittedSettlementRelease(spec) {
		return payment.SettlementSpec{}, fmt.Errorf("%w: escrow type %q must use settlement-actions; client-signed legacy routes are retired",
			coreiface.ErrBadRequest, spec.EscrowType)
	}
	return spec, nil
}

func errRetiredClientSignedModeratedSettlement(action string) error {
	return fmt.Errorf("%w: moderated client-signed %s is retired; use POST /v1/orders/{orderID}/settlement-actions/%s",
		coreiface.ErrBadRequest, action, payment.SettlementActionPathSegment(action))
}

// orderEscrowUsesBackendSubmittedSettlementRelease reports escrow types whose
// moderated release/complete flows use settlement-actions (relay or sync).
func orderEscrowUsesBackendSubmittedSettlementRelease(spec payment.SettlementSpec) bool {
	return spec.UsesManagedEscrow() || spec.UsesSolanaEscrow() || spec.UsesUTXOScript()
}

// orderEscrowUsesRelaySettlementRelease reports escrow types whose release is
// submitted asynchronously via relay + action store (ManagedEscrow, Solana Anchor).
func orderEscrowUsesRelaySettlementRelease(spec payment.SettlementSpec) bool {
	return spec.UsesManagedEscrow() || spec.UsesSolanaEscrow()
}

func settlementActionName(action models.SettlementActionSnapshot) string {
	name := strings.ToLower(strings.TrimSpace(action.SettlementAction))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(action.Action))
	}
	return name
}

// completeSettlementReleaseReady reports whether monitored complete release
// evidence exists. Completion needs a concrete release tx hash so the
// ORDER_COMPLETE message can carry auditable release info.
func completeSettlementReleaseReady(order *models.Order, txid iwallet.TransactionID) bool {
	return settlementReleaseReady(order, txid, "complete")
}

// completeSettlementReleasePending reports an in-flight settlement complete
// action that has not yet produced a tx hash.
func completeSettlementReleasePending(order *models.Order, txid iwallet.TransactionID) bool {
	return settlementReleasePending(order, txid, "complete")
}

func syncSettlementActionID(orderID, action string) string {
	return fmt.Sprintf("sync-%s-%s", action, orderID)
}

func staleSyncSettlementAction(actionID, state, txHash string, updatedAt time.Time, now time.Time) bool {
	if strings.TrimSpace(txHash) != "" || !strings.HasPrefix(strings.TrimSpace(actionID), "sync-") {
		return false
	}
	state = strings.ToLower(strings.TrimSpace(state))
	if state != "submitting" && state != "submitted" {
		return false
	}
	if updatedAt.IsZero() || now.Before(updatedAt) {
		return false
	}
	return now.Sub(updatedAt) > syncSettlementActionStaleAfter
}

func (s *OrderAppService) loadSyncBackendSettlementAction(orderID, action string) (*models.SettlementAction, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	actionID := syncSettlementActionID(orderID, action)
	var row models.SettlementAction
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", actionID).First(&row).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// beginSyncBackendSettlementAction reserves a deterministic sync action row
// before UTXO sign+broadcast so retries do not double-spend on chain.
func (s *OrderAppService) beginSyncBackendSettlementAction(
	orderID, action, settlementCoin, grossAmount string,
) (actionID string, existingTxHash string, err error) {
	if s == nil || s.db == nil {
		return "", "", fmt.Errorf("database not initialized")
	}
	actionID = syncSettlementActionID(orderID, action)
	existing, err := s.loadSyncBackendSettlementAction(orderID, action)
	if err != nil {
		return "", "", err
	}
	if existing != nil {
		if existing.TxHash != "" {
			return actionID, existing.TxHash, nil
		}
		state := strings.ToLower(strings.TrimSpace(existing.State))
		if state == "submitting" || state == "submitted" || state == "confirmed" {
			if staleSyncSettlementAction(existing.ActionID, existing.State, existing.TxHash, existing.UpdatedAt, time.Now().UTC()) {
				goto reserve
			}
			return "", "", fmt.Errorf("%w: settlement %s release is still pending; retry after tx hash is available",
				coreiface.ErrBadRequest, action)
		}
	}

reserve:
	now := time.Now().UTC()
	row := &models.SettlementAction{
		ActionID:       actionID,
		OrderID:        orderID,
		ActionKind:     action,
		State:          "submitting",
		SettlementCoin: settlementCoin,
		GrossAmount:    grossAmount,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if existing != nil {
		row.CreatedAt = existing.CreatedAt
	}
	if err := s.db.Update(func(tx database.Tx) error {
		return tx.Save(row)
	}); err != nil {
		return "", "", err
	}
	return actionID, "", nil
}

func (s *OrderAppService) confirmSyncBackendSettlementAction(actionID, txHash string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if txHash == "" {
		return fmt.Errorf("settlement action %s confirmed without tx hash", actionID)
	}
	now := time.Now().UTC()
	return s.db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(map[string]interface{}{
			"state":        "confirmed",
			"tx_hash":      txHash,
			"confirmed_at": now,
			"updated_at":   now,
		}, map[string]interface{}{
			"action_id = ?": actionID,
		}, &models.SettlementAction{})
		return err
	})
}

func (s *OrderAppService) failSyncBackendSettlementAction(actionID, reason string) {
	if s == nil || s.db == nil || strings.TrimSpace(actionID) == "" {
		return
	}
	reason = strings.TrimSpace(reason)
	if len(reason) > 2048 {
		reason = reason[:2048]
	}
	now := time.Now().UTC()
	_ = s.db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(map[string]interface{}{
			"state":      "failed",
			"last_error": reason,
			"updated_at": now,
		}, map[string]interface{}{
			"action_id = ?": actionID,
		}, &models.SettlementAction{})
		return err
	})
}

func errSettlementReleaseActionRequired(orderID models.OrderID, action string) error {
	return fmt.Errorf("%w: submit POST /v1/orders/%s/settlement-actions/%s before continuing",
		coreiface.ErrBadRequest, orderID, payment.SettlementActionPathSegment(action))
}

// disputeSettlementReleaseReady reports whether monitored dispute release
// evidence exists before accepting the arbitration payout.
func disputeSettlementReleaseReady(order *models.Order, txid iwallet.TransactionID) bool {
	return settlementReleaseReady(order, txid, "dispute_release")
}

// disputeSettlementReleasePending reports an in-flight settlement dispute
// release action that has not yet produced a tx hash.
func disputeSettlementReleasePending(order *models.Order, txid iwallet.TransactionID) bool {
	return settlementReleasePending(order, txid, "dispute_release")
}

func settlementReleaseReady(order *models.Order, _ iwallet.TransactionID, actionName string) bool {
	return settlementActionTxHash(order, actionName) != ""
}

func settlementReleasePending(order *models.Order, _ iwallet.TransactionID, actionName string) bool {
	if settlementActionTxHash(order, actionName) != "" {
		return false
	}
	if order == nil {
		return false
	}
	for _, action := range order.SettlementActions {
		if settlementActionName(action) != actionName {
			continue
		}
		state := strings.ToLower(strings.TrimSpace(action.State))
		if action.TxHash != "" {
			return false
		}
		if state == "submitting" || state == "submitted" || state == "confirmed" {
			return true
		}
	}
	return false
}

// evaluateMonitoredSettlementRelease checks pending/ready state for a backend
// settlement release action (complete or dispute_release).
func evaluateMonitoredSettlementRelease(
	order *models.Order,
	txid iwallet.TransactionID,
	actionName string,
) (resolvedTxid iwallet.TransactionID, releaseAlreadySubmitted bool, err error) {
	if settlementReleasePending(order, txid, actionName) {
		return "", false, fmt.Errorf("%w: settlement %s release is still pending; retry after tx hash is available",
			coreiface.ErrBadRequest, actionName)
	}
	if settlementReleaseReady(order, txid, actionName) {
		resolved := settlementActionTxHash(order, actionName)
		if resolved == "" {
			return "", false, nil
		}
		if txid != "" && txid != resolved {
			return "", false, fmt.Errorf("%w: txID does not match settlement %s release hash",
				coreiface.ErrBadRequest, actionName)
		}
		return resolved, true, nil
	}
	return "", false, nil
}

func settlementActionTxHash(order *models.Order, actionName string) iwallet.TransactionID {
	if order == nil {
		return ""
	}
	for _, action := range order.SettlementActions {
		if settlementActionName(action) != actionName {
			continue
		}
		if action.TxHash != "" {
			return iwallet.TransactionID(action.TxHash)
		}
		break
	}
	return ""
}

// existingMonitoredSettlementActionResult returns an in-flight or completed backend
// settlement action so ExecuteSettlement*Action endpoints stay idempotent on retry.
func existingMonitoredSettlementActionResult(order *models.Order, actionName string) (*payment.ActionResult, bool) {
	if order == nil {
		return nil, false
	}
	var pending *payment.ActionResult
	for _, action := range order.SettlementActions {
		if settlementActionName(action) != actionName {
			continue
		}
		if action.TxHash != "" {
			return &payment.ActionResult{
				Mode:            payment.ActionModeSubmitted,
				ActionID:        action.ActionID,
				SubmittedTxHash: action.TxHash,
			}, true
		}
		state := strings.ToLower(strings.TrimSpace(action.State))
		if state == "submitting" || state == "submitted" || state == "confirmed" {
			if staleSyncSettlementAction(action.ActionID, action.State, action.TxHash, action.UpdatedAt, time.Now().UTC()) {
				continue
			}
			pending = &payment.ActionResult{
				Mode:     payment.ActionModeSubmitted,
				ActionID: action.ActionID,
			}
		}
	}
	if pending != nil {
		return pending, true
	}
	return nil, false
}

// ExecuteSettlementCompleteAction submits backend escrow release for moderated
// ManagedEscrow / Solana Anchor (relay) or UTXO (sync sign+broadcast) orders via
// settlement-actions/complete.
func (s *OrderAppService) ExecuteSettlementCompleteAction(
	ctx context.Context,
	orderID models.OrderID,
) (*payment.ActionResult, iwallet.CoinType, error) {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return nil, "", err
	}

	if !order.CanComplete() {
		return nil, "", fmt.Errorf("%w: order cannot be completed in its current state",
			coreiface.ErrBadRequest)
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, "", fmt.Errorf("%w: payment not recorded for this order", coreiface.ErrBadRequest)
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, "", err
	}

	if !orderRequiresMonitoredSettlementActions(&order, paymentSent, coinType, s.paymentRegistry) {
		method, ok := payment.ResolvedPaymentMethod(&order, paymentSent)
		if ok && payment.MethodIsModerated(method) {
			return nil, coinType, errRetiredClientSignedModeratedSettlement("complete")
		}
		return &payment.ActionResult{Mode: payment.ActionModeCompleted}, coinType, nil
	}
	if _, err := requireBackendSubmittedSettlementSpec(&order, paymentSent); err != nil {
		return nil, coinType, err
	}

	if err := s.attachSettlementActions(&order); err != nil {
		return nil, coinType, fmt.Errorf("load settlement actions for order %s: %w", orderID, err)
	}
	if existing, ok := existingMonitoredSettlementActionResult(&order, "complete"); ok {
		return existing, coinType, nil
	}

	shipments, err := order.OrderShipmentMessages()
	if err != nil {
		return nil, coinType, fmt.Errorf("order shipment messages: %w", err)
	}
	if len(shipments) == 0 || shipments[0].ReleaseInfo == nil {
		return nil, coinType, fmt.Errorf("%w: shipment release info is missing", coreiface.ErrBadRequest)
	}

	result, release, tx, handled, err := s.runMonitoredSettlementComplete(
		ctx,
		&order,
		coinType,
		paymentSent,
		shipments[0].ReleaseInfo,
	)
	if err != nil {
		return nil, coinType, err
	}
	if !handled {
		return nil, coinType, fmt.Errorf("%w: settlement complete is not supported for coin %s",
			coreiface.ErrBadRequest, coinType)
	}
	if result == nil {
		result = &payment.ActionResult{Mode: payment.ActionModeCompleted}
	}
	if result.Mode == payment.ActionModeInstructionsRequired || result.Instructions != nil {
		return nil, coinType, fmt.Errorf("%w: settlement-actions only support backend-submitted flows for coin %s",
			coreiface.ErrBadRequest, coinType)
	}
	if result.SubmittedTxHash == "" {
		if tx != nil && tx.ID != "" {
			result.SubmittedTxHash = tx.ID.String()
		} else if release != nil && release.Txid != "" {
			result.SubmittedTxHash = release.Txid
		}
	}
	_ = release
	return result, coinType, nil
}

func (s *OrderAppService) runMonitoredSettlementComplete(
	ctx context.Context,
	order *models.Order,
	coinType iwallet.CoinType,
	paymentSent *pb.PaymentSent,
	releaseInfo *pb.EscrowRelease,
) (*payment.ActionResult, *pb.EscrowRelease, *iwallet.Transaction, bool, error) {
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return nil, nil, nil, false, err
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		return nil, nil, nil, false, nil
	}
	spec, err := requireBackendSubmittedSettlementSpec(order, paymentSent)
	if err != nil {
		return nil, nil, nil, true, err
	}
	if spec.UsesUTXOScript() {
		return s.runUTXOSyncSettlementComplete(ctx, order, coinType, paymentSent, releaseInfo)
	}
	if !orderEscrowUsesRelaySettlementRelease(spec) {
		return nil, nil, nil, false, nil
	}

	release := cloneEscrowRelease(releaseInfo)
	if release == nil {
		return nil, nil, nil, true, fmt.Errorf("settlement complete release info is nil")
	}

	result, err := strategy.Complete(ctx, payment.ActionParams{
		OrderID:       order.ID.String(),
		PaymentCoin:   string(coinType),
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     order,
		ReleaseInfo:   release,
	})
	if err != nil {
		return nil, nil, nil, true, err
	}

	txHash := actionRelayTxHash(ctx, strategy, result)
	if txHash == "" {
		txHash = release.Txid
	}
	var tx *iwallet.Transaction
	if txHash != "" {
		release.Txid = txHash
		tx = &iwallet.Transaction{ID: iwallet.TransactionID(txHash)}
	}
	return result, release, tx, true, nil
}

func (s *OrderAppService) runUTXOSyncSettlementComplete(
	ctx context.Context,
	order *models.Order,
	coinType iwallet.CoinType,
	paymentSent *pb.PaymentSent,
	releaseInfo *pb.EscrowRelease,
) (*payment.ActionResult, *pb.EscrowRelease, *iwallet.Transaction, bool, error) {
	_ = ctx
	orderID := order.ID.String()
	actionID, existingTxHash, err := s.beginSyncBackendSettlementAction(
		orderID, "complete", string(coinType), paymentSent.Amount,
	)
	if err != nil {
		return nil, nil, nil, true, err
	}
	if existingTxHash != "" {
		release := cloneEscrowRelease(releaseInfo)
		if release != nil {
			release.Txid = existingTxHash
		}
		tx := &iwallet.Transaction{ID: iwallet.TransactionID(existingTxHash)}
		return &payment.ActionResult{
			Mode:            payment.ActionModeCompleted,
			ActionID:        actionID,
			SubmittedTxHash: existingTxHash,
		}, release, tx, true, nil
	}

	wallet, err := s.multiwallet.WalletForCurrencyCode(string(coinType))
	if err != nil {
		s.failSyncBackendSettlementAction(actionID, err.Error())
		return nil, nil, nil, true, err
	}
	release, tx, err := s.executeUTXOSyncModeratedCompleteRelease(order, wallet, releaseInfo)
	if err != nil {
		s.failSyncBackendSettlementAction(actionID, err.Error())
		return nil, nil, nil, true, err
	}
	txHash := ""
	if tx != nil && tx.ID != "" {
		txHash = tx.ID.String()
	} else if release != nil {
		txHash = release.Txid
	}
	if err := s.confirmSyncBackendSettlementAction(actionID, txHash); err != nil {
		s.failSyncBackendSettlementAction(actionID, err.Error())
		return nil, nil, nil, true, err
	}
	return &payment.ActionResult{
		Mode:            payment.ActionModeCompleted,
		ActionID:        actionID,
		SubmittedTxHash: txHash,
	}, release, tx, true, nil
}

func (s *OrderAppService) submitSettlementCancelAction(ctx context.Context, order *models.Order, coinType iwallet.CoinType, paymentSent *pb.PaymentSent, payoutAddr string) (iwallet.TransactionID, *iwallet.Transaction, bool, error) {
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return "", nil, false, err
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		return "", nil, false, nil
	}
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	if !ok || !orderEscrowUsesRelaySettlementRelease(spec) {
		// UTXO cancelable confirm/cancel still uses ConfirmOrder / escrow inline release.
		return "", nil, false, nil
	}

	if payoutAddr == "" {
		if paymentSent.RefundAddress != "" {
			payoutAddr = paymentSent.RefundAddress
		} else if paymentSent.PayerAddress != "" {
			payoutAddr = paymentSent.PayerAddress
		}
	}

	result, err := strategy.Cancel(ctx, payment.ActionParams{
		OrderID:       order.ID.String(),
		PaymentCoin:   string(coinType),
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     order,
		PayoutAddr:    payoutAddr,
	})
	if err != nil {
		return "", nil, true, err
	}

	txHash := actionRelayTxHash(ctx, strategy, result)
	if txHash == "" {
		return "", nil, true, fmt.Errorf("settlement cancel action submitted without tx hash (order %s)", order.ID)
	}
	txid := iwallet.TransactionID(txHash)
	return txid, &iwallet.Transaction{ID: txid}, true, nil
}

func (s *OrderAppService) runMonitoredSettlementDisputeRelease(
	ctx context.Context,
	order *models.Order,
	coinType iwallet.CoinType,
	paymentSent *pb.PaymentSent,
	releaseInfo *pb.DisputeClose_ModeratedEscrowRelease,
) (*payment.ActionResult, *iwallet.Transaction, bool, error) {
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return nil, nil, false, err
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		return nil, nil, false, nil
	}
	spec, err := requireBackendSubmittedSettlementSpec(order, paymentSent)
	if err != nil {
		return nil, nil, true, err
	}
	if spec.UsesUTXOScript() {
		return s.runUTXOSyncSettlementDisputeRelease(order, coinType, paymentSent, releaseInfo)
	}
	if !orderEscrowUsesRelaySettlementRelease(spec) {
		return nil, nil, false, nil
	}

	release := cloneDisputeRelease(releaseInfo)
	if release == nil {
		return nil, nil, true, fmt.Errorf("settlement dispute release info is nil")
	}

	result, err := strategy.DisputeRelease(ctx, payment.ActionParams{
		OrderID:       order.ID.String(),
		PaymentCoin:   string(coinType),
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     order,
		ReleaseInfo:   release,
	})
	if err != nil {
		return nil, nil, true, err
	}

	txHash := actionRelayTxHash(ctx, strategy, result)
	var tx *iwallet.Transaction
	if txHash != "" {
		tx = &iwallet.Transaction{ID: iwallet.TransactionID(txHash)}
	}
	return result, tx, true, nil
}

func (s *OrderAppService) runUTXOSyncSettlementDisputeRelease(
	order *models.Order,
	coinType iwallet.CoinType,
	paymentSent *pb.PaymentSent,
	releaseInfo *pb.DisputeClose_ModeratedEscrowRelease,
) (*payment.ActionResult, *iwallet.Transaction, bool, error) {
	orderID := order.ID.String()
	actionID, existingTxHash, err := s.beginSyncBackendSettlementAction(
		orderID, "dispute_release", string(coinType), paymentSent.Amount,
	)
	if err != nil {
		return nil, nil, true, err
	}
	if existingTxHash != "" {
		tx := &iwallet.Transaction{ID: iwallet.TransactionID(existingTxHash)}
		return &payment.ActionResult{
			Mode:            payment.ActionModeCompleted,
			ActionID:        actionID,
			SubmittedTxHash: existingTxHash,
		}, tx, true, nil
	}

	releaseTx, err := s.BuildDisputeReleaseTransaction(releaseInfo, paymentSent)
	if err != nil {
		s.failSyncBackendSettlementAction(actionID, err.Error())
		return nil, nil, true, err
	}
	disputeClose := &pb.DisputeClose{ReleaseInfo: releaseInfo}
	if err := s.signAndSendReleaseTransaction(&releaseTx, paymentSent, disputeClose); err != nil {
		s.failSyncBackendSettlementAction(actionID, err.Error())
		return nil, nil, true, err
	}
	if err := s.confirmSyncBackendSettlementAction(actionID, releaseTx.ID.String()); err != nil {
		s.failSyncBackendSettlementAction(actionID, err.Error())
		return nil, nil, true, err
	}
	return &payment.ActionResult{
		Mode:            payment.ActionModeCompleted,
		ActionID:        actionID,
		SubmittedTxHash: releaseTx.ID.String(),
	}, &releaseTx, true, nil
}

// ExecuteSettlementDisputeReleaseAction submits backend escrow release for
// DECIDED moderated ManagedEscrow / Solana Anchor (relay) or UTXO (sync) disputes via
// settlement-actions/dispute-release.
func (s *OrderAppService) ExecuteSettlementDisputeReleaseAction(
	ctx context.Context,
	orderID models.OrderID,
) (*payment.ActionResult, iwallet.CoinType, error) {
	if err := s.requireDisputeReleaseParticipant(orderID); err != nil {
		return nil, "", err
	}

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return nil, "", err
	}

	disputeClose, err := order.DisputeClosedMessage()
	if err != nil {
		return nil, "", fmt.Errorf("%w: dispute close message is missing", coreiface.ErrBadRequest)
	}
	if disputeClose.ReleaseInfo == nil {
		return nil, "", fmt.Errorf("%w: dispute release info is missing", coreiface.ErrBadRequest)
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, "", fmt.Errorf("%w: payment not recorded for this order", coreiface.ErrBadRequest)
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, "", err
	}

	if !orderRequiresMonitoredSettlementActions(&order, paymentSent, coinType, s.paymentRegistry) {
		return nil, coinType, errRetiredClientSignedModeratedSettlement("dispute_release")
	}
	if _, err := requireBackendSubmittedSettlementSpec(&order, paymentSent); err != nil {
		return nil, coinType, err
	}

	if err := s.attachSettlementActions(&order); err != nil {
		return nil, coinType, fmt.Errorf("load settlement actions for order %s: %w", orderID, err)
	}
	if existing, ok := existingMonitoredSettlementActionResult(&order, "dispute_release"); ok {
		return existing, coinType, nil
	}

	result, tx, handled, err := s.runMonitoredSettlementDisputeRelease(
		ctx,
		&order,
		coinType,
		paymentSent,
		disputeClose.ReleaseInfo,
	)
	if err != nil {
		return nil, coinType, err
	}
	if !handled {
		return nil, coinType, fmt.Errorf("%w: settlement dispute release is not supported for coin %s",
			coreiface.ErrBadRequest, coinType)
	}
	if result == nil {
		result = &payment.ActionResult{Mode: payment.ActionModeCompleted}
	}
	if result.Mode == payment.ActionModeInstructionsRequired || result.Instructions != nil {
		return nil, coinType, fmt.Errorf("%w: settlement-actions only support backend-submitted flows for coin %s",
			coreiface.ErrBadRequest, coinType)
	}
	if result.SubmittedTxHash == "" {
		if tx != nil && tx.ID != "" {
			result.SubmittedTxHash = tx.ID.String()
		}
	}
	return result, coinType, nil
}

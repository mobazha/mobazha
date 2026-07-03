package settlement

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const syncActionStaleAfter = 2 * time.Minute

// CloneEscrowRelease deep-copies an escrow release payload before an adapter
// mutates tx hash or signature fields.
func CloneEscrowRelease(release *pb.EscrowRelease) *pb.EscrowRelease {
	if release == nil {
		return nil
	}
	cloned, ok := proto.Clone(release).(*pb.EscrowRelease)
	if !ok {
		return nil
	}
	return cloned
}

// CloneDisputeRelease deep-copies a moderated dispute release payload before
// backend settlement submission mutates chain-specific fields.
func CloneDisputeRelease(release *pb.DisputeClose_ModeratedEscrowRelease) *pb.DisputeClose_ModeratedEscrowRelease {
	if release == nil {
		return nil
	}
	cloned, ok := proto.Clone(release).(*pb.DisputeClose_ModeratedEscrowRelease)
	if !ok {
		return nil
	}
	return cloned
}

// ActionRelayTxHash prefers the hash returned synchronously from relay submit,
// then falls back to GetActionStatus for recently recorded actions.
func ActionRelayTxHash(ctx context.Context, strategy payment.ChainEscrowV2, result *payment.ActionResult) string {
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

// EscrowUsesBackendSubmittedRelease reports escrow types whose moderated
// release/complete flows use settlement-actions (relay or sync).
func EscrowUsesBackendSubmittedRelease(spec payment.SettlementSpec) bool {
	return spec.UsesManagedEscrow() || spec.UsesSolanaEscrow() || spec.UsesUTXOScript()
}

// EscrowUsesRelayRelease reports escrow types whose release is submitted
// asynchronously via relay + action store (backend-managed contract rails).
func EscrowUsesRelayRelease(spec payment.SettlementSpec) bool {
	return spec.UsesManagedEscrow() || spec.UsesSolanaEscrow()
}

// ActionName returns the canonical action name carried by a snapshot.
func ActionName(action models.SettlementActionSnapshot) string {
	name := strings.ToLower(strings.TrimSpace(action.SettlementAction))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(action.Action))
	}
	return name
}

func CompleteReleaseReady(order *models.Order, txid iwallet.TransactionID) bool {
	return ReleaseReady(order, txid, "complete")
}

func CompleteReleasePending(order *models.Order, txid iwallet.TransactionID) bool {
	return ReleasePending(order, txid, "complete")
}

func DisputeReleaseReady(order *models.Order, txid iwallet.TransactionID) bool {
	return ReleaseReady(order, txid, "dispute_release")
}

func DisputeReleasePending(order *models.Order, txid iwallet.TransactionID) bool {
	return ReleasePending(order, txid, "dispute_release")
}

func ReleaseReady(order *models.Order, _ iwallet.TransactionID, actionName string) bool {
	return ActionTxHash(order, actionName) != ""
}

func ReleasePending(order *models.Order, _ iwallet.TransactionID, actionName string) bool {
	if ActionTxHash(order, actionName) != "" {
		return false
	}
	if order == nil {
		return false
	}
	for _, action := range order.SettlementActions {
		if ActionName(action) != actionName {
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

func ActionTxHash(order *models.Order, actionName string) iwallet.TransactionID {
	if order == nil {
		return ""
	}
	for _, action := range order.SettlementActions {
		if ActionName(action) != actionName {
			continue
		}
		if action.TxHash != "" {
			return iwallet.TransactionID(action.TxHash)
		}
	}
	return ""
}

// EvaluateRelease checks pending/ready state for a backend settlement release
// action (complete or dispute_release).
func EvaluateRelease(
	order *models.Order,
	txid iwallet.TransactionID,
	actionName string,
) (resolvedTxid iwallet.TransactionID, releaseAlreadySubmitted bool, err error) {
	if ReleasePending(order, txid, actionName) {
		return "", false, fmt.Errorf("settlement %s release is still pending; retry after tx hash is available", actionName)
	}
	if ReleaseReady(order, txid, actionName) {
		resolved := ActionTxHash(order, actionName)
		if resolved == "" {
			return "", false, nil
		}
		if txid != "" && txid != resolved {
			return "", false, fmt.Errorf("txID does not match settlement %s release hash", actionName)
		}
		return resolved, true, nil
	}
	return "", false, nil
}

func SyncActionID(orderID, action string) string {
	return fmt.Sprintf("sync-%s-%s", action, orderID)
}

func StaleSyncAction(actionID, state, txHash string, updatedAt time.Time, now time.Time) bool {
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
	return now.Sub(updatedAt) > syncActionStaleAfter
}

// ExistingActionResult returns an in-flight or completed backend settlement
// action so settlement-action endpoints stay idempotent on retry.
func ExistingActionResult(order *models.Order, actionName string) (*payment.ActionResult, bool) {
	if order == nil {
		return nil, false
	}
	var pending *payment.ActionResult
	for _, action := range order.SettlementActions {
		if ActionName(action) != actionName {
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
			if StaleSyncAction(action.ActionID, action.State, action.TxHash, action.UpdatedAt, time.Now().UTC()) {
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

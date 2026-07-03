// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	evm "github.com/mobazha/mobazha/internal/chains/evm"
	"github.com/mobazha/mobazha/internal/core/guest"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	protocol "github.com/mobazha/mobazha/pkg/evm"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	"github.com/mobazha/mobazha/pkg/relay"
)

// minRelayTxConfirmations gates "confirmed" in local projections. Business
// finality for orders still uses Settlement / PaymentObservation depth.
const minRelayTxConfirmations = 1

const (
	managedRelayConfirmationTimeout   = 5 * time.Minute
	managedRelayConfirmationBatchSize = 100
	managedRelayMaxBroadcastAttempts  = 3
)

var pendingManagedRelayStates = []string{"submitting", "submitted"}

type relayConfirmationClient interface {
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	TransactionByHash(ctx context.Context, txHash common.Hash) (*types.Transaction, bool, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
}

// runSettlementActionConfirmationsOnce advances backend settlement action rows
// from submitting/submitted -> confirmed/failed/abandoned across all
// backend-submitted rails.
func (n *MobazhaNode) runSettlementActionConfirmationsOnce(ctx context.Context) {
	if n == nil || n.db == nil || n.multiwallet == nil {
		return
	}
	n.runManagedSettlementReconciliationsOnce(ctx)
	n.runManagedRelayConfirmationsOnce(ctx)
}

// runManagedRelayConfirmationsOnce reconciles backend-managed EVM settlement actions.
func (n *MobazhaNode) runManagedRelayConfirmationsOnce(ctx context.Context) {
	if n == nil || n.db == nil || n.multiwallet == nil {
		return
	}
	rows, err := n.loadPendingManagedRelayActions(ctx, managedRelayConfirmationBatchSize)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "managed relay confirmations: load pending actions: %v", err)
		return
	}
	for i := range rows {
		if err := ctx.Err(); err != nil {
			return
		}
		n.reconcileManagedRelayAction(ctx, rows[i])
	}
	if n.guestOrderService != nil {
		n.guestOrderService.RecoverEVMManagedEscrowPendingSettlements(ctx)
	}
}

func (n *MobazhaNode) loadPendingManagedRelayActions(ctx context.Context, limit int) ([]models.SettlementAction, error) {
	var rows []models.SettlementAction
	err := n.db.View(func(tx database.Tx) error {
		q := tx.Read().WithContext(ctx).
			Where("state IN ? AND chain_id > ?", pendingManagedRelayStates, 0).
			Order("updated_at ASC")
		if limit > 0 {
			q = q.Limit(limit)
		}
		return q.Find(&rows).Error
	})
	return rows, err
}

func (n *MobazhaNode) reconcileManagedRelayAction(ctx context.Context, row models.SettlementAction) {
	if row.ActionID == "" {
		return
	}
	if !common.IsHexHash(row.TxHash) {
		if row.State == "submitting" && strings.TrimSpace(row.TxHash) == "" {
			if time.Since(row.UpdatedAt.UTC()) < managedRelayConfirmationTimeout {
				return
			}
			if retried, reason := n.resubmitDroppedManagedRelayAction(ctx, row); retried {
				return
			} else if reason != "" {
				n.markSettlementActionTerminal(row, "abandoned", reason)
				return
			}
			n.markSettlementActionTerminal(row, "abandoned", "relay submission wait timed out")
			return
		}
		n.recordSettlementActionStatus(row, SettlementActionStatusUpdate{
			State:     "failed",
			LastError: "relay projection missing valid tx hash",
		})
		return
	}

	client, err := n.ethClientForRelayConfirmations(row.ChainID)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "managed relay confirmations: chain client unavailable for action %s: %v", row.ActionID, err)
		return
	}
	if client == nil {
		return
	}

	result, ok := n.reconcileManagedRelayAttempts(ctx, client, row)
	if ok {
		n.applyManagedRelayReceipt(ctx, client, row, result.hash, result.receipt)
		return
	}
	if result.wait {
		return
	}
	if result.err != nil {
		if time.Since(row.UpdatedAt.UTC()) >= managedRelayConfirmationTimeout {
			if retried, reason := n.resubmitDroppedManagedRelayAction(ctx, row); retried {
				return
			} else if reason != "" {
				n.markSettlementActionTerminal(row, "abandoned", reason)
				return
			}
			n.markSettlementActionTerminal(row, "abandoned", "confirmation wait timed out")
		}
		return
	}
}

type managedRelayAttemptResult struct {
	hash    common.Hash
	receipt *types.Receipt
	wait    bool
	err     error
}

func (n *MobazhaNode) reconcileManagedRelayAttempts(ctx context.Context, client relayConfirmationClient, row models.SettlementAction) (managedRelayAttemptResult, bool) {
	hashes := managedRelayAttemptHashes(row)
	if len(hashes) == 0 {
		return managedRelayAttemptResult{err: ethereum.NotFound}, false
	}
	var failed managedRelayAttemptResult
	for _, hash := range hashes {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if err == nil && receipt != nil {
			if receipt.Status == types.ReceiptStatusSuccessful {
				return managedRelayAttemptResult{hash: hash, receipt: receipt}, true
			}
			if failed.receipt == nil {
				failed = managedRelayAttemptResult{hash: hash, receipt: receipt}
			}
			continue
		}
		if managedRelayTxStillKnown(ctx, client, hash) {
			return managedRelayAttemptResult{wait: true}, false
		}
	}
	if failed.receipt != nil {
		return failed, true
	}
	return managedRelayAttemptResult{err: ethereum.NotFound}, false
}

func (n *MobazhaNode) applyManagedRelayReceipt(ctx context.Context, client relayConfirmationClient, row models.SettlementAction, txHash common.Hash, receipt *types.Receipt) {
	confirms := confirmationsFromHead(ctx, client, receipt)
	if receipt.Status != types.ReceiptStatusSuccessful {
		n.recordSettlementActionStatus(row, SettlementActionStatusUpdate{
			State:         "failed",
			TxHash:        txHash.Hex(),
			Confirmations: confirms,
			LastError:     "relay transaction reverted on-chain",
		})
		return
	}
	if confirms < minRelayTxConfirmations {
		n.recordSettlementActionStatus(row, SettlementActionStatusUpdate{
			TxHash:        txHash.Hex(),
			Confirmations: confirms,
			LastError:     "",
		})
		return
	}
	if row.ActionKind == payment.ManagedEscrowGuestSettlementAction {
		if err := n.validateManagedEscrowReceipt(ctx, row, txHash, receipt); err != nil {
			n.recordSettlementActionStatus(row, SettlementActionStatusUpdate{
				State:         "failed",
				TxHash:        txHash.Hex(),
				Confirmations: confirms,
				LastError:     err.Error(),
			})
			return
		}
	}

	if err := n.recordSettlementActionStatus(row, SettlementActionStatusUpdate{
		State:         "confirmed",
		TxHash:        txHash.Hex(),
		Confirmations: confirms,
		LastError:     "",
	}); err == nil {
		confirmed := row
		confirmed.TxHash = txHash.Hex()
		n.notifyGuestManagedEscrowSettlementConfirmed(confirmed)
		n.emitOrderAutoConfirmAfterManagedRelayConfirm(confirmed)
	}
}

func managedRelayTxStillKnown(ctx context.Context, client relayConfirmationClient, txHash common.Hash) bool {
	if client == nil {
		return false
	}
	tx, _, err := client.TransactionByHash(ctx, txHash)
	if err == nil && tx != nil {
		return true
	}
	return err != nil && !errors.Is(err, ethereum.NotFound)
}

func managedRelayAttemptHashes(row models.SettlementAction) []common.Hash {
	var out []common.Hash
	seen := make(map[common.Hash]struct{})
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if !common.IsHexHash(raw) {
			return
		}
		hash := common.HexToHash(raw)
		if hash == (common.Hash{}) {
			return
		}
		if _, ok := seen[hash]; ok {
			return
		}
		seen[hash] = struct{}{}
		out = append(out, hash)
	}
	add(row.TxHash)
	for _, raw := range strings.FieldsFunc(row.AttemptTxHashes, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		add(raw)
	}
	return out
}

func (n *MobazhaNode) resubmitDroppedManagedRelayAction(ctx context.Context, row models.SettlementAction) (bool, string) {
	if row.Attempts >= managedRelayMaxBroadcastAttempts {
		return false, "confirmation wait timed out after relay retries"
	}
	request, reason := relayRequestFromSettlementAction(row)
	if reason != "" {
		return false, reason
	}

	store := NewSettlementActionStore(n.db)
	service := n.evmRelay
	if service == nil {
		service = n.distributionEVMRelayService()
	}
	if service == nil || !service.IsAvailable() {
		n.deferSettlementActionRetry(store, row, "confirmation wait timed out; relay service unavailable for retry")
		return true, ""
	}
	chainType, err := service.ChainTypeForID(row.ChainID)
	if err != nil {
		n.deferSettlementActionRetry(store, row, "confirmation wait timed out; relay chain unavailable for retry")
		return true, ""
	}
	request.ChainType = chainType
	nextAttempt := row.Attempts + 1
	if nextAttempt <= 0 {
		nextAttempt = 1
	}
	historyBeforeRetry, claimed, err := store.ClaimRetry(row, nextAttempt)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "managed relay confirmations: claim retry action %s: %v", row.ActionID, err)
		return true, ""
	}
	if !claimed {
		return true, ""
	}

	response, err := service.Execute(ctx, request)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "managed relay confirmations: retry action %s: %v", row.ActionID, err)
		n.deferSettlementActionRetry(store, row, "confirmation wait timed out; relay retry failed")
		return true, ""
	}
	if response == nil || !common.IsHexHash(strings.TrimSpace(response.TxHash)) {
		n.deferSettlementActionRetry(store, row, "confirmation wait timed out; relay retry returned zero tx hash")
		return true, ""
	}
	hash := common.HexToHash(response.TxHash)
	if hash == (common.Hash{}) {
		n.deferSettlementActionRetry(store, row, "confirmation wait timed out; relay retry returned zero tx hash")
		return true, ""
	}
	if err := store.RecordRetrySubmitted(row, hash.Hex(), historyBeforeRetry, nextAttempt); err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "managed relay confirmations: persist retry action %s: %v", row.ActionID, err)
		return true, ""
	}
	return true, ""
}

func relayRequestFromSettlementAction(row models.SettlementAction) (*relay.EVMRelayRequest, string) {
	if !common.IsHexAddress(row.To) {
		return nil, "confirmation wait timed out; relay target unavailable for retry"
	}
	callData := strings.TrimSpace(row.Data)
	if callData == "" {
		return nil, "confirmation wait timed out; relay calldata unavailable for retry"
	}
	_, err := hexutil.Decode(callData)
	if err != nil {
		return nil, "confirmation wait timed out; relay calldata invalid for retry"
	}
	return &relay.EVMRelayRequest{
		To:               common.HexToAddress(row.To).Hex(),
		Data:             callData,
		OrderID:          row.OrderID,
		SettlementAction: row.ActionKind,
		ClientActionID:   row.ActionID,
	}, ""
}

func (n *MobazhaNode) deferSettlementActionRetry(store *SettlementActionStore, row models.SettlementAction, reason string) {
	if store == nil {
		return
	}
	if err := store.DeferRetry(row, reason); err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "managed relay confirmations: defer retry action %s: %v", row.ActionID, err)
	}
}

func (n *MobazhaNode) markSettlementActionTerminal(row models.SettlementAction, state, reason string) error {
	store := NewSettlementActionStore(n.db)
	if store == nil {
		return nil
	}
	err := store.MarkTerminal(row, state, reason)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "managed relay confirmations: mark action %s %s: %v", row.ActionID, state, err)
	}
	return err
}

func (n *MobazhaNode) recordSettlementActionStatus(row models.SettlementAction, update SettlementActionStatusUpdate) error {
	store := NewSettlementActionStore(n.db)
	if store == nil {
		return nil
	}
	err := store.RecordStatus(row, update)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "managed relay confirmations: update action %s status: %v", row.ActionID, err)
	}
	return err
}

func (n *MobazhaNode) validateManagedEscrowReceipt(
	ctx context.Context,
	row models.SettlementAction,
	txHash common.Hash,
	receipt *types.Receipt,
) error {
	if n == nil || n.managedEscrowReceiptValidator == nil {
		return errors.New("managed escrow receipt validator is unavailable")
	}
	return n.managedEscrowReceiptValidator.ValidateManagedEscrowReceipt(ctx, payment.ManagedEscrowReceiptValidationRequest{
		ActionID:      row.ActionID,
		OrderID:       row.OrderID,
		ActionKind:    row.ActionKind,
		ChainID:       row.ChainID,
		EscrowAddress: row.To,
		TxHash:        txHash.Hex(),
		Receipt:       receipt,
	})
}

// emitOrderAutoConfirmAfterManagedRelayConfirm emits ORDER_CONFIRMATION for
// normal Order-table orders after a managed escrow confirm relay succeeds on-chain.
// Guest checkout settlements are handled by guestOrderService.
func (n *MobazhaNode) emitOrderAutoConfirmAfterManagedRelayConfirm(row models.SettlementAction) {
	if n == nil || n.eventBus == nil || n.db == nil {
		return
	}
	if row.ActionKind != "confirm" {
		return
	}
	if strings.HasPrefix(row.OrderID, guest.OrderTokenPrefix) {
		return
	}
	if !common.IsHexHash(row.TxHash) {
		return
	}

	var order models.Order
	err := n.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", row.OrderID).First(&order).Error
	})
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "managed relay confirm: load order %s for auto-confirm: %v", row.OrderID, err)
		return
	}
	if order.SerializedOrderConfirmation != nil {
		return
	}
	if !order.CanConfirm() {
		return
	}
	if n.settlementService == nil {
		return
	}
	unlock := n.settlementService.TryLockAutoConfirm(row.OrderID)
	if unlock == nil {
		return
	}
	defer unlock()

	// Re-check under lock: another worker may have emitted while we waited.
	if err := n.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", row.OrderID).First(&order).Error
	}); err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "managed relay confirm: reload order %s for auto-confirm: %v", row.OrderID, err)
		return
	}
	if order.SerializedOrderConfirmation != nil || !order.CanConfirm() {
		return
	}

	payoutAddress := ""
	paymentSent, psErr := order.PaymentSentMessage()
	if psErr == nil {
		if addr, addrErr := n.settlementService.GetPayoutAddress(paymentSent.Coin); addrErr == nil {
			payoutAddress = addr.String()
		}
	}
	authorization, err := n.confirmationAuthorizationForAction(row.OrderID, row.ActionID, row.TxHash)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "managed relay confirm: resolve extension authorization for order %s: %v", row.OrderID, err)
		return
	}
	attestationID := ""
	if authorization != nil {
		attestationID = authorization.AttestationID
		payoutAddress = authorization.PayoutAddress
	}

	n.eventBus.Emit(&events.OrderAutoConfirmRequest{
		OrderID: row.OrderID, TxID: row.TxHash, PayoutAddress: payoutAddress,
		SettlementAttestationID: attestationID,
	})
}

func (n *MobazhaNode) notifyGuestManagedEscrowSettlementConfirmed(row models.SettlementAction) {
	if n == nil || n.guestOrderService == nil {
		return
	}
	if row.ActionKind != payment.ManagedEscrowGuestSettlementAction {
		return
	}
	if !strings.HasPrefix(row.OrderID, guest.OrderTokenPrefix) {
		return
	}
	n.guestOrderService.OnManagedEscrowSettlementConfirmed(row.OrderID)
}

func confirmationsFromHead(ctx context.Context, client relayConfirmationClient, receipt *types.Receipt) int {
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || receipt == nil || receipt.BlockNumber == nil {
		return 0
	}
	conf := head.Number.Uint64() - receipt.BlockNumber.Uint64() + 1
	return int(conf)
}

func (n *MobazhaNode) ethClientForRelayConfirmations(chainID uint64) (relayConfirmationClient, error) {
	if n.multiwallet == nil {
		return nil, nil
	}

	chain, ok := protocol.ChainTypeForID(chainID)
	if !ok {
		return nil, fmt.Errorf("relay confirmation: unsupported EVM chain id %d", chainID)
	}

	wallet, ok := n.multiwallet.WalletForChain(chain)
	if !ok || wallet == nil {
		return nil, nil
	}

	ethW, ok := wallet.(*evm.ETHWallet)
	if !ok || ethW.ChainClient == nil {
		return nil, nil
	}

	ec, ok := ethW.ChainClient.(*evm.EthClient)
	if !ok || ec.Client == nil {
		return nil, nil
	}

	return ec.Client, nil
}

// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const utxoSettlementConfirmationBatchSize = 100

func (n *MobazhaNode) runUTXOSettlementConfirmationsOnce(ctx context.Context) {
	if n == nil || n.db == nil || n.monitorService == nil {
		return
	}
	rows, err := n.loadPendingUTXOSettlementActions(ctx, utxoSettlementConfirmationBatchSize)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "UTXO settlement confirmations: load pending actions: %v", err)
		return
	}
	for i := range rows {
		if ctx.Err() != nil {
			return
		}
		n.reconcileUTXOSettlementAction(rows[i])
	}
}

func (n *MobazhaNode) loadPendingUTXOSettlementActions(ctx context.Context, limit int) ([]models.SettlementAction, error) {
	var candidates []models.SettlementAction
	err := n.db.View(func(tx database.Tx) error {
		q := tx.Read().WithContext(ctx).
			Where("state IN ? AND tx_hash <> ? AND settlement_coin <> ?", pendingManagedRelayStates, "", "").
			Order("updated_at ASC")
		if limit > 0 {
			q = q.Limit(limit)
		}
		return q.Find(&candidates).Error
	})
	if err != nil {
		return nil, err
	}
	rows := make([]models.SettlementAction, 0, len(candidates))
	for _, row := range candidates {
		coin, ok := payment.NormalizeSettlementPaymentCoin(row.SettlementCoin)
		if !ok {
			continue
		}
		info, err := payment.SettlementCoinInfoForCoin(coin)
		if err != nil || !affiliateUTXOSettlementChain(info.Chain) {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func affiliateUTXOSettlementChain(chain iwallet.ChainType) bool {
	return chain == iwallet.ChainBitcoin || chain == iwallet.ChainBitcoinCash || chain == iwallet.ChainLitecoin
}

func (n *MobazhaNode) reconcileUTXOSettlementAction(row models.SettlementAction) {
	coin, ok := payment.NormalizeSettlementPaymentCoin(row.SettlementCoin)
	if !ok {
		return
	}
	info, err := payment.SettlementCoinInfoForCoin(coin)
	if err != nil || !affiliateUTXOSettlementChain(info.Chain) {
		return
	}
	confirmations, err := n.monitorService.GetTxConfirmations(info.Chain, row.TxHash)
	if err != nil || confirmations < minRelayTxConfirmations {
		return
	}
	tx, err := n.monitorService.GetTransaction(info.Chain, row.TxHash)
	if err != nil || tx == nil {
		return
	}
	planned := models.DecodeSettlementPayoutLines(row.PlannedLines)
	observed, err := observedUTXOSettlementLines(planned, *tx, row.TxHash)
	if err != nil {
		if persistErr := n.recordSettlementActionStatus(row, SettlementActionStatusUpdate{
			State: "failed", Confirmations: confirmations, LastError: err.Error(),
		}); persistErr != nil {
			logger.LogWarningWithIDf(log, n.nodeID, "UTXO settlement confirmations: persist failed action %s: %v", row.ActionID, persistErr)
		}
		return
	}
	if err := n.recordSettlementActionStatus(row, SettlementActionStatusUpdate{
		State: "confirmed", TxHash: row.TxHash, Confirmations: confirmations, ObservedLines: observed,
	}); err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "UTXO settlement confirmations: persist action %s: %v", row.ActionID, err)
	}
}

func observedUTXOSettlementLines(planned []models.SettlementPayoutLine, tx iwallet.Transaction, txHash string) ([]models.SettlementPayoutLine, error) {
	if len(planned) == 0 {
		return nil, fmt.Errorf("UTXO settlement projection has no planned outputs")
	}
	used := make([]bool, len(tx.To))
	observed := make([]models.SettlementPayoutLine, 0, len(planned))
	for _, line := range planned {
		matched := -1
		for i, output := range tx.To {
			if !used[i] && output.Address.String() == line.Address && output.Amount.String() == line.Amount {
				matched = i
				break
			}
		}
		if matched < 0 {
			return nil, fmt.Errorf("confirmed UTXO transaction does not contain planned %s output", strings.TrimSpace(line.Type))
		}
		used[matched] = true
		line.TxHash = txHash
		observed = append(observed, line)
	}
	return observed, nil
}

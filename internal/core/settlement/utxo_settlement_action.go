package settlement

import (
	"errors"
	"fmt"
	"strings"
	"time"

	ordersettlement "github.com/mobazha/mobazha/internal/core/order/settlement"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"gorm.io/gorm"
)

func (s *SettlementService) beginUTXOCancelableConfirmAction(orderID, settlementCoin, grossAmount string) (string, string, error) {
	actionID := ordersettlement.SyncActionID(orderID, "confirm")
	var existing models.SettlementAction
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", actionID).First(&existing).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", err
	}
	if err == nil {
		state := strings.ToLower(strings.TrimSpace(existing.State))
		if existing.TxHash != "" && (state == "submitted" || state == "confirmed") {
			return actionID, existing.TxHash, nil
		}
		if state == "submitting" && !ordersettlement.StaleSyncAction(existing.ActionID, existing.State, existing.TxHash, existing.UpdatedAt, time.Now().UTC()) {
			return "", "", fmt.Errorf("settlement confirm release is still pending; retry after submission completes")
		}
	}
	now := time.Now().UTC()
	row := models.SettlementAction{
		ActionID: actionID, OrderID: orderID, ActionKind: "confirm", State: "submitting",
		SettlementCoin: settlementCoin, GrossAmount: grossAmount, CreatedAt: now, UpdatedAt: now,
	}
	if err == nil && !existing.CreatedAt.IsZero() {
		row.CreatedAt = existing.CreatedAt
	}
	if err := s.db.Update(func(tx database.Tx) error { return tx.Save(&row) }); err != nil {
		return "", "", err
	}
	return actionID, "", nil
}

func (s *SettlementService) recordUTXOCancelableConfirmSubmission(actionID, txHash string, lines []models.SettlementPayoutLine, state string) error {
	return s.db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(map[string]interface{}{
			"state": state, "tx_hash": txHash, "attempt_tx_hashes": txHash,
			"planned_lines": models.EncodeSettlementPayoutLines(lines), "last_error": "", "updated_at": time.Now().UTC(),
		}, map[string]interface{}{"action_id = ?": actionID}, &models.SettlementAction{})
		return err
	})
}

func (s *SettlementService) failUTXOCancelableConfirmAction(actionID, reason string) {
	if err := s.db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(map[string]interface{}{
			"state": "failed", "tx_hash": "", "attempt_tx_hashes": "", "last_error": reason, "updated_at": time.Now().UTC(),
		}, map[string]interface{}{"action_id = ?": actionID}, &models.SettlementAction{})
		return err
	}); err != nil {
		logger.LogWarningWithIDf(log, s.nodeID, "Persist failed UTXO CANCELABLE settlement action %s: %v", actionID, err)
	}
}

func utxoCancelableConfirmPayoutLines(tx iwallet.Transaction, coin string, affiliate *models.AffiliateSettlementPayout) ([]models.SettlementPayoutLine, error) {
	if len(tx.To) == 0 {
		return nil, fmt.Errorf("CANCELABLE release has no seller output")
	}
	lines := []models.SettlementPayoutLine{{
		Type: "seller", Amount: tx.To[0].Amount.String(), Address: tx.To[0].Address.String(), Coin: coin,
	}}
	if affiliate == nil {
		if len(tx.To) != 1 {
			return nil, fmt.Errorf("CANCELABLE release has unexpected payout outputs")
		}
		return lines, nil
	}
	if len(tx.To) != 2 || tx.To[1].Address.String() != affiliate.Address || tx.To[1].Amount.String() != affiliate.Amount {
		return nil, fmt.Errorf("CANCELABLE release affiliate output does not match frozen settlement terms")
	}
	return append(lines, models.SettlementPayoutLine{
		Type: "affiliate", Amount: affiliate.Amount, Address: affiliate.Address, Coin: coin,
	}), nil
}

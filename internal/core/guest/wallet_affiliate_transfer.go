package guest

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

func guestWalletAffiliateActionID(orderToken string) string {
	return "guest-wallet-affiliate-" + orderToken
}

func guestWalletAffiliateTransferKey(orderToken string) string {
	return "guest-wallet-affiliate:" + orderToken
}

// settleGuestWalletAffiliate creates the exact Affiliate output from the
// AccountGuest UTXO while returning seller change to the same wallet account.
// It never creates a Guest/Affiliate payout state machine: WalletTransfer owns
// transaction state and SettlementAction projects the business settlement.
func (s *GuestOrderAppService) settleGuestWalletAffiliate(ctx context.Context, order models.GuestOrder) (bool, error) {
	if s == nil || s.walletAccounts == nil {
		return false, fmt.Errorf("guest affiliate wallet transfer is unavailable")
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(order.PaymentCoin))
	if err != nil || !coinInfo.Chain.IsUTXOChain() {
		return false, fmt.Errorf("guest affiliate wallet transfer requires a UTXO rail")
	}
	amount, err := strconv.ParseUint(strings.TrimSpace(order.AffiliatePayoutAmount), 10, 64)
	if err != nil || amount == 0 {
		return false, fmt.Errorf("guest affiliate payout amount is invalid")
	}
	if strings.TrimSpace(order.AffiliatePayoutAddress) == "" {
		return false, fmt.Errorf("guest affiliate payout address is missing")
	}
	if err := s.ensureGuestWalletAffiliateAction(order); err != nil {
		return false, err
	}

	receipt, transferErr := s.walletAccounts.Transfer(ctx, contracts.WalletTransferRequest{
		RailID: order.PaymentCoin, Role: contracts.AccountGuest, ReferenceID: order.OrderToken,
		Destination: order.AffiliatePayoutAddress, Amount: amount,
		IdempotencyKey: guestWalletAffiliateTransferKey(order.OrderToken),
	})
	if err := s.projectGuestWalletAffiliateAction(order, receipt, transferErr); err != nil {
		return false, err
	}
	if transferErr != nil {
		return false, transferErr
	}
	return receipt.State == contracts.WalletTransferConfirmed, nil
}

func (s *GuestOrderAppService) ensureGuestWalletAffiliateAction(order models.GuestOrder) error {
	return s.db.Update(func(tx database.Tx) error {
		var existing models.SettlementAction
		err := tx.Read().Where("action_id = ?", guestWalletAffiliateActionID(order.OrderToken)).First(&existing).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := time.Now().UTC()
		return tx.Create(&models.SettlementAction{
			ActionID: guestWalletAffiliateActionID(order.OrderToken), OrderID: order.OrderToken,
			ActionKind: "guest_affiliate_transfer", State: "submitting",
			SettlementCoin: order.PaymentCoin, GrossAmount: order.PaymentAmount,
			IntentKey: guestWalletAffiliateTransferKey(order.OrderToken),
			PlannedLines: models.EncodeSettlementPayoutLines([]models.SettlementPayoutLine{{
				Type: "affiliate", Amount: order.AffiliatePayoutAmount,
				Address: order.AffiliatePayoutAddress, Coin: order.PaymentCoin,
			}}),
			CreatedAt: now, UpdatedAt: now,
		})
	})
}

func (s *GuestOrderAppService) projectGuestWalletAffiliateAction(
	order models.GuestOrder,
	receipt contracts.WalletTransfer,
	transferErr error,
) error {
	state := "submitting"
	switch receipt.State {
	case contracts.WalletTransferSubmitted:
		state = "submitted"
	case contracts.WalletTransferConfirmed:
		state = "confirmed"
	case contracts.WalletTransferReorged:
		state = "reorged"
	}
	lastError := receipt.LastError
	if transferErr != nil {
		lastError = transferErr.Error()
	}
	values := map[string]interface{}{
		"state": state, "tx_hash": receipt.TxHash, "confirmations": receipt.Confirmations,
		"last_error": lastError, "updated_at": time.Now().UTC(),
	}
	if strings.TrimSpace(receipt.TxHash) != "" {
		values["attempt_tx_hashes"] = receipt.TxHash
		values["observed_lines"] = models.EncodeSettlementPayoutLines([]models.SettlementPayoutLine{{
			Type: "affiliate", Amount: order.AffiliatePayoutAmount, Address: order.AffiliatePayoutAddress,
			Coin: order.PaymentCoin, TxHash: receipt.TxHash,
		}})
	}
	if state == "confirmed" {
		now := time.Now().UTC()
		values["confirmed_at"] = &now
	} else {
		values["confirmed_at"] = nil
	}
	return s.db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(values,
			map[string]interface{}{"action_id = ?": guestWalletAffiliateActionID(order.OrderToken)},
			&models.SettlementAction{})
		return err
	})
}

func isGuestWalletAffiliateOrder(order *models.GuestOrder) bool {
	if order == nil || order.HasManagedEscrowGuestFundingTarget() || strings.TrimSpace(order.SweepToAddress) != "" {
		return false
	}
	return strings.TrimSpace(order.AffiliatePayoutAddress) != "" && strings.TrimSpace(order.AffiliatePayoutAmount) != ""
}

// RecoverGuestWalletAffiliateTransfers resumes funded AccountGuest Affiliate
// outputs after restart. Confirmation is delegated to WalletTransfer.
func (s *GuestOrderAppService) RecoverGuestWalletAffiliateTransfers(ctx context.Context) {
	if s == nil || s.walletAccounts == nil {
		return
	}
	var orders []models.GuestOrder
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"state IN ? AND sweep_to_address = ? AND affiliate_payout_address <> ? AND affiliate_payout_amount <> ?",
			[]models.GuestOrderState{models.GuestOrderFunded, models.GuestOrderShipped, models.GuestOrderCompleted}, "", "", "",
		).Find(&orders).Error
	}); err != nil {
		return
	}
	for i := range orders {
		confirmed, err := s.settleGuestWalletAffiliate(ctx, orders[i])
		if err != nil {
			continue
		}
		if confirmed {
			s.emitGuestOrderFunded(orders[i].OrderToken)
		}
	}
}

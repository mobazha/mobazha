// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"gorm.io/gorm"
)

type standardOrderUTXOPaymentWatcher interface {
	WatchPaymentAddress(string, string, iwallet.ChainType, []byte) error
}

func (n *MobazhaNode) watchFrozenStandardOrderUTXOAttempt(
	ctx context.Context,
	tenantID, attemptID string,
) error {
	if n == nil || n.db == nil || n.multiwallet == nil || n.paymentService == nil {
		return fmt.Errorf("frozen standard order UTXO watch is not configured")
	}
	rawProvider, ok := n.db.(rawSettlementAuthorizationDB)
	if !ok || rawProvider.RawDB() == nil {
		return fmt.Errorf("frozen standard order UTXO watch raw database is unavailable")
	}
	return watchFrozenStandardOrderUTXOAttempt(
		ctx, rawProvider.RawDB(), n.multiwallet, n.paymentService, tenantID, attemptID,
	)
}

func watchFrozenStandardOrderUTXOAttempt(
	ctx context.Context,
	db *gorm.DB,
	wallets contracts.WalletOperator,
	watcher standardOrderUTXOPaymentWatcher,
	tenantID, attemptID string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tenantID = strings.TrimSpace(tenantID)
	attemptID = strings.TrimSpace(attemptID)
	if db == nil || wallets == nil || watcher == nil || attemptID == "" {
		return fmt.Errorf("frozen standard order UTXO watch dependencies and attempt are required")
	}
	var attempt models.PaymentAttempt
	if err := db.Where("tenant_id = ? AND attempt_id = ?", tenantID, attemptID).First(&attempt).Error; err != nil {
		return fmt.Errorf("load frozen standard order UTXO attempt: %w", err)
	}
	if attempt.State != models.PaymentAttemptFundingTargetReady || attempt.ExpectedModeratorPeerID != "" {
		return fmt.Errorf("standard order UTXO watch requires an unmoderated frozen attempt")
	}
	target, err := attempt.GetFundingTarget()
	if err != nil || target == nil {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	bundle, err := attempt.GetAuthorizationBundle()
	if err != nil || bundle == nil {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	var route models.PaymentRouteBinding
	if err := db.Where(
		"tenant_id = ? AND route_binding_id = ?", tenantID, attempt.RouteBindingID,
	).First(&route).Error; err != nil {
		return fmt.Errorf("load frozen standard order UTXO route: %w", err)
	}
	projection, err := (standardOrderUTXOFundingTargetProjector{wallets: wallets}).project(
		ctx, attempt, route, bundle.Offers,
	)
	if err != nil {
		return err
	}
	projectedBytes, _, err := projection.Target.CanonicalBytesAndHash()
	if err != nil {
		return err
	}
	storedBytes, _, err := target.CanonicalBytesAndHash()
	if err != nil || !bytes.Equal(projectedBytes, storedBytes) {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	redeemScript, err := hex.DecodeString(target.RedeemScriptHex)
	if err != nil || len(redeemScript) == 0 || !bytes.Equal(redeemScript, projection.RedeemScript) {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(attempt.Currency))
	if err != nil || !coinInfo.IsNative || !coinInfo.Chain.IsUTXOChain() {
		return fmt.Errorf("frozen standard order funding target requires a native UTXO rail")
	}
	wallet, err := wallets.WalletForCurrencyCode(attempt.Currency)
	if err != nil {
		return fmt.Errorf("load frozen standard order UTXO wallet: %w", err)
	}
	addressUtilities, ok := wallet.(iwallet.UTXOAddressUtilities)
	if !ok {
		return fmt.Errorf("wallet for %s cannot derive a UTXO scriptPubKey", attempt.Currency)
	}
	scriptPubKey, err := addressUtilities.AddressToScriptPubKey(target.Address)
	if err != nil {
		return fmt.Errorf("derive frozen standard order UTXO scriptPubKey: %w", err)
	}
	if len(scriptPubKey) == 0 {
		return fmt.Errorf("frozen standard order UTXO scriptPubKey is empty")
	}
	if err := watcher.WatchPaymentAddress(
		attempt.OrderID, target.Address, coinInfo.Chain, append([]byte(nil), scriptPubKey...),
	); err != nil {
		return fmt.Errorf("watch frozen standard order UTXO funding target: %w", err)
	}
	return nil
}

// RecoverFrozenStandardOrderUTXOWatches restores durable UTXO watches from
// frozen attempt state without consulting or mutating legacy Order payment
// address fields.
func (n *MobazhaNode) RecoverFrozenStandardOrderUTXOWatches(ctx context.Context) error {
	if n == nil || n.db == nil || n.multiwallet == nil || n.paymentService == nil {
		return nil
	}
	rawProvider, ok := n.db.(rawSettlementAuthorizationDB)
	if !ok || rawProvider.RawDB() == nil {
		return fmt.Errorf("frozen standard order UTXO recovery raw database is unavailable")
	}
	var attempts []models.PaymentAttempt
	if err := rawProvider.RawDB().Where(
		"kind = ? AND state = ?",
		models.PaymentAttemptKindCryptoFundingTarget, models.PaymentAttemptFundingTargetReady,
	).Find(&attempts).Error; err != nil {
		return fmt.Errorf("list frozen standard order UTXO attempts: %w", err)
	}
	var recoveryErrors []error
	for _, attempt := range attempts {
		coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(attempt.Currency))
		if err != nil || !coinInfo.IsNative || !coinInfo.Chain.IsUTXOChain() || attempt.ExpectedModeratorPeerID != "" {
			continue
		}
		if err := n.watchFrozenStandardOrderUTXOAttempt(ctx, attempt.TenantID, attempt.AttemptID); err != nil {
			recoveryErrors = append(recoveryErrors, fmt.Errorf("recover attempt %s: %w", attempt.AttemptID, err))
		}
	}
	return errors.Join(recoveryErrors...)
}

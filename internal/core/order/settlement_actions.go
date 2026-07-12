package order

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	ordersettlement "github.com/mobazha/mobazha/internal/core/order/settlement"
	nodepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"gorm.io/gorm"
)

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
	if sigs, handled, err := s.signFrozenStandardOrderUTXOAction(ctx, coinType, action, params); handled {
		return sigs, true, err
	}
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return nil, false, err
	}
	if attemptSigner, ok := strategy.(payment.AttemptSettlementActionAuthorizer); ok && params.OrderData != nil {
		attemptContext, handled, loadErr := s.frozenSettlementAttemptActionContext(ctx, params.OrderData, coinType)
		if loadErr != nil {
			return nil, handled, loadErr
		}
		if handled {
			if action == payment.SettlementActionComplete &&
				attemptContext.localOffer.ParticipantRole == models.SettlementParticipantModerator {
				return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
			}
			applyFrozenSettlementAttemptActionParams(&params, attemptContext)
			ownerSigs, signErr := attemptSigner.SignAttemptSettlementAction(ctx, payment.AttemptSettlementActionSignRequest{
				Action: action, Sequence: params.AttemptSequence,
				TenantID: params.AttemptTenantID, LocalRole: params.AttemptLocalRole,
				Authorization: *params.AttemptAuthorization, Params: params,
			})
			if signErr != nil {
				return nil, true, signErr
			}
			return settlementActionOwnerSignaturesToProto(ownerSigs), true, nil
		}
	}
	actionSigner, ok := strategy.(payment.ActionSigner)
	if !ok {
		return nil, false, nil
	}
	ownerSigs, err := actionSigner.SignAction(ctx, action, params)
	if err != nil {
		return nil, true, err
	}
	return settlementActionOwnerSignaturesToProto(ownerSigs), true, nil
}

func settlementActionOwnerSignaturesToProto(ownerSigs []payment.ActionOwnerSignature) []*pb.Signature {
	out := make([]*pb.Signature, 0, len(ownerSigs))
	for _, sig := range ownerSigs {
		out = append(out, &pb.Signature{
			From:      []byte(sig.From),
			Signature: append([]byte(nil), sig.Signature...),
			Index:     sig.Index,
		})
	}
	return out
}

func (s *OrderAppService) signFrozenStandardOrderUTXOAction(
	ctx context.Context,
	coinType iwallet.CoinType,
	action string,
	params payment.ActionParams,
) ([]*pb.Signature, bool, error) {
	if s == nil || s.db == nil || s.signer == nil || s.settlementSigner == nil || params.OrderData == nil {
		return nil, false, nil
	}
	utxoSigner, ok := s.settlementSigner.(contracts.UTXOSettlementSigner)
	if !ok {
		return nil, false, nil
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil || !coinInfo.IsNative || !coinInfo.Chain.IsUTXOChain() {
		return nil, false, nil
	}
	if params.ReleaseInfo == nil {
		return nil, false, nil
	}
	var attempts []models.PaymentAttempt
	tenantID := strings.TrimSpace(params.OrderData.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(s.nodeID)
	}
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"tenant_id = ? AND order_id = ? AND kind = ? AND state = ?",
			tenantID, params.OrderID,
			models.PaymentAttemptKindCryptoFundingTarget, models.PaymentAttemptFundingTargetReady,
		).Find(&attempts).Error
	}); err != nil {
		return nil, true, err
	}
	if len(attempts) == 0 {
		return nil, false, nil
	}
	if len(attempts) != 1 {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	attempt := attempts[0]
	terms, err := attempt.GetSettlementTerms()
	if err != nil || terms == nil || terms.ModeratorPeerID == "" || attempt.Currency != string(coinType) {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	target, err := attempt.GetFundingTarget()
	if err != nil || target == nil || !strings.EqualFold(target.RedeemScriptHex, strings.TrimSpace(params.Script)) {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	bundle, err := attempt.GetAuthorizationBundle()
	if err != nil || bundle == nil {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	var localOffer *models.SettlementKeyOffer
	for i := range bundle.Offers {
		if bundle.Offers[i].ParticipantPeerID == s.signer.PeerID().String() {
			localOffer = &bundle.Offers[i]
			break
		}
	}
	if localOffer == nil {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	if action == payment.SettlementActionComplete && localOffer.ParticipantRole == models.SettlementParticipantModerator {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	keyRef := contracts.SettlementKeyRef{
		TenantID: attempt.TenantID, RailID: attempt.Currency,
		Purpose:     contracts.StandardOrderSettlementKeyPurpose + ":" + string(localOffer.ParticipantRole),
		ReferenceID: attempt.AuthorizationContextID,
	}
	publicKey, err := s.settlementSigner.PublicKey(ctx, keyRef)
	if err != nil {
		return nil, true, err
	}
	if !bytes.Equal(publicKey, localOffer.PublicKey) {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	txn := iwallet.Transaction{}
	appendOutput := func(address, amount string) {
		if strings.TrimSpace(address) != "" && iwallet.NewAmount(amount).Cmp(iwallet.NewAmount(0)) > 0 {
			txn.To = append(txn.To, iwallet.SpendInfo{Address: iwallet.NewAddress(address, coinType), Amount: iwallet.NewAmount(amount)})
		}
	}
	switch release := params.ReleaseInfo.(type) {
	case *pb.EscrowRelease:
		if action != payment.SettlementActionComplete ||
			validateFrozenStandardOrderCompleteRelease(release, *terms, *target) != nil {
			return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
		}
		for _, outpoint := range release.Outpoints {
			txn.From = append(txn.From, iwallet.SpendInfo{ID: outpoint.FromID, Amount: iwallet.NewAmount(outpoint.Value)})
		}
		appendOutput(release.ToAddress, release.ToAmount)
		appendOutput(release.PlatformAddress, release.PlatformAmount)
		appendOutput(release.AffiliateAddress, release.AffiliateAmount)
	case *pb.DisputeClose_ModeratedEscrowRelease:
		if action != payment.SettlementActionDisputeRelease ||
			validateFrozenStandardOrderDisputeRelease(release, *terms, *target) != nil {
			return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
		}
		for _, outpoint := range release.Outpoints {
			txn.From = append(txn.From, iwallet.SpendInfo{ID: outpoint.FromID, Amount: iwallet.NewAmount(outpoint.Value)})
		}
		appendOutput(release.BuyerAddress, release.BuyerAmount)
		appendOutput(release.VendorAddress, release.VendorAmount)
		appendOutput(release.ModeratorAddress, release.ModeratorAmount)
	default:
		return nil, false, nil
	}
	script, err := hex.DecodeString(target.RedeemScriptHex)
	if err != nil || len(script) == 0 {
		return nil, true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	sigs, err := utxoSigner.SignUTXOMultisig(ctx, contracts.UTXOMultisigSettlementSignRequest{
		KeyRef: keyRef, OrderID: attempt.OrderID, AttemptID: attempt.AttemptID,
		Action: action, Sequence: 1, TermsHash: attempt.SettlementTermsHash,
		CoinCode: attempt.Currency, Transaction: txn, RedeemScript: script,
	})
	if err != nil {
		return nil, true, err
	}
	out := make([]*pb.Signature, 0, len(sigs))
	for _, sig := range sigs {
		out = append(out, &pb.Signature{Signature: append([]byte(nil), sig.Signature...), Index: uint32(sig.Index)})
	}
	return out, true, nil
}

func validateFrozenStandardOrderCompleteRelease(
	release *pb.EscrowRelease,
	terms models.PaymentAttemptSettlementTerms,
	target models.PaymentAttemptFundingTarget,
) error {
	if release == nil || terms.ModeratorPeerID == "" || terms.SellerGrossBasis != terms.FundingAmount ||
		target.AmountAtomic != terms.FundingAmount || !payment.SameUTXOAddress(release.ToAddress, terms.SellerAddress) {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	inputTotal, err := settlementReleaseInputTotal(release.Outpoints)
	if err != nil || inputTotal.String() != target.AmountAtomic {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	transactionFee, err := canonicalSettlementActionAmount(release.TransactionFee, false)
	if err != nil {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	platformAmount, err := canonicalSettlementActionAmount(release.PlatformAmount, false)
	if err != nil || platformAmount.String() != terms.PlatformReleaseFee.Amount ||
		!sameSettlementActionAddress(release.PlatformAddress, terms.PlatformReleaseFee.Address, platformAmount) {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	affiliateAmount := new(big.Int)
	if terms.Affiliate == nil {
		// Empty affiliate fields mean that no affiliate terms were seller-signed.
		// Keep that distinct from an explicit zero, which is evidence that terms
		// exist but round to no executable output.
		if strings.TrimSpace(release.AffiliateAddress) != "" || strings.TrimSpace(release.AffiliateAmount) != "" {
			return models.ErrPaymentAttemptSettlementTermsConflict
		}
	} else {
		affiliateAmount, err = canonicalSettlementActionAmount(release.AffiliateAmount, false)
		if err != nil || affiliateAmount.String() != terms.Affiliate.Amount ||
			!sameSettlementActionAddress(release.AffiliateAddress, terms.Affiliate.Address, affiliateAmount) {
			return models.ErrPaymentAttemptSettlementTermsConflict
		}
	}
	sellerAmount, err := canonicalSettlementActionAmount(release.ToAmount, true)
	if err != nil {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	expectedSellerAmount := new(big.Int).Set(inputTotal)
	expectedSellerAmount.Sub(expectedSellerAmount, transactionFee)
	expectedSellerAmount.Sub(expectedSellerAmount, platformAmount)
	expectedSellerAmount.Sub(expectedSellerAmount, affiliateAmount)
	if expectedSellerAmount.Sign() <= 0 || sellerAmount.Cmp(expectedSellerAmount) != 0 {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	return nil
}

func validateFrozenStandardOrderDisputeRelease(
	release *pb.DisputeClose_ModeratedEscrowRelease,
	terms models.PaymentAttemptSettlementTerms,
	target models.PaymentAttemptFundingTarget,
) error {
	if release == nil || terms.ModeratorPeerID == "" || terms.ModeratorFee == nil || terms.Affiliate != nil ||
		target.AmountAtomic != terms.FundingAmount {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	inputTotal, err := settlementReleaseInputTotal(release.Outpoints)
	if err != nil || inputTotal.String() != target.AmountAtomic {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	transactionFee, err := canonicalSettlementActionAmount(release.TransactionFee, false)
	if err != nil {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	moderatorAmount, err := canonicalSettlementActionAmount(release.ModeratorAmount, false)
	if err != nil || moderatorAmount.String() != terms.ModeratorFee.Amount ||
		!sameSettlementActionAddress(release.ModeratorAddress, terms.ModeratorFee.Address, moderatorAmount) {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	buyerAmount, err := canonicalSettlementActionAmount(release.BuyerAmount, false)
	if err != nil || (buyerAmount.Sign() > 0 && strings.TrimSpace(release.BuyerAddress) == "") {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	vendorAmount, err := canonicalSettlementActionAmount(release.VendorAmount, false)
	if err != nil || (vendorAmount.Sign() > 0 && strings.TrimSpace(release.VendorAddress) == "") {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	expectedParticipantTotal := new(big.Int).Set(inputTotal)
	expectedParticipantTotal.Sub(expectedParticipantTotal, transactionFee)
	expectedParticipantTotal.Sub(expectedParticipantTotal, moderatorAmount)
	actualParticipantTotal := new(big.Int).Add(buyerAmount, vendorAmount)
	if expectedParticipantTotal.Sign() < 0 || actualParticipantTotal.Cmp(expectedParticipantTotal) != 0 {
		return models.ErrPaymentAttemptSettlementTermsConflict
	}
	return nil
}

func settlementReleaseInputTotal(outpoints []*pb.Outpoint) (*big.Int, error) {
	if len(outpoints) == 0 {
		return nil, models.ErrPaymentAttemptSettlementTermsConflict
	}
	total := new(big.Int)
	for _, outpoint := range outpoints {
		if outpoint == nil || len(outpoint.FromID) == 0 {
			return nil, models.ErrPaymentAttemptSettlementTermsConflict
		}
		amount, err := canonicalSettlementActionAmount(outpoint.Value, true)
		if err != nil {
			return nil, err
		}
		total.Add(total, amount)
	}
	return total, nil
}

func canonicalSettlementActionAmount(raw string, positive bool) (*big.Int, error) {
	raw = strings.TrimSpace(raw)
	amount, ok := new(big.Int).SetString(raw, 10)
	if !ok || amount.Sign() < 0 || (positive && amount.Sign() == 0) || amount.String() != raw {
		return nil, models.ErrPaymentAttemptSettlementTermsConflict
	}
	return amount, nil
}

func sameSettlementActionAddress(actual, expected string, amount *big.Int) bool {
	if amount == nil {
		return false
	}
	actual = strings.TrimSpace(actual)
	expected = strings.TrimSpace(expected)
	if amount.Sign() == 0 {
		return actual == "" && expected == ""
	}
	return actual != "" && expected != "" && payment.SameUTXOAddress(actual, expected)
}

func (s *OrderAppService) frozenStandardOrderSettlementTerms(order *models.Order) (*models.PaymentAttemptSettlementTerms, error) {
	if s == nil || s.db == nil || order == nil {
		return nil, nil
	}
	var attempts []models.PaymentAttempt
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"tenant_id = ? AND order_id = ? AND kind = ? AND state = ?",
			strings.TrimSpace(order.TenantID), order.ID.String(),
			models.PaymentAttemptKindCryptoFundingTarget, models.PaymentAttemptFundingTargetReady,
		).Find(&attempts).Error
	}); err != nil {
		return nil, err
	}
	if len(attempts) == 0 {
		return nil, nil
	}
	if len(attempts) != 1 {
		return nil, models.ErrPaymentAttemptSettlementTermsConflict
	}
	terms, err := attempts[0].GetSettlementTerms()
	if err != nil {
		return nil, err
	}
	bundle, err := attempts[0].GetAuthorizationBundle()
	if err != nil || bundle == nil {
		return nil, models.ErrPaymentAttemptSettlementTermsConflict
	}
	return terms, nil
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
	if !ordersettlement.EscrowUsesBackendSubmittedRelease(spec) {
		return payment.SettlementSpec{}, fmt.Errorf("%w: escrow type %q must use settlement-actions; client-signed legacy routes are retired",
			coreiface.ErrBadRequest, spec.EscrowType)
	}
	return spec, nil
}

func errRetiredClientSignedModeratedSettlement(action string) error {
	return fmt.Errorf("%w: moderated client-signed %s is retired; use POST /v1/orders/{orderID}/settlement-actions/%s",
		coreiface.ErrBadRequest, action, payment.SettlementActionPathSegment(action))
}

func errBalanceMonitoredEscrowRequiresSettlementAction(order *models.Order, paymentSent *pb.PaymentSent, action string) error {
	if paymentSent == nil {
		return nil
	}
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	if !ok {
		return nil
	}
	switch {
	case spec.UsesManagedEscrow():
		return fmt.Errorf("%w: backend-managed orders must use POST /v1/orders/{orderID}/settlement-actions/%s",
			coreiface.ErrBadRequest, payment.SettlementActionPathSegment(action))
	case spec.UsesSolanaEscrow():
		return fmt.Errorf("%w: Solana escrow orders must use POST /v1/orders/{orderID}/settlement-actions/%s",
			coreiface.ErrBadRequest, payment.SettlementActionPathSegment(action))
	}
	return nil
}

func (s *OrderAppService) loadSyncBackendSettlementAction(orderID, action string) (*models.SettlementAction, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	actionID := ordersettlement.SyncActionID(orderID, action)
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
	actionID = ordersettlement.SyncActionID(orderID, action)
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
			if ordersettlement.StaleSyncAction(existing.ActionID, existing.State, existing.TxHash, existing.UpdatedAt, time.Now().UTC()) {
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

func (s *OrderAppService) recordSyncBackendSettlementSubmission(
	actionID, txHash string,
	plannedLines []models.SettlementPayoutLine,
) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if txHash == "" {
		return fmt.Errorf("settlement action %s submitted without tx hash", actionID)
	}
	if len(plannedLines) == 0 {
		return fmt.Errorf("settlement action %s submitted without planned payout lines", actionID)
	}
	now := time.Now().UTC()
	return s.db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(map[string]interface{}{
			"state":             "submitted",
			"tx_hash":           txHash,
			"attempt_tx_hashes": txHash,
			"planned_lines":     models.EncodeSettlementPayoutLines(plannedLines),
			"last_error":        "",
			"confirmed_at":      nil,
			"updated_at":        now,
		}, map[string]interface{}{
			"action_id = ?": actionID,
		}, &models.SettlementAction{})
		return err
	})
}

func syncUTXOSettlementPayoutLines(
	tx *iwallet.Transaction,
	coin string,
	affiliatePayout *models.AffiliateSettlementPayout,
) ([]models.SettlementPayoutLine, error) {
	if tx == nil || len(tx.To) == 0 {
		return nil, errors.New("UTXO settlement release has no payout outputs")
	}
	coin = strings.TrimSpace(coin)
	affiliateOutputIndex := -1
	if affiliatePayout != nil {
		affiliateOutputIndex = len(tx.To) - 1
		output := tx.To[affiliateOutputIndex]
		if output.Address.String() != strings.TrimSpace(affiliatePayout.Address) ||
			output.Amount.String() != strings.TrimSpace(affiliatePayout.Amount) {
			return nil, errors.New("UTXO settlement release does not contain the frozen affiliate payout")
		}
	}
	lines := make([]models.SettlementPayoutLine, 0, len(tx.To))
	for i, output := range tx.To {
		lineType := "recipient"
		if i == affiliateOutputIndex {
			lineType = "affiliate"
		}
		lines = append(lines, models.SettlementPayoutLine{
			Type: lineType, Amount: output.Amount.String(), Address: output.Address.String(), Coin: coin,
		})
	}
	return lines, nil
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

type settlementActionIntent string

const (
	settlementIntentBuyerCancel               settlementActionIntent = "buyer_cancel"
	settlementIntentSellerDeclineFundedRefund settlementActionIntent = "seller_decline_funded_refund"
)

func (s *OrderAppService) settlementActionForIntent(
	order *models.Order,
	paymentSent *pb.PaymentSent,
	method pb.PaymentSent_Method,
	coinType iwallet.CoinType,
	intent settlementActionIntent,
) (string, bool) {
	if order == nil || paymentSent == nil {
		return "", false
	}
	if !payment.MethodIsCancelable(method) && !payment.MethodIsModerated(method) {
		return "", false
	}
	if payment.MethodIsModerated(method) && order.SerializedOrderConfirmation != nil {
		return "", false
	}
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil || strategy.Model() != payment.PaymentModelMonitored {
		return "", false
	}
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	if !ok || !ordersettlement.EscrowUsesRelayRelease(spec) {
		return "", false
	}
	switch intent {
	case settlementIntentBuyerCancel:
		return payment.SettlementActionCancel, true
	case settlementIntentSellerDeclineFundedRefund:
		if _, ok := strategy.(payment.SellerDeclineRefunder); ok {
			return payment.SettlementActionSellerDeclineRefund, true
		}
		return payment.SettlementActionCancel, true
	default:
		return "", false
	}
}

func (s *OrderAppService) canSellerDeclineFundedRefund(order *models.Order) (bool, error) {
	if order == nil || order.Role() != models.RoleVendor {
		return false, nil
	}
	if order.SerializedOrderDecline != nil ||
		order.SerializedOrderCancel != nil ||
		order.SerializedOrderConfirmation != nil ||
		order.SerializedOrderShipments != nil ||
		order.SerializedOrderComplete != nil ||
		order.SerializedDisputeOpen != nil ||
		order.SerializedDisputeUpdate != nil ||
		order.SerializedDisputeClosed != nil ||
		order.SerializedRefunds != nil ||
		order.SerializedPaymentFinalized != nil {
		return false, nil
	}
	funded, err := order.IsFunded()
	if err != nil || !funded {
		return false, err
	}
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		if models.IsMessageNotExistError(err) {
			return false, nil
		}
		return false, err
	}
	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return false, err
	}
	method, ok := payment.ResolvedPaymentMethod(order, paymentSent)
	if !ok || (!payment.MethodIsCancelable(method) && !payment.MethodIsModerated(method)) {
		return false, nil
	}
	_, ok = s.settlementActionForIntent(order, paymentSent, method, coinType, settlementIntentSellerDeclineFundedRefund)
	return ok, nil
}

// evaluateMonitoredSettlementRelease checks pending/ready state for a backend
// settlement release action (complete or dispute_release).
func evaluateMonitoredSettlementRelease(
	order *models.Order,
	txid iwallet.TransactionID,
	actionName string,
) (resolvedTxid iwallet.TransactionID, releaseAlreadySubmitted bool, err error) {
	resolved, submitted, err := ordersettlement.EvaluateRelease(order, txid, actionName)
	if err != nil {
		return "", false, fmt.Errorf("%w: %s", coreiface.ErrBadRequest, err)
	}
	return resolved, submitted, nil
}

func (s *OrderAppService) submitSettlementCancelAction(ctx context.Context, order *models.Order, coinType iwallet.CoinType, paymentSent *pb.PaymentSent, payoutAddr string, releaseInfo ...any) (iwallet.TransactionID, *iwallet.Transaction, bool, error) {
	return s.submitSettlementAction(ctx, payment.SettlementActionCancel, order, coinType, paymentSent, payoutAddr, releaseInfo...)
}

func (s *OrderAppService) submitSettlementSellerDeclineRefundAction(ctx context.Context, order *models.Order, coinType iwallet.CoinType, paymentSent *pb.PaymentSent, payoutAddr string, releaseInfo ...any) (iwallet.TransactionID, *iwallet.Transaction, bool, error) {
	return s.submitSettlementAction(ctx, payment.SettlementActionSellerDeclineRefund, order, coinType, paymentSent, payoutAddr, releaseInfo...)
}

func (s *OrderAppService) submitSettlementAction(ctx context.Context, action string, order *models.Order, coinType iwallet.CoinType, paymentSent *pb.PaymentSent, payoutAddr string, releaseInfo ...any) (iwallet.TransactionID, *iwallet.Transaction, bool, error) {
	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return "", nil, false, err
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		return "", nil, false, nil
	}
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	if !ok || !ordersettlement.EscrowUsesRelayRelease(spec) {
		// UTXO cancelable confirm/cancel still uses ConfirmOrder / escrow inline release.
		return "", nil, false, nil
	}

	if payoutAddr == "" && (action == payment.SettlementActionCancel || action == payment.SettlementActionSellerDeclineRefund) {
		observations := payment.RefundResolutionObservations(s.db, order, paymentSent)
		refundResult := nodepayment.ResolveBuyerRefundForLocalNode(s.db, order, paymentSent, coinType, observations, false)
		if !refundResult.Found() {
			return "", nil, false, fmt.Errorf("%w: no buyer refund address available for settlement %s (%s)",
				models.ErrRefundAddressRequired, action, refundResult.Reason)
		}
		payoutAddr = refundResult.Address
	}

	params := payment.ActionParams{
		OrderID:       order.ID.String(),
		PaymentCoin:   string(coinType),
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     order,
		PayoutAddr:    payoutAddr,
	}
	if len(releaseInfo) > 0 {
		params.ReleaseInfo = releaseInfo[0]
	}
	if _, ok := strategy.(payment.AttemptSettlementActionAuthorizer); ok {
		attemptContext, handled, loadErr := s.frozenSettlementAttemptActionContext(ctx, order, coinType)
		if loadErr != nil {
			return "", nil, handled, loadErr
		}
		if handled {
			applyFrozenSettlementAttemptActionParams(&params, attemptContext)
		}
	}
	var result *payment.ActionResult
	switch action {
	case payment.SettlementActionCancel:
		result, err = strategy.Cancel(ctx, params)
	case payment.SettlementActionSellerDeclineRefund:
		refunder, ok := strategy.(payment.SellerDeclineRefunder)
		if !ok {
			return "", nil, true, fmt.Errorf("%w: settlement action %s is not supported for %s", payment.ErrUnsupportedAction, action, coinType)
		}
		result, err = refunder.SellerDeclineRefund(ctx, params)
	default:
		return "", nil, true, fmt.Errorf("%w: unsupported settlement action %s", payment.ErrUnsupportedAction, action)
	}
	if err != nil {
		return "", nil, true, err
	}

	txHash := ordersettlement.ActionRelayTxHash(ctx, strategy, result)
	if txHash == "" {
		return "", nil, true, fmt.Errorf("settlement %s action submitted without tx hash (order %s)", action, order.ID)
	}
	txid := iwallet.TransactionID(txHash)
	return txid, &iwallet.Transaction{ID: txid}, true, nil
}

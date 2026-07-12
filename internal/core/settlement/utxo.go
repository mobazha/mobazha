package settlement

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type frozenStandardOrderUTXOReleaseAuthorization struct {
	attempt models.PaymentAttempt
	target  models.PaymentAttemptFundingTarget
	offer   models.SettlementKeyOffer
	role    models.SettlementParticipantRole
	action  string
}

// ── EscrowOperations port: ReleaseFromCancelableAddressWithParams ───────

// ReleaseFromCancelableAddressWithParams is the core UTXO escrow release implementation.
// It signs a multisig release transaction using the escrow master key from KeyProvider.
func (s *SettlementService) ReleaseFromCancelableAddressWithParams(order *models.Order, params contracts.ReleaseFromCancelableParams) (iwallet.Tx, *iwallet.Transaction, error) {
	logger.LogDebugWithIDf(log, s.nodeID, "ReleaseFromCancelableAddressWithParams: starting for order %s", order.ID)

	wallet, err := s.multiwallet.WalletForCurrencyCode(params.CoinCode)
	if err != nil {
		return nil, nil, err
	}

	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return nil, nil, errors.New("wallet does not support escrow")
	}

	txs, err := s.transactionsForCancelableRelease(wallet, order, params)
	if err != nil {
		return nil, nil, err
	}

	txn, totalOut := collectCancelableReleaseInputs(txs, params.PaymentAddress)
	if len(txn.From) == 0 {
		return nil, nil, errors.New("payment address is empty")
	}

	if err := s.verifyUTXOsOnChain(params.CoinCode, params.PaymentAddress, txn.From); err != nil {
		return nil, nil, fmt.Errorf("UTXO chain verification failed: %w", err)
	}

	nOuts := 1
	if params.AffiliatePayout != nil {
		nOuts++
	}
	escrowFee, err := escrowWallet.EstimateEscrowFee(len(txn.From), 1, nOuts, iwallet.FlNormal)
	if err != nil {
		return nil, nil, err
	}

	if escrowFee.Cmp(totalOut) >= 0 {
		return nil, nil, fmt.Errorf("insufficient funds: total input %s is less than or equal to fee %s", totalOut.String(), escrowFee.String())
	}

	sellerAmount := totalOut.Sub(escrowFee)
	var affiliateSpend *iwallet.SpendInfo
	if params.AffiliatePayout != nil {
		spend, err := cancelableAffiliateUTXOSpend(wallet, iwallet.CoinType(params.CoinCode), params.AffiliatePayout, sellerAmount)
		if err != nil {
			return nil, nil, err
		}
		sellerAmount = sellerAmount.Sub(spend.Amount)
		affiliateSpend = &spend
	}
	txn.To = append(txn.To, iwallet.SpendInfo{
		Address: params.ToAddress,
		Amount:  sellerAmount,
	})
	if affiliateSpend != nil {
		txn.To = append(txn.To, *affiliateSpend)
	}

	frozenAuthorization, err := s.frozenStandardOrderUTXOReleaseAuthorization(order, params)
	if err != nil {
		return nil, nil, err
	}
	if frozenAuthorization != nil {
		switch frozenAuthorization.action {
		case payment.SettlementActionComplete:
			if totalOut.String() != frozenAuthorization.target.AmountAtomic {
				return nil, nil, models.ErrPaymentAttemptSettlementTermsConflict
			}
		case payment.SettlementActionCancel:
			expected := iwallet.NewAmount(frozenAuthorization.target.AmountAtomic)
			if totalOut.Cmp(iwallet.NewAmount(0)) <= 0 || totalOut.Cmp(expected) > 0 {
				return nil, nil, models.ErrPaymentAttemptSettlementTermsConflict
			}
		default:
			return nil, nil, models.ErrPaymentAttemptSettlementTermsConflict
		}
	}
	scriptHex := params.ScriptHex
	if frozenAuthorization != nil {
		scriptHex = frozenAuthorization.target.RedeemScriptHex
	}
	script, err := hex.DecodeString(scriptHex)
	if err != nil {
		return nil, nil, err
	}
	var sigs []iwallet.EscrowSignature
	if frozenAuthorization != nil {
		utxoSigner, ok := s.settlementSigner.(contracts.UTXOSettlementSigner)
		if !ok || s.settlementSigner == nil {
			return nil, nil, fmt.Errorf("attempt-scoped UTXO settlement signer is not configured")
		}
		keyRef := contracts.SettlementKeyRef{
			TenantID:    frozenAuthorization.attempt.TenantID,
			RailID:      frozenAuthorization.attempt.Currency,
			Purpose:     contracts.StandardOrderSettlementKeyPurpose + ":" + string(frozenAuthorization.role),
			ReferenceID: frozenAuthorization.attempt.AuthorizationContextID,
		}
		publicKey, err := s.settlementSigner.PublicKey(context.Background(), keyRef)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve local attempt-scoped UTXO settlement public key: %w", err)
		}
		if !bytes.Equal(publicKey, frozenAuthorization.offer.PublicKey) {
			return nil, nil, models.ErrPaymentAttemptSettlementTermsConflict
		}
		sigs, err = utxoSigner.SignUTXOMultisig(
			context.Background(),
			contracts.UTXOMultisigSettlementSignRequest{
				KeyRef: keyRef, OrderID: order.ID.String(), AttemptID: frozenAuthorization.attempt.AttemptID,
				Action: frozenAuthorization.action, Sequence: 1,
				TermsHash: frozenAuthorization.attempt.SettlementTermsHash,
				CoinCode:  params.CoinCode, Transaction: txn, RedeemScript: script,
			},
		)
		if err != nil {
			return nil, nil, err
		}
	} else {
		chainCode, err := hex.DecodeString(params.ChaincodeHex)
		if err != nil {
			return nil, nil, err
		}
		escrowMasterKey, err := s.keys.EscrowMasterKey()
		if err != nil {
			return nil, nil, fmt.Errorf("get escrow master key: %w", err)
		}
		key, err := utils.GenerateEscrowPrivateKey(escrowMasterKey, chainCode)
		if err != nil {
			return nil, nil, err
		}
		sigs, err = escrowWallet.SignMultisigTransaction(txn, *key, script)
		if err != nil {
			return nil, nil, err
		}
	}

	dbTx, err := wallet.Begin()
	if err != nil {
		return nil, nil, err
	}

	txid, err := escrowWallet.BuildAndSend(dbTx, txn, [][]iwallet.EscrowSignature{sigs}, script, params.FinishType)
	if err != nil {
		return nil, nil, err
	}

	txn.ID = txid
	txn.Timestamp = time.Now()

	logger.LogInfoWithIDf(log, s.nodeID, "Released escrow funds: txid=%s, to=%s, amount=%s",
		txid, params.ToAddress, sellerAmount.String())

	return dbTx, &txn, nil
}

func (s *SettlementService) frozenStandardOrderUTXOReleaseAuthorization(
	order *models.Order,
	params contracts.ReleaseFromCancelableParams,
) (*frozenStandardOrderUTXOReleaseAuthorization, error) {
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
		return nil, fmt.Errorf("load frozen standard order UTXO release attempt: %w", err)
	}
	if len(attempts) == 0 {
		return nil, nil
	}
	var matched *models.PaymentAttempt
	var target *models.PaymentAttemptFundingTarget
	for i := range attempts {
		candidate, err := attempts[i].GetFundingTarget()
		if err != nil {
			return nil, err
		}
		if candidate == nil || !payment.SameUTXOAddress(candidate.Address, params.PaymentAddress) {
			continue
		}
		if matched != nil {
			return nil, fmt.Errorf("multiple frozen attempts match UTXO release target")
		}
		matched = &attempts[i]
		target = candidate
	}
	if matched == nil || target == nil {
		return nil, models.ErrPaymentAttemptSettlementTermsConflict
	}
	if matched.Currency != strings.TrimSpace(params.CoinCode) || target.RedeemScriptHex == "" ||
		(strings.TrimSpace(params.ScriptHex) != "" && !strings.EqualFold(params.ScriptHex, target.RedeemScriptHex)) ||
		params.AffiliatePayout != nil {
		return nil, models.ErrPaymentAttemptSettlementTermsConflict
	}
	terms, err := matched.GetSettlementTerms()
	if err != nil || terms == nil || terms.ModeratorPeerID != "" || terms.Affiliate != nil ||
		terms.PlatformReleaseFee.Amount != "0" || terms.BuyerCancellationFee.Amount != "0" ||
		terms.SellerGrossBasis != terms.FundingAmount {
		return nil, models.ErrPaymentAttemptSettlementTermsConflict
	}
	bundle, err := matched.GetAuthorizationBundle()
	if err != nil || bundle == nil {
		return nil, models.ErrPaymentAttemptSettlementTermsConflict
	}
	role := models.SettlementParticipantBuyer
	action := payment.SettlementActionCancel
	if order.Role() == models.RoleVendor {
		role = models.SettlementParticipantSeller
		action = payment.SettlementActionComplete
		if params.FinishType != iwallet.ORDER_FINISH_COMPLETE ||
			!payment.SameUTXOAddress(params.ToAddress.String(), terms.SellerAddress) {
			return nil, models.ErrPaymentAttemptSettlementTermsConflict
		}
	} else {
		refundAddress := strings.TrimSpace(order.RefundAddress)
		if order.Role() != models.RoleBuyer || params.FinishType != iwallet.ORDER_FINISH_CANCEL ||
			refundAddress == "" || !payment.SameUTXOAddress(params.ToAddress.String(), refundAddress) {
			return nil, models.ErrPaymentAttemptSettlementTermsConflict
		}
	}
	var offer *models.SettlementKeyOffer
	for i := range bundle.Offers {
		if bundle.Offers[i].ParticipantRole == role {
			offer = &bundle.Offers[i]
			break
		}
	}
	if offer == nil || offer.Purpose != contracts.StandardOrderSettlementKeyPurpose+":"+string(role) {
		return nil, models.ErrPaymentAttemptSettlementTermsConflict
	}
	return &frozenStandardOrderUTXOReleaseAuthorization{
		attempt: *matched, target: *target, offer: *offer, role: role, action: action,
	}, nil
}

type cancelableAffiliateUTXODustChecker interface {
	IsDust(iwallet.Address, iwallet.Amount) bool
}

func cancelableAffiliateUTXOSpend(wallet iwallet.Wallet, coinType iwallet.CoinType, payout *models.AffiliateSettlementPayout, available iwallet.Amount) (iwallet.SpendInfo, error) {
	amount, ok := new(big.Int).SetString(strings.TrimSpace(payout.Amount), 10)
	if wallet == nil || !ok || amount.Sign() <= 0 {
		return iwallet.SpendInfo{}, fmt.Errorf("affiliate UTXO payout amount is invalid")
	}
	spend := iwallet.SpendInfo{Address: iwallet.NewAddress(strings.TrimSpace(payout.Address), coinType), Amount: iwallet.NewAmount(amount)}
	if err := wallet.ValidateAddress(spend.Address); err != nil {
		return iwallet.SpendInfo{}, fmt.Errorf("validate affiliate UTXO payout address: %w", err)
	}
	if spend.Amount.Cmp(available) >= 0 {
		return iwallet.SpendInfo{}, fmt.Errorf("affiliate UTXO payout exceeds seller release")
	}
	dustChecker, ok := wallet.(cancelableAffiliateUTXODustChecker)
	if !ok || dustChecker.IsDust(spend.Address, spend.Amount) {
		return iwallet.SpendInfo{}, fmt.Errorf("affiliate UTXO payout is dust")
	}
	return spend, nil
}

func (s *SettlementService) transactionsForCancelableRelease(
	wallet iwallet.Wallet,
	order *models.Order,
	params contracts.ReleaseFromCancelableParams,
) ([]iwallet.Transaction, error) {
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		if !models.IsMessageNotExistError(err) {
			return nil, err
		}
		txs, txErr := order.GetTransactions()
		if txErr != nil {
			return nil, fmt.Errorf("UTXO settlement requires PaymentSent funding facts or recovered transactions: %w", txErr)
		}
		return txs, nil
	}
	if len(paymentSent.GetFundingFacts()) == 0 {
		return nil, fmt.Errorf("UTXO settlement requires PaymentSent funding facts")
	}

	return s.resolveUTXOFundingTransactionsFromPaymentSent(wallet, order, paymentSent, params)
}

func collectCancelableReleaseInputs(txs []iwallet.Transaction, paymentAddress string) (iwallet.Transaction, iwallet.Amount) {
	return payment.CollectUnspentOutputsForAddress(txs, paymentAddress)
}

func (s *SettlementService) resolveUTXOFundingTransactionsFromPaymentSent(
	wallet iwallet.Wallet,
	order *models.Order,
	paymentSent *pb.PaymentSent,
	params contracts.ReleaseFromCancelableParams,
) ([]iwallet.Transaction, error) {
	txs, err := payment.ResolveUTXOFundingTransactionsFromPaymentSent(wallet, iwallet.CoinType(params.CoinCode), paymentSent, params.PaymentAddress)
	if err != nil {
		return nil, err
	}
	for _, tx := range txs {
		if err := order.PutTransaction(tx); err != nil {
			if !models.IsDuplicateTransactionError(err) {
				return nil, err
			}
			if err := order.UpdateTransaction(tx); err != nil {
				return nil, err
			}
		}
	}
	logger.LogInfoWithIDf(log, s.nodeID, "Resolved %d UTXO funding transaction(s) from PaymentSent funding facts for order %s", len(txs), order.ID)
	return txs, nil
}

// verifyUTXOsOnChain queries the chain to verify that expected UTXOs are still unspent.
// Best-effort: if the monitor is unavailable, the address is not watched, or the chain
// query fails, verification is skipped and the caller proceeds with local data.
func (s *SettlementService) verifyUTXOsOnChain(coinCode string, paymentAddress string, expectedUTXOs []iwallet.SpendInfo) error {
	if s.monitorService == nil {
		return nil
	}

	coinType := iwallet.CoinType(coinCode)
	coinInfo, err := payment.SettlementCoinInfoForCoin(coinType)
	if err != nil || !coinInfo.Chain.IsUTXOChain() {
		return nil
	}

	wa := s.monitorService.GetWatchedAddress(paymentAddress)
	if wa == nil || len(wa.ScriptPubKey) == 0 {
		logger.LogWarningWithIDf(log, s.nodeID, "Cannot verify UTXOs: address %s not watched or missing scriptPubKey, skipping chain verification", paymentAddress)
		return nil
	}

	if handled, err := s.verifyUTXOsWithListUnspent(coinInfo.Chain, paymentAddress, wa.ScriptPubKey, expectedUTXOs); handled {
		return err
	}

	chainTxs, err := s.monitorService.GetAddressTransactions(coinInfo.Chain, paymentAddress, wa.ScriptPubKey)
	if err != nil {
		logger.LogWarningWithIDf(log, s.nodeID, "Chain UTXO verification query failed for %s: %v, proceeding with local data", paymentAddress, err)
		return nil
	}

	chainSpent := make(map[string]bool)
	for _, tx := range chainTxs {
		for _, from := range tx.From {
			chainSpent[hex.EncodeToString(from.ID)] = true
		}
	}
	chainUnspent := make(map[string]bool)
	for _, tx := range chainTxs {
		for _, to := range tx.To {
			id := hex.EncodeToString(to.ID)
			if !chainSpent[id] && payment.SameUTXOAddress(to.Address.String(), paymentAddress) {
				chainUnspent[id] = true
			}
		}
	}

	for _, utxo := range expectedUTXOs {
		id := hex.EncodeToString(utxo.ID)
		if chainSpent[id] {
			return fmt.Errorf("%w: outpoint %s is already spent from address %s", contracts.ErrUTXOAlreadySpent, id, paymentAddress)
		}
		if !chainUnspent[id] {
			logger.LogWarningWithIDf(log, s.nodeID, "Chain UTXO verification could not find outpoint %s in reconstructed unspent set for address %s; proceeding because no spend was observed", id, paymentAddress)
		}
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Chain UTXO verification passed: %d UTXOs confirmed unspent at %s", len(expectedUTXOs), paymentAddress)
	return nil
}

func (s *SettlementService) verifyUTXOsWithListUnspent(
	chain iwallet.ChainType,
	paymentAddress string,
	scriptPubKey []byte,
	expectedUTXOs []iwallet.SpendInfo,
) (bool, error) {
	utxos, err := s.monitorService.ListUnspent(chain, scriptPubKey)
	if err != nil {
		logger.LogWarningWithIDf(log, s.nodeID, "ListUnspent UTXO verification query failed for %s: %v; falling back to address history", paymentAddress, err)
		return false, nil
	}

	unspent := make(map[string]struct{}, len(utxos))
	for _, utxo := range utxos {
		id, ok := payment.UTXOOutpointID(utxo.TxHash, utxo.OutputIndex)
		if !ok {
			continue
		}
		unspent[hex.EncodeToString(id)] = struct{}{}
	}
	for _, expected := range expectedUTXOs {
		id := hex.EncodeToString(expected.ID)
		if _, ok := unspent[id]; !ok {
			return true, fmt.Errorf("%w: outpoint %s not found in current unspent set for address %s", contracts.ErrUTXOAlreadySpent, id, paymentAddress)
		}
	}

	logger.LogInfoWithIDf(log, s.nodeID, "ListUnspent verification passed: %d UTXOs confirmed unspent at %s", len(expectedUTXOs), paymentAddress)
	return true, nil
}

// ── Partial Payment Release ─────────────────────────────────────────────

// ReleasePartialPayment releases funds from a CANCELABLE address when PaymentSent doesn't exist yet.
func (s *SettlementService) ReleasePartialPayment(order *models.Order) (iwallet.Tx, *iwallet.Transaction, error) {
	pendingInfo, err := order.GetPendingPaymentInfo()
	if err != nil || pendingInfo == nil {
		return nil, nil, fmt.Errorf("no pending payment info")
	}

	coinType := iwallet.CoinType(pendingInfo.Coin)

	wallet, err := s.multiwallet.WalletForCurrencyCode(pendingInfo.Coin)
	if err != nil {
		return nil, nil, fmt.Errorf("get wallet failed: %v", err)
	}

	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return nil, nil, fmt.Errorf("wallet does not support escrow")
	}

	keys, err := s.utxoKeyDeriver.GetUTXOEscrowKeys(context.Background(), order, "")
	if err != nil {
		return nil, nil, err
	}

	_, script, err := escrowWallet.CreateMultisigAddress(
		[]btcec.PublicKey{*keys.BuyerKey, *keys.VendorKey}, keys.Chaincode, 1)
	if err != nil {
		return nil, nil, fmt.Errorf("create multisig address: %v", err)
	}

	txs, err := order.GetTransactions()
	if err != nil {
		return nil, nil, fmt.Errorf("get transactions failed: %v", err)
	}
	refundAddress, err := getRefundAddressFromTransactions(txs, coinType)
	if err != nil {
		return nil, nil, fmt.Errorf("get refund address failed: %v", err)
	}

	params := contracts.ReleaseFromCancelableParams{
		CoinCode:       pendingInfo.Coin,
		PaymentAddress: order.PaymentAddress,
		ScriptHex:      hex.EncodeToString(script),
		ChaincodeHex:   hex.EncodeToString(keys.Chaincode),
		ToAddress:      refundAddress,
		FinishType:     iwallet.ORDER_FINISH_CANCEL,
	}

	return s.ReleaseFromCancelableAddressWithParams(order, params)
}

// CancelPartialPayment cancels partial payment and returns funds to buyer.
func (s *SettlementService) CancelPartialPayment(orderID string) (txid string, refundedAmount uint64, err error) {
	order, err := s.fetchOrderByID(orderID)
	if err != nil {
		return "", 0, fmt.Errorf("get order failed: %v", err)
	}

	if _, err := order.PaymentSentMessage(); err == nil {
		return "", 0, fmt.Errorf("cannot cancel partial payment: PaymentSent already exists, use normal cancel")
	}

	pendingInfo, _ := order.GetPendingPaymentInfo()
	if order.PaymentAddress == "" || pendingInfo == nil || pendingInfo.Coin == "" {
		return s.cancelFrozenStandardOrderPartialPayment(order)
	}

	totalPaid, err := calculateTotalPaidToAddress(order, order.PaymentAddress)
	if err != nil {
		return "", 0, fmt.Errorf("calculate paid amount failed: %v", err)
	}

	if totalPaid.Cmp(iwallet.NewAmount(0)) <= 0 {
		return "", 0, fmt.Errorf("no payments found to cancel")
	}

	wTx, releaseTx, err := s.ReleasePartialPayment(order)
	if err != nil {
		return "", 0, fmt.Errorf("release partial payment failed: %v", err)
	}

	oldPaymentAddress := order.PaymentAddress

	if err := wTx.Commit(); err != nil {
		return "", 0, fmt.Errorf("commit transaction failed: %v", err)
	}

	if err := s.db.Update(func(dbtx database.Tx) error {
		if releaseTx != nil {
			if err := order.PutTransaction(*releaseTx); err != nil && !models.IsDuplicateTransactionError(err) {
				return fmt.Errorf("save release transaction: %w", err)
			}
		}
		order.PaymentAddress = ""
		order.ClearPendingPaymentInfo()
		return dbtx.Save(order)
	}); err != nil {
		return "", 0, fmt.Errorf("save order failed: %v", err)
	}

	if oldPaymentAddress != "" && s.monitorService != nil {
		if err := s.monitorService.UnwatchAddress(oldPaymentAddress); err != nil {
			logger.LogWarningWithIDf(log, s.nodeID, "Failed to unwatch payment address for order %s: %v", orderID, err)
		}
	}

	var txidStr string
	if releaseTx != nil {
		txidStr = releaseTx.ID.String()
	}
	return txidStr, totalPaid.Uint64(), nil
}

func (s *SettlementService) cancelFrozenStandardOrderPartialPayment(
	order *models.Order,
) (txid string, refundedAmount uint64, err error) {
	if s == nil || s.db == nil || s.multiwallet == nil || s.monitorService == nil || order == nil {
		return "", 0, fmt.Errorf("frozen partial payment cancellation is not configured")
	}
	if order.Role() != models.RoleBuyer {
		return "", 0, fmt.Errorf("frozen partial payment cancellation requires the buyer order")
	}
	var attempts []models.PaymentAttempt
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"tenant_id = ? AND order_id = ? AND kind = ? AND state = ?",
			strings.TrimSpace(order.TenantID), order.ID.String(),
			models.PaymentAttemptKindCryptoFundingTarget, models.PaymentAttemptFundingTargetReady,
		).Find(&attempts).Error
	}); err != nil {
		return "", 0, fmt.Errorf("load frozen partial payment attempt: %w", err)
	}
	if len(attempts) != 1 {
		return "", 0, fmt.Errorf("frozen partial payment cancellation requires exactly one active attempt")
	}
	attempt := attempts[0]
	target, err := attempt.GetFundingTarget()
	if err != nil || target == nil || strings.TrimSpace(target.RedeemScriptHex) == "" {
		return "", 0, models.ErrPaymentAttemptSettlementTermsConflict
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(attempt.Currency))
	if err != nil || !coinInfo.IsNative || !coinInfo.Chain.IsUTXOChain() {
		return "", 0, models.ErrPaymentAttemptSettlementTermsConflict
	}
	wallet, err := s.multiwallet.WalletForCurrencyCode(attempt.Currency)
	if err != nil {
		return "", 0, fmt.Errorf("load frozen partial payment wallet: %w", err)
	}
	addressUtilities, ok := wallet.(iwallet.UTXOAddressUtilities)
	if !ok {
		return "", 0, fmt.Errorf("wallet for %s cannot derive UTXO scriptPubKey", attempt.Currency)
	}
	scriptPubKey, err := addressUtilities.AddressToScriptPubKey(target.Address)
	if err != nil {
		return "", 0, fmt.Errorf("derive frozen partial payment scriptPubKey: %w", err)
	}
	if len(scriptPubKey) == 0 {
		return "", 0, fmt.Errorf("frozen partial payment scriptPubKey is empty")
	}
	txs, err := s.frozenPartialPaymentTransactions(
		wallet, order, target, coinInfo.Chain, scriptPubKey,
	)
	if err != nil {
		return "", 0, err
	}
	releasePlan, totalPaid := collectCancelableReleaseInputs(txs, target.Address)
	if len(releasePlan.From) == 0 || totalPaid.Cmp(iwallet.NewAmount(0)) <= 0 {
		return "", 0, fmt.Errorf("no payments found to cancel")
	}
	if totalPaid.Cmp(iwallet.NewAmount(target.AmountAtomic)) > 0 {
		return "", 0, models.ErrPaymentAttemptSettlementTermsConflict
	}
	for _, tx := range txs {
		if err := order.PutTransaction(tx); err != nil && !models.IsDuplicateTransactionError(err) {
			return "", 0, fmt.Errorf("retain frozen partial payment transaction: %w", err)
		}
	}
	refundAddress := strings.TrimSpace(order.RefundAddress)
	if refundAddress == "" {
		return "", 0, models.ErrRefundAddressRequired
	}
	refund := iwallet.NewAddress(refundAddress, iwallet.CoinType(attempt.Currency))
	if err := wallet.ValidateAddress(refund); err != nil {
		return "", 0, fmt.Errorf("validate frozen partial payment refund address: %w", err)
	}
	walletTx, releaseTx, err := s.ReleaseFromCancelableAddressWithParams(order, contracts.ReleaseFromCancelableParams{
		CoinCode: attempt.Currency, PaymentAddress: target.Address,
		ScriptHex: target.RedeemScriptHex, ToAddress: refund,
		FinishType: iwallet.ORDER_FINISH_CANCEL,
	})
	if err != nil {
		return "", 0, fmt.Errorf("release frozen partial payment: %w", err)
	}
	if err := walletTx.Commit(); err != nil {
		return "", 0, fmt.Errorf("commit frozen partial payment cancellation: %w", err)
	}
	if err := s.db.Update(func(tx database.Tx) error {
		if releaseTx != nil {
			if err := order.PutTransaction(*releaseTx); err != nil && !models.IsDuplicateTransactionError(err) {
				return fmt.Errorf("save frozen partial payment release: %w", err)
			}
		}
		result := tx.Read().Model(&models.PaymentAttempt{}).Where(
			"tenant_id = ? AND attempt_id = ? AND state = ?",
			attempt.TenantID, attempt.AttemptID, models.PaymentAttemptFundingTargetReady,
		).Updates(map[string]any{
			"state": models.PaymentAttemptAbandoned, "last_error": "buyer cancelled partial funding",
		})
		if result.Error != nil {
			return fmt.Errorf("abandon frozen partial payment attempt: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("abandon frozen partial payment attempt: state changed concurrently")
		}
		return tx.Save(order)
	}); err != nil {
		return "", 0, fmt.Errorf("persist frozen partial payment cancellation: %w", err)
	}
	if err := s.monitorService.UnwatchAddress(target.Address); err != nil {
		logger.LogWarningWithIDf(log, s.nodeID, "Failed to unwatch frozen partial payment address for order %s: %v", order.ID, err)
	}
	if releaseTx == nil {
		return "", totalPaid.Uint64(), nil
	}
	return releaseTx.ID.String(), totalPaid.Uint64(), nil
}

func (s *SettlementService) frozenPartialPaymentTransactions(
	wallet iwallet.Wallet,
	order *models.Order,
	target *models.PaymentAttemptFundingTarget,
	chain iwallet.ChainType,
	scriptPubKey []byte,
) ([]iwallet.Transaction, error) {
	if s == nil || s.db == nil || wallet == nil || order == nil || target == nil {
		return nil, fmt.Errorf("frozen partial payment transaction recovery is not configured")
	}
	var observations []models.PaymentObservation
	if err := s.db.View(func(tx database.Tx) error {
		if !tx.Read().Migrator().HasTable(&models.PaymentObservation{}) {
			return nil
		}
		return tx.Read().Where(
			"tenant_id = ? AND order_id = ? AND status IN ?",
			strings.TrimSpace(order.TenantID), order.ID.String(),
			[]string{models.PaymentObservationStatusPending, models.PaymentObservationStatusConfirmed},
		).Order("created_at ASC, id ASC").Find(&observations).Error
	}); err != nil {
		return nil, fmt.Errorf("load frozen partial payment observations: %w", err)
	}
	seen := make(map[string]struct{}, len(observations))
	txs := make([]iwallet.Transaction, 0, len(observations))
	for _, observation := range observations {
		txHash := strings.TrimSpace(observation.TxHash)
		if txHash == "" || models.NormalizePaymentTxHashSource(observation.TxHashSource) != models.PaymentTxHashSourceChainTx ||
			!payment.SameUTXOAddress(observation.ToAddress, target.Address) {
			continue
		}
		if _, exists := seen[txHash]; exists {
			continue
		}
		seen[txHash] = struct{}{}
		tx, err := wallet.GetTransaction(iwallet.TransactionID(txHash), iwallet.CoinType(target.AssetID))
		if err != nil {
			return nil, fmt.Errorf("resolve frozen partial payment transaction %s: %w", txHash, err)
		}
		if tx != nil {
			txs = append(txs, *tx)
		}
	}
	if len(txs) > 0 {
		return txs, nil
	}
	if s.monitorService == nil {
		return nil, fmt.Errorf("no frozen partial payment transactions found")
	}
	txs, err := s.monitorService.GetAddressTransactions(chain, target.Address, scriptPubKey)
	if err != nil {
		return nil, fmt.Errorf("query frozen partial payment transactions: %w", err)
	}
	return txs, nil
}

// getRefundAddressFromTransactions extracts the buyer's refund address from transaction inputs.
func getRefundAddressFromTransactions(txs []iwallet.Transaction, coinType iwallet.CoinType) (iwallet.Address, error) {
	for _, tx := range txs {
		for _, from := range tx.From {
			if from.Address.String() != "" {
				return from.Address, nil
			}
		}
	}
	return iwallet.NewAddress("", coinType), fmt.Errorf("no refund address found in transaction inputs for %s", coinType)
}

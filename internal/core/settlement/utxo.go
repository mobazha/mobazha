//go:build !private_distribution

package settlement

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

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

	txs, err := order.GetTransactions()
	if err != nil {
		return nil, nil, err
	}

	var (
		txn      iwallet.Transaction
		totalOut = iwallet.NewAmount(0)
	)
	spent := make(map[string]bool)
	for _, tx := range txs {
		for _, from := range tx.From {
			spent[hex.EncodeToString(from.ID)] = true
		}
	}
	for _, tx := range txs {
		for _, to := range tx.To {
			if !spent[hex.EncodeToString(to.ID)] && to.Address.String() == params.PaymentAddress {
				txn.From = append(txn.From, to)
				totalOut = totalOut.Add(to.Amount)
			}
		}
	}

	if len(txn.From) == 0 {
		return nil, nil, errors.New("payment address is empty")
	}

	if err := s.verifyUTXOsOnChain(params.CoinCode, params.PaymentAddress, txn.From); err != nil {
		return nil, nil, fmt.Errorf("UTXO chain verification failed: %w", err)
	}

	escrowFee, err := escrowWallet.EstimateEscrowFee(len(txn.From), 1, 1, iwallet.FlNormal)
	if err != nil {
		return nil, nil, err
	}

	if escrowFee.Cmp(totalOut) >= 0 {
		return nil, nil, fmt.Errorf("insufficient funds: total input %s is less than or equal to fee %s", totalOut.String(), escrowFee.String())
	}

	txn.To = append(txn.To, iwallet.SpendInfo{
		Address: params.ToAddress,
		Amount:  totalOut.Sub(escrowFee),
	})

	script, err := hex.DecodeString(params.ScriptHex)
	if err != nil {
		return nil, nil, err
	}

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

	sigs, err := escrowWallet.SignMultisigTransaction(txn, *key, script)
	if err != nil {
		return nil, nil, err
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
		txid, params.ToAddress, totalOut.Sub(escrowFee).String())

	return dbTx, &txn, nil
}

// verifyUTXOsOnChain queries the chain to verify that expected UTXOs are still unspent.
// Best-effort: if the monitor is unavailable, the address is not watched, or the chain
// query fails, verification is skipped and the caller proceeds with local data.
func (s *SettlementService) verifyUTXOsOnChain(coinCode string, paymentAddress string, expectedUTXOs []iwallet.SpendInfo) error {
	if s.monitorService == nil {
		return nil
	}

	coinType := iwallet.CoinType(coinCode)
	coinInfo, err := coinType.CoinInfo()
	if err != nil || !coinInfo.Chain.IsUTXOChain() {
		return nil
	}

	wa := s.monitorService.GetWatchedAddress(paymentAddress)
	if wa == nil || len(wa.ScriptPubKey) == 0 {
		logger.LogWarningWithIDf(log, s.nodeID, "Cannot verify UTXOs: address %s not watched or missing scriptPubKey, skipping chain verification", paymentAddress)
		return nil
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
			if !chainSpent[id] && to.Address.String() == paymentAddress {
				chainUnspent[id] = true
			}
		}
	}

	for _, utxo := range expectedUTXOs {
		id := hex.EncodeToString(utxo.ID)
		if !chainUnspent[id] {
			return fmt.Errorf("%w: outpoint %s not found in unspent set for address %s", contracts.ErrUTXOAlreadySpent, id, paymentAddress)
		}
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Chain UTXO verification passed: %d UTXOs confirmed unspent at %s", len(expectedUTXOs), paymentAddress)
	return nil
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
		return "", 0, fmt.Errorf("no pending payment to cancel")
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

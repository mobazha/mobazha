//go:build !private_distribution

package payment

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/assetid"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	settlementpayment "github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

const (
	// AddressMonitorDuration is how long the backend monitors a UTXO payment
	// address for incoming transactions. This is a backend safety net,
	// independent of the user-facing payment window (UTXOPaymentWindowDuration
	// in escrow_handler.go). Kept at 24h to catch delayed broadcasts, network
	// congestion, or node-offline scenarios.
	AddressMonitorDuration = 24 * time.Hour
)

// ── UTXO Payment Monitor (business logic) ───────────────────────────────
//
// Infrastructure lifecycle (start/stop/configure) stays in MobazhaNode.
// Business logic (payment detection, aggregation, excess refund) lives here.

// SetMonitorService injects the UTXO monitor after MobazhaNode creates it
// (shared or standalone). Called from MobazhaNode.startUTXOPaymentMonitor.
func (s *PaymentAppService) SetMonitorService(ms utxo.UTXOMonitorService) {
	s.monitorService = ms
}

// CheckPendingPaymentsOnStartup checks for pending payments on node restart.
// Does NOT subscribe to addresses — only does a one-time check using GetTransactions.
// Subscribe only happens when buyer opens payment UI (via WatchPaymentAddress).
func (s *PaymentAppService) CheckPendingPaymentsOnStartup() {
	var orders []models.Order
	activeStates := []models.OrderState{
		models.OrderState_AWAITING_PAYMENT,
		models.OrderState_AWAITING_PAYMENT_VERIFICATION,
		models.OrderState_PENDING,
	}
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("state IN ?", activeStates).Find(&orders).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to load orders for pending payment check: %v", err)
		return
	}

	checkedCount := 0
	for _, order := range orders {
		if s.checkOrderPendingPayment(&order) {
			checkedCount++
		}
	}
	logger.LogInfoWithIDf(log, s.nodeID, "Checked %d orders for pending UTXO payments", checkedCount)
}

func (s *PaymentAppService) checkOrderPendingPayment(order *models.Order) bool {
	paymentSent, err := order.PaymentSentMessage()

	if spec := paymentSent.GetSettlementSpec(); err == nil && spec != nil && spec.GetMethod() == pb.PaymentSent_CANCELABLE && order.Role() == models.RoleVendor {
		coinType := iwallet.CoinType(paymentSent.Coin)
		coinInfo, coinErr := settlementpayment.SettlementCoinInfoForCoin(coinType)
		if coinErr != nil {
			return false
		}

		if coinInfo.Chain.IsUTXOChain() || coinInfo.IsEthTypeChain() {
			logger.LogInfoWithIDf(log, s.nodeID, "Emitting CancelablePaymentReady for pending order %s (chain=%s)", order.ID, coinInfo.Chain)
			s.eventBus.Emit(&events.CancelablePaymentReady{
				TenantID: order.TenantID,
				OrderID:  order.ID.String(),
				Coin:     string(coinType),
			})
			return true
		}

		return false
	}

	pendingInfo, _ := order.GetPendingPaymentInfo()
	if order.State == models.OrderState_AWAITING_PAYMENT &&
		order.PaymentAddress != "" &&
		pendingInfo != nil && pendingInfo.Coin != "" {

		coinType := iwallet.CoinType(pendingInfo.Coin)
		coinInfo, coinErr := settlementpayment.SettlementCoinInfoForCoin(coinType)
		if coinErr != nil || !coinInfo.Chain.IsUTXOChain() {
			return false
		}

		go s.recoverPendingUTXOPayment(order, pendingInfo, coinInfo.Chain)
		return true
	}

	return false
}

func (s *PaymentAppService) recoverPendingUTXOPayment(order *models.Order, pendingInfo *models.PendingUTXOPaymentInfo, chainType iwallet.ChainType) {
	logger.LogInfoWithIDf(log, s.nodeID, "Recovering pending UTXO payment watch for order %s at address %s",
		order.ID, order.PaymentAddress)

	if len(pendingInfo.ScriptPubKey) == 0 {
		logger.LogWarningWithIDf(log, s.nodeID, "No scriptPubKey stored for order %s, skipping recovery check", order.ID)
		return
	}

	if s.monitorService == nil {
		logger.LogWarningWithIDf(log, s.nodeID, "No UTXO monitor available for order %s", order.ID)
		return
	}

	wa := &utxo.WatchedAddress{
		Address:      order.PaymentAddress,
		ScriptPubKey: pendingInfo.ScriptPubKey,
		ChainType:    chainType,
		OrderID:      order.ID.String(),
		NodeID:       s.nodeID,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(AddressMonitorDuration),
	}
	if err := s.monitorService.WatchAddress(wa); err != nil {
		logger.LogWarningWithIDf(log, s.nodeID, "Failed to restore UTXO watch for order %s: %v", order.ID, err)
	}

	txs, err := s.monitorService.GetAddressTransactions(chainType, order.PaymentAddress, pendingInfo.ScriptPubKey)
	if err != nil {
		logger.LogWarningWithIDf(log, s.nodeID, "Failed to get transactions for order %s: %v", order.ID, err)
		return
	}

	if len(txs) == 0 {
		return
	}

	for _, tx := range txs {
		logger.LogInfoWithIDf(log, s.nodeID, "Found missed payment for order %s: tx=%s", order.ID, tx.ID)
		s.HandleUTXOPayment(tx, wa)
	}
}

// WatchPaymentAddress starts watching a specific payment address for incoming transactions.
// Called when buyer gets UTXO payment info.
func (s *PaymentAppService) WatchPaymentAddress(orderID string, address string, chainType iwallet.ChainType, scriptPubKey []byte) error {
	if address == "" || chainType == "" {
		return fmt.Errorf("address and chain type required")
	}

	if s.monitorService == nil {
		return fmt.Errorf("UTXO monitor not initialized")
	}

	wa := &utxo.WatchedAddress{
		Address:      address,
		ScriptPubKey: scriptPubKey,
		ChainType:    chainType,
		OrderID:      orderID,
		NodeID:       s.nodeID,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(AddressMonitorDuration),
	}
	if err := s.monitorService.WatchAddress(wa); err != nil {
		return fmt.Errorf("failed to watch address: %w", err)
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Started watching address %s for order %s (scriptPubKey len=%d)", address, orderID, len(scriptPubKey))
	return nil
}

// StopWatchingPayment stops watching a payment address for an order.
// If no payment has been made, clears PendingPaymentCoin/Amount so next open gets fresh rate.
func (s *PaymentAppService) StopWatchingPayment(orderID string) error {
	var order models.Order
	err := s.db.View(func(dbtx database.Tx) error {
		return dbtx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	if order.PaymentAddress != "" && s.monitorService != nil {
		if err := s.monitorService.UnwatchAddress(order.PaymentAddress); err != nil {
			logger.LogWarningWithIDf(log, s.nodeID, "Failed to unwatch payment address for order %s: %v", orderID, err)
		} else {
			logger.LogInfoWithIDf(log, s.nodeID, "Stopped watching payment address for order %s", orderID)
		}
	}

	pendingInfo, _ := order.GetPendingPaymentInfo()
	if pendingInfo != nil && pendingInfo.Coin != "" && order.PaymentAddress != "" {
		totalPaid, err := s.GetTotalPaidToAddress(&order)
		if err != nil {
			logger.LogWarningWithIDf(log, s.nodeID, "Failed to get total paid for order %s, skipping clear: %v", orderID, err)
		} else if totalPaid == 0 {
			order.PaymentAddress = ""
			order.ClearPendingPaymentInfo()

			if err := s.db.Update(func(dbtx database.Tx) error {
				return dbtx.Save(&order)
			}); err != nil {
				logger.LogWarningWithIDf(log, s.nodeID, "Failed to clear pending payment info for order %s: %v", orderID, err)
			} else {
				logger.LogInfoWithIDf(log, s.nodeID, "Cleared pending payment info for order %s (no payment made)", orderID)
			}
		}
	}

	return nil
}

// HandleUTXOPayment processes a detected UTXO payment transaction.
// Registered as monitor callback via MobazhaNode.startUTXOPaymentMonitor.
func (s *PaymentAppService) HandleUTXOPayment(tx iwallet.Transaction, wa *utxo.WatchedAddress) {
	if wa == nil {
		logger.LogWarningWithIDf(log, s.nodeID, "HandleUTXOPayment called with nil WatchedAddress for tx %s", tx.ID)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Detected UTXO payment: txid=%s, address=%s, amount=%s", tx.ID, wa.Address, tx.Value.String())

	var order models.Order
	err := s.db.View(func(dbtx database.Tx) error {
		return dbtx.Read().Where("id = ?", wa.OrderID).First(&order).Error
	})
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.LogErrorWithIDf(log, s.nodeID, "Error finding order %s for UTXO payment: %v", wa.OrderID, err)
		} else {
			logger.LogWarningWithIDf(log, s.nodeID, "Order %s not found for UTXO payment", wa.OrderID)
		}
		return
	}

	coinType, err := iwallet.RequireCanonicalNativeCoinType(wa.ChainType)
	if err != nil {
		if wa.ChainType == iwallet.ChainMock {
			coinType = iwallet.CtMock
		} else {
			logger.LogErrorWithIDf(log, s.nodeID, "No canonical native coin for watched UTXO chain %s (order %s, tx %s): %v", wa.ChainType, wa.OrderID, tx.ID, err)
			return
		}
	}

	// Persist the chain fact and let AggregatingVerifier advance
	// PaymentSession / PaymentSent. UTXO funding does not have a parallel
	// buyer-side verification path.
	s.dispatchUTXOObservation(&order, &tx, wa.Address, coinType)

	if order.Role() != models.RoleBuyer {
		return
	}

	var freshOrder models.Order
	if err := s.db.View(func(dbtx database.Tx) error {
		return dbtx.Read().Where("id = ?", order.ID).First(&freshOrder).Error
	}); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Error reloading order %s after UTXO observation: %v", order.ID, err)
		return
	}
	s.handleBuyerUTXOExcessPayment(&freshOrder, &tx)
}

func (s *PaymentAppService) handleBuyerUTXOExcessPayment(order *models.Order, tx *iwallet.Transaction) {
	if order == nil || tx == nil {
		return
	}
	existingPaymentSent, _ := order.PaymentSentMessage()
	if existingPaymentSent == nil {
		return
	}
	if existingPaymentSent.TransactionID == tx.ID.String() {
		return
	}
	observed, err := s.buyerUTXOPaymentContributedToPaymentSent(order, tx.ID.String(), existingPaymentSent)
	if err != nil {
		logger.LogWarningWithIDf(log, s.nodeID, "Unable to determine whether UTXO payment tx %s belongs to order %s; skipping excess cancel: %v", tx.ID, order.ID, err)
		return
	}
	if observed {
		logger.LogDebugWithIDf(log, s.nodeID, "Ignoring re-detection of aggregated UTXO payment for order %s: txid=%s", order.ID, tx.ID)
		return
	}
	logger.LogWarningWithIDf(log, s.nodeID, "Order %s already has PaymentSent, canceling excess payment txid=%s", order.ID, tx.ID)
	go s.cancelExcessPayment(order, tx)
}

func (s *PaymentAppService) buyerUTXOPaymentContributedToPaymentSent(order *models.Order, txID string, paymentSent *pb.PaymentSent) (bool, error) {
	if s == nil || s.observationDispatcher == nil || order == nil || strings.TrimSpace(txID) == "" {
		return false, nil
	}
	cutoff := time.Time{}
	if order.PaidAt != nil {
		cutoff = order.PaidAt.UTC()
	} else if paymentSent != nil && paymentSent.GetTimestamp() != nil {
		cutoff = paymentSent.GetTimestamp().AsTime()
	}
	if cutoff.IsZero() {
		return false, nil
	}
	tenantID := strings.TrimSpace(order.TenantID)
	if tenantID == "" {
		tenantID = database.StandaloneTenantID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := s.observationDispatcher.repo.ListByOrder(ctx, tenantID, order.ID.String())
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if !row.HasChainTxHash() || row.TxHash != txID || row.Status == models.PaymentObservationStatusReverted {
			continue
		}
		if row.CreatedAt.IsZero() || !row.CreatedAt.After(cutoff) {
			return true, nil
		}
	}
	return false, nil
}

func (s *PaymentAppService) cancelExcessPayment(order *models.Order, tx *iwallet.Transaction) {
	logger.LogInfoWithIDf(log, s.nodeID, "Canceling excess payment for order %s (txid=%s)", order.ID, tx.ID)

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get PaymentSent for excess payment cancel: %v", err)
		return
	}

	spec := paymentSent.GetSettlementSpec()
	if spec == nil {
		logger.LogWarningWithIDf(log, s.nodeID, "Cannot auto-cancel excess payment for order %s without settlement spec", order.ID)
		return
	}
	if spec.GetMethod() != pb.PaymentSent_CANCELABLE {
		logger.LogWarningWithIDf(log, s.nodeID, "Cannot auto-cancel excess payment for non-CANCELABLE order %s", order.ID)
		return
	}

	wallet, err := s.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get wallet for excess payment cancel: %v", err)
		return
	}

	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		logger.LogErrorWithIDf(log, s.nodeID, "Wallet does not support escrow for order %s", order.ID)
		return
	}

	var excessUTXO *iwallet.SpendInfo
	var excessAmount = iwallet.NewAmount(0)
	for _, to := range tx.To {
		if to.Address.String() == paymentSent.ToAddress {
			excessUTXO = &to
			excessAmount = to.Amount
			break
		}
	}

	if excessUTXO == nil {
		logger.LogWarningWithIDf(log, s.nodeID, "No UTXO found in excess transaction for payment address %s", paymentSent.ToAddress)
		return
	}

	refundAddress, err := s.GetRefundAddressFromTransactions([]iwallet.Transaction{*tx}, iwallet.CoinType(paymentSent.Coin))
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get refund address: %v", err)
		return
	}

	script, err := hex.DecodeString(paymentSent.Script)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to decode script: %v", err)
		return
	}

	chainCode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to decode chaincode: %v", err)
		return
	}

	var refundTxn iwallet.Transaction
	refundTxn.From = append(refundTxn.From, *excessUTXO)

	escrowFee, err := escrowWallet.EstimateEscrowFee(1, 1, 1, iwallet.FlNormal)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to estimate fee: %v", err)
		return
	}

	refundTxn.To = append(refundTxn.To, iwallet.SpendInfo{
		Address: refundAddress,
		Amount:  excessAmount.Sub(escrowFee),
	})

	escrowMasterKey, err := s.keys.EscrowMasterKey()
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get escrow master key: %v", err)
		return
	}
	key, err := utils.GenerateEscrowPrivateKey(escrowMasterKey, chainCode)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to generate signing key: %v", err)
		return
	}

	sigs, err := escrowWallet.SignMultisigTransaction(refundTxn, *key, script)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to sign excess refund transaction: %v", err)
		return
	}

	dbTx, err := wallet.Begin()
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to begin wallet transaction: %v", err)
		return
	}

	txid, err := escrowWallet.BuildAndSend(dbTx, refundTxn, [][]iwallet.EscrowSignature{sigs}, script, iwallet.ORDER_FINISH_CANCEL)
	if err != nil {
		dbTx.Rollback()
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to build and send excess refund: %v", err)
		return
	}

	if err := dbTx.Commit(); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to commit excess refund transaction: %v", err)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Successfully refunded excess payment for order %s (refund txid=%s, amount=%s)", order.ID, txid, excessAmount.String())

	s.eventBus.Emit(&events.ExcessPaymentRefunded{
		OrderID:        order.ID.String(),
		RefundTxID:     txid.String(),
		RefundedAmount: excessAmount.Uint64(),
		Coin:           paymentSent.Coin,
	})
}

// utxoObservationChainRef resolves chain namespace and reference for UTXO
// payment_observations audit rows. Canonical crypto:* IDs are parsed directly;
// legacy native tickers (btc, ltc, …) are normalized via TryNormalizePaymentCoin.
func utxoObservationChainRef(coinType iwallet.CoinType) (namespace string, chainRef string, ok bool) {
	if canon, normalized := iwallet.TryNormalizePaymentCoin(string(coinType)); normalized {
		coinType = canon
	}

	parsed, err := assetid.Parse(string(coinType))
	if err != nil {
		return "", "", false
	}
	switch parsed.Namespace {
	case assetid.NamespaceBIP122:
		return string(assetid.NamespaceBIP122), parsed.ChainRef, true
	case assetid.NamespaceBitcoinCash:
		return string(assetid.NamespaceBitcoinCash), parsed.ChainRef, true
	case assetid.NamespaceZCash:
		return string(assetid.NamespaceZCash), parsed.ChainRef, true
	default:
		return "", "", false
	}
}

// dispatchUTXOObservation writes the UTXO payment into payment_observations
// via ObservationDispatcher.
func (s *PaymentAppService) dispatchUTXOObservation(order *models.Order, tx *iwallet.Transaction, matchedAddress string, coinType iwallet.CoinType) {
	if s.observationDispatcher == nil {
		logger.LogErrorWithIDf(log, s.nodeID, "UTXO observation dispatcher is not configured for order %s tx %s", order.ID, tx.ID)
		return
	}

	chainNamespace, chainRef, ok := utxoObservationChainRef(coinType)
	if !ok {
		return
	}

	outputs := paymentOutputsForAddress(tx, matchedAddress)
	if len(outputs) == 0 {
		return
	}

	blockTime := tx.Timestamp
	if blockTime.IsZero() {
		blockTime = time.Now()
	}
	knownConfirmed := tx.Height > 0
	blockNumber := int64(tx.Height) // 0 = mempool / unconfirmed; never coerce to 1

	var fromAddr string
	for _, from := range tx.From {
		if from.Address.String() != "" {
			fromAddr = from.Address.String()
			break
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, output := range outputs {
		evt := FundingEvent{
			OrderID:        order.ID.String(),
			ChainNamespace: chainNamespace,
			ChainReference: chainRef,
			TxHash:         string(tx.ID),
			EventIndex:     output.eventIndex,
			EventType:      models.PaymentEventUTXOFunding,
			FromAddress:    fromAddr,
			ToAddress:      matchedAddress,
			Amount:         new(big.Int).SetUint64(output.spend.Amount.Uint64()),
			BlockNumber:    blockNumber,
			BlockTime:      blockTime,
		}

		if err := s.observationDispatcher.OnFundingEvent(ctx, evt); err != nil {
			logger.LogWarningWithIDf(log, s.nodeID,
				"UTXO observation dispatch failed (non-fatal) for order %s tx %s output %d: %v", order.ID, tx.ID, output.eventIndex, err)
		}
	}
	if knownConfirmed {
		if err := s.observationDispatcher.OnNewBlock(ctx, chainNamespace, chainRef, blockNumber, 0); err != nil {
			logger.LogWarningWithIDf(log, s.nodeID,
				"UTXO observation confirmation refresh failed (non-fatal) for order %s tx %s: %v", order.ID, tx.ID, err)
		}
	}
}

type utxoPaymentOutput struct {
	eventIndex int
	spend      iwallet.SpendInfo
}

func paymentOutputsForAddress(tx *iwallet.Transaction, address string) []utxoPaymentOutput {
	if tx == nil {
		return nil
	}
	var out []utxoPaymentOutput
	for i, spend := range tx.To {
		if !sameUTXOAddress(spend.Address.String(), address) {
			continue
		}
		eventIndex := i
		if idx, ok := outputIndexFromOutpointID(spend.ID); ok {
			eventIndex = int(idx)
		}
		out = append(out, utxoPaymentOutput{eventIndex: eventIndex, spend: spend})
	}
	return out
}

func outputIndexFromOutpointID(id []byte) (uint32, bool) {
	if len(id) < 36 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(id[len(id)-4:]), true
}

func amountPaidToAddress(tx *iwallet.Transaction, address string) uint64 {
	if tx == nil {
		return 0
	}
	var total uint64
	for _, out := range tx.To {
		if sameUTXOAddress(out.Address.String(), address) {
			total += out.Amount.Uint64()
		}
	}
	return total
}

func sameUTXOAddress(a, b string) bool {
	a = normalizeUTXOAddressForCompare(a)
	b = normalizeUTXOAddressForCompare(b)
	return a != "" && strings.EqualFold(a, b)
}

func normalizeUTXOAddressForCompare(address string) string {
	address = strings.TrimSpace(address)
	if i := strings.LastIndex(address, ":"); i >= 0 {
		address = address[i+1:]
	}
	return address
}

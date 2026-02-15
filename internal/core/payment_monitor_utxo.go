package core

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/multiwallet/base"
	internalutxo "github.com/mobazha/mobazha3.0/internal/multiwallet/utxo"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

// startUTXOPaymentMonitor starts the UTXO payment monitoring service
// This is called from Node.Start() and monitors payment addresses for incoming transactions
//
// Two modes are supported:
// 1. Shared mode (hosting): Uses HostService's shared UTXOMonitorService, reducing resource usage
// 2. Standalone mode: Creates its own monitor with dedicated Electrum/Mempool connections
func (n *MobazhaNode) startUTXOPaymentMonitor() {
	// Try shared mode first
	if n.hostService != nil {
		if sharedMonitor := n.hostService.GetUTXOMonitor(); sharedMonitor != nil {
			n.monitorService = sharedMonitor
			logger.LogInfoWithIDf(log, n.nodeID, "Using shared UTXO monitor from HostService")
		}
	}

	// Fall back to standalone mode
	if n.monitorService == nil {
		_ = internalutxo.DefaultMonitorConfig // Ensure internal package is imported
		monitor, err := internalutxo.CreateMonitor(context.Background(), n.UsingWalletTestnet())
		if err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Failed to create UTXO monitor: %v", err)
			return
		}
		n.monitorService = monitor
		n.monitorService.Start()
		logger.LogInfoWithIDf(log, n.nodeID, "Created standalone UTXO monitor")
	}

	// Register callback for transaction notifications
	if err := n.monitorService.RegisterNodeCallback(n.nodeID, n.handleUTXOPayment); err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to register node callback: %v", err)
		return
	}

	// Configure UTXO wallets for chain operations
	n.configureUTXOWallets(n.monitorService)

	// Check for pending payments on existing orders
	n.checkPendingPaymentsOnStartup()
}

// configureUTXOWallets sets up UTXO wallets to use UTXOChainClient
// This works for both standalone mode (*Monitor) and shared mode (UTXOMonitorService)
// since both implement the ChainOperations interface
func (n *MobazhaNode) configureUTXOWallets(ops utxo.ChainOperations) {
	if n.multiwallet == nil || ops == nil {
		return
	}

	// Iterate all supported chains; configure UTXO ones with UTXOChainClient.
	for _, chain := range n.multiwallet.SupportedChains() {
		if !chain.IsUTXOChain() {
			continue
		}

		wallet, ok := n.multiwallet.WalletForChain(chain)
		if !ok {
			continue
		}

		// Replace ChainClient with UTXOChainClient
		if setter, ok := wallet.(base.ChainClientSetter); ok {
			client := internalutxo.NewUTXOChainClient(ops, chain)
			setter.SetChainClient(client)
			logger.LogInfoWithIDf(log, n.nodeID, "Configured %s wallet with UTXOChainClient", chain)
		}
	}
}

// handleCancelablePaymentForUTXO handles CancelablePaymentReady event for UTXO chains
// Called by dispatchCancelablePayment in payment_dispatcher.go
func (n *MobazhaNode) handleCancelablePaymentForUTXO(event *events.CancelablePaymentReady) {
	logger.LogInfoWithIDf(log, n.nodeID, "Handling UTXO CANCELABLE payment ready event for order %s", event.OrderID)

	// Get the order using shared helper (doesn't mark as read)
	order, err := n.fetchOrderByID(event.OrderID)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to get order %s for UTXO CANCELABLE auto-confirm: %v", event.OrderID, err)
		return
	}

	// Trigger auto-confirm (releaseFromCancelableAddress will collect all UTXOs)
	n.autoConfirmCancelablePayment(order)
}

// checkPendingPaymentsOnStartup checks for pending payments on node restart
// This does NOT subscribe to addresses - it only does a one-time check using GetTransactions
// Subscribe only happens when buyer opens payment UI (via WatchPaymentAddress)
func (n *MobazhaNode) checkPendingPaymentsOnStartup() {
	var orders []models.Order
	// Query orders that may need recovery:
	// - AWAITING_PAYMENT: buyer may have paid but node crashed before processing
	// - PENDING: vendor may need to auto-confirm CANCELABLE payment
	activeStates := []models.OrderState{
		models.OrderState_AWAITING_PAYMENT,
		models.OrderState_PENDING,
	}
	err := n.db.View(func(tx database.Tx) error {
		return tx.Read().Where("state IN ?", activeStates).Find(&orders).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to load orders for pending payment check: %v", err)
		return
	}

	checkedCount := 0
	for _, order := range orders {
		if n.checkOrderPendingPayment(&order) {
			checkedCount++
		}
	}
	logger.LogInfoWithIDf(log, n.nodeID, "Checked %d orders for pending UTXO payments", checkedCount)
}

// checkOrderPendingPayment checks a single order for pending payment on node restart
// Returns true if a check was performed
func (n *MobazhaNode) checkOrderPendingPayment(order *models.Order) bool {
	paymentSent, err := order.PaymentSentMessage()

	// Case 1: Vendor with CANCELABLE payment that has PaymentSent but no OrderConfirmation
	// This handles the case where node crashed after receiving payment but before confirming
	// Will attempt to release funds and send OrderConfirmation
	if err == nil && paymentSent.Method == pb.PaymentSent_CANCELABLE && order.Role() == models.RoleVendor {
		coinType := iwallet.CoinType(paymentSent.Coin)
		coinInfo, coinErr := coinType.CoinInfo()
		if coinErr != nil {
			return false
		}

		// UTXO chains: release via multisig
		if coinInfo.Chain.IsUTXOChain() {
			logger.LogInfoWithIDf(log, n.nodeID, "Checking pending UTXO CANCELABLE payment auto-confirm for order %s", order.ID)
			go n.autoConfirmCancelablePayment(order)
			return true
		}

		// EVM chains: release via platform relay API
		if coinInfo.IsEthTypeChain() && n.relayAPIURL != "" {
			logger.LogInfoWithIDf(log, n.nodeID, "Checking pending EVM CANCELABLE payment auto-confirm for order %s (chain=%s)", order.ID, coinInfo.Chain)
			go n.autoConfirmEVMCancelablePayment(order, string(coinInfo.Chain))
			return true
		}

		// Solana: not yet supported for auto-retry
		// Note: Solana relay service is not yet implemented
		return false
	}

	// Case 2: Buyer with AWAITING_PAYMENT and has pending payment info - check for missed payments
	pendingInfo, _ := order.GetPendingPaymentInfo()
	if order.State == models.OrderState_AWAITING_PAYMENT &&
		order.Role() == models.RoleBuyer &&
		order.PaymentAddress != "" &&
		pendingInfo != nil && pendingInfo.Coin != "" {

		coinType := iwallet.CoinType(pendingInfo.Coin)
		coinInfo, coinErr := coinType.CoinInfo()
		if coinErr != nil || !coinInfo.Chain.IsUTXOChain() {
			return false
		}

		go n.checkBuyerMissedPayment(order, pendingInfo, coinInfo.Chain)
		return true
	}

	return false
}

// checkBuyerMissedPayment checks if buyer has made payments that weren't processed
// Uses one-time GetTransactions instead of subscribing
func (n *MobazhaNode) checkBuyerMissedPayment(order *models.Order, pendingInfo *models.PendingUTXOPaymentInfo, chainType iwallet.ChainType) {
	logger.LogInfoWithIDf(log, n.nodeID, "Checking missed payments for order %s at address %s",
		order.ID, order.PaymentAddress)

	// Get scriptPubKey from pending info (required for Electrum)
	if len(pendingInfo.ScriptPubKey) == 0 {
		logger.LogWarningWithIDf(log, n.nodeID, "No scriptPubKey stored for order %s, skipping recovery check", order.ID)
		return
	}

	if n.monitorService == nil {
		logger.LogWarningWithIDf(log, n.nodeID, "No UTXO monitor available for order %s", order.ID)
		return
	}

	txs, err := n.monitorService.GetAddressTransactions(chainType, order.PaymentAddress, pendingInfo.ScriptPubKey)
	if err != nil {
		logger.LogWarningWithIDf(log, n.nodeID, "Failed to get transactions for order %s: %v", order.ID, err)
		return
	}

	if len(txs) == 0 {
		return
	}

	// Convert coin string to coinType
	coinType := iwallet.CoinType(pendingInfo.Coin)

	ctx := context.Background()

	// Process each detected transaction directly with handleBuyerUTXOPayment
	// We already have the order info, no need to look up via watched addresses
	for _, tx := range txs {
		logger.LogInfoWithIDf(log, n.nodeID, "Found missed payment for order %s: tx=%s", order.ID, tx.ID)
		n.handleBuyerUTXOPayment(ctx, order, &tx, order.PaymentAddress, coinType)
	}
}

// WatchPaymentAddress starts watching a specific payment address
// This should be called when buyer gets UTXO payment info (GetUTXOPaymentInfo)
// scriptPubKey is the output script for the address, required for Electrum subscription
func (n *MobazhaNode) WatchPaymentAddress(orderID string, address string, chainType iwallet.ChainType, scriptPubKey []byte) error {
	if address == "" || chainType == "" {
		return fmt.Errorf("address and chain type required")
	}

	if n.monitorService == nil {
		return fmt.Errorf("UTXO monitor not initialized")
	}

	wa := &utxo.WatchedAddress{
		Address:      address,
		ScriptPubKey: scriptPubKey,
		ChainType:    chainType,
		OrderID:      orderID,
		NodeID:       n.nodeID,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	if err := n.monitorService.WatchAddress(wa); err != nil {
		return fmt.Errorf("failed to watch address: %w", err)
	}

	logger.LogInfoWithIDf(log, n.nodeID, "Started watching address %s for order %s (scriptPubKey len=%d)", address, orderID, len(scriptPubKey))
	return nil
}

// StopWatchingPayment stops watching a payment address for an order
// This should be called when:
// - Buyer closes the payment UI
// - PaymentSent has been successfully sent
// - Order is completed/cancelled
// If no payment has been made, clears PendingPaymentCoin/Amount so next open gets fresh rate
func (n *MobazhaNode) StopWatchingPayment(orderID string) error {
	// Get order to find the payment address
	var order models.Order
	err := n.db.View(func(dbtx database.Tx) error {
		return dbtx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	// Unwatch the payment address
	if order.PaymentAddress != "" && n.monitorService != nil {
		if err := n.monitorService.UnwatchAddress(order.PaymentAddress); err != nil {
			logger.LogWarningWithIDf(log, n.nodeID, "Failed to unwatch payment address for order %s: %v", orderID, err)
		} else {
			logger.LogInfoWithIDf(log, n.nodeID, "Stopped watching payment address for order %s", orderID)
		}
	}

	// If no payment has been made, clear pending payment info
	// This allows fresh exchange rate on next payment attempt
	pendingInfo, _ := order.GetPendingPaymentInfo()
	if pendingInfo != nil && pendingInfo.Coin != "" && order.PaymentAddress != "" {
		totalPaid, err := n.GetTotalPaidToAddress(&order)
		if err != nil {
			// Don't clear payment info if we can't verify payment status
			logger.LogWarningWithIDf(log, n.nodeID, "Failed to get total paid for order %s, skipping clear: %v", orderID, err)
		} else if totalPaid == 0 {
			// No payment made, clear all pending payment info
			order.PaymentAddress = ""
			order.ClearPendingPaymentInfo()

			if err := n.db.Update(func(dbtx database.Tx) error {
				return dbtx.Save(&order)
			}); err != nil {
				logger.LogWarningWithIDf(log, n.nodeID, "Failed to clear pending payment info for order %s: %v", orderID, err)
			} else {
				logger.LogInfoWithIDf(log, n.nodeID, "Cleared pending payment info for order %s (no payment made)", orderID)
			}
		}
	}

	return nil
}

// handleUTXOPayment processes a detected UTXO payment transaction
// This is called via RegisterNodeCallback when the monitor detects a transaction
//
// Parameters:
// - tx: the detected transaction
// - wa: the WatchedAddress (always provided by monitor callback)
//
// Buyer vs Seller behavior:
// - Buyer: Aggregates payments, sends PAYMENT_SENT when total >= expected, emits PartialPaymentReceived if insufficient
// - Seller: Records transaction and triggers CANCELABLE payment auto-confirm
func (n *MobazhaNode) handleUTXOPayment(tx iwallet.Transaction, wa *utxo.WatchedAddress) {
	if wa == nil {
		logger.LogWarningWithIDf(log, n.nodeID, "handleUTXOPayment called with nil WatchedAddress for tx %s", tx.ID)
		return
	}

	logger.LogInfoWithIDf(log, n.nodeID, "Detected UTXO payment: txid=%s, address=%s, amount=%s", tx.ID, wa.Address, tx.Value.String())

	// Get order directly by OrderID from WatchedAddress
	var order models.Order
	err := n.db.View(func(dbtx database.Tx) error {
		return dbtx.Read().Where("id = ?", wa.OrderID).First(&order).Error
	})
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.LogErrorWithIDf(log, n.nodeID, "Error finding order %s for UTXO payment: %v", wa.OrderID, err)
		} else {
			logger.LogWarningWithIDf(log, n.nodeID, "Order %s not found for UTXO payment", wa.OrderID)
		}
		return
	}

	// Only buyers handle UTXO payments via monitor
	// Vendors handle payments via CancelablePaymentReady event (triggered by PAYMENT_SENT message)
	if order.Role() != models.RoleBuyer {
		return
	}

	ctx := context.Background()
	n.handleBuyerUTXOPayment(ctx, &order, &tx, wa.Address, iwallet.CoinType(wa.ChainType))
}

// handleBuyerUTXOPayment handles UTXO payment detection for buyers
// Implements multi-payment aggregation: only sends PaymentSent when total >= expected
func (n *MobazhaNode) handleBuyerUTXOPayment(ctx context.Context, order *models.Order, tx *iwallet.Transaction, matchedAddress string, coinType iwallet.CoinType) {
	// First check if this transaction is already recorded (duplicate detection)
	existingTxs, err := order.GetTransactions()
	if err == nil {
		for _, existingTx := range existingTxs {
			if existingTx.ID == tx.ID {
				// This transaction was already processed, ignore duplicate notification
				logger.LogDebugWithIDf(log, n.nodeID, "Ignoring duplicate transaction notification for order %s: txid=%s", order.ID, tx.ID)
				return
			}
		}
	}

	// Check if PaymentSent already exists
	existingPaymentSent, _ := order.PaymentSentMessage()
	if existingPaymentSent != nil {
		// Check if this is the same transaction that was used for PaymentSent
		if existingPaymentSent.TransactionID == tx.ID.String() {
			logger.LogDebugWithIDf(log, n.nodeID, "Ignoring re-detection of PaymentSent transaction for order %s: txid=%s", order.ID, tx.ID)
			return
		}
		// This is truly a new excess payment - cancel and refund
		logger.LogWarningWithIDf(log, n.nodeID, "Order %s already has PaymentSent, canceling excess payment txid=%s", order.ID, tx.ID)
		go n.cancelExcessPayment(order, tx)
		return
	}

	// Record the transaction (only for payments before PaymentSent)
	if err := order.PutTransaction(*tx); err != nil {
		if !models.IsDuplicateTransactionError(err) {
			logger.LogErrorWithIDf(log, n.nodeID, "Error recording transaction for order %s: %v", order.ID, err)
		}
		// If duplicate, continue to check totals (might need to send PaymentSent)
	}

	// Save order with new transaction
	if err := n.db.Update(func(dbtx database.Tx) error {
		return dbtx.Save(order)
	}); err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error saving order %s after recording transaction: %v", order.ID, err)
	}

	// Calculate total paid to the payment address
	totalPaid, err := n.calculateTotalPaidToAddress(order, matchedAddress)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error calculating total paid for order %s: %v", order.ID, err)
		return
	}

	// Get pending payment info
	pendingInfo, err := order.GetPendingPaymentInfo()
	if err != nil || pendingInfo == nil || pendingInfo.Amount == 0 {
		logger.LogWarningWithIDf(log, n.nodeID, "Order %s has no pending payment info", order.ID)
		return
	}

	// Get expected amount from locked amount in pendingInfo
	expectedAmount := iwallet.NewAmount(pendingInfo.Amount)

	logger.LogInfoWithIDf(log, n.nodeID, "Order %s payment status: paid=%s, expected=%s", order.ID, totalPaid.String(), expectedAmount.String())

	if totalPaid.Cmp(expectedAmount) >= 0 {
		// Total is sufficient, send PaymentSent
		// Use the info stored from GetUTXOPaymentInfo to ensure consistency
		// This handles both CANCELABLE (1-of-2) and MODERATED (2-of-3) payments correctly
		if pendingInfo.Script == "" {
			logger.LogErrorWithIDf(log, n.nodeID, "No pending payment script stored for order %s", order.ID)
			return
		}

		// Get payer address from transaction inputs (populated by Electrum/Mempool source)
		var payerAddress string
		for _, from := range tx.From {
			if from.Address.String() != "" {
				payerAddress = from.Address.String()
				break
			}
		}
		if payerAddress == "" {
			logger.LogWarningWithIDf(log, n.nodeID, "Could not extract payer address for order %s", order.ID)
		}

		// Determine payment method based on stored moderator info
		method := pb.PaymentSent_CANCELABLE
		if pendingInfo.Moderator != "" {
			method = pb.PaymentSent_MODERATED
		}

		paymentData := &models.PaymentData{
			OrderID:          order.ID.String(),
			TransactionID:    tx.ID.String(),
			Coin:             coinType,
			Method:           method,
			Amount:           totalPaid.Uint64(),
			ToAddress:        matchedAddress,
			Timestamp:        tx.Timestamp,
			Script:           pendingInfo.Script,
			PayerAddress:     payerAddress,
			RefundAddress:    payerAddress,
			Moderator:        pendingInfo.Moderator,
			ModeratorAddress: pendingInfo.ModeratorPubkey,
			UnlockHours:      pendingInfo.UnlockHours,
		}

		if err := n.ProcessOrderPayment(ctx, paymentData); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Error processing UTXO payment for order %s: %v", order.ID, err)
			return
		}
		logger.LogInfoWithIDf(log, n.nodeID, "Buyer sent PAYMENT_SENT for order %s (total paid: %s, method: %s)", order.ID, totalPaid.String(), method.String())

		// Stop watching after PaymentSent is sent - no need to monitor anymore
		if n.monitorService != nil && matchedAddress != "" {
			if err := n.monitorService.UnwatchAddress(matchedAddress); err != nil {
				logger.LogWarningWithIDf(log, n.nodeID, "Failed to unwatch address after PaymentSent for order %s: %v", order.ID, err)
			} else {
				logger.LogInfoWithIDf(log, n.nodeID, "Stopped watching address after PaymentSent for order %s", order.ID)
			}
		}

		// Clear temporary pending payment info after PaymentSent is sent
		if err := n.db.Update(func(dbtx database.Tx) error {
			return updateFreshOrder(dbtx, order.ID, func(o *models.Order) error {
				o.ClearPendingPaymentInfo()
				return nil
			})
		}); err != nil {
			logger.LogWarningWithIDf(log, n.nodeID, "Failed to clear pending payment info for order %s: %v", order.ID, err)
		}
	} else {
		// Total is insufficient, emit PartialPaymentReceived event
		remaining := expectedAmount.Sub(totalPaid)

		n.eventBus.Emit(&events.PartialPaymentReceived{
			OrderID:         order.ID.String(),
			PaidAmount:      totalPaid.Uint64(),
			ExpectedAmount:  expectedAmount.Uint64(),
			RemainingAmount: remaining.Uint64(),
			Coin:            string(coinType),
			PaymentAddress:  matchedAddress,
		})
		logger.LogInfoWithIDf(log, n.nodeID, "Order %s partial payment received: paid=%s, remaining=%s", order.ID, totalPaid.String(), remaining.String())
	}
}

// cancelExcessPayment cancels an excess payment (received after PaymentSent was already sent)
// This releases the funds back to the buyer's original address
func (n *MobazhaNode) cancelExcessPayment(order *models.Order, tx *iwallet.Transaction) {
	logger.LogInfoWithIDf(log, n.nodeID, "Canceling excess payment for order %s (txid=%s)", order.ID, tx.ID)

	// Get PaymentSent to retrieve script and chaincode
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to get PaymentSent for excess payment cancel: %v", err)
		return
	}

	// Only handle CANCELABLE payments (buyer can sign to refund)
	if paymentSent.Method != pb.PaymentSent_CANCELABLE {
		logger.LogWarningWithIDf(log, n.nodeID, "Cannot auto-cancel excess payment for non-CANCELABLE order %s", order.ID)
		return
	}

	// Get wallet
	wallet, err := n.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to get wallet for excess payment cancel: %v", err)
		return
	}

	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		logger.LogErrorWithIDf(log, n.nodeID, "Wallet does not support escrow for order %s", order.ID)
		return
	}

	// Find the UTXO from this excess transaction that goes to the payment address
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
		logger.LogWarningWithIDf(log, n.nodeID, "No UTXO found in excess transaction for payment address %s", paymentSent.ToAddress)
		return
	}

	// Get refund address from the transaction's input (buyer's address)
	refundAddress, err := n.getRefundAddressFromTransactions([]iwallet.Transaction{*tx}, iwallet.CoinType(paymentSent.Coin))
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to get refund address: %v", err)
		return
	}

	// Decode script and chaincode
	script, err := hex.DecodeString(paymentSent.Script)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to decode script: %v", err)
		return
	}

	chainCode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to decode chaincode: %v", err)
		return
	}

	// Build transaction with only the excess UTXO
	var refundTxn iwallet.Transaction
	refundTxn.From = append(refundTxn.From, *excessUTXO)

	// Estimate fee
	escrowFee, err := escrowWallet.EstimateEscrowFee(1, 1, iwallet.FlNormal)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to estimate fee: %v", err)
		return
	}

	// Output to refund address
	refundTxn.To = append(refundTxn.To, iwallet.SpendInfo{
		Address: refundAddress,
		Amount:  excessAmount.Sub(escrowFee),
	})

	// Sign with buyer's key
	key, err := utils.GenerateEscrowPrivateKey(n.escrowMasterKey, chainCode)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to generate signing key: %v", err)
		return
	}

	sigs, err := escrowWallet.SignMultisigTransaction(refundTxn, *key, script)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to sign excess refund transaction: %v", err)
		return
	}

	// Build and send
	dbTx, err := wallet.Begin()
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to begin wallet transaction: %v", err)
		return
	}

	txid, err := escrowWallet.BuildAndSend(dbTx, refundTxn, [][]iwallet.EscrowSignature{sigs}, script, iwallet.ORDER_FINISH_CANCEL)
	if err != nil {
		dbTx.Rollback()
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to build and send excess refund: %v", err)
		return
	}

	if err := dbTx.Commit(); err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to commit excess refund transaction: %v", err)
		return
	}

	logger.LogInfoWithIDf(log, n.nodeID, "Successfully refunded excess payment for order %s (refund txid=%s, amount=%s)", order.ID, txid, excessAmount.String())

	// Emit event to notify frontend
	n.eventBus.Emit(&events.ExcessPaymentRefunded{
		OrderID:        order.ID.String(),
		RefundTxID:     txid.String(),
		RefundedAmount: excessAmount.Uint64(),
		Coin:           paymentSent.Coin,
	})
}

// autoConfirmCancelablePayment automatically confirms a CANCELABLE payment
// This is a thin wrapper that delegates to ConfirmOrder which handles:
// - Auto-fetching payout address (if not provided)
// - Releasing funds from CANCELABLE address (for UTXO chains)
// - Saving the release transaction
// - Sending OrderConfirmation to buyer
func (n *MobazhaNode) autoConfirmCancelablePayment(order *models.Order) {
	// Use shared lock to prevent concurrent processing
	unlock := n.tryLockAutoConfirm(order.ID.String())
	if unlock == nil {
		return // Already being processed
	}
	defer unlock()

	logger.LogInfoWithIDf(log, n.nodeID, "Auto-confirming UTXO CANCELABLE payment for order %s", order.ID)

	// Delegate to ConfirmOrder which handles everything:
	// - Auto-fetch payout address
	// - CanConfirm() check
	// - UTXO release (if txid is empty)
	// - Transaction saving
	// - Message sending
	// - Wallet transaction commit
	if err := n.ConfirmOrder(order.ID, "", "", nil); err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to auto-confirm order %s: %v", order.ID, err)
		return
	}

	logger.LogInfoWithIDf(log, n.nodeID, "Successfully auto-confirmed CANCELABLE payment for order %s", order.ID)
}

// StopUTXOPaymentMonitor stops the UTXO payment monitor
// Unregisters node callback, removes watched addresses, and stops monitor
// Note: For shared monitors (isShared=true), Stop() is a no-op
func (n *MobazhaNode) StopUTXOPaymentMonitor() {
	if n.monitorService != nil {
		n.monitorService.UnregisterNode(n.nodeID)
		n.monitorService.Stop() // No-op if shared (handled inside Monitor)
		n.monitorService = nil
		logger.LogInfoWithIDf(log, n.nodeID, "Stopped UTXO monitor")
	}
}

// SetUTXOMonitor sets a custom UTXO monitor (primarily for testing)
// This allows injecting a mock monitor with mock payment sources
func (n *MobazhaNode) SetUTXOMonitor(monitor *utxo.Monitor) {
	n.monitorService = monitor
}

// GetMonitorService returns the monitor service (primarily for testing)
func (n *MobazhaNode) GetMonitorService() utxo.UTXOMonitorService {
	return n.monitorService
}

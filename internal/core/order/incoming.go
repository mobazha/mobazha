//go:build !private_distribution

package order

import (
	"context"
	"errors"
	"fmt"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// PreProcessContext carries the results of pre-processing I/O performed
// before the deterministic ProcessMessage call.
type PreProcessContext struct {
	// PAYMENT_SENT: PVS verification result (Phase 2)
	VerifiedPayment *contracts.VerifiedPayment

	// CONFIRMATION/CANCEL/REFUND: outgoing chain tx fetched in preProcess
	OutgoingTx *iwallet.Transaction

	// DECLINE (buyer, UTXO CANCELABLE): escrow release committed
	CancelableReleaseCommitted bool

	// DECLINE (buyer, Fiat): refund already processed
	FiatRefundProcessed bool

	// REFUND (buyer, MODERATED UTXO): escrow release committed
	EscrowRefundCommitted bool
}

// HandleIncomingOrderMessage is the main entry point for processing incoming
// P2P order messages. It orchestrates:
//  1. Per-order lock acquisition (serializes processing for the same order)
//  2. preProcess — external I/O before deterministic processing
//  3. OrderProcessor.ProcessMessage — deterministic state machine
//  4. postProcessInTx — post-processing within the same DB transaction
//
// The returned order reflects the pre-ProcessMessage state (loaded before
// the handler runs). The caller (handleOrderMessage in network.go) uses
// it for post-commit side effects such as NetDB rating storage.
func (s *OrderAppService) HandleIncomingOrderMessage(ctx context.Context, orderMsg *npb.OrderMessage) (event interface{}, order models.Order, err error) {
	orderID := models.OrderID(orderMsg.OrderID)

	if err := s.acquireOrderLock(orderID); err != nil {
		return nil, models.Order{}, fmt.Errorf("failed to acquire order lock for %s: %w", orderID, err)
	}
	defer s.releaseOrderLock(orderID)

	if orderMsg.MessageType == npb.OrderMessage_AFTER_SALE_DISPUTE_OPEN {
		evt, handleErr := s.handleAfterSaleDisputeOpen(orderMsg)
		return evt, models.Order{}, handleErr
	}

	ppCtx, err := s.preProcess(ctx, orderMsg)
	if err != nil {
		return nil, models.Order{}, err
	}

	err = s.db.Update(func(tx database.Tx) error {
		tx.Read().Where("id = ?", orderMsg.OrderID).First(&order)

		var processErr error
		event, processErr = s.orderProcessor.ProcessMessage(tx, orderMsg)
		if processErr != nil {
			return processErr
		}

		return s.postProcessInTx(tx, orderMsg, ppCtx, &order)
	})

	return event, order, err
}

// preProcess performs external I/O before the deterministic ProcessMessage.
// Dispatches to message-type-specific handlers.
func (s *OrderAppService) preProcess(ctx context.Context, orderMsg *npb.OrderMessage) (*PreProcessContext, error) {
	switch orderMsg.MessageType {
	case npb.OrderMessage_PAYMENT_SENT:
		return s.preProcessPaymentSent(ctx, orderMsg)
	case npb.OrderMessage_ORDER_CONFIRMATION:
		return s.preProcessOrderConfirmation(ctx, orderMsg)
	case npb.OrderMessage_ORDER_CANCEL:
		return s.preProcessOrderCancel(ctx, orderMsg)
	case npb.OrderMessage_ORDER_DECLINE:
		return s.preProcessOrderDecline(ctx, orderMsg)
	case npb.OrderMessage_REFUND:
		return s.preProcessRefund(ctx, orderMsg)
	default:
		return nil, nil
	}
}

// preProcessPaymentSent handles the pre-processing I/O for PAYMENT_SENT messages:
//  1. Load order from DB (read-only) — if not found, skip (message will be parked)
//  2. Parse PaymentSent and OrderOpen
//  3. Validate via PVS.ValidateMessage (pure computation)
//  4. Check if transaction is already known — if so, skip
//  5. Call PVS.FetchAndVerify to check on-chain status
//
// Returns nil ppCtx (not an error) when the tx is not yet on-chain — the async
// verification loop will retry later.
func (s *OrderAppService) preProcessPaymentSent(ctx context.Context, orderMsg *npb.OrderMessage) (*PreProcessContext, error) {
	if s.paymentVerifier == nil {
		return nil, nil
	}

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderMsg.OrderID).First(&order).Error
	})
	if err != nil {
		return nil, nil
	}

	paymentSent := new(pb.PaymentSent)
	if err := orderMsg.Message.UnmarshalTo(paymentSent); err != nil {
		return nil, nil
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return nil, nil
	}

	coinType, err := canonicalPaymentCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, fmt.Errorf("payment validation failed for order %s: %w", orderMsg.OrderID, err)
	}

	if err := s.paymentVerifier.ValidateMessage(coinType, orderOpen, paymentSent, paymentSent.EscrowTimeoutHours); err != nil {
		return nil, fmt.Errorf("payment validation failed for order %s: %w", orderMsg.OrderID, err)
	}

	txs, err := order.GetTransactions()
	if err == nil {
		for _, tx := range txs {
			if tx.ID.String() == paymentSent.TransactionID {
				if order.IsPaymentVerified() {
					return nil, nil
				}
				break
			}
		}
	}

	vp, err := s.paymentVerifier.FetchAndVerify(ctx, orderOpen, paymentSent, paymentSent.ToAddress)
	if err != nil {
		if errors.Is(err, contracts.ErrPaymentAddressMismatch) {
			return nil, fmt.Errorf("deposit verification failed for order %s: %w", orderMsg.OrderID, err)
		}
		logger.LogInfoWithIDf(log, s.nodeID,
			"Payment tx %s not yet verified for order %s, will retry: %v",
			paymentSent.TransactionID, orderMsg.OrderID, err)
		return nil, nil
	}

	return &PreProcessContext{VerifiedPayment: vp}, nil
}

// preProcessOrderConfirmation fetches the chain transaction and optionally verifies
// the EVM receipt for ORDER_CONFIRMATION messages. The fetched transaction is stored
// in ppCtx.OutgoingTx for postProcessOutgoingTxInTx to record.
func (s *OrderAppService) preProcessOrderConfirmation(_ context.Context, orderMsg *npb.OrderMessage) (*PreProcessContext, error) {
	orderConf := new(pb.OrderConfirmation)
	if err := orderMsg.Message.UnmarshalTo(orderConf); err != nil {
		return nil, nil
	}
	if orderConf.TransactionID == "" {
		return nil, nil
	}

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderMsg.OrderID).First(&order).Error
	})
	if err != nil {
		return nil, nil
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, nil
	}

	if paymentSent.Method != pb.PaymentSent_CANCELABLE {
		return nil, nil
	}

	coinType, err := canonicalPaymentCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, fmt.Errorf("invalid payment coin for order %s: %w", orderMsg.OrderID, err)
	}

	if s.receiptVerifier != nil && s.shouldVerifyReceipt(coinType) {
		if verifyErr := s.receiptVerifier.VerifyTransactionReceipt(context.Background(), paymentSent.Coin, orderConf.TransactionID); verifyErr != nil {
			return nil, fmt.Errorf("receipt verification failed for order %s tx %s: %w",
				orderMsg.OrderID, orderConf.TransactionID, verifyErr)
		}
	}

	coinInfo, _ := coinType.CoinInfo()
	tx, err := s.fetchOutgoingTx(paymentSent.Coin, orderConf.TransactionID, order.PaymentAddress, &coinInfo)
	if err != nil || tx == nil {
		return nil, nil
	}
	return &PreProcessContext{OutgoingTx: tx}, nil
}

// preProcessOrderCancel fetches the chain transaction for ORDER_CANCEL messages
// (CANCELABLE method only). The fetched transaction is stored in ppCtx.OutgoingTx.
func (s *OrderAppService) preProcessOrderCancel(_ context.Context, orderMsg *npb.OrderMessage) (*PreProcessContext, error) {
	orderCancel := new(pb.OrderCancel)
	if err := orderMsg.Message.UnmarshalTo(orderCancel); err != nil {
		return nil, nil
	}
	if orderCancel.TransactionID == "" {
		return nil, nil
	}

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderMsg.OrderID).First(&order).Error
	})
	if err != nil {
		return nil, nil
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, nil
	}

	if paymentSent.Method != pb.PaymentSent_CANCELABLE {
		return nil, nil
	}

	coinType, err := canonicalPaymentCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, fmt.Errorf("invalid payment coin for order %s: %w", orderMsg.OrderID, err)
	}

	coinInfo, _ := coinType.CoinInfo()
	tx, err := s.fetchOutgoingTx(paymentSent.Coin, orderCancel.TransactionID, order.PaymentAddress, &coinInfo)
	if err != nil || tx == nil {
		return nil, nil
	}
	return &PreProcessContext{OutgoingTx: tx}, nil
}

// preProcessOrderDecline handles pre-processing for ORDER_DECLINE messages:
//   - Fiat: trigger fiat refund via provider
//   - UTXO CANCELABLE: release funds from cancelable address
func (s *OrderAppService) preProcessOrderDecline(ctx context.Context, orderMsg *npb.OrderMessage) (*PreProcessContext, error) {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderMsg.OrderID).First(&order).Error
	})
	if err != nil {
		return nil, nil
	}

	if order.Role() != models.RoleBuyer {
		return nil, nil
	}
	if order.State == models.OrderState_AWAITING_PAYMENT {
		return nil, nil
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, nil
	}

	coinType, err := canonicalPaymentCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, fmt.Errorf("invalid payment coin for order %s: %w", order.ID, err)
	}

	if paymentSent.Method == pb.PaymentSent_FIAT || coinType.IsFiatPayment() {
		if s.fiatOps != nil {
			if _, err := s.refundFiatPayment(ctx, &order, paymentSent, "requested_by_customer"); err != nil && !errors.Is(err, contracts.ErrAlreadyRefunded) {
				return nil, fmt.Errorf("fiat refund on decline failed for order %s: %w", order.ID, err)
			}
			return &PreProcessContext{FiatRefundProcessed: true}, nil
		}
		return nil, nil
	}

	if !s.hasClientSignedEscrow(coinType) {
		if order.CanCancel() && paymentSent.Method == pb.PaymentSent_CANCELABLE {
			result, err := s.ReleaseFromCancelableAddress(&order)
			if err != nil {
				return nil, fmt.Errorf("UTXO cancelable release on decline failed for order %s: %w", order.ID, err)
			}
			result.WalletTx.Commit()
			return &PreProcessContext{CancelableReleaseCommitted: true}, nil
		}
	}

	return nil, nil
}

// preProcessRefund handles pre-processing for REFUND messages:
//   - DIRECT: fetch outgoing chain transaction
//   - MODERATED UTXO: release escrow funds with buyer co-signature
func (s *OrderAppService) preProcessRefund(_ context.Context, orderMsg *npb.OrderMessage) (*PreProcessContext, error) {
	refund := new(pb.Refund)
	if err := orderMsg.Message.UnmarshalTo(refund); err != nil {
		return nil, nil
	}

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderMsg.OrderID).First(&order).Error
	})
	if err != nil {
		return nil, nil
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, nil
	}

	coinType, err := canonicalPaymentCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, fmt.Errorf("invalid payment coin for order %s: %w", orderMsg.OrderID, err)
	}

	if refund.GetTransactionID() != "" && paymentSent.Method == pb.PaymentSent_DIRECT {
		coinInfo, _ := coinType.CoinInfo()
		tx, err := s.fetchOutgoingTx(paymentSent.Coin, refund.GetTransactionID(), order.PaymentAddress, &coinInfo)
		if err != nil || tx == nil {
			return nil, nil
		}
		return &PreProcessContext{OutgoingTx: tx}, nil
	}

	if order.Role() == models.RoleBuyer && refund.GetReleaseInfo() != nil && paymentSent.Method == pb.PaymentSent_MODERATED {
		wallet, err := s.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
		if err != nil {
			return nil, nil
		}
		if wallet.CoinCategory() == iwallet.CoinCategoryBitcoin {
			if err := s.releaseRefundEscrowFunds(wallet, paymentSent, refund.GetReleaseInfo()); err != nil {
				logger.LogInfoWithIDf(log, s.nodeID,
					"Error releasing funds from escrow during refund processing for order %s: %v",
					orderMsg.OrderID, err)
				return nil, fmt.Errorf("refund escrow release failed for order %s: %w", orderMsg.OrderID, err)
			}
			return &PreProcessContext{EscrowRefundCommitted: true}, nil
		}
	}

	return nil, nil
}

// fetchOutgoingTx fetches a chain transaction and filters it by the order's
// payment address. For client-signed chains (EVM/Solana/TRON), all transactions
// from the payment address match; for UTXO chains, only transactions with
// matching From inputs.
func (s *OrderAppService) fetchOutgoingTx(coinCode string, txID string, paymentAddress string, coinInfo *iwallet.CoinInfo) (*iwallet.Transaction, error) {
	wallet, err := s.multiwallet.WalletForCurrencyCode(coinCode)
	if err != nil {
		return nil, err
	}

	tx, err := wallet.GetTransaction(iwallet.TransactionID(txID), iwallet.CoinType(coinCode))
	if err != nil || tx == nil {
		return nil, err
	}

	if s.hasClientSignedEscrow(iwallet.CoinType(coinCode)) {
		return tx, nil
	}

	for _, from := range tx.From {
		if from.Address.String() == paymentAddress {
			return tx, nil
		}
	}
	return nil, nil
}

// postProcessOutgoingTxInTx records the outgoing transaction on the order after
// ProcessMessage has saved the order state. Shared by CONFIRMATION, CANCEL, REFUND.
func (s *OrderAppService) postProcessOutgoingTxInTx(tx database.Tx, orderMsg *npb.OrderMessage, ppCtx *PreProcessContext, order *models.Order) error {
	if ppCtx.OutgoingTx == nil {
		return nil
	}

	if err := tx.Read().Where("id = ?", orderMsg.OrderID).First(order).Error; err != nil {
		return err
	}

	return s.orderProcessor.RecordOutgoingTransaction(tx, order, *ppCtx.OutgoingTx)
}

// postProcessInTx performs post-processing within the DB transaction after
// ProcessMessage succeeds. Dispatches to message-type-specific handlers.
func (s *OrderAppService) postProcessInTx(tx database.Tx, orderMsg *npb.OrderMessage, ppCtx *PreProcessContext, order *models.Order) error {
	if ppCtx == nil {
		return nil
	}

	switch orderMsg.MessageType {
	case npb.OrderMessage_PAYMENT_SENT:
		return s.postProcessPaymentSentInTx(tx, orderMsg, ppCtx, order)
	case npb.OrderMessage_ORDER_CONFIRMATION, npb.OrderMessage_ORDER_CANCEL, npb.OrderMessage_REFUND:
		return s.postProcessOutgoingTxInTx(tx, orderMsg, ppCtx, order)
	default:
		return nil
	}
}

// postProcessPaymentSentInTx records the verified payment after ProcessMessage
// has saved the order. Re-loads the order to get the post-ProcessMessage state.
func (s *OrderAppService) postProcessPaymentSentInTx(tx database.Tx, orderMsg *npb.OrderMessage, ppCtx *PreProcessContext, order *models.Order) error {
	if ppCtx.VerifiedPayment == nil {
		return nil
	}

	if err := tx.Read().Where("id = ?", orderMsg.OrderID).First(order).Error; err != nil {
		return err
	}

	return s.orderProcessor.RecordVerifiedPayment(tx, order, ppCtx.VerifiedPayment.Transaction)
}

// shouldVerifyReceipt checks whether the chain supports receipt verification
// by querying the ChainEscrow's Capabilities. Falls back to false if the
// registry or strategy is unavailable.
func (s *OrderAppService) shouldVerifyReceipt(coinType iwallet.CoinType) bool {
	if s.paymentRegistry == nil {
		return false
	}
	strategy, err := s.paymentRegistry.ForCoin(coinType)
	if err != nil {
		return false
	}
	return strategy.Capabilities().HasReceiptVerification
}

// hasClientSignedEscrow checks whether the chain uses client-signed escrow
// (EVM/Solana/TRON smart contracts vs UTXO multisig).
func (s *OrderAppService) hasClientSignedEscrow(coinType iwallet.CoinType) bool {
	if s.paymentRegistry == nil {
		return false
	}
	strategy, err := s.paymentRegistry.ForCoin(coinType)
	if err != nil {
		return false
	}
	return strategy.Capabilities().HasClientSignedEscrow
}

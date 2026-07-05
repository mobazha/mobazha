package order

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mobazha/mobazha/internal/core/digital"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/orders/utils"
	pkgconfig "github.com/mobazha/mobazha/pkg/config"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
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
		s.recordIncomingOrderMessageError(orderMsg, err)
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
	if err != nil {
		s.recordIncomingOrderMessageError(orderMsg, err)
	}

	return event, order, err
}

// recordIncomingOrderMessageError attaches a failed incoming message to the
// order's ErroredMessages list outside the failed transaction. Best-effort by
// design: it never masks the original error returned to the caller.
func (s *OrderAppService) recordIncomingOrderMessageError(orderMsg *npb.OrderMessage, msgErr error) {
	if orderMsg == nil {
		return
	}
	dbErr := s.db.Update(func(tx database.Tx) error {
		var order models.Order
		if loadErr := tx.Read().Where("id = ?", orderMsg.OrderID).First(&order).Error; loadErr != nil {
			return loadErr
		}
		if putErr := order.PutErrorMessage(orderMsg); putErr != nil {
			return putErr
		}
		return tx.Save(&order)
	})
	if dbErr != nil {
		logger.LogInfoWithIDf(log, s.nodeID,
			"[TD-025] failed to record incoming order message error for order %s: %v (original error: %v)",
			orderMsg.OrderID, dbErr, msgErr)
	}
}

// recordPreProcessError is kept for focused tests and older call sites.
func (s *OrderAppService) recordPreProcessError(orderMsg *npb.OrderMessage, ppErr error) {
	s.recordIncomingOrderMessageError(orderMsg, ppErr)
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

	if err := s.ensureIncomingManagedEscrowIntent(&order, paymentSent); err != nil {
		return nil, fmt.Errorf("payment validation failed for order %s: %w", orderMsg.OrderID, err)
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, fmt.Errorf("payment validation failed for order %s: %w", orderMsg.OrderID, err)
	}

	expectedPaymentAmount := ""
	expectedPaymentCoin := ""
	if lockedCoin, ok := payment.PendingPaymentCoinFromOrder(&order); ok {
		expectedPaymentCoin = string(lockedCoin)
		expectedPaymentAmount = order.ExpectedPaymentAmountString()
	}
	if err := s.paymentVerifier.ValidateMessage(coinType, payment.PaymentMessageParams{
		OrderOpen:             orderOpen,
		PaymentSent:           paymentSent,
		ExpectedPaymentAmount: expectedPaymentAmount,
		ExpectedPaymentCoin:   expectedPaymentCoin,
		EscrowTimeoutHours:    paymentSent.EscrowTimeoutHours,
	}); err != nil {
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

func (s *OrderAppService) ensureIncomingManagedEscrowIntent(order *models.Order, paymentSent *pb.PaymentSent) error {
	if order == nil || paymentSent == nil {
		return nil
	}
	spec, ok := payment.ResolveSettlementSpec(nil, paymentSent)
	if !ok || !spec.UsesManagedEscrow() {
		return nil
	}
	if existing, err := order.GetPendingManagedEscrowInfo(); err != nil {
		return err
	} else if existing != nil {
		return nil
	}

	info, refundAddress, err := pendingManagedEscrowInfoFromPaymentSent(paymentSent, spec)
	if err != nil {
		return err
	}

	return s.db.Update(func(tx database.Tx) error {
		var current models.Order
		if err := tx.Read().Where("id = ?", order.ID.String()).First(&current).Error; err != nil {
			return err
		}
		if existing, err := current.GetPendingManagedEscrowInfo(); err != nil {
			return err
		} else if existing != nil {
			*order = current
			return nil
		}

		current.PaymentAddress = info.Address
		current.CancelFeeAmount = info.CancelFeeAmount
		if refundAddress != "" {
			current.RefundAddress = refundAddress
		}
		if err := current.SetPendingManagedEscrowInfo(info); err != nil {
			return err
		}
		if err := paymentintent.UpsertSharedPaymentIntent(tx.Read(), current.ID.String(), info.Address, refundAddress, info); err != nil {
			return err
		}
		if err := tx.Save(&current); err != nil {
			return err
		}
		*order = current
		return nil
	})
}

func pendingManagedEscrowInfoFromPaymentSent(paymentSent *pb.PaymentSent, spec payment.SettlementSpec) (*models.PendingManagedEscrowInfo, string, error) {
	coin := strings.TrimSpace(paymentSent.Coin)
	if coin == "" {
		return nil, "", errors.New("managed escrow payment coin is required")
	}
	normalizedCoin, ok := payment.NormalizeSettlementPaymentCoin(coin)
	if !ok {
		return nil, "", fmt.Errorf("invalid managed escrow payment coin %q", coin)
	}
	amount, err := strconv.ParseUint(strings.TrimSpace(paymentSent.Amount), 10, 64)
	if err != nil || amount == 0 {
		return nil, "", fmt.Errorf("managed escrow payment amount %q is invalid", paymentSent.Amount)
	}
	escrowAddress := strings.TrimSpace(paymentSent.ContractAddress)
	if escrowAddress == "" {
		escrowAddress = strings.TrimSpace(paymentSent.ToAddress)
	}
	if escrowAddress == "" {
		return nil, "", errors.New("managed escrow payment address is required")
	}

	info := &models.PendingManagedEscrowInfo{
		Coin:             string(normalizedCoin),
		Amount:           amount,
		Address:          escrowAddress,
		Moderated:        payment.MethodIsModerated(spec.Method),
		Moderator:        strings.TrimSpace(paymentSent.Moderator),
		ModeratorAddress: strings.TrimSpace(paymentSent.ModeratorAddress),
		PlatformAmount:   strings.TrimSpace(paymentSent.PlatformAmount),
		PlatformAddr:     strings.TrimSpace(paymentSent.PlatformAddr),
		CancelFeeAmount:  strings.TrimSpace(paymentSent.CancelFeeAmount),
		SettlementSpec:   spec.ToPending(),
	}
	return info, strings.TrimSpace(paymentSent.RefundAddress), nil
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
	method, ok := payment.ResolvedPaymentMethod(&order, paymentSent)
	if !ok {
		return nil, fmt.Errorf("order %s payment settlement spec is missing", orderMsg.OrderID)
	}

	if !payment.MethodIsCancelable(method) {
		return nil, nil
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, fmt.Errorf("invalid payment coin for order %s: %w", orderMsg.OrderID, err)
	}

	if s.receiptVerifier != nil && s.shouldVerifyReceipt(coinType) {
		if verifyErr := s.receiptVerifier.VerifyTransactionReceipt(context.Background(), string(coinType), orderConf.TransactionID); verifyErr != nil {
			return nil, fmt.Errorf("receipt verification failed for order %s tx %s: %w",
				orderMsg.OrderID, orderConf.TransactionID, verifyErr)
		}
	}

	coinInfo, _ := coinType.CoinInfo()
	tx, err := s.fetchOutgoingTx(string(coinType), orderConf.TransactionID, order.PaymentAddress, &coinInfo)
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
	method, ok := payment.ResolvedPaymentMethod(&order, paymentSent)
	if !ok {
		return nil, fmt.Errorf("order %s payment settlement spec is missing", orderMsg.OrderID)
	}

	if !payment.MethodIsCancelable(method) {
		return nil, nil
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, fmt.Errorf("invalid payment coin for order %s: %w", orderMsg.OrderID, err)
	}

	coinInfo, _ := coinType.CoinInfo()
	tx, err := s.fetchOutgoingTx(string(coinType), orderCancel.TransactionID, order.PaymentAddress, &coinInfo)
	if err != nil || tx == nil {
		return nil, nil
	}
	return &PreProcessContext{OutgoingTx: tx}, nil
}

// preProcessOrderDecline handles pre-processing for ORDER_DECLINE messages:
//   - Fiat: trigger fiat refund via provider
//   - UTXO CANCELABLE: release funds from cancelable address
func (s *OrderAppService) preProcessOrderDecline(ctx context.Context, orderMsg *npb.OrderMessage) (*PreProcessContext, error) {
	orderDecline := new(pb.OrderDecline)
	if err := orderMsg.Message.UnmarshalTo(orderDecline); err != nil {
		return nil, err
	}

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
	method, ok := payment.ResolvedPaymentMethod(&order, paymentSent)
	if !ok {
		return nil, fmt.Errorf("order %s payment settlement spec is missing", order.ID)
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, fmt.Errorf("invalid payment coin for order %s: %w", order.ID, err)
	}

	if payment.IsFiatPaymentRoute(method, coinType) {
		if s.fiatOps != nil {
			if _, err := s.refundFiatPayment(ctx, &order, paymentSent, "requested_by_customer"); err != nil && !errors.Is(err, contracts.ErrAlreadyRefunded) {
				return nil, fmt.Errorf("fiat refund on decline failed for order %s: %w", order.ID, err)
			}
			return &PreProcessContext{FiatRefundProcessed: true}, nil
		}
		return nil, nil
	}

	if order.CanCancel() && payment.MethodIsCancelable(method) {
		if payment.UsesUTXOScriptEscrow(&order, paymentSent) {
			result, err := s.ReleaseFromCancelableAddress(&order)
			if err != nil {
				return nil, fmt.Errorf("UTXO cancelable release on decline failed for order %s: %w", order.ID, err)
			}
			result.WalletTx.Commit()
			return &PreProcessContext{CancelableReleaseCommitted: true}, nil
		}
		action, actionOK := s.settlementActionForIntent(&order, paymentSent, method, coinType, settlementIntentBuyerCancel)
		if strings.TrimSpace(orderDecline.TransactionID) != "" {
			logger.LogInfoWithIDf(log, s.nodeID,
				"Skipping settlement %s on decline for order %s; decline already carries transaction %s",
				action, order.ID, orderDecline.TransactionID)
			return &PreProcessContext{CancelableReleaseCommitted: true}, nil
		}
		if !actionOK {
			return nil, nil
		}
		var handled bool
		switch action {
		case payment.SettlementActionCancel:
			_, _, handled, err = s.submitSettlementCancelAction(ctx, &order, coinType, paymentSent, "")
		default:
			err = fmt.Errorf("%w: unsupported buyer decline fallback settlement action %s", payment.ErrUnsupportedAction, action)
		}
		if err != nil {
			return nil, fmt.Errorf("settlement %s release on decline failed for order %s: %w", action, order.ID, err)
		}
		if handled {
			return &PreProcessContext{CancelableReleaseCommitted: true}, nil
		}
	}

	return nil, nil
}

// preProcessRefund handles pre-processing for REFUND messages:
//   - DIRECT: fetch outgoing chain transaction
//   - MODERATED UTXO: release escrow funds with buyer co-signature
func (s *OrderAppService) preProcessRefund(ctx context.Context, orderMsg *npb.OrderMessage) (*PreProcessContext, error) {
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

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, fmt.Errorf("invalid payment coin for order %s: %w", orderMsg.OrderID, err)
	}

	if refund.GetTransactionID() != "" && payment.IsNonEscrowDirectPayment(&order, paymentSent) {
		coinInfo, _ := coinType.CoinInfo()
		tx, err := s.fetchOutgoingTx(string(coinType), refund.GetTransactionID(), order.PaymentAddress, &coinInfo)
		if err != nil || tx == nil {
			return nil, nil
		}
		return &PreProcessContext{OutgoingTx: tx}, nil
	}

	if method, ok := payment.ResolvedPaymentMethod(&order, paymentSent); order.Role() == models.RoleBuyer && refund.GetReleaseInfo() != nil && ok && payment.MethodIsModerated(method) {
		wallet, err := s.multiwallet.WalletForCurrencyCode(string(coinType))
		if err != nil {
			return nil, nil
		}
		if wallet.CoinCategory() == iwallet.CoinCategoryBitcoin {
			if err := s.releaseRefundEscrowFunds(wallet, &order, paymentSent, refund.GetReleaseInfo()); err != nil {
				logger.LogInfoWithIDf(log, s.nodeID,
					"Error releasing funds from escrow during refund processing for order %s: %v",
					orderMsg.OrderID, err)
				return nil, fmt.Errorf("refund escrow release failed for order %s: %w", orderMsg.OrderID, err)
			}
			return &PreProcessContext{EscrowRefundCommitted: true}, nil
		}
		if spec, ok := payment.ResolveSettlementSpec(&order, paymentSent); ok && spec.UsesManagedEscrow() {
			release := refund.GetReleaseInfo()
			_, settlementTx, handled, err := s.submitSettlementCancelAction(ctx, &order, coinType, paymentSent, release.GetToAddress(), release)
			if err != nil {
				logger.LogInfoWithIDf(log, s.nodeID,
					"Error releasing managed EVM escrow during refund processing for order %s: %v",
					orderMsg.OrderID, err)
				return nil, fmt.Errorf("refund managed EVM escrow release failed for order %s: %w", orderMsg.OrderID, err)
			}
			if handled {
				return &PreProcessContext{OutgoingTx: settlementTx}, nil
			}
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
		if payment.SameUTXOAddress(from.Address.String(), paymentAddress) {
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
	switch orderMsg.MessageType {
	case npb.OrderMessage_ORDER_OPEN:
		return s.postProcessOrderOpenInTx(tx, orderMsg)
	case npb.OrderMessage_ORDER_CONFIRMATION:
		if err := s.commitStandardOrderSupplyInTx(tx, orderMsg.OrderID); err != nil {
			return err
		}
		if ppCtx != nil {
			return s.postProcessOutgoingTxInTx(tx, orderMsg, ppCtx, order)
		}
		return nil
	case npb.OrderMessage_ORDER_CANCEL:
		if err := s.releaseStandardOrderSupplyInTx(tx, orderMsg.OrderID, "cancelled"); err != nil {
			return err
		}
		if ppCtx != nil {
			return s.postProcessOutgoingTxInTx(tx, orderMsg, ppCtx, order)
		}
		return nil
	case npb.OrderMessage_ORDER_DECLINE:
		return s.releaseStandardOrderSupplyInTx(tx, orderMsg.OrderID, "declined")
	}

	if ppCtx == nil {
		return nil
	}

	switch orderMsg.MessageType {
	case npb.OrderMessage_PAYMENT_SENT:
		return s.postProcessPaymentSentInTx(tx, orderMsg, ppCtx, order)
	case npb.OrderMessage_REFUND:
		return s.postProcessOutgoingTxInTx(tx, orderMsg, ppCtx, order)
	default:
		return nil
	}
}

type orderTransactionalSupplyAvailabilityService interface {
	ReserveOrderTx(context.Context, database.Tx, contracts.ReserveOrderSupplyRequest) (*contracts.ReserveOrderSupplyResult, error)
	CommitOrderTx(context.Context, database.Tx, string, string) error
	ReleaseOrderTx(context.Context, database.Tx, string, string, string) error
}

type transactionalDigitalSupplyLineResolver interface {
	SupplyAvailabilityLinesForOrderItemsTx(database.Tx, []digital.OrderLineItem) ([]contracts.SupplyLine, error)
}

func (s *OrderAppService) postProcessOrderOpenInTx(tx database.Tx, orderMsg *npb.OrderMessage) error {
	var stored models.Order
	if err := tx.Read().Where("id = ?", orderMsg.OrderID).First(&stored).Error; err != nil {
		return err
	}
	if stored.Role() != models.RoleVendor {
		return nil
	}
	if !stored.Open || len(stored.SerializedOrderDecline) > 0 || len(stored.SerializedOrderCancel) > 0 {
		return nil
	}

	orderOpen := new(pb.OrderOpen)
	if err := orderMsg.Message.UnmarshalTo(orderOpen); err != nil {
		return err
	}
	if err := s.persistOrderExtensions(context.Background(), tx, orderMsg.OrderID, orderOpen); err != nil {
		return err
	}

	txService, ok := s.authoritativeSupplyAvailabilityTxService(context.Background())
	if !ok {
		return nil
	}
	lines, err := s.standardOrderSupplyLinesFromOrderOpen(tx, orderMsg.OrderID, orderOpen, true)
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return nil
	}
	reservableLines, manualActionLines := contracts.PartitionReservableSupplyLines(lines)
	for _, line := range manualActionLines {
		logger.LogInfoWithIDf(log, s.nodeID,
			"standard order %s external supply line %s for listing %s requires manual action; no external hold created",
			orderMsg.OrderID, line.LineID, line.ListingSlug,
		)
	}
	if len(reservableLines) == 0 {
		return nil
	}

	expiresAt := time.Now().Add(time.Hour)
	if stored.ExpiresAt != nil && !stored.ExpiresAt.IsZero() {
		expiresAt = *stored.ExpiresAt
	}
	if _, err := txService.ReserveOrderTx(context.Background(), tx, contracts.ReserveOrderSupplyRequest{
		OrderRef:  orderMsg.OrderID,
		OrderType: models.OrderTypeStandard,
		Lines:     reservableLines,
		ExpiresAt: expiresAt,
	}); err != nil {
		return fmt.Errorf("reserve standard order supply: %w", err)
	}
	return nil
}

func (s *OrderAppService) commitStandardOrderSupplyInTx(tx database.Tx, orderID string) error {
	txService, ok := s.authoritativeSupplyAvailabilityTxService(context.Background())
	if !ok {
		return nil
	}
	if ok, err := standardOrderIsVendorInTx(tx, orderID); err != nil || !ok {
		return err
	}
	if err := txService.CommitOrderTx(context.Background(), tx, orderID, models.OrderTypeStandard); err != nil {
		return fmt.Errorf("commit standard order supply: %w", err)
	}
	return nil
}

func (s *OrderAppService) releaseStandardOrderSupplyInTx(tx database.Tx, orderID string, reason string) error {
	txService, ok := s.authoritativeSupplyAvailabilityTxService(context.Background())
	if !ok {
		return nil
	}
	if ok, err := standardOrderIsVendorInTx(tx, orderID); err != nil || !ok {
		return err
	}
	if err := txService.ReleaseOrderTx(context.Background(), tx, orderID, models.OrderTypeStandard, reason); err != nil {
		return fmt.Errorf("release standard order supply: %w", err)
	}
	return nil
}

func (s *OrderAppService) authoritativeSupplyAvailabilityTxService(ctx context.Context) (orderTransactionalSupplyAvailabilityService, bool) {
	if s == nil || s.supplyAvailability == nil || s.resolver == nil {
		return nil, false
	}
	if !s.resolver.IsEnabled(ctx, pkgconfig.FeatureSupplyAvailabilityEnabled.Key) {
		return nil, false
	}
	txService, ok := s.supplyAvailability.(orderTransactionalSupplyAvailabilityService)
	if !ok {
		logger.LogInfoWithIDf(log, s.nodeID, "supplyAvailabilityEnabled is true but order service does not support transactional order supply operations; skipping standard order supply integration")
		return nil, false
	}
	return txService, true
}

func standardOrderIsVendorInTx(tx database.Tx, orderID string) (bool, error) {
	var stored models.Order
	if err := tx.Read().
		Where("id = ? AND my_role = ?", orderID, string(models.RoleVendor)).
		First(&stored).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *OrderAppService) standardOrderSupplyLinesFromOrderOpen(tx database.Tx, orderID string, orderOpen *pb.OrderOpen, requireDigitalResolver bool) ([]contracts.SupplyLine, error) {
	var digitalResolver DigitalSupplyLineResolver
	if s != nil {
		digitalResolver = s.digitalSupplyLines
	}
	return standardOrderSupplyLinesFromOrderOpen(tx, orderID, orderOpen, digitalResolver, requireDigitalResolver)
}

func standardOrderSupplyLinesFromOrderOpen(tx database.Tx, orderID string, orderOpen *pb.OrderOpen, digitalResolver DigitalSupplyLineResolver, requireDigitalResolver bool) ([]contracts.SupplyLine, error) {
	if orderOpen == nil || len(orderOpen.Items) == 0 {
		return nil, nil
	}
	lines := make([]contracts.SupplyLine, 0, len(orderOpen.Items))
	for i, item := range orderOpen.Items {
		listing, err := extractOrderOpenListing(item.ListingHash, orderOpen.Listings)
		if err != nil {
			return nil, err
		}
		qty, err := strconv.Atoi(strings.TrimSpace(item.Quantity))
		if err != nil || qty <= 0 {
			return nil, fmt.Errorf("item %d quantity must be a positive integer", i)
		}
		if listing.Metadata == nil {
			continue
		}
		if listing.Metadata.ContractType == pb.Listing_Metadata_DIGITAL_GOOD {
			if digitalResolver == nil {
				if requireDigitalResolver {
					return nil, fmt.Errorf("digital supply resolver unavailable for listing %q", listing.Slug)
				}
				continue
			}
			variantSKU, err := standardOrderVariantSKUFromOptions(listing, item.Options)
			if err != nil {
				return nil, fmt.Errorf("select digital sku for %q: %w", listing.Slug, err)
			}
			items := []digital.OrderLineItem{{
				ListingSlug: listing.Slug,
				VariantSKU:  variantSKU,
				Quantity:    uint32(qty),
			}}
			var digitalLines []contracts.SupplyLine
			if txResolver, ok := digitalResolver.(transactionalDigitalSupplyLineResolver); ok && tx != nil {
				digitalLines, err = txResolver.SupplyAvailabilityLinesForOrderItemsTx(tx, items)
			} else {
				digitalLines, err = digitalResolver.SupplyAvailabilityLinesForOrderItems(items)
			}
			if err != nil {
				return nil, fmt.Errorf("resolve digital supply for %q: %w", listing.Slug, err)
			}
			lines = append(lines, digitalLines...)
			continue
		}
		if listing.Metadata.ContractType != pb.Listing_Metadata_PHYSICAL_GOOD {
			continue
		}
		externalLine, err := standardOrderExternalSupplyLineInTx(tx, orderID, i, listing.Slug, qty)
		if err != nil {
			return nil, err
		}
		if externalLine != nil {
			lines = append(lines, *externalLine)
			continue
		}
		localListing, err := tx.GetListing(listing.Slug)
		if err != nil {
			return nil, fmt.Errorf("load local listing %q for supply reserve: %w", listing.Slug, err)
		}
		if localListing == nil || localListing.Listing == nil || localListing.Listing.Item == nil {
			return nil, fmt.Errorf("local listing %q is incomplete", listing.Slug)
		}
		// Seller-local listing is authoritative for inventory reserve (same as guest
		// checkout). The embedded order listing may omit SKU rows or quantities.
		sku, err := selectedStandardOrderSku(localListing.Listing, item.Options)
		if err != nil {
			sku, err = selectedStandardOrderSku(listing, item.Options)
		}
		if err != nil {
			return nil, fmt.Errorf("select sku for %q: %w", listing.Slug, err)
		}
		if sku == nil {
			continue
		}
		stockSku := authoritativeStandardStockSku(localListing.Listing, sku, item.Options)
		stockLimit, tracked, err := skuTrackedStockLimit(stockSku)
		if err != nil {
			return nil, fmt.Errorf("parse sku quantity for %q: %w", listing.Slug, err)
		}
		if !tracked {
			continue
		}
		lines = append(lines, contracts.SupplyLine{
			LineID:       fmt.Sprintf("%s:%d", orderID, i),
			ListingSlug:  listing.Slug,
			VariantHash:  standardOrderVariantHashFromSku(stockSku),
			Quantity:     qty,
			SupplyKind:   contracts.SupplyKindSkuQuantity,
			StockTracked: true,
			StockLimit:   stockLimit,
		})
	}
	return lines, nil
}

func standardOrderExternalSupplyLineInTx(tx database.Tx, orderID string, itemIndex int, listingSlug string, quantity int) (*contracts.SupplyLine, error) {
	var mapping models.SyncedProductMapping
	err := tx.Read().
		Where("listing_slug = ?", listingSlug).
		Order("last_sync_at DESC").
		First(&mapping).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if isMissingExternalSupplyMappingTable(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load external supply mapping for %q: %w", listingSlug, err)
	}
	return &contracts.SupplyLine{
		LineID:      fmt.Sprintf("%s:%d:external", orderID, itemIndex),
		ListingSlug: listingSlug,
		Quantity:    quantity,
		SupplyKind:  contracts.SupplyKindExternalSupply,
		ProviderID:  mapping.ProviderID,
		ProviderRef: standardOrderExternalProviderRef(mapping),
	}, nil
}

func isMissingExternalSupplyMappingTable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "synced_product_mappings") &&
		(strings.Contains(msg, "no such table") || strings.Contains(msg, "does not exist"))
}

func standardOrderExternalProviderRef(mapping models.SyncedProductMapping) string {
	if mapping.SyncProductID != "" {
		return mapping.SyncProductID
	}
	if mapping.ExternalID != "" {
		return mapping.ExternalID
	}
	return mapping.ID
}

func extractOrderOpenListing(listingHash string, listings []*pb.SignedListing) (*pb.Listing, error) {
	for _, sl := range listings {
		if sl == nil || sl.Listing == nil {
			continue
		}
		ser, err := proto.Marshal(sl)
		if err != nil {
			return nil, err
		}
		hash, err := utils.MultihashSha256(ser)
		if err != nil {
			return nil, err
		}
		if hash.B58String() == listingHash {
			return sl.Listing, nil
		}
	}
	return nil, fmt.Errorf("listing not found in order for item %s", listingHash)
}

func selectedStandardOrderSku(listing *pb.Listing, options []*pb.OrderOpen_Item_Option) (*pb.Listing_Item_Sku, error) {
	if listing == nil || listing.Item == nil {
		return nil, nil
	}
	if len(listing.Item.Skus) == 0 {
		return nil, nil
	}
	if len(listing.Item.Options) == 0 && len(listing.Item.Skus) == 1 {
		return listing.Item.Skus[0], nil
	}
	if len(listing.Item.Options) > 0 && len(options) == 0 {
		return nil, errors.New("selected sku not found in listing")
	}
	opts := standardOrderOptionMap(options)
	for _, sku := range listing.Item.Skus {
		if skuSelectionsMatchOptions(sku.GetSelections(), opts) {
			return sku, nil
		}
	}
	return nil, errors.New("selected sku not found in listing")
}

func standardOrderOptionMap(options []*pb.OrderOpen_Item_Option) map[string]string {
	opts := make(map[string]string, len(options))
	for _, option := range options {
		name := strings.ToLower(strings.TrimSpace(option.GetName()))
		if name == "" {
			continue
		}
		opts[name] = strings.ToLower(strings.TrimSpace(option.GetValue()))
	}
	return opts
}

func skuSelectionsMatchOptions(selections []*pb.Listing_Item_Sku_Selection, opts map[string]string) bool {
	if len(selections) != len(opts) {
		return false
	}
	for _, sel := range selections {
		option := strings.ToLower(strings.TrimSpace(sel.GetOption()))
		if option == "" {
			return false
		}
		if opts[option] != strings.ToLower(strings.TrimSpace(sel.GetVariant())) {
			return false
		}
	}
	return true
}

func matchingLocalSku(local *pb.Listing, sku *pb.Listing_Item_Sku) *pb.Listing_Item_Sku {
	if local == nil || local.Item == nil || sku == nil {
		return nil
	}
	if len(local.Item.Options) == 0 && len(local.Item.Skus) == 1 && len(sku.Selections) == 0 {
		return local.Item.Skus[0]
	}
	productID := strings.TrimSpace(sku.GetProductID())
	if productID != "" {
		for _, candidate := range local.Item.Skus {
			if strings.TrimSpace(candidate.GetProductID()) == productID {
				return candidate
			}
		}
	}
	if len(sku.Selections) == 0 {
		return nil
	}
	opts := make(map[string]string, len(sku.GetSelections()))
	for _, sel := range sku.GetSelections() {
		option := strings.ToLower(strings.TrimSpace(sel.GetOption()))
		if option == "" {
			continue
		}
		opts[option] = strings.ToLower(strings.TrimSpace(sel.GetVariant()))
	}
	for _, candidate := range local.Item.Skus {
		if skuSelectionsMatchOptions(candidate.GetSelections(), opts) {
			return candidate
		}
	}
	return nil
}

func authoritativeStandardStockSku(local *pb.Listing, embeddedSku *pb.Listing_Item_Sku, options []*pb.OrderOpen_Item_Option) *pb.Listing_Item_Sku {
	if embeddedSku == nil {
		return nil
	}
	if localSku := matchingLocalSku(local, embeddedSku); localSku != nil {
		return localSku
	}
	if local != nil {
		if localSku, err := selectedStandardOrderSku(local, options); err == nil && localSku != nil {
			return localSku
		}
	}
	return embeddedSku
}

func skuTrackedStockLimit(sku *pb.Listing_Item_Sku) (int64, bool, error) {
	if sku == nil {
		return 0, false, nil
	}
	qty := strings.TrimSpace(sku.GetQuantity())
	if qty == "" {
		return 0, false, nil
	}
	stockLimit, err := strconv.ParseInt(qty, 10, 64)
	if err != nil {
		return 0, false, err
	}
	if stockLimit < 0 {
		return 0, false, nil
	}
	return stockLimit, true, nil
}

func standardOrderVariantSKUFromOptions(listing *pb.Listing, options []*pb.OrderOpen_Item_Option) (string, error) {
	sku, err := selectedStandardOrderSku(listing, options)
	if err != nil {
		return "", err
	}
	if sku == nil {
		return "", nil
	}
	return strings.TrimSpace(sku.GetProductID()), nil
}

func standardOrderVariantHashFromSku(sku *pb.Listing_Item_Sku) string {
	if sku == nil || len(sku.Selections) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(sku.Selections))
	for _, sel := range sku.Selections {
		k := strings.ToLower(strings.TrimSpace(sel.Option))
		v := strings.ToLower(strings.TrimSpace(sel.Variant))
		if k == "" {
			continue
		}
		pairs = append(pairs, k+"="+v)
	}
	if len(pairs) == 0 {
		return ""
	}
	sort.Strings(pairs)
	sum := sha256.Sum256([]byte(strings.Join(pairs, "\x00")))
	return hex.EncodeToString(sum[:8])
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
	caps, ok := s.chainEscrowCapabilities(coinType)
	return ok && caps.HasReceiptVerification
}

// hasClientSignedEscrow checks whether the chain uses client-signed escrow
// (EVM/Solana/TRON smart contracts vs UTXO multisig).
func (s *OrderAppService) hasClientSignedEscrow(coinType iwallet.CoinType) bool {
	caps, ok := s.chainEscrowCapabilities(coinType)
	return ok && caps.HasClientSignedEscrow
}

func (s *OrderAppService) chainEscrowCapabilities(coinType iwallet.CoinType) (payment.ChainCapabilities, bool) {
	if s == nil || s.paymentRegistry == nil {
		return payment.ChainCapabilities{}, false
	}
	strategy, err := s.paymentRegistry.ForCoinV2(coinType)
	if err != nil {
		return payment.ChainCapabilities{}, false
	}
	return strategy.Capabilities(), true
}

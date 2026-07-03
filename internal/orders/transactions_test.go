package orders

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/models/factory"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestProcessOrderPayment_FiatKnownTxPromotesPending(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	op.stateValidator = &mockStateBridge{}
	op.multiwallet = nil

	orderID := "tx-fiat-1"
	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:     models.OrderID(orderID),
		MyRole: string(models.RoleVendor),
	}
	order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}

	const (
		txID      = "pi_test_123"
		toAddress = "acct_test_dest"
	)
	paymentSent := &pb.PaymentSent{
		TransactionID:  txID,
		Coin:           "fiat:stripe:USD",
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_FIAT),
		ToAddress:      toAddress,
		Amount:         "100",
		Timestamp:      timestamppb.Now(),
	}

	msg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(msg, op.signer); err != nil {
		t.Fatal(err)
	}

	tx := iwallet.Transaction{
		ID: iwallet.TransactionID(txID),
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(toAddress, iwallet.CoinType(paymentSent.Coin)),
				Amount:  iwallet.NewAmount(100),
			},
		},
		Value: iwallet.NewAmount(100),
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Save(order); err != nil {
			return err
		}
		if err := op.ProcessOrderPayment(dbtx, order, msg, tx); err != nil {
			return err
		}
		return dbtx.Save(order)
	}); err != nil {
		t.Fatal(err)
	}

	var stored models.Order
	if err := op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}

	if !stored.IsPaymentVerified() {
		t.Fatal("expected fiat payment to be marked verified")
	}
	if stored.State != models.OrderState_PENDING {
		t.Fatalf("expected state PENDING after verified fiat payment, got %s", stored.State)
	}
}

func TestProcessMessage_FiatKnownTxPromotesPending(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	op.stateValidator = &mockStateBridge{}
	op.multiwallet = nil

	orderID := "tx-fiat-process-msg-1"
	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:     models.OrderID(orderID),
		MyRole: string(models.RoleVendor),
	}
	order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}

	const (
		txID      = "pi_process_message_123"
		toAddress = "acct_process_message_dest"
	)
	paymentSent := &pb.PaymentSent{
		TransactionID:  txID,
		Coin:           "fiat:stripe:USD",
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_FIAT),
		ToAddress:      toAddress,
		Amount:         "100",
		Timestamp:      timestamppb.Now(),
	}

	msg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(msg, op.signer); err != nil {
		t.Fatal(err)
	}

	tx := iwallet.Transaction{
		ID: iwallet.TransactionID(txID),
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(toAddress, iwallet.CoinType(paymentSent.Coin)),
				Amount:  iwallet.NewAmount(100),
			},
		},
		Value: iwallet.NewAmount(100),
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Save(order); err != nil {
			return err
		}
		if err := order.PutTransaction(tx); err != nil {
			return err
		}
		if _, err := op.processMessage(dbtx, order, msg); err != nil {
			return err
		}
		return dbtx.Save(order)
	}); err != nil {
		t.Fatal(err)
	}

	var stored models.Order
	if err := op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}

	if !stored.IsPaymentVerified() {
		t.Fatal("expected fiat payment to be marked verified")
	}
	if stored.State != models.OrderState_PENDING {
		t.Fatalf("expected state PENDING after processMessage, got %s", stored.State)
	}
}

func TestProcessOrderPayment_CryptoKnownTxStaysAwaitingVerification(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	op.stateValidator = &mockStateBridge{}
	op.multiwallet = nil

	orderID := "tx-crypto-1"
	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:     models.OrderID(orderID),
		MyRole: string(models.RoleVendor),
	}
	order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}

	const (
		txID      = "tx_crypto_123"
		toAddress = "mock_dest_addr"
	)
	paymentSent := &pb.PaymentSent{
		TransactionID:  txID,
		Coin:           string(iwallet.CtMock),
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_DIRECT),
		ToAddress:      toAddress,
		Amount:         "100",
		Timestamp:      timestamppb.Now(),
	}

	msg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(msg, op.signer); err != nil {
		t.Fatal(err)
	}

	tx := iwallet.Transaction{
		ID: iwallet.TransactionID(txID),
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(toAddress, iwallet.CoinType(paymentSent.Coin)),
				Amount:  iwallet.NewAmount(100),
			},
		},
		Value: iwallet.NewAmount(100),
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Save(order); err != nil {
			return err
		}
		if err := op.ProcessOrderPayment(dbtx, order, msg, tx); err != nil {
			return err
		}
		return dbtx.Save(order)
	}); err != nil {
		t.Fatal(err)
	}

	var stored models.Order
	if err := op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}

	if stored.IsPaymentVerified() {
		t.Fatal("expected crypto known tx to remain unverified until explicit verification")
	}
	if stored.State != models.OrderState_AWAITING_PAYMENT_VERIFICATION {
		t.Fatalf("expected state AWAITING_PAYMENT_VERIFICATION, got %s", stored.State)
	}
}

func TestProcessOrderPayment_CryptoKnownConfirmedTxPromotesPending(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	op.stateValidator = &mockStateBridge{}
	op.multiwallet = nil

	orderID := "tx-crypto-confirmed-1"
	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:     models.OrderID(orderID),
		MyRole: string(models.RoleVendor),
	}
	order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}

	const (
		txID      = "tx_crypto_confirmed_123"
		toAddress = "mock_dest_addr_confirmed"
	)
	paymentSent := &pb.PaymentSent{
		TransactionID:  txID,
		Coin:           string(iwallet.CtMock),
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_DIRECT),
		ToAddress:      toAddress,
		Amount:         "100",
		Timestamp:      timestamppb.Now(),
	}

	msg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(msg, op.signer); err != nil {
		t.Fatal(err)
	}

	tx := iwallet.Transaction{
		ID:     iwallet.TransactionID(txID),
		Height: 12345,
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(toAddress, iwallet.CoinType(paymentSent.Coin)),
				Amount:  iwallet.NewAmount(100),
			},
		},
		Value: iwallet.NewAmount(100),
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Save(order); err != nil {
			return err
		}
		if err := op.ProcessOrderPayment(dbtx, order, msg, tx); err != nil {
			return err
		}
		return dbtx.Save(order)
	}); err != nil {
		t.Fatal(err)
	}

	var stored models.Order
	if err := op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}

	if !stored.IsPaymentVerified() {
		t.Fatal("expected confirmed crypto tx to be marked verified")
	}
	if stored.State != models.OrderState_PENDING {
		t.Fatalf("expected state PENDING after confirmed crypto tx, got %s", stored.State)
	}
}

func TestProcessOrderPayment_CancelableConfirmedTxEmitsSingleReadyEvent(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	op.stateValidator = &mockStateBridge{}
	op.multiwallet = nil

	sub, err := op.bus.Subscribe(&events.CancelablePaymentReady{}, events.BufSize(4))
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	orderID := "tx-cancelable-confirmed-ready-1"
	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:     models.OrderID(orderID),
		MyRole: string(models.RoleVendor),
	}
	order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}

	const (
		txID      = "tx_cancelable_confirmed_ready_123"
		toAddress = "mock_dest_addr_cancelable_confirmed"
	)
	paymentSent := &pb.PaymentSent{
		TransactionID:  txID,
		Coin:           string(iwallet.CtMock),
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_CANCELABLE),
		ToAddress:      toAddress,
		Amount:         "123",
		Timestamp:      timestamppb.Now(),
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			Id:        "fact-" + txID,
			TxHash:    txID,
			ToAddress: toAddress,
			Amount:    "123",
			Status:    models.PaymentObservationStatusConfirmed,
		}},
	}

	msg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(msg, op.signer); err != nil {
		t.Fatal(err)
	}

	tx := iwallet.Transaction{
		ID:     iwallet.TransactionID(txID),
		Height: 12345,
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(toAddress, iwallet.CoinType(paymentSent.Coin)),
				Amount:  iwallet.NewAmount(60),
			},
		},
		Value: iwallet.NewAmount(60),
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Save(order); err != nil {
			return err
		}
		if err := op.ProcessOrderPayment(dbtx, order, msg, tx); err != nil {
			return err
		}
		return dbtx.Save(order)
	}); err != nil {
		t.Fatal(err)
	}

	var readyEvents []*events.CancelablePaymentReady
	for {
		select {
		case evt := <-sub.Out():
			ready, ok := evt.(*events.CancelablePaymentReady)
			if !ok {
				t.Fatalf("unexpected event type %T", evt)
			}
			readyEvents = append(readyEvents, ready)
		default:
			if len(readyEvents) != 1 {
				t.Fatalf("expected one CancelablePaymentReady event, got %d", len(readyEvents))
			}
			ready := readyEvents[0]
			if ready.OrderID != orderID {
				t.Fatalf("event orderID = %s, want %s", ready.OrderID, orderID)
			}
			if ready.TransactionID != txID {
				t.Fatalf("event txid = %s, want %s", ready.TransactionID, txID)
			}
			if ready.Coin != paymentSent.Coin {
				t.Fatalf("event coin = %s, want %s", ready.Coin, paymentSent.Coin)
			}
			if ready.Amount != "123" {
				t.Fatalf("event amount = %q, want aggregated PaymentSent amount 123", ready.Amount)
			}
			return
		}
	}
}

func TestRecordVerifiedPaymentBeforePaymentSentWaitsForPaymentSent(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	op.stateValidator = &mockStateBridge{}
	op.multiwallet = nil

	orderID := "tx-verified-before-payment-sent-1"
	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:     models.OrderID(orderID),
		MyRole: string(models.RoleVendor),
	}
	order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}

	const (
		txID      = "tx_verified_before_payment_sent_123"
		toAddress = "mock_dest_addr_verified_before_payment_sent"
	)
	tx := iwallet.Transaction{
		ID:     iwallet.TransactionID(txID),
		Height: 12345,
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(toAddress, iwallet.CoinType(iwallet.CtMock)),
				Amount:  iwallet.NewAmount(100),
			},
		},
		Value: iwallet.NewAmount(100),
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Save(order); err != nil {
			return err
		}
		return op.RecordVerifiedPayment(dbtx, order, tx)
	}); err != nil {
		t.Fatal(err)
	}

	var stored models.Order
	if err := op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}
	if !stored.IsPaymentVerified() {
		t.Fatal("expected observed payment to be marked verified")
	}
	if stored.SerializedPaymentSent != nil {
		t.Fatal("expected payment sent message to remain absent")
	}
	if stored.State != models.OrderState_AWAITING_PAYMENT {
		t.Fatalf("expected state to wait for PAYMENT_SENT, got %s", stored.State)
	}

	paymentSent := &pb.PaymentSent{
		TransactionID:  txID,
		Coin:           string(iwallet.CtMock),
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_DIRECT),
		ToAddress:      toAddress,
		Amount:         "100",
		Timestamp:      timestamppb.Now(),
	}
	msg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(msg, op.signer); err != nil {
		t.Fatal(err)
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Read().Where("id = ?", orderID).First(&stored).Error; err != nil {
			return err
		}
		if err := op.ProcessOrderPayment(dbtx, &stored, msg, tx); err != nil {
			return err
		}
		return dbtx.Save(&stored)
	}); err != nil {
		t.Fatal(err)
	}

	if err := op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}
	if stored.SerializedPaymentSent == nil {
		t.Fatal("expected payment sent message to be recorded")
	}
	if stored.State != models.OrderState_PENDING {
		t.Fatalf("expected state PENDING after PAYMENT_SENT consumes verified payment, got %s", stored.State)
	}
}

func TestProcessOrderPayment_NormalizesPendingWithoutPaymentSent(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	op.stateValidator = &mockStateBridge{}
	op.multiwallet = nil

	orderID := "tx-pending-without-payment-sent-1"
	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:     models.OrderID(orderID),
		MyRole: string(models.RoleVendor),
	}
	order.SetFSMState(models.OrderState_PENDING)
	order.MarkPaymentVerified()
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}

	const (
		txID      = "tx_pending_without_payment_sent_123"
		toAddress = "mock_dest_addr_pending_without_payment_sent"
	)
	paymentSent := &pb.PaymentSent{
		TransactionID:  txID,
		Coin:           string(iwallet.CtMock),
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_DIRECT),
		ToAddress:      toAddress,
		Amount:         "100",
		Timestamp:      timestamppb.Now(),
	}
	msg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(msg, op.signer); err != nil {
		t.Fatal(err)
	}

	tx := iwallet.Transaction{
		ID:     iwallet.TransactionID(txID),
		Height: 12345,
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(toAddress, iwallet.CoinType(iwallet.CtMock)),
				Amount:  iwallet.NewAmount(100),
			},
		},
		Value: iwallet.NewAmount(100),
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Save(order); err != nil {
			return err
		}
		if err := op.ProcessOrderPayment(dbtx, order, msg, tx); err != nil {
			return err
		}
		return dbtx.Save(order)
	}); err != nil {
		t.Fatal(err)
	}

	var stored models.Order
	if err := op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}
	if stored.SerializedPaymentSent == nil {
		t.Fatal("expected payment sent message to be recorded")
	}
	if stored.State != models.OrderState_PENDING {
		t.Fatalf("expected state PENDING after normalized PAYMENT_SENT, got %s", stored.State)
	}
}

func TestProcessOrderPayment_DuplicatePaymentSentWithConfirmedTxDoesNotVerify(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	op.stateValidator = &mockStateBridge{}
	op.multiwallet = nil

	orderID := "tx-duplicate-payment-sent-confirmed-1"
	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	const (
		txID      = "tx_duplicate_payment_sent_confirmed_123"
		toAddress = "mock_dest_addr_duplicate_payment_sent"
	)
	paymentSent := &pb.PaymentSent{
		TransactionID:  txID,
		Coin:           string(iwallet.CtMock),
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_CANCELABLE),
		ToAddress:      toAddress,
		Amount:         "100",
		Timestamp:      timestamppb.Now(),
	}
	msg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(msg, op.signer); err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:     models.OrderID(orderID),
		MyRole: string(models.RoleVendor),
	}
	order.SetFSMState(models.OrderState_PENDING)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(msg); err != nil {
		t.Fatal(err)
	}

	tx := iwallet.Transaction{
		ID:     iwallet.TransactionID(txID),
		Height: 12345,
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(toAddress, iwallet.CoinType(iwallet.CtMock)),
				Amount:  iwallet.NewAmount(100),
			},
		},
		Value: iwallet.NewAmount(100),
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Save(order); err != nil {
			return err
		}
		if err := op.ProcessOrderPayment(dbtx, order, msg, tx); err != nil {
			return err
		}
		return dbtx.Save(order)
	}); err != nil {
		t.Fatal(err)
	}

	var stored models.Order
	if err := op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}
	if stored.IsPaymentVerified() {
		t.Fatal("duplicate PAYMENT_SENT must not mark payment verified; verified recording belongs to RecordVerifiedPayment")
	}
	if stored.State != models.OrderState_PENDING {
		t.Fatalf("expected duplicate PAYMENT_SENT to leave state PENDING, got %s", stored.State)
	}
}

func TestProcessOrderPayment_DuplicateAddressMonitoredVerifiedPendingPaymentSentNoOps(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	op.stateValidator = &mockStateBridge{}
	op.multiwallet = nil

	sub, err := op.bus.Subscribe(&events.CancelablePaymentReady{}, events.BufSize(4))
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	orderID := "tx-duplicate-verified-pending-ready-1"
	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	const (
		txID      = "legacy_representative_tx_123"
		knownTxID = "different_observed_tx_456"
		toAddress = "mock_dest_addr_duplicate_verified_pending"
	)
	paymentSent := &pb.PaymentSent{
		TransactionID:  txID,
		Coin:           string(iwallet.CtMock),
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_CANCELABLE),
		ToAddress:      toAddress,
		Amount:         "456",
		Timestamp:      timestamppb.Now(),
	}
	msg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(msg, op.signer); err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:     models.OrderID(orderID),
		MyRole: string(models.RoleVendor),
	}
	order.SetFSMState(models.OrderState_PENDING)
	order.MarkPaymentVerified()
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(msg); err != nil {
		t.Fatal(err)
	}

	tx := iwallet.Transaction{
		ID: iwallet.TransactionID(knownTxID),
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(toAddress, iwallet.CoinType(iwallet.CtMock)),
				Amount:  iwallet.NewAmount(456),
			},
		},
		Value: iwallet.NewAmount(456),
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Save(order); err != nil {
			return err
		}
		if err := op.ProcessOrderPayment(dbtx, order, msg, tx); err != nil {
			return err
		}
		return dbtx.Save(order)
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case evt := <-sub.Out():
		t.Fatalf("duplicate verified PAYMENT_SENT emitted unexpected event %T", evt)
	default:
	}
}

func TestProcessOrderPayment_DuplicatePaymentSentAcceptsPersistedFundingFactsAsNoOp(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	op.stateValidator = &mockStateBridge{}
	op.multiwallet = nil

	sub, err := op.bus.Subscribe(&events.CancelablePaymentReady{}, events.BufSize(4))
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	orderID := "tx-duplicate-funding-facts-1"
	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	const (
		firstTxID  = "funding_fact_tx_1"
		secondTxID = "funding_fact_tx_2"
		toAddress  = "mock_dest_addr_duplicate_facts"
	)
	persistedPayment := &pb.PaymentSent{
		TransactionID:      firstTxID,
		Coin:               string(iwallet.CtMock),
		SettlementSpec:     testPaymentSentSpec(pb.PaymentSent_CANCELABLE),
		ToAddress:          toAddress,
		Amount:             "1000",
		Timestamp:          timestamppb.Now(),
		ConfirmationPolicy: models.PaymentConfirmationPolicyMempoolAccepted,
		FundingFacts: []*pb.PaymentSent_FundingFact{
			{TxHash: firstTxID, Amount: "400", ToAddress: toAddress, Status: models.PaymentObservationStatusConfirmed},
			{TxHash: secondTxID, Amount: "600", ToAddress: toAddress, Status: models.PaymentObservationStatusPending},
		},
	}
	persistedMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(persistedPayment),
	}
	if err := utils.SignOrderMessage(persistedMsg, op.signer); err != nil {
		t.Fatal(err)
	}

	incomingPartial := &pb.PaymentSent{
		TransactionID:  secondTxID,
		Coin:           string(iwallet.CtMock),
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_CANCELABLE),
		ToAddress:      toAddress,
		Amount:         "600",
		Timestamp:      timestamppb.Now(),
	}
	incomingMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(incomingPartial),
	}
	if err := utils.SignOrderMessage(incomingMsg, op.signer); err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:             models.OrderID(orderID),
		MyRole:         string(models.RoleVendor),
		PaymentAddress: toAddress,
	}
	order.SetFSMState(models.OrderState_PENDING)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(persistedMsg); err != nil {
		t.Fatal(err)
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Save(order); err != nil {
			return err
		}
		tx := iwallet.Transaction{
			ID:    iwallet.TransactionID(secondTxID),
			To:    []iwallet.SpendInfo{{Address: iwallet.NewAddress(toAddress, iwallet.CoinType(iwallet.CtMock)), Amount: iwallet.NewAmount(600)}},
			Value: iwallet.NewAmount(600),
		}
		if err := op.ProcessOrderPayment(dbtx, order, incomingMsg, tx); err != nil {
			return err
		}
		return dbtx.Save(order)
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case evt := <-sub.Out():
		t.Fatalf("compatible duplicate PAYMENT_SENT emitted unexpected event %T", evt)
	default:
	}

	var stored models.Order
	if err := op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}
	if stored.IsPaymentVerified() {
		t.Fatal("duplicate PAYMENT_SENT must remain an idempotent message no-op; verification belongs to PVS/RecordVerifiedPayment")
	}
}

func TestRecordVerifiedPayment_ReplacesProvisionalTransaction(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	op.stateValidator = &mockStateBridge{}
	op.multiwallet = nil

	orderID := "tx-replace-provisional-1"
	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	const (
		txID      = "tx_replace_provisional_123"
		toAddress = "mock_dest_addr_replace_provisional"
	)
	paymentSent := &pb.PaymentSent{
		TransactionID:  txID,
		Coin:           string(iwallet.CtMock),
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_CANCELABLE),
		ToAddress:      toAddress,
		Amount:         "100",
		Timestamp:      timestamppb.Now(),
	}
	msg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(msg, op.signer); err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:             models.OrderID(orderID),
		MyRole:         string(models.RoleVendor),
		PaymentAddress: toAddress,
	}
	order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}

	provisionalToID := []byte("provisional-outpoint-000000000000")
	verifiedToID := []byte("verified-outpoint-0000000000000000")
	provisionalTx := iwallet.Transaction{
		ID: iwallet.TransactionID(txID),
		To: []iwallet.SpendInfo{
			{
				ID:      provisionalToID,
				Address: iwallet.NewAddress(toAddress, iwallet.CoinType(iwallet.CtMock)),
				Amount:  iwallet.NewAmount(100),
			},
		},
		Value: iwallet.NewAmount(100),
	}
	verifiedTx := iwallet.Transaction{
		ID:     iwallet.TransactionID(txID),
		Height: 12345,
		To: []iwallet.SpendInfo{
			{
				ID:      verifiedToID,
				Address: iwallet.NewAddress(toAddress, iwallet.CoinType(iwallet.CtMock)),
				Amount:  iwallet.NewAmount(100),
			},
		},
		Value: iwallet.NewAmount(100),
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Save(order); err != nil {
			return err
		}
		if err := op.ProcessOrderPayment(dbtx, order, msg, provisionalTx); err != nil {
			return err
		}
		if err := op.RecordVerifiedPayment(dbtx, order, verifiedTx); err != nil {
			return err
		}
		return dbtx.Save(order)
	}); err != nil {
		t.Fatal(err)
	}

	var stored models.Order
	if err := op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}
	txs, err := stored.GetTransactions()
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 1 {
		t.Fatalf("expected one merged transaction, got %d", len(txs))
	}
	if got := string(txs[0].To[0].ID); got != string(verifiedToID) {
		t.Fatalf("stored ToID = %q, want verified ToID %q", got, string(verifiedToID))
	}
	if txs[0].Height != verifiedTx.Height {
		t.Fatalf("stored height = %d, want %d", txs[0].Height, verifiedTx.Height)
	}
}

func TestRecordVerifiedPayment_DoesNotRestartDeclinedOrder(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	op.stateValidator = &mockStateBridge{}
	op.multiwallet = nil

	sub, err := op.bus.Subscribe(&events.CancelablePaymentReady{}, events.BufSize(4))
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	orderID := "tx-verified-after-decline-1"
	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	const (
		txID      = "tx_verified_after_decline_123"
		toAddress = "mock_dest_addr_after_decline"
	)
	paymentSent := &pb.PaymentSent{
		TransactionID:  txID,
		Coin:           string(iwallet.CtMock),
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_CANCELABLE),
		ToAddress:      toAddress,
		Amount:         "100",
		Timestamp:      timestamppb.Now(),
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			Id:        "fact-" + txID,
			TxHash:    txID,
			ToAddress: toAddress,
			Amount:    "100",
			Status:    models.PaymentObservationStatusConfirmed,
		}},
	}
	paymentMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(paymentMsg, op.signer); err != nil {
		t.Fatal(err)
	}
	declineMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_DECLINE,
		Message: mustBuildAny(&pb.OrderDecline{
			Type:          pb.OrderDecline_USER_DECLINE,
			Reason:        "seller declined after escrow funding",
			TransactionID: "seller_decline_refund_tx",
			Timestamp:     timestamppb.Now(),
		}),
	}
	if err := utils.SignOrderMessage(declineMsg, op.signer); err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:             models.OrderID(orderID),
		MyRole:         string(models.RoleVendor),
		PaymentAddress: toAddress,
	}
	order.SetFSMState(models.OrderState_DECLINED)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(paymentMsg); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(declineMsg); err != nil {
		t.Fatal(err)
	}

	verifiedTx := iwallet.Transaction{
		ID:     iwallet.TransactionID(txID),
		Height: 12345,
		To: []iwallet.SpendInfo{{
			Address: iwallet.NewAddress(toAddress, iwallet.CoinType(iwallet.CtMock)),
			Amount:  iwallet.NewAmount(100),
		}},
		Value: iwallet.NewAmount(100),
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Save(order); err != nil {
			return err
		}
		return op.RecordVerifiedPayment(dbtx, order, verifiedTx)
	}); err != nil {
		t.Fatal(err)
	}

	var stored models.Order
	if err := op.db.View(func(dbtx database.Tx) error {
		return dbtx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}
	if !stored.IsPaymentVerified() {
		t.Fatal("expected payment verification fact to be recorded")
	}
	if stored.State != models.OrderState_DECLINED {
		t.Fatalf("expected declined order to stay DECLINED, got %s", stored.State)
	}
	if stored.PaymentSettlementSignaledAt != nil {
		t.Fatal("declined order must not be marked ready for settlement auto-confirm")
	}

	select {
	case evt := <-sub.Out():
		t.Fatalf("declined order emitted unexpected settlement event %T", evt)
	default:
	}
}

func TestRecordVerifiedPayment_AlreadyVerifiedVendorRecoversCancelableReady(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	op.stateValidator = &mockStateBridge{}
	op.multiwallet = nil

	sub, err := op.bus.Subscribe(&events.CancelablePaymentReady{}, events.BufSize(4))
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	orderID := "tx-already-verified-recovers-ready-1"
	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	const (
		txID      = "tx_already_verified_recovery_123"
		toAddress = "mock_dest_addr_recovery"
	)
	paymentSent := &pb.PaymentSent{
		TransactionID:  txID,
		Coin:           string(iwallet.CtMock),
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_CANCELABLE),
		ToAddress:      toAddress,
		Amount:         "100",
		Timestamp:      timestamppb.Now(),
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			Id:        "fact-" + txID,
			TxHash:    txID,
			ToAddress: toAddress,
			Amount:    "100",
			Status:    models.PaymentObservationStatusConfirmed,
		}},
	}
	msg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(msg, op.signer); err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:             models.OrderID(orderID),
		MyRole:         string(models.RoleVendor),
		PaymentAddress: toAddress,
	}
	order.SetFSMState(models.OrderState_PENDING)
	order.MarkPaymentVerified()
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(msg); err != nil {
		t.Fatal(err)
	}

	verifiedTx := iwallet.Transaction{
		ID:     iwallet.TransactionID(txID),
		Height: 12345,
		To: []iwallet.SpendInfo{{
			Address: iwallet.NewAddress(toAddress, iwallet.CoinType(iwallet.CtMock)),
			Amount:  iwallet.NewAmount(100),
		}},
		Value: iwallet.NewAmount(100),
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		if err := dbtx.Save(order); err != nil {
			return err
		}
		return op.RecordVerifiedPayment(dbtx, order, verifiedTx)
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case evt := <-sub.Out():
		ready, ok := evt.(*events.CancelablePaymentReady)
		if !ok {
			t.Fatalf("expected CancelablePaymentReady, got %T", evt)
		}
		if ready.OrderID != orderID || ready.TransactionID != txID || ready.Amount != "100" {
			t.Fatalf("unexpected ready event: %+v", ready)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for recovered CancelablePaymentReady")
	}

	var stored models.Order
	if err := op.db.View(func(dbtx database.Tx) error {
		return dbtx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}
	if stored.PaymentSettlementSignaledAt == nil {
		t.Fatal("PaymentSettlementSignaledAt was not persisted")
	}

	if err := op.db.Update(func(dbtx database.Tx) error {
		var fresh models.Order
		if err := dbtx.Read().Where("id = ?", orderID).First(&fresh).Error; err != nil {
			return err
		}
		return op.RecordVerifiedPayment(dbtx, &fresh, verifiedTx)
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case evt := <-sub.Out():
		t.Fatalf("repeated RecordVerifiedPayment emitted unexpected event %T", evt)
	default:
	}
}

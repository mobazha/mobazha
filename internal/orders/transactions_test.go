package orders

import (
	"testing"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
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
		TransactionID: txID,
		Coin:          "fiat:stripe:USD",
		Method:        pb.PaymentSent_FIAT,
		ToAddress:     toAddress,
		Amount:        "100",
		Timestamp:     timestamppb.Now(),
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
		TransactionID: txID,
		Coin:          "fiat:stripe:USD",
		Method:        pb.PaymentSent_FIAT,
		ToAddress:     toAddress,
		Amount:        "100",
		Timestamp:     timestamppb.Now(),
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
		TransactionID: txID,
		Coin:          string(iwallet.CtMock),
		Method:        pb.PaymentSent_DIRECT,
		ToAddress:     toAddress,
		Amount:        "100",
		Timestamp:     timestamppb.Now(),
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
		TransactionID: txID,
		Coin:          string(iwallet.CtMock),
		Method:        pb.PaymentSent_DIRECT,
		ToAddress:     toAddress,
		Amount:        "100",
		Timestamp:     timestamppb.Now(),
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

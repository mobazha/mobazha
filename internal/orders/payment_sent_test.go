package orders

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/chains"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/anypb"
)

func testPaymentSentSpec(method pb.PaymentSent_Method) *pb.PaymentSent_SettlementSpec {
	switch method {
	case pb.PaymentSent_FIAT:
		return &pb.PaymentSent_SettlementSpec{Method: method, PayMode: "provider", EscrowType: "fiat_provider"}
	case pb.PaymentSent_CANCELABLE, pb.PaymentSent_MODERATED:
		return &pb.PaymentSent_SettlementSpec{Method: method, PayMode: "address_monitored", EscrowType: "utxo_script"}
	default:
		return &pb.PaymentSent_SettlementSpec{Method: method, PayMode: "address_monitored", EscrowType: "none"}
	}
}

func Test_processPaymentSentMessage(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	wn := wallet.NewMockWalletNetwork(1)
	txCh := wn.Wallets()[0].SubscribeTransactions()
	go wn.Start()

	addr, err := wn.Wallets()[0].CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}
	if err := wn.GenerateToAddress(addr, iwallet.NewAmount(100000)); err != nil {
		t.Fatal(err)
	}

	// Wait for the transaction to be processed by the wallet goroutine.
	generatedTx := <-txCh

	txs := []iwallet.Transaction{generatedTx}

	mw := op.multiwallet.(*chains.Multiwallet)
	(*mw)[iwallet.ChainMock] = wn.Wallets()[0]

	orderOpen, paymentSent, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}
	paymentSent.TransactionID = txs[0].ID.String()
	paymentSent.ToAddress = txs[0].To[0].Address.String()
	paymentSent.SettlementSpec = testPaymentSentSpec(pb.PaymentSent_DIRECT)
	paymentSent.ContractAddress = ""
	paymentSent.Script = ""
	paymentSent.Moderator = ""
	paymentSent.ModeratorAddress = ""

	paymentAny := &anypb.Any{}
	if err := paymentAny.MarshalFrom(paymentSent); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     "1234",
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     paymentAny,
	}
	if err := utils.SignOrderMessage(orderMsg, op.signer); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		setup         func(order *models.Order) error
		expectedError error
		expectedEvent interface{}
		checkTxs      func(order *models.Order) error
	}{
		{
			// Normal case where order open exists.
			// Note: processPaymentSentMessage no longer records transactions inline;
			// that is handled by the orchestration layer (postProcessPaymentSentInTx).
			setup: func(order *models.Order) error {
				order.ID = "1234"
				order.PaymentAddress = addr.String()
				err := order.PutMessage(&npb.OrderMessage{
					Signature: []byte("abc"),
					Message:   mustBuildAny(orderOpen),
				})
				return err
			},
			expectedError: nil,
			expectedEvent: &events.PaymentSentReceived{
				OrderID: "1234",
				Txid:    txs[0].ID.String(),
			},
			checkTxs: func(order *models.Order) error {
				return nil
			},
		},
		{
			// Duplicate payment
			setup: func(order *models.Order) error {
				return order.PutMessage(orderMsg)
			},
			expectedError: nil,
			expectedEvent: nil,
			checkTxs: func(order *models.Order) error {
				return nil
			},
		},
		{
			// Out of order.
			setup: func(order *models.Order) error {
				order.SerializedOrderOpen = nil
				return nil
			},
			expectedError: ErrMessageParked,
			expectedEvent: nil,
			checkTxs: func(order *models.Order) error {
				return nil
			},
		},
	}

	for i, test := range tests {
		order := &models.Order{}
		if err := test.setup(order); err != nil {
			t.Errorf("Test %d setup error: %s", i, err)
			continue
		}
		err := op.db.Update(func(tx database.Tx) error {
			event, err := op.processPaymentSentMessage(tx, order, orderMsg)
			if !errors.Is(err, test.expectedError) {
				return fmt.Errorf("incorrect error returned. Expected %v, got %v", test.expectedError, err)
			}
			if !reflect.DeepEqual(event, test.expectedEvent) {
				return fmt.Errorf("incorrect event returned")
			}

			return test.checkTxs(order)
		})
		if err != nil {
			t.Errorf("Error executing db update in test %d: %s", i, err)
		}
	}
}

func TestProcessPaymentSentMessage_ManagedEscrowEnvelopeSkipsLegacyEscrowValidation(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:     "managed_escrow-payment-sent",
		MyRole: string(models.RoleVendor),
	}
	order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     order.ID.String(),
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("order-open-sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}

	managed_escrowAddress := "0x1111111111111111111111111111111111111111"
	paymentSent := &pb.PaymentSent{
		TransactionID:   "0xmanagedescrow",
		Coin:            "crypto:eip155:1:native",
		SettlementSpec:  &pb.PaymentSent_SettlementSpec{Method: pb.PaymentSent_CANCELABLE, PayMode: "address_monitored", EscrowType: "managed_escrow"},
		ContractAddress: managed_escrowAddress,
		ToAddress:       managed_escrowAddress,
		Amount:          "1000",
	}
	orderMsg := &npb.OrderMessage{
		OrderID:     order.ID.String(),
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(orderMsg, op.signer); err != nil {
		t.Fatal(err)
	}

	err = op.db.Update(func(tx database.Tx) error {
		event, err := op.processPaymentSentMessage(tx, order, orderMsg)
		if err != nil {
			return err
		}
		if event == nil {
			return errors.New("expected payment sent event")
		}
		if len(order.SerializedPaymentSent) == 0 {
			return errors.New("expected serialized payment sent")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestProcessMessage_ErrorRecordsErroredMessageWhenTransactionCommits(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	orderID := "payment-sent-error-recorded"
	order := &models.Order{
		ID:     models.OrderID(orderID),
		MyRole: string(models.RoleVendor),
	}
	order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("order-open-sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}

	paymentSent := &pb.PaymentSent{
		TransactionID: "bad-tx",
		Coin:          "not-canonical",
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_CANCELABLE),
		ToAddress:     "bad-address",
		Amount:        "1000",
	}
	orderMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(orderMsg, op.signer); err != nil {
		t.Fatal(err)
	}

	err = op.db.Update(func(tx database.Tx) error {
		if err := tx.Save(order); err != nil {
			return err
		}
		if _, err := op.ProcessMessage(tx, orderMsg); err == nil {
			return errors.New("expected process error")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	var stored models.Order
	if err := op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}
	errored, err := stored.GetErroredMessages()
	if err != nil {
		t.Fatal(err)
	}
	if len(errored.Messages) != 1 {
		t.Fatalf("expected one errored message, got %d", len(errored.Messages))
	}
	if errored.Messages[0].MessageType != npb.OrderMessage_PAYMENT_SENT {
		t.Fatalf("expected PAYMENT_SENT errored message, got %s", errored.Messages[0].MessageType)
	}
}

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
	paymentSent.Method = pb.PaymentSent_DIRECT
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

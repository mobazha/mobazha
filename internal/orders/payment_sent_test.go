package orders

import (
	"crypto/rand"
	"errors"
	"fmt"
	"reflect"
	"testing"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
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
	go wn.Start()

	addr, err := wn.Wallets()[0].CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}
	if err := wn.GenerateToAddress(addr, iwallet.NewAmount(100000)); err != nil {
		t.Fatal(err)
	}

	txs, err := wn.Wallets()[0].Transactions(-1, iwallet.TransactionID(""))
	if err != nil {
		t.Fatal(err)
	}

	op.multiwallet["MCK"] = wn.Wallets()[0]

	_, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	remotePeer, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}

	paymentMsg := &pb.PaymentSent{
		TransactionID: txs[0].ID.String(),
	}

	paymentAny := &anypb.Any{}
	if err := paymentAny.MarshalFrom(paymentMsg); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     "1234",
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     paymentAny,
	}

	tests := []struct {
		setup         func(order *models.Order) error
		expectedError error
		expectedEvent interface{}
		checkTxs      func(order *models.Order) error
	}{
		{
			// Normal case where order open exists.
			setup: func(order *models.Order) error {
				order.ID = "1234"
				order.PaymentAddress = addr.String()
				return order.PutMessage(&npb.OrderMessage{
					Signature: []byte("abc"),
					Message: mustBuildAny(&pb.OrderOpen{
						Payment: &pb.OrderOpen_Payment{
							Coin:   "MCK",
							Amount: "1000",
						},
					}),
				})
			},
			expectedError: nil,
			expectedEvent: &events.PaymentSentReceived{
				OrderID: "1234",
				Txid:    txs[0].ID.String(),
			},
			checkTxs: func(order *models.Order) error {
				orderTxs, err := order.GetTransactions()
				if err != nil {
					return err
				}
				if len(orderTxs) == 0 {
					return errors.New("failed to record tx")
				}
				if orderTxs[0].ID != txs[0].ID {
					return errors.New("failed to record tx")
				}
				return nil
			},
		},
		{
			// Duplicate payment
			setup: func(order *models.Order) error {
				return order.PutMessage(&npb.OrderMessage{
					Signature: []byte("abc"),
					Message: mustBuildAny(&pb.PaymentSent{
						TransactionID: "xyz",
					}),
					MessageType: npb.OrderMessage_PAYMENT_SENT,
				})
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
			expectedError: nil,
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
			event, err := op.processPaymentSentMessage(tx, order, remotePeer, orderMsg)
			if err != test.expectedError {
				return fmt.Errorf("incorrect error returned. Expected %t, got %t", test.expectedError, err)
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

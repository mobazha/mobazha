package orders

import (
	"crypto/rand"
	"fmt"
	"reflect"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/events"
	"github.com/mobazha/mobazha3.0/internal/models"
	npb "github.com/mobazha/mobazha3.0/internal/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/internal/orders/mbzpb"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestOrderProcessor_processPaymentFinalizeMessage(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	_, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	remotePeer, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}

	orderID := "1234"

	paymentFinalizedMsg := &pb.PaymentFinalized{}

	paymentFinalizedAny := &anypb.Any{}
	if err := paymentFinalizedAny.MarshalFrom(paymentFinalizedMsg); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_FINALIZED,
		Message:     paymentFinalizedAny,
	}

	tests := []struct {
		setup         func(order *models.Order) error
		expectedError error
		expectedEvent interface{}
	}{
		{
			// Normal case where order open exists.
			setup: func(order *models.Order) error {
				order.ID = models.OrderID(orderID)
				return nil
			},
			expectedError: nil,
			expectedEvent: &events.VendorFinalizedPayment{
				OrderID: orderID,
			},
		},
		{
			// DisputeAccept already exists.
			setup: func(order *models.Order) error {
				order.SerializedPaymentFinalized = []byte{0x00}
				return err
			},
			expectedError: ErrChangedMessage,
			expectedEvent: nil,
		},
		{
			// DisputeAccept already exists.
			setup: func(order *models.Order) error {
				order.SerializedDisputeClosed = []byte{0x00}
				return err
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
	}

	for i, test := range tests {
		order := &models.Order{}
		if err := test.setup(order); err != nil {
			t.Errorf("Test %d setup error: %s", i, err)
			continue
		}
		err := op.db.Update(func(tx database.Tx) error {
			event, err := op.processPaymentFinalizeMessage(tx, order, remotePeer, orderMsg)
			if err != test.expectedError {
				return fmt.Errorf("incorrect error returned. Expected %t, got %t", test.expectedError, err)
			}
			if !reflect.DeepEqual(event, test.expectedEvent) {
				fmt.Println(event)
				fmt.Println(test.expectedEvent)
				return fmt.Errorf("incorrect event returned")
			}
			return nil
		})
		if err != nil {
			t.Errorf("Error executing db update in test %d: %s", i, err)
		}
	}
}

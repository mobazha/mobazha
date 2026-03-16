package orders

import (
	"crypto/rand"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/database"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

func mustBuildAny(msg protoreflect.ProtoMessage) *anypb.Any {
	a := &anypb.Any{}
	if err := a.MarshalFrom(msg); err != nil {
		panic(err)
	}
	return a
}

func TestOrderProcessor_processCancelMessage(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	_, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubkeyBytes, err := crypto.MarshalPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	remotePeer, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}

	orderID := "1234"

	cancelMsg := &pb.OrderCancel{}

	cancelAny := &anypb.Any{}
	if err := cancelAny.MarshalFrom(cancelMsg); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_CANCEL,
		Message:     cancelAny,
	}

	var (
		buyerPeerID    = remotePeer.String()
		buyerHandle    = "abc"
		smallImageHash = "aaaa"
		tinyImageHash  = "bbbb"
	)
	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			{
				Listing: &pb.Listing{
					Item: &pb.Listing_Item{
						Images: []*pb.Image{
							{
								Small: smallImageHash,
								Tiny:  tinyImageHash,
							},
						},
					},
				},
			},
		},
		BuyerID: &pb.ID{
			PeerID: buyerPeerID,
			Handle: buyerHandle,
			Name:   buyerHandle,
			Pubkeys: &pb.ID_Pubkeys{
				Identity: pubkeyBytes,
			},
		},
	}

	paymentSent := &pb.PaymentSent{
		Coin: iwallet.CtMock.String(),
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

				err := order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})
				if err != nil {
					return err
				}
				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(paymentSent),
					MessageType: npb.OrderMessage_PAYMENT_SENT,
				})
			},
			expectedError: nil,
			expectedEvent: &events.OrderCancel{
				OrderID: orderID,
				Thumbnail: events.Thumbnail{
					Tiny:  tinyImageHash,
					Small: smallImageHash,
				},
				BuyerName: buyerHandle,
				BuyerID:   buyerPeerID,
			},
		},
		{
			// Order decline already exists.
			setup: func(order *models.Order) error {
				order.SerializedOrderDecline = []byte{0x00}
				return nil
			},
			expectedError: ErrMessageParked,
			expectedEvent: nil,
		},
		{
			// Order confirmation already exists — cancel must be rejected.
			setup: func(order *models.Order) error {
				order.SerializedOrderDecline = nil
				order.SerializedOrderConfirmation = []byte{0x00}
				return nil
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
		{
			// Duplicate order cancel.
			setup: func(order *models.Order) error {
				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(cancelMsg),
					MessageType: npb.OrderMessage_ORDER_CANCEL,
				})
			},
			expectedError: nil,
			expectedEvent: nil,
		},
		{
			// Out of order.
			setup: func(order *models.Order) error {
				order.SerializedOrderOpen = nil
				return nil
			},
			expectedError: ErrMessageParked,
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
			event, err := op.processOrderCancelMessage(tx, order, orderMsg)
			if !errors.Is(err, test.expectedError) {
				return fmt.Errorf("incorrect error returned. Expected %v, got %v", test.expectedError, err)
			}
			if !reflect.DeepEqual(event, test.expectedEvent) {
				return fmt.Errorf("incorrect event returned")
			}
			return nil
		})
		if err != nil {
			t.Errorf("Error executing db update in test %d: %s", i, err)
		}
	}
}

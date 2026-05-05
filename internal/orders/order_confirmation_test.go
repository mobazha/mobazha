package orders

import (
	"crypto/rand"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestOrderProcessor_processOrderConfirmationMessage(t *testing.T) {
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

	confirmMsg := &pb.OrderConfirmation{}

	confirmationAny := &anypb.Any{}
	if err := confirmationAny.MarshalFrom(confirmMsg); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_CONFIRMATION,
		Message:     confirmationAny,
	}

	var (
		vendorPeerID   = remotePeer.String()
		vendorHandle   = "abc"
		smallImageHash = "aaaa"
		tinyImageHash  = "bbbb"
	)
	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			{
				Listing: &pb.Listing{
					VendorID: &pb.ID{
						PeerID: vendorPeerID,
						Handle: vendorHandle,
						Name:   vendorHandle,
						Pubkeys: &pb.ID_Pubkeys{
							Identity: pubkeyBytes,
						},
					},
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
	}

	paymentSent := &pb.PaymentSent{
		Coin: iwallet.CtMock.String(),
	}

	tests := []struct {
		name          string
		setup         func(order *models.Order) error
		expectedError error
		expectedEvent interface{}
	}{
		{
			name: "Normal case with verified payment",
			setup: func(order *models.Order) error {
				order.ID = models.OrderID(orderID)
				err := order.PutMessage(&npb.OrderMessage{
					Signature: []byte("abc"),
					Message:   mustBuildAny(orderOpen),
				})
				if err != nil {
					return err
				}
				if err := order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(paymentSent),
					MessageType: npb.OrderMessage_PAYMENT_SENT,
				}); err != nil {
					return err
				}
				order.MarkPaymentVerified()
				return nil
			},
			expectedError: nil,
			expectedEvent: &events.OrderConfirmation{
				OrderID: orderID,
				Thumbnail: events.Thumbnail{
					Tiny:  tinyImageHash,
					Small: smallImageHash,
				},
				VendorName: vendorHandle,
				VendorID:   vendorPeerID,
			},
		},
		{
			name: "Park when payment not yet verified",
			setup: func(order *models.Order) error {
				order.ID = models.OrderID(orderID)
				err := order.PutMessage(&npb.OrderMessage{
					Signature: []byte("abc"),
					Message:   mustBuildAny(orderOpen),
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
			expectedError: ErrMessageParked,
			expectedEvent: nil,
		},
		{
			name: "Order decline already exists",
			setup: func(order *models.Order) error {
				order.SerializedOrderDecline = []byte{0x00}
				return nil
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
		{
			name: "Order cancel already exists",
			setup: func(order *models.Order) error {
				order.SerializedOrderDecline = nil
				order.SerializedOrderCancel = []byte{0x00}
				return nil
			},
			expectedError: ErrMessageParked,
			expectedEvent: nil,
		},
		{
			name: "Duplicate order confirmation",
			setup: func(order *models.Order) error {
				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(confirmMsg),
					MessageType: npb.OrderMessage_ORDER_CONFIRMATION,
				})
			},
			expectedError: nil,
			expectedEvent: nil,
		},
		{
			name: "Out of order - no order open",
			setup: func(order *models.Order) error {
				order.SerializedOrderOpen = nil
				return nil
			},
			expectedError: ErrMessageParked,
			expectedEvent: nil,
		},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			order := &models.Order{}
			if err := test.setup(order); err != nil {
				t.Fatalf("Test %d setup error: %s", i, err)
			}
			err := op.db.Update(func(tx database.Tx) error {
				event, err := op.processOrderConfirmationMessage(tx, order, orderMsg)
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
		})
	}
}

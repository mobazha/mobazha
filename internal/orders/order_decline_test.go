package orders

import (
	"crypto/rand"
	"fmt"
	"reflect"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestOrderProcessor_processOrderDeclineMessage(t *testing.T) {
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
	orderID := "1234"

	orderDecline := &pb.OrderDecline{
		Type:   pb.OrderDecline_VALIDATION_ERROR,
		Reason: "Test",
	}

	declineAny := &anypb.Any{}
	if err := declineAny.MarshalFrom(orderDecline); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_DECLINE,
		Message:     declineAny,
	}

	var (
		vendorPeerID   = "xyz"
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
		setup         func(order *models.Order) error
		expectedError error
		expectedEvent interface{}
	}{
		{
			// Normal case where order open exists.
			setup: func(order *models.Order) error {
				order.ID = "1234"
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
			expectedError: nil,
			expectedEvent: &events.OrderDeclined{
				OrderID: "1234",
				Thumbnail: events.Thumbnail{
					Tiny:  tinyImageHash,
					Small: smallImageHash,
				},
				VendorHandle: vendorHandle,
				VendorID:     vendorPeerID,
			},
		},
		{
			// Order confirmation already exists.
			setup: func(order *models.Order) error {
				order.SerializedOrderDecline = nil
				order.SerializedOrderConfirmation = []byte{0x00}
				return nil
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
		{
			// Order cancel already exists.
			setup: func(order *models.Order) error {
				order.SerializedOrderDecline = nil
				order.SerializedOrderCancel = []byte{0x00}
				return nil
			},
			expectedError: nil,
			expectedEvent: nil,
		},
		{
			// Duplicate order decline.
			setup: func(order *models.Order) error {
				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderDecline),
					MessageType: npb.OrderMessage_ORDER_DECLINE,
				})
			},
			expectedError: nil,
			expectedEvent: nil,
		},
		{
			// Duplicate but different.
			setup: func(order *models.Order) error {
				msg2 := *orderDecline
				msg2.Type = pb.OrderDecline_USER_DECLINE
				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(&msg2),
					MessageType: npb.OrderMessage_ORDER_DECLINE,
				})
			},
			expectedError: ErrChangedMessage,
			expectedEvent: nil,
		},
		{
			// Out of order.
			setup: func(order *models.Order) error {
				order.SerializedOrderOpen = nil
				return nil
			},
			expectedError: nil,
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
			event, err := op.processOrderDeclineMessage(tx, order, orderMsg)
			if err != test.expectedError {
				return fmt.Errorf("incorrect error returned. Expected %t, got %t", test.expectedError, err)
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

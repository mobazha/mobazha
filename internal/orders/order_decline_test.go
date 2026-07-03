package orders

import (
	"crypto/rand"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"google.golang.org/protobuf/proto"
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
			name: "funded order decline",
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
				VendorName: vendorHandle,
				VendorID:   vendorPeerID,
			},
		},
		{
			name: "unfunded order decline (AWAITING_PAYMENT, no PaymentSent)",
			setup: func(order *models.Order) error {
				order.ID = "1234"
				order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
				return order.PutMessage(&npb.OrderMessage{
					Signature: []byte("abc"),
					Message:   mustBuildAny(orderOpen),
				})
			},
			expectedError: nil,
			expectedEvent: &events.OrderDeclined{
				OrderID: "1234",
				Thumbnail: events.Thumbnail{
					Tiny:  tinyImageHash,
					Small: smallImageHash,
				},
				VendorName: vendorHandle,
				VendorID:   vendorPeerID,
			},
		},
		{
			name: "order confirmation already exists",
			setup: func(order *models.Order) error {
				order.SerializedOrderDecline = nil
				order.SerializedOrderConfirmation = []byte{0x00}
				return nil
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
		{
			name: "order cancel already exists",
			setup: func(order *models.Order) error {
				order.SerializedOrderDecline = nil
				order.SerializedOrderCancel = []byte{0x00}
				return nil
			},
			expectedError: ErrMessageParked,
			expectedEvent: nil,
		},
		{
			name: "duplicate order decline",
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
			name: "duplicate but different",
			setup: func(order *models.Order) error {
				msg2 := proto.Clone(orderDecline).(*pb.OrderDecline)
				msg2.Type = pb.OrderDecline_USER_DECLINE
				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(msg2),
					MessageType: npb.OrderMessage_ORDER_DECLINE,
				})
			},
			expectedError: ErrChangedMessage,
			expectedEvent: nil,
		},
		{
			name: "out of order (no OrderOpen)",
			setup: func(order *models.Order) error {
				order.SerializedOrderOpen = nil
				return nil
			},
			expectedError: ErrMessageParked,
			expectedEvent: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			order := &models.Order{}
			if err := test.setup(order); err != nil {
				t.Fatalf("setup error: %s", err)
			}
			err := op.db.Update(func(tx database.Tx) error {
				event, err := op.processOrderDeclineMessage(tx, order, orderMsg)
				var errMatch bool
				if test.expectedError == nil {
					errMatch = err == nil
				} else {
					errMatch = errors.Is(err, test.expectedError)
				}
				if !errMatch {
					return fmt.Errorf("incorrect error returned. Expected %v, got %v", test.expectedError, err)
				}
				if !reflect.DeepEqual(event, test.expectedEvent) {
					return fmt.Errorf("incorrect event returned. Expected %v, got %v", test.expectedEvent, event)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Error executing db update: %s", err)
			}
		})
	}
}

func TestOrderProcessor_ReplayParkedMessages_OrderDecline(t *testing.T) {
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

	const orderID = "replay-parked-decline"
	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			{
				Listing: &pb.Listing{
					VendorID: &pb.ID{
						PeerID: op.identity.String(),
						Pubkeys: &pb.ID_Pubkeys{
							Identity: pubkeyBytes,
						},
					},
					Item: &pb.Listing_Item{
						Images: []*pb.Image{{Small: "small", Tiny: "tiny"}},
					},
				},
			},
		},
	}

	declineMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_DECLINE,
		Message: mustBuildAny(&pb.OrderDecline{
			Type:   pb.OrderDecline_VALIDATION_ERROR,
			Reason: "test replay",
		}),
	}
	if err := utils.SignOrderMessage(declineMsg, op.signer); err != nil {
		t.Fatal(err)
	}

	err = op.db.Update(func(tx database.Tx) error {
		order := &models.Order{
			ID:     models.OrderID(orderID),
			MyRole: string(models.RoleBuyer),
		}
		order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
		if err := order.PutMessage(&npb.OrderMessage{
			Signature:   []byte("order-open-sig"),
			Message:     mustBuildAny(orderOpen),
			MessageType: npb.OrderMessage_ORDER_OPEN,
		}); err != nil {
			return err
		}
		if err := order.ParkMessage(declineMsg); err != nil {
			return err
		}
		return tx.Save(order)
	})
	if err != nil {
		t.Fatal(err)
	}

	err = op.db.Update(func(tx database.Tx) error {
		return op.ReplayParkedMessages(tx, orderID)
	})
	if err != nil {
		t.Fatal(err)
	}

	err = op.db.View(func(tx database.Tx) error {
		var got models.Order
		if err := tx.Read().Where("id = ?", orderID).First(&got).Error; err != nil {
			return err
		}
		if got.SerializedOrderDecline == nil {
			return fmt.Errorf("expected order decline to be processed")
		}
		parked, err := got.GetParkedMessages()
		if err != nil {
			return err
		}
		if len(parked.Messages) != 0 {
			return fmt.Errorf("expected parked messages to be cleared, got %d", len(parked.Messages))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

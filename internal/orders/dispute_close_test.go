package orders

import (
	"crypto/rand"
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

func TestOrderProcessor_processDisputeCloseMessage(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	wal, err := op.multiwallet.WalletForCurrencyCode("MCK")
	if err != nil {
		t.Fatal(err)
	}
	vendorAddress, err := wal.CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}

	_, localPub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	localPubkeyBytes, err := crypto.MarshalPublicKey(localPub)
	if err != nil {
		t.Fatal(err)
	}
	localPeer, err := peer.IDFromPublicKey(localPub)
	if err != nil {
		t.Fatal(err)
	}

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

	_, modPub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	modPeer, err := peer.IDFromPublicKey(modPub)
	if err != nil {
		t.Fatal(err)
	}

	orderID := "1234"

	disputeCloseMsg := &pb.DisputeClose{
		Verdict: "Resolve dispute",
		ReleaseInfo: &pb.DisputeClose_ModeratedEscrowRelease{
			EscrowSignatures: []*pb.Signature{
				{
					From:      []byte{0x00},
					Signature: []byte{0x01},
					Index:     0,
				},
			},
			Outpoints: []*pb.Outpoint{{
				FromID: []byte{0x00},
				Value:  "18350",
			}},
			BuyerAddress:     "123",
			BuyerAmount:      "9000",
			VendorAddress:    vendorAddress.String(),
			VendorAmount:     "9000",
			ModeratorAddress: "abc",
			ModeratorAmount:  "300",
			TransactionFee:   "50",
		},
	}

	disputeCloseAny := &anypb.Any{}
	if err := disputeCloseAny.MarshalFrom(disputeCloseMsg); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_DISPUTE_CLOSE,
		Message:     disputeCloseAny,
	}

	var (
		buyerPeerID    = remotePeer.String()
		buyerHandle    = "abc"
		localHandle    = "xyz"
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
					VendorID: &pb.ID{
						PeerID: localPeer.String(),
						Handle: localHandle,
						Pubkeys: &pb.ID_Pubkeys{
							Identity: localPubkeyBytes,
						},
					},
				},
			},
		},
		BuyerID: &pb.ID{
			PeerID: buyerPeerID,
			Handle: buyerHandle,
			Pubkeys: &pb.ID_Pubkeys{
				Identity: pubkeyBytes,
			},
		},
		Payment: &pb.OrderOpen_Payment{
			Coin:      iwallet.CtMock,
			Moderator: "12D3KooWHnpVyu9XDeFoAVayqr9hvc9xPqSSHtCSFLEkKgcz5Wro",
			Method:    pb.OrderOpen_Payment_MODERATED,
		},
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
				order.SetRole(models.RoleVendor)

				err := order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})
				return err
			},
			expectedError: nil,
			expectedEvent: &events.DisputeClose{
				OrderID: orderID,
				Thumbnail: events.Thumbnail{
					Tiny:  tinyImageHash,
					Small: smallImageHash,
				},
				OtherPartyID:     buyerPeerID,
				OtherPartyHandle: buyerHandle,
				Buyer:            orderOpen.BuyerID.PeerID,
			},
		},
		{
			// DisputeClose already exists.
			setup: func(order *models.Order) error {
				err := order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})

				order.SerializedDisputeClosed = []byte{0x00}
				return err
			},
			expectedError: ErrChangedMessage,
			expectedEvent: nil,
		},
		{
			// DisputeClose already exists.
			setup: func(order *models.Order) error {
				err := order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})

				order.SerializedOrderComplete = []byte{0x00}
				return err
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
		{
			// DisputeClose already exists.
			setup: func(order *models.Order) error {
				err := order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})

				order.SerializedPaymentFinalized = []byte{0x00}
				return err
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
		{
			// DisputeClose already exists.
			setup: func(order *models.Order) error {
				err := order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})

				order.SerializedOrderReject = []byte{0x00}
				return err
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
		{
			// DisputeClose already exists.
			setup: func(order *models.Order) error {
				err := order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})

				order.SerializedOrderCancel = []byte{0x00}
				return err
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
		{
			// Duplicate dispute close.
			setup: func(order *models.Order) error {
				order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})

				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     disputeCloseAny,
					MessageType: npb.OrderMessage_DISPUTE_CLOSE,
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
			event, err := op.processDisputeCloseMessage(tx, order, modPeer, orderMsg)
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

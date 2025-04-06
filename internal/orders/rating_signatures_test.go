package orders

import (
	"crypto/rand"
	"fmt"
	"reflect"
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/events"
	"github.com/mobazha/mobazha3.0/internal/models"
	iwallet "github.com/mobazha/mobazha3.0/internal/multiwallet/wallet-interface"
	npb "github.com/mobazha/mobazha3.0/internal/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/internal/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func Test_processRatingSignaturesMessage(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
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

	ratingKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
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
					Slug: "cool tshirt",
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
		Payment: &pb.OrderOpen_Payment{
			Coin: iwallet.CtMock,
		},
		RatingKeys: [][]byte{
			ratingKey.PubKey().SerializeCompressed(),
		},
	}

	h, err := utils.HashListing(orderOpen.Listings[0])
	if err != nil {
		t.Fatal(err)
	}

	orderOpen.Items = append(orderOpen.Items, &pb.OrderOpen_Item{
		ListingHash: h.B58String(),
	})

	sig := &pb.RatingSignature{
		Slug:      orderOpen.Listings[0].Listing.Slug,
		RatingKey: orderOpen.RatingKeys[0],
	}

	ser, err := proto.Marshal(sig)
	if err != nil {
		t.Fatal(err)
	}

	sigBytes, err := priv.Sign(ser)
	if err != nil {
		t.Fatal(err)
	}
	sig.VendorSignature = sigBytes

	rsMsg := &pb.RatingSignatures{
		Sigs: []*pb.RatingSignature{sig},
	}

	rsAny := &anypb.Any{}
	if err := rsAny.MarshalFrom(rsMsg); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_RATING_SIGNATURES,
		Message:     rsAny,
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
				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})
			},
			expectedError: nil,
			expectedEvent: &events.RatingSignaturesReceived{
				OrderID: "1234",
			},
		},
		{
			// Duplicate message.
			setup: func(order *models.Order) error {
				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(rsMsg),
					MessageType: npb.OrderMessage_RATING_SIGNATURES,
				})
			},
			expectedError: nil,
			expectedEvent: nil,
		},
		{
			// Duplicate but different.
			setup: func(order *models.Order) error {
				msg2 := *rsMsg
				msg2.Sigs = nil
				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(&msg2),
					MessageType: npb.OrderMessage_RATING_SIGNATURES,
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
			event, err := op.processRatingSignaturesMessage(tx, order, remotePeer, orderMsg)
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

func TestOrderProcessor_sendRatingSignatures(t *testing.T) {
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
	buyerID, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	orderID := "1234"

	ratingKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	var (
		vendorHandle   = "abc"
		smallImageHash = "aaaa"
		tinyImageHash  = "bbbb"
	)

	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			{
				Listing: &pb.Listing{
					Slug: "cool tshirt",
					VendorID: &pb.ID{
						PeerID: op.identity.String(),
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
		BuyerID: &pb.ID{
			PeerID: buyerID.String(),
		},
		Payment: &pb.OrderOpen_Payment{
			Coin: iwallet.CtMock,
		},
		RatingKeys: [][]byte{
			ratingKey.PubKey().SerializeCompressed(),
		},
	}

	h, err := utils.HashListing(orderOpen.Listings[0])
	if err != nil {
		t.Fatal(err)
	}

	orderOpen.Items = append(orderOpen.Items, &pb.OrderOpen_Item{
		ListingHash: h.B58String(),
	})

	order := &models.Order{
		ID: models.OrderID(orderID),
	}

	if err := order.PutMessage(utils.MustWrapOrderMessage(orderOpen)); err != nil {
		t.Fatal(err)
	}

	err = op.db.Update(func(tx database.Tx) error {
		return op.sendRatingSignatures(tx, order, orderOpen)
	})
	if err != nil {
		t.Fatal(err)
	}

	var messages []models.OutgoingMessage
	err = op.db.View(func(tx database.Tx) error {
		return tx.Read().Find(&messages).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 1 {
		t.Errorf("Expected 1 saved outgoing message. Got %d", len(messages))
	}

	rs, err := order.RatingSignaturesMessage()
	if err != nil {
		t.Fatal(err)
	}

	cpy := proto.Clone(rs.Sigs[0])
	cpy.(*pb.RatingSignature).VendorSignature = nil

	ser, err := proto.Marshal(cpy)
	if err != nil {
		t.Fatal(err)
	}

	valid, err := op.identityPrivateKey.GetPublic().Verify(ser, rs.Sigs[0].VendorSignature)
	if err != nil {
		t.Fatal(err)
	}

	if !valid {
		t.Error("invalid signature")
	}
}

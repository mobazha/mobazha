package orders

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestOrderProcessor_processOrderCompleteMessage(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	vendorPriv, vendorPub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubkeyBytes, err := crypto.MarshalPublicKey(vendorPub)
	if err != nil {
		t.Fatal(err)
	}
	vendor, err := peer.IDFromPublicKey(vendorPub)
	if err != nil {
		t.Fatal(err)
	}

	buyerPriv, buyerPub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	buyerPubkeyBytes, err := crypto.MarshalPublicKey(buyerPub)
	if err != nil {
		t.Fatal(err)
	}
	buyer, err := peer.IDFromPublicKey(buyerPub)
	if err != nil {
		t.Fatal(err)
	}
	op.identity = vendor

	orderID := "1234"

	chaincode := make([]byte, 32)
	rand.Read(chaincode)

	ratingMaster, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	ratingKeys, err := utils.GenerateRatingPrivateKeys(ratingMaster, 1, chaincode)
	if err != nil {
		t.Fatal(err)
	}

	vendorSig := &pb.RatingSignature{
		Slug:      "abc",
		RatingKey: ratingKeys[0].PubKey().SerializeCompressed(),
	}

	ser, err := proto.Marshal(vendorSig)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := vendorPriv.Sign(ser)
	if err != nil {
		t.Fatal(err)
	}
	vendorSig.VendorSignature = sig

	var (
		vendorPeerID   = vendor.String()
		vendorHandle   = "abc"
		smallImageHash = "aaaa"
		tinyImageHash  = "bbbb"
	)

	hashedRatingKey := sha256.Sum256(ratingKeys[0].PubKey().SerializeCompressed())
	buyerSig, err := buyerPriv.Sign(hashedRatingKey[:])
	if err != nil {
		t.Fatal(err)
	}

	orderComplete := &pb.OrderComplete{
		Ratings: []*pb.Rating{
			{
				VendorSig: vendorSig,
				VendorID: &pb.ID{
					PeerID: vendorPeerID,
					Handle: vendorHandle,
					Name:   vendorHandle,
					Pubkeys: &pb.ID_Pubkeys{
						Identity: pubkeyBytes,
					},
				},
				Timestamp: timestamppb.Now(),
				BuyerID: &pb.ID{
					PeerID: buyer.String(),
					Handle: "aaa",
					Name:   "aaa",
					Pubkeys: &pb.ID_Pubkeys{
						Identity: buyerPubkeyBytes,
					},
				},
				BuyerName: "Ernie",
				BuyerSig:  buyerSig,

				Overall: 5,
				Review:  "sucked",
			},
		},
	}
	ser, err = proto.Marshal(orderComplete.Ratings[0])
	if err != nil {
		t.Fatal(err)
	}
	hashed := sha256.Sum256(ser)
	ratingSig := ecdsa.Sign(ratingKeys[0], hashed[:])

	orderComplete.Ratings[0].RatingSignature = ratingSig.Serialize()

	completeAny := &anypb.Any{}
	if err := completeAny.MarshalFrom(orderComplete); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_COMPLETE,
		Message:     completeAny,
	}

	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			{
				Listing: &pb.Listing{
					Slug: "abc",
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
		RatingKeys: [][]byte{
			ratingKeys[0].PubKey().SerializeCompressed(),
		},
		Items: []*pb.OrderOpen_Item{
			{
				ListingHash: "1234",
			},
		},
		BuyerID: &pb.ID{
			PeerID: buyer.String(),
			Handle: "aaa",
			Name:   "aaa",
			Pubkeys: &pb.ID_Pubkeys{
				Identity: buyerPubkeyBytes,
			},
		},
	}

	paymentSent := &pb.PaymentSent{
		Coin:      iwallet.CtMock.String(),
		Chaincode: hex.EncodeToString(chaincode),
	}

	shipment := &pb.OrderShipment{}

	tests := []struct {
		setup         func(order *models.Order) error
		expectedError error
		expectedEvent interface{}
	}{
		{
			// Normal case where order open and shipment exist.
			setup: func(order *models.Order) error {
				order.SetRole(models.RoleVendor)
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
				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(shipment),
					MessageType: npb.OrderMessage_ORDER_SHIPMENT,
				})
			},
			expectedError: nil,
			expectedEvent: &events.OrderCompletion{
				OrderID: orderID,
				Thumbnail: events.Thumbnail{
					Tiny:  tinyImageHash,
					Small: smallImageHash,
				},
				BuyerName: "aaa",
				BuyerID:   buyer.String(),
			},
		},
		{
			// Order cancel already exists.
			setup: func(order *models.Order) error {
				order.SerializedOrderCancel = []byte{0x00}
				return nil
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
		{
			// Duplicate order complete.
			setup: func(order *models.Order) error {
				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     completeAny,
					MessageType: npb.OrderMessage_ORDER_COMPLETE,
				})
			},
			expectedError: nil,
			expectedEvent: nil,
		},
		{
			// Duplicate but different.
			setup: func(order *models.Order) error {
				a := proto.Clone(orderComplete)
				a.(*pb.OrderComplete).Ratings[0].Review = "fasdfad"
				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(a),
					MessageType: npb.OrderMessage_ORDER_COMPLETE,
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
			expectedError: ErrMessageParked,
			expectedEvent: nil,
		},
		{
			// Missing shipment — should park the message.
			setup: func(order *models.Order) error {
				order.SetRole(models.RoleVendor)
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
	}

	for i, test := range tests {
		order := &models.Order{}
		if err := test.setup(order); err != nil {
			t.Errorf("Test %d setup error: %s", i, err)
			continue
		}
		err := op.db.Update(func(tx database.Tx) error {
			event, err := op.processOrderCompleteMessage(tx, order, orderMsg)
			if !errors.Is(err, test.expectedError) {
				return fmt.Errorf("incorrect error returned. Expected %v, got %v", test.expectedError, err)
			}
			if !reflect.DeepEqual(event, test.expectedEvent) {
				fmt.Println(event, test.expectedEvent)
				return fmt.Errorf("incorrect event returned")
			}
			return nil
		})
		if err != nil {
			t.Errorf("Error executing db update in test %d: %s", i, err)
		}
	}

	err = op.db.View(func(tx database.Tx) error {
		index, err := tx.GetRatingIndex()
		if err != nil {
			return err
		}
		if len(index) != 1 {
			return fmt.Errorf("expected index len 1 got %d", len(index))
		}
		if index[0].Slug != "abc" {
			return fmt.Errorf("expected slug abc got %s", index[0].Slug)
		}
		if index[0].Average != 5 {
			return fmt.Errorf("expected average 5 got %f", index[0].Average)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOrderComplete_WithoutRatings(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	vendorPriv, vendorPub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_ = vendorPriv
	pubkeyBytes, err := crypto.MarshalPublicKey(vendorPub)
	if err != nil {
		t.Fatal(err)
	}
	vendor, err := peer.IDFromPublicKey(vendorPub)
	if err != nil {
		t.Fatal(err)
	}

	_, buyerPub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	buyerPubkeyBytes, err := crypto.MarshalPublicKey(buyerPub)
	if err != nil {
		t.Fatal(err)
	}
	buyer, err := peer.IDFromPublicKey(buyerPub)
	if err != nil {
		t.Fatal(err)
	}
	op.identity = vendor

	orderID := "complete-no-rating"

	chaincode := make([]byte, 32)
	rand.Read(chaincode)

	vendorPeerID := vendor.String()
	tinyImageHash := "tiny1"
	smallImageHash := "small1"

	orderComplete := &pb.OrderComplete{
		Timestamp: timestamppb.Now(),
	}

	completeAny := &anypb.Any{}
	if err := completeAny.MarshalFrom(orderComplete); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_COMPLETE,
		Message:     completeAny,
	}

	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			{
				Listing: &pb.Listing{
					Slug: "abc",
					VendorID: &pb.ID{
						PeerID: vendorPeerID,
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
		Items: []*pb.OrderOpen_Item{
			{ListingHash: "1234"},
		},
		BuyerID: &pb.ID{
			PeerID: buyer.String(),
			Pubkeys: &pb.ID_Pubkeys{
				Identity: buyerPubkeyBytes,
			},
		},
	}

	paymentSent := &pb.PaymentSent{
		Coin:      iwallet.CtMock.String(),
		Chaincode: hex.EncodeToString(chaincode),
	}

	shipment := &pb.OrderShipment{}

	order := &models.Order{}
	order.SetRole(models.RoleVendor)
	order.ID = models.OrderID(orderID)
	if err := order.PutMessage(&npb.OrderMessage{
		Signature: []byte("abc"),
		Message:   mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(&npb.OrderMessage{
		Signature:   []byte("abc"),
		Message:     mustBuildAny(paymentSent),
		MessageType: npb.OrderMessage_PAYMENT_SENT,
	}); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(&npb.OrderMessage{
		Signature:   []byte("abc"),
		Message:     mustBuildAny(shipment),
		MessageType: npb.OrderMessage_ORDER_SHIPMENT,
	}); err != nil {
		t.Fatal(err)
	}

	err = op.db.Update(func(tx database.Tx) error {
		event, err := op.processOrderCompleteMessage(tx, order, orderMsg)
		if err != nil {
			return fmt.Errorf("unexpected error: %s", err)
		}
		completion, ok := event.(*events.OrderCompletion)
		if !ok {
			return fmt.Errorf("expected *events.OrderCompletion, got %T", event)
		}
		if completion.OrderID != orderID {
			return fmt.Errorf("expected orderID %s, got %s", orderID, completion.OrderID)
		}
		if order.Open {
			return fmt.Errorf("order should be closed")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOrderComplete_RatingSupplement(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	vendorPriv, vendorPub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubkeyBytes, err := crypto.MarshalPublicKey(vendorPub)
	if err != nil {
		t.Fatal(err)
	}
	vendor, err := peer.IDFromPublicKey(vendorPub)
	if err != nil {
		t.Fatal(err)
	}

	buyerPriv, buyerPub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	buyerPubkeyBytes, err := crypto.MarshalPublicKey(buyerPub)
	if err != nil {
		t.Fatal(err)
	}
	buyer, err := peer.IDFromPublicKey(buyerPub)
	if err != nil {
		t.Fatal(err)
	}
	op.identity = vendor

	orderID := "supplement-rating"

	chaincode := make([]byte, 32)
	rand.Read(chaincode)

	ratingMaster, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	ratingKeys, err := utils.GenerateRatingPrivateKeys(ratingMaster, 1, chaincode)
	if err != nil {
		t.Fatal(err)
	}

	vendorSig := &pb.RatingSignature{
		Slug:      "abc",
		RatingKey: ratingKeys[0].PubKey().SerializeCompressed(),
	}
	ser, err := proto.Marshal(vendorSig)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := vendorPriv.Sign(ser)
	if err != nil {
		t.Fatal(err)
	}
	vendorSig.VendorSignature = sig

	vendorPeerID := vendor.String()

	hashedRatingKey := sha256.Sum256(ratingKeys[0].PubKey().SerializeCompressed())
	buyerSig, err := buyerPriv.Sign(hashedRatingKey[:])
	if err != nil {
		t.Fatal(err)
	}

	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			{
				Listing: &pb.Listing{
					Slug: "abc",
					VendorID: &pb.ID{
						PeerID: vendorPeerID,
						Pubkeys: &pb.ID_Pubkeys{
							Identity: pubkeyBytes,
						},
					},
					Item: &pb.Listing_Item{
						Images: []*pb.Image{
							{Small: "s", Tiny: "t"},
						},
					},
				},
			},
		},
		RatingKeys: [][]byte{
			ratingKeys[0].PubKey().SerializeCompressed(),
		},
		Items: []*pb.OrderOpen_Item{
			{ListingHash: "1234"},
		},
		BuyerID: &pb.ID{
			PeerID: buyer.String(),
			Pubkeys: &pb.ID_Pubkeys{
				Identity: buyerPubkeyBytes,
			},
		},
	}

	paymentSent := &pb.PaymentSent{
		Coin:      iwallet.CtMock.String(),
		Chaincode: hex.EncodeToString(chaincode),
	}

	shipment := &pb.OrderShipment{}

	// Step 1: Complete without ratings
	noRatingComplete := &pb.OrderComplete{Timestamp: timestamppb.Now()}
	completeAny := &anypb.Any{}
	if err := completeAny.MarshalFrom(noRatingComplete); err != nil {
		t.Fatal(err)
	}

	order := &models.Order{}
	order.SetRole(models.RoleVendor)
	order.ID = models.OrderID(orderID)
	if err := order.PutMessage(&npb.OrderMessage{
		Signature: []byte("abc"),
		Message:   mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(&npb.OrderMessage{
		Signature:   []byte("abc"),
		Message:     mustBuildAny(paymentSent),
		MessageType: npb.OrderMessage_PAYMENT_SENT,
	}); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(&npb.OrderMessage{
		Signature:   []byte("abc"),
		Message:     mustBuildAny(shipment),
		MessageType: npb.OrderMessage_ORDER_SHIPMENT,
	}); err != nil {
		t.Fatal(err)
	}

	err = op.db.Update(func(tx database.Tx) error {
		event, err := op.processOrderCompleteMessage(tx, order, &npb.OrderMessage{
			OrderID:     orderID,
			MessageType: npb.OrderMessage_ORDER_COMPLETE,
			Message:     completeAny,
		})
		if err != nil {
			return fmt.Errorf("step 1: unexpected error: %s", err)
		}
		if event == nil {
			return fmt.Errorf("step 1: expected OrderCompletion event")
		}
		if order.Open {
			return fmt.Errorf("step 1: order should be closed")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Step 2: Send rating supplement
	ratingPB := &pb.Rating{
		Timestamp: timestamppb.Now(),
		VendorSig: vendorSig,
		VendorID: &pb.ID{
			PeerID: vendorPeerID,
			Pubkeys: &pb.ID_Pubkeys{
				Identity: pubkeyBytes,
			},
		},
		BuyerID: &pb.ID{
			PeerID: buyer.String(),
			Pubkeys: &pb.ID_Pubkeys{
				Identity: buyerPubkeyBytes,
			},
		},
		BuyerSig: buyerSig,
		Overall:  4,
		Review:   "good stuff",
	}
	ser, err = proto.Marshal(ratingPB)
	if err != nil {
		t.Fatal(err)
	}
	hashed := sha256.Sum256(ser)
	ratingSig := ecdsa.Sign(ratingKeys[0], hashed[:])
	ratingPB.RatingSignature = ratingSig.Serialize()

	supplementComplete := &pb.OrderComplete{
		Timestamp: noRatingComplete.Timestamp,
		Ratings:   []*pb.Rating{ratingPB},
	}
	supplementAny := &anypb.Any{}
	if err := supplementAny.MarshalFrom(supplementComplete); err != nil {
		t.Fatal(err)
	}

	err = op.db.Update(func(tx database.Tx) error {
		event, err := op.processOrderCompleteMessage(tx, order, &npb.OrderMessage{
			OrderID:     orderID,
			MessageType: npb.OrderMessage_ORDER_COMPLETE,
			Message:     supplementAny,
		})
		if err != nil {
			return fmt.Errorf("step 2: unexpected error: %s", err)
		}
		rated, ok := event.(*events.OrderRated)
		if !ok || rated == nil {
			return fmt.Errorf("step 2: expected OrderRated event for vendor supplement, got %T", event)
		}
		if rated.OrderID != orderID {
			return fmt.Errorf("step 2: expected orderID %s, got %s", orderID, rated.OrderID)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify ratings were saved
	err = op.db.View(func(tx database.Tx) error {
		index, err := tx.GetRatingIndex()
		if err != nil {
			return err
		}
		if len(index) != 1 {
			return fmt.Errorf("expected 1 rating in index, got %d", len(index))
		}
		if index[0].Average != 4 {
			return fmt.Errorf("expected average 4, got %f", index[0].Average)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOrderComplete_SupplementRejectedWhenAlreadyRated(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	vendorPriv, vendorPub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubkeyBytes, err := crypto.MarshalPublicKey(vendorPub)
	if err != nil {
		t.Fatal(err)
	}
	vendor, err := peer.IDFromPublicKey(vendorPub)
	if err != nil {
		t.Fatal(err)
	}

	buyerPriv, buyerPub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	buyerPubkeyBytes, err := crypto.MarshalPublicKey(buyerPub)
	if err != nil {
		t.Fatal(err)
	}
	buyer, err := peer.IDFromPublicKey(buyerPub)
	if err != nil {
		t.Fatal(err)
	}
	op.identity = vendor

	orderID := "already-rated"

	chaincode := make([]byte, 32)
	rand.Read(chaincode)

	ratingMaster, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	ratingKeys, err := utils.GenerateRatingPrivateKeys(ratingMaster, 1, chaincode)
	if err != nil {
		t.Fatal(err)
	}

	vendorSig := &pb.RatingSignature{
		Slug:      "abc",
		RatingKey: ratingKeys[0].PubKey().SerializeCompressed(),
	}
	ser, err := proto.Marshal(vendorSig)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := vendorPriv.Sign(ser)
	if err != nil {
		t.Fatal(err)
	}
	vendorSig.VendorSignature = sig

	vendorPeerID := vendor.String()

	hashedRatingKey := sha256.Sum256(ratingKeys[0].PubKey().SerializeCompressed())
	buyerSig, err := buyerPriv.Sign(hashedRatingKey[:])
	if err != nil {
		t.Fatal(err)
	}

	ratingPB := &pb.Rating{
		Timestamp: timestamppb.Now(),
		VendorSig: vendorSig,
		VendorID:  &pb.ID{PeerID: vendorPeerID, Pubkeys: &pb.ID_Pubkeys{Identity: pubkeyBytes}},
		BuyerID:   &pb.ID{PeerID: buyer.String(), Pubkeys: &pb.ID_Pubkeys{Identity: buyerPubkeyBytes}},
		BuyerSig:  buyerSig,
		Overall:   5,
		Review:    "great",
	}
	ser, err = proto.Marshal(ratingPB)
	if err != nil {
		t.Fatal(err)
	}
	hashed := sha256.Sum256(ser)
	ratingSig := ecdsa.Sign(ratingKeys[0], hashed[:])
	ratingPB.RatingSignature = ratingSig.Serialize()

	// Simulate an order that was completed WITH ratings
	existingComplete := &pb.OrderComplete{
		Timestamp: timestamppb.Now(),
		Ratings:   []*pb.Rating{ratingPB},
	}

	order := &models.Order{}
	order.ID = models.OrderID(orderID)
	if err := order.PutMessage(&npb.OrderMessage{
		Signature:   []byte("abc"),
		Message:     mustBuildAny(existingComplete),
		MessageType: npb.OrderMessage_ORDER_COMPLETE,
	}); err != nil {
		t.Fatal(err)
	}

	// Try to send a different ORDER_COMPLETE with different ratings
	ratingPB2 := proto.Clone(ratingPB).(*pb.Rating)
	ratingPB2.Review = "changed review"
	newComplete := &pb.OrderComplete{
		Timestamp: existingComplete.Timestamp,
		Ratings:   []*pb.Rating{ratingPB2},
	}
	newAny := &anypb.Any{}
	if err := newAny.MarshalFrom(newComplete); err != nil {
		t.Fatal(err)
	}

	err = op.db.Update(func(tx database.Tx) error {
		_, err := op.processOrderCompleteMessage(tx, order, &npb.OrderMessage{
			OrderID:     orderID,
			MessageType: npb.OrderMessage_ORDER_COMPLETE,
			Message:     newAny,
		})
		if err != ErrChangedMessage {
			return fmt.Errorf("expected ErrChangedMessage, got %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

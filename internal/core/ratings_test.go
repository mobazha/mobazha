package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestOpenBazaarNode_Ratings(t *testing.T) {
	mockNet, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}
	defer mockNet.TearDown()

	rating, err := newTestRating()
	if err != nil {
		t.Fatal(err)
	}
	if err := utils.ValidateRating(rating); err != nil {
		t.Fatal(err)
	}

	var id cid.Cid
	err = mockNet.Nodes()[0].repo.DB().Update(func(tx database.Tx) error {
		if err := tx.SetRating(rating); err != nil {
			return err
		}

		m := protojson.MarshalOptions{Indent: "    "}
		out := m.Format(rating)

		id, err = mockNet.Nodes()[0].cid([]byte(out))
		if err != nil {
			return err
		}

		var index models.RatingIndex
		if err := index.AddRating(rating, id); err != nil {
			return err
		}
		return tx.SetRatingIndex(index)
	})
	if err != nil {
		t.Fatal(err)
	}

	ratings, err := mockNet.Nodes()[0].GetMyRatings()
	if err != nil {
		t.Fatal(err)
	}
	if len(ratings) != 1 {
		t.Errorf("Expected 1 rating, got %d", len(ratings))
	}
	if ratings[0].Count != 1 {
		t.Errorf("Expected 1 rating count, got %d", ratings[0].Count)
	}
	if ratings[0].Ratings[0] != id.String() {
		t.Errorf("Expected cid %s, got %s", id, ratings[0].Ratings[0])
	}

	done := make(chan struct{})
	mockNet.Nodes()[0].Publish(done)
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("timed out while publishing")
	}

	ratings, err = mockNet.Nodes()[1].GetRatings(context.Background(), mockNet.Nodes()[0].Identity(), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(ratings) != 1 {
		t.Errorf("Expected 1 rating, got %d", len(ratings))
	}
	if ratings[0].Count != 1 {
		t.Errorf("Expected 1 rating count, got %d", ratings[0].Count)
	}
	if ratings[0].Ratings[0] != id.String() {
		t.Errorf("Expected cid %s, got %s", id, ratings[0].Ratings[0])
	}

	rating2, err := mockNet.Nodes()[1].GetRating(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	if rating2.Review != rating.Review {
		t.Errorf("Expected review %s, got %s", rating.Review, rating2.Review)
	}
}

func newTestRating() (*pb.Rating, error) {
	vendorPrivkey, vendorPubkey, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	vendorPubkeyBytes, err := crypto.MarshalPublicKey(vendorPubkey)
	if err != nil {
		return nil, err
	}
	vendorRatingKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, err
	}
	vendorID, err := peer.IDFromPublicKey(vendorPubkey)
	if err != nil {
		return nil, err
	}
	idHash := sha256.Sum256([]byte(vendorID.String()))
	vendorIDSig := ecdsa.Sign(vendorRatingKey, idHash[:])

	buyerPrivkey, buyerPubkey, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	buyerPubkeyBytes, err := crypto.MarshalPublicKey(buyerPubkey)
	if err != nil {
		return nil, err
	}
	buyerRatingKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, err
	}
	buyerID, err := peer.IDFromPublicKey(buyerPubkey)
	if err != nil {
		return nil, err
	}
	idHash = sha256.Sum256(buyerRatingKey.PubKey().SerializeCompressed())
	buyerIDSig, err := buyerPrivkey.Sign(idHash[:])
	if err != nil {
		return nil, err
	}

	r := &pb.RatingSignature{
		Slug:      "slug",
		RatingKey: buyerRatingKey.PubKey().SerializeCompressed(),
	}

	ser, err := proto.Marshal(r)
	if err != nil {
		return nil, err
	}

	sig, err := vendorPrivkey.Sign(ser)
	if err != nil {
		return nil, err
	}
	r.VendorSignature = sig

	rating := &pb.Rating{
		Timestamp: timestamppb.Now(),

		VendorSig: r,
		VendorID: &pb.ID{
			PeerID: vendorID.String(),
			Handle: "@handle",
			Pubkeys: &pb.ID_Pubkeys{
				Identity: vendorPubkeyBytes,
				Escrow:   vendorRatingKey.PubKey().SerializeCompressed(),
			},
			Sig: vendorIDSig.Serialize(),
		},
		BuyerID: &pb.ID{
			PeerID: buyerID.String(),
			Handle: "@handle",
			Pubkeys: &pb.ID_Pubkeys{
				Identity: buyerPubkeyBytes,
				Escrow:   buyerRatingKey.PubKey().SerializeCompressed(),
			},
			Sig: vendorIDSig.Serialize(),
		},
		BuyerSig: buyerIDSig,

		Overall:         uint32(5),
		Quality:         uint32(4),
		CustomerService: uint32(3),
		Description:     uint32(2),
		DeliverySpeed:   uint32(1),
		Review:          "excellent",

		BuyerName: "Bob",
	}

	ser, err = proto.Marshal(rating)
	if err != nil {
		return nil, err
	}

	hashed := sha256.Sum256(ser)

	ratingSig := ecdsa.Sign(buyerRatingKey, hashed[:])
	rating.RatingSignature = ratingSig.Serialize()

	return rating, nil
}

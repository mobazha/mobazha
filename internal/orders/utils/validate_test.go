package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"strings"
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestValidateRating(t *testing.T) {
	buyerPriv, buyerPub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	buyerPubkeyBytes, err := crypto.MarshalPublicKey(buyerPub)
	if err != nil {
		t.Fatal(err)
	}

	vendorPriv, vendorPub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	vendorPubkeyBytes, err := crypto.MarshalPublicKey(vendorPub)
	if err != nil {
		t.Fatal(err)
	}

	buyerID, err := peer.IDFromPublicKey(buyerPub)
	if err != nil {
		t.Fatal(err)
	}

	vendorID, err := peer.IDFromPublicKey(vendorPub)
	if err != nil {
		t.Fatal(err)
	}

	ratingKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	vendorSig := &pb.RatingSignature{
		Slug:      "slug",
		RatingKey: ratingKey.PubKey().SerializeCompressed(),
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

	ratingSigHash := sha256.Sum256(ratingKey.PubKey().SerializeCompressed())
	buyerSig, err := buyerPriv.Sign(ratingSigHash[:])
	if err != nil {
		t.Fatal(err)
	}

	rating := &pb.Rating{
		Timestamp: timestamppb.Now(),
		VendorSig: vendorSig,
		VendorID: &pb.ID{
			PeerID: vendorID.String(),
			Pubkeys: &pb.ID_Pubkeys{
				Identity: vendorPubkeyBytes,
			},
		},
		BuyerID: &pb.ID{
			PeerID: buyerID.String(),
			Pubkeys: &pb.ID_Pubkeys{
				Identity: buyerPubkeyBytes,
			},
		},
		BuyerName: "Frank",
		BuyerSig:  buyerSig,

		Overall: 5,
		Review:  "asdf",
	}

	ser, err = proto.Marshal(rating)
	if err != nil {
		t.Fatal(err)
	}

	hashed := sha256.Sum256(ser)
	signature := ecdsa.Sign(ratingKey, hashed[:])
	if err != nil {
		t.Fatal(err)
	}
	rating.RatingSignature = signature.Serialize()

	tests := []struct {
		name  string
		setup func() *pb.Rating
		valid bool
	}{

		{
			name: "valid rating",
			setup: func() *pb.Rating {
				return rating
			},
			valid: true,
		},
		{
			name: "vendor ID is nil",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).VendorID = nil
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "vendor Pubkeys is nil",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).VendorID.Pubkeys = nil
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "vendor sig is nil",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).VendorSig = nil
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "vendor sig rating key is nil",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).VendorSig.RatingKey = nil
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "overall less than zero",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).Overall = 0
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "overall greater than five",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).Overall = 6
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "review too long",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).Review = strings.Repeat("s", 10001)
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "invalid rating key",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).VendorSig.RatingKey = []byte{0x00}
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "malformed rating signature",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).RatingSignature = []byte{0x00}
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "invalid rating signature",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).RatingSignature[10] = 0xff
				cpy.(*pb.Rating).RatingSignature[11] = 0xff
				cpy.(*pb.Rating).RatingSignature[12] = 0xff
				cpy.(*pb.Rating).RatingSignature[13] = 0xff
				cpy.(*pb.Rating).RatingSignature[14] = 0xff
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "malformed vendor identity key",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).VendorID.Pubkeys.Identity = []byte{0xff}
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "vendor ID does not match",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).VendorID.PeerID = "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7"
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "buyer ID does not match",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).BuyerID.PeerID = "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7"
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "buyer pubkey is nil",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).BuyerID.Pubkeys = nil
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "buyer identity pubkey is nil",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).BuyerID.Pubkeys.Identity = nil
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "buyer signature nil",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).BuyerSig = nil
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
		{
			name: "invalid buyer ID",
			setup: func() *pb.Rating {
				cpy := proto.Clone(rating)
				cpy.(*pb.Rating).BuyerID.PeerID = "fasdf"
				return cpy.(*pb.Rating)
			},
			valid: false,
		},
	}

	for _, test := range tests {
		rating := test.setup()

		err = ValidateRating(rating)
		if test.valid && err != nil {
			t.Errorf("Test %s: Validate rating failed when it should not have: %s", test.name, err)
		}
		if !test.valid && err == nil {
			t.Errorf("Test %s: Validate rating did not fail when it should have", test.name)
		}
	}
}

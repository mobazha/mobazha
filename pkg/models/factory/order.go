package factory

import (
	"crypto/sha256"
	"encoding/hex"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/multiformats/go-multihash"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewOrder() (*pb.OrderOpen, error) {
	privKeyBytes, err := hex.DecodeString("080112406e22f498c42014ea4485c2d4bdffd90fb3c4ee394f0aaa49a61a7b4e51235e016efc82dba17659db9daf4c8d1e39818f0d41ce9919876e299f56c71031375944")
	if err != nil {
		return nil, err
	}
	privkey, err := crypto.UnmarshalPrivateKey(privKeyBytes)
	if err != nil {
		return nil, err
	}
	pubkeyBytes, err := crypto.MarshalPublicKey(privkey.GetPublic())
	if err != nil {
		return nil, err
	}

	pid, err := peer.IDFromPublicKey(privkey.GetPublic())
	if err != nil {
		return nil, err
	}

	escrowPrivkeyBytes, err := hex.DecodeString("e93fc130413a742e96844ac2d2b38b380081b0a54ddc3aac4e5bdaecb598ff38")
	if err != nil {
		return nil, err
	}
	escrowPrivkey, escrowPubkey := btcec.PrivKeyFromBytes(escrowPrivkeyBytes)

	sigHash := sha256.Sum256([]byte(pid.String()))
	sig := ecdsa.Sign(escrowPrivkey, sigHash[:])

	ratingKey, err := hex.DecodeString("02fcaa2903a6aeff06eb5660d82cf3cd6ce686e7d2e2c23a12b23ea0cbbaf04e99")
	if err != nil {
		return nil, err
	}

	listing := NewSignedListing()
	ser, err := proto.Marshal(listing)
	if err != nil {
		return nil, err
	}
	h := sha256.Sum256(ser)
	encoded, err := multihash.Encode(h[:], multihash.SHA2_256)
	if err != nil {
		return nil, err
	}
	listingHash, err := multihash.Cast(encoded)
	if err != nil {
		return nil, err
	}

	chaincode, err := hex.DecodeString("ab4363f632d094270418472d5a5e99c12d38a21a5ded12ddb086102235d69206")
	if err != nil {
		return nil, err
	}

	myPubkey, err := utils.GenerateEscrowPublicKey(escrowPubkey, chaincode)
	if err != nil {
		return nil, err
	}

	vendorEscrowKey, err := btcec.ParsePubKey(listing.Listing.VendorID.Pubkeys.Escrow)
	if err != nil {
		return nil, err
	}

	theirPubkey, err := utils.GenerateEscrowPublicKey(vendorEscrowKey, chaincode)
	if err != nil {
		return nil, err
	}

	w := wallet.NewMockWallet()
	addr, script, err := w.CreateMultisigAddress([]btcec.PublicKey{*myPubkey, *theirPubkey}, chaincode, 1)
	if err != nil {
		return nil, err
	}

	order := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			listing,
		},
		RefundAddress: "01ce26dc69094af9246ea7e7ce9970aff2b81cc9",
		Shipping: &pb.OrderOpen_Shipping{
			ShipTo:       "Peter Griffin",
			Address:      "31 Spooner Street",
			City:         "Quahog",
			State:        "RI",
			PostalCode:   "90210",
			Country:      pb.CountryCode_UNITED_STATES,
			AddressNotes: "Don't leave in on the porch. Cleveland steals my packages.",
		},
		BuyerID: &pb.ID{
			PeerID: pid.String(),
			Handle: "@assman",
			Pubkeys: &pb.ID_Pubkeys{
				Identity: pubkeyBytes,
				Escrow:   escrowPubkey.SerializeCompressed(),
			},
			Sig: sig.Serialize(),
		},
		Timestamp: timestamppb.Now(),
		Items: []*pb.OrderOpen_Item{
			{
				ListingHash: listingHash.B58String(),
				Quantity:    "1",
				Options: []*pb.OrderOpen_Item_Option{
					{
						Name:  "size",
						Value: "large",
					},
					{
						Name:  "color",
						Value: "red",
					},
				},
				ShippingOption: &pb.OrderOpen_Item_ShippingOption{
					Name:    "usps",
					Service: "standard",
				},
			},
		},
		Payment: &pb.OrderOpen_Payment{
			Method:           pb.OrderOpen_Payment_CANCELABLE,
			Amount:           "4992221",
			Address:          addr.String(),
			Coin:             "MCK",
			EscrowReleaseFee: "10",
			Script:           hex.EncodeToString(script),
			Chaincode:        hex.EncodeToString(chaincode),
		},
		RatingKeys:           [][]byte{ratingKey},
		AlternateContactInfo: "peter@familyguy.net",
	}

	return order, nil
}

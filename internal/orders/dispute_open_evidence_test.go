package orders

import (
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestPersistInboundDisputeEvidence_skipsWhenEmptyOrAlreadySet(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	order := &models.Order{ID: "evidence-order-1"}
	order.SetRole(models.RoleVendor)

	err = op.db.Update(func(tx database.Tx) error {
		if err := persistInboundDisputeEvidence(tx, order, nil); err != nil {
			return err
		}
		order.DisputeEvidenceHashes = models.StringSlice{"QmExisting"}
		return persistInboundDisputeEvidence(tx, order, []string{"QmNew"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(order.DisputeEvidenceHashes) != 1 || order.DisputeEvidenceHashes[0] != "QmExisting" {
		t.Fatalf("expected existing hash preserved, got %#v", order.DisputeEvidenceHashes)
	}
}

func TestProcessDisputeOpenMessage_persistsEvidenceHashesOnVendor(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

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

	orderID := "evidence-order-2"
	evidence := []string{"QmEvidenceA", "QmEvidenceB"}

	disputeOpenMsg := &pb.DisputeOpen{
		OpenedBy:       pb.DisputeOpen_BUYER,
		EvidenceHashes: evidence,
	}
	disputeOpenAny := &anypb.Any{}
	if err := disputeOpenAny.MarshalFrom(disputeOpenMsg); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_DISPUTE_OPEN,
		Message:     disputeOpenAny,
	}

	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			{
				Listing: &pb.Listing{
					Item: &pb.Listing_Item{
						Images: []*pb.Image{{Small: "img-small", Tiny: "img-tiny"}},
					},
					VendorID: &pb.ID{
						PeerID:  localPeer.String(),
						Handle:  "vendor",
						Name:    "vendor",
						Pubkeys: &pb.ID_Pubkeys{Identity: localPubkeyBytes},
					},
				},
			},
		},
		BuyerID: &pb.ID{
			PeerID:  remotePeer.String(),
			Handle:  "buyer",
			Name:    "buyer",
			Pubkeys: &pb.ID_Pubkeys{Identity: pubkeyBytes},
		},
	}

	paymentSent := &pb.PaymentSent{
		Coin:           iwallet.CtMock.String(),
		Moderator:      "12D3KooWHnpVyu9XDeFoAVayqr9hvc9xPqSSHtCSFLEkKgcz5Wro",
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_MODERATED),
	}

	order := &models.Order{ID: models.OrderID(orderID)}
	order.SetRole(models.RoleVendor)

	if err := order.PutMessage(&npb.OrderMessage{
		Signature:   []byte("abc"),
		Message:     mustBuildAny(orderOpen),
		MessageType: npb.OrderMessage_ORDER_OPEN,
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

	err = op.db.Update(func(tx database.Tx) error {
		_, err := op.processDisputeOpenMessage(tx, order, orderMsg)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(order.DisputeEvidenceHashes) != 2 {
		t.Fatalf("expected 2 evidence hashes on vendor order, got %#v", order.DisputeEvidenceHashes)
	}
	if order.DisputeEvidenceHashes[0] != evidence[0] || order.DisputeEvidenceHashes[1] != evidence[1] {
		t.Fatalf("unexpected hashes: %#v", order.DisputeEvidenceHashes)
	}
}

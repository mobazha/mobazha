package utils

import (
	"testing"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/identity"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestSignOrderMessage(t *testing.T) {
	// Create a signer using mobazha-core
	keyPair, err := identity.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	peerID, err := identity.PeerIDFromPublicKey(keyPair.PubKey)
	if err != nil {
		t.Fatal(err)
	}
	signer := contracts.NewKeyPairSigner(keyPair, peerID)

	orderOpen := pb.OrderOpen{
		AlternateContactInfo: "1234",
	}

	a := &anypb.Any{}
	if err := a.MarshalFrom(&orderOpen); err != nil {
		t.Fatal(err)
	}

	order := npb.OrderMessage{
		Message:     a,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		OrderID:     "abc",
	}

	err = SignOrderMessage(&order, signer)
	if err != nil {
		t.Fatal(err)
	}

	// Verify PeerID was set correctly
	if order.SenderPeerID != peerID.String() {
		t.Errorf("expected SenderPeerID %s, got %s", peerID, order.SenderPeerID)
	}

	// Verify signature using the signer's Verify method
	cpy := proto.Clone(&order)
	cpy.(*npb.OrderMessage).Signature = nil

	ser, err := proto.Marshal(cpy)
	if err != nil {
		t.Fatal(err)
	}

	valid, err := signer.Verify(ser, order.Signature)
	if err != nil {
		t.Fatal(err)
	}

	if !valid {
		t.Error("invalid signature")
	}
}

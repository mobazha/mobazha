package utils

import (
	"crypto/rand"
	"testing"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestSignOrderMessage(t *testing.T) {
	priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

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

	err = SignOrderMessage(&order, priv)
	if err != nil {
		t.Fatal(err)
	}

	cpy := proto.Clone(&order)
	cpy.(*npb.OrderMessage).Signature = nil

	ser, err := proto.Marshal(cpy)
	if err != nil {
		t.Fatal(err)
	}

	valid, err := pub.Verify(ser, order.Signature)
	if err != nil {
		t.Fatal(err)
	}

	if !valid {
		t.Error("invalid signature")
	}
}

package utils

import (
	"errors"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	"google.golang.org/protobuf/proto"
)

// SignOrderMessage signs an order message using the provided Signer.
// The protobuf serialization of the message (without the signature field)
// is what gets signed. It also sets the SenderPeerID from the signer's PeerID.
func SignOrderMessage(message *pb.OrderMessage, signer contracts.Signer) error {
	if signer == nil {
		return errors.New("signer must not be nil")
	}

	// Set sender peer ID from signer
	message.SenderPeerID = signer.PeerID().String()

	// Clear signature, marshal, sign, then set signature
	message.Signature = nil
	ser, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	sig, err := signer.Sign(ser)
	if err != nil {
		return err
	}

	message.Signature = sig
	return nil
}

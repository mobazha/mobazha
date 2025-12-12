package utils

import (
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	"google.golang.org/protobuf/proto"
)

// SignOrderMessage puts a signature on an order message using the IPFS private
// key. The protobuf serialization of the message object without the signature
// is what is signed. It also sets the SenderPeerID from the private key.
func SignOrderMessage(message *pb.OrderMessage, privKey crypto.PrivKey) error {
	// Derive and set sender peer ID from private key
	senderID, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		return err
	}
	message.SenderPeerID = senderID.String()

	message.Signature = nil
	ser, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	sig, err := privKey.Sign(ser)
	if err != nil {
		return err
	}

	message.Signature = sig
	return nil
}

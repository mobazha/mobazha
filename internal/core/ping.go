//go:build !private_distribution

package core

import (
	"context"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/events"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
)

const maxPongDelay = time.Second * 10

// PingNode sends a PING message to the provided peer. If we are able to successfully
// connect and receive an PONG message back we return nil. If we don't receive a
// PONG message back an error is returned.
func (n *MobazhaNode) PingNode(ctx context.Context, peer peer.ID) error {
	sub, err := n.eventBus.Subscribe(&events.PongReceived{})
	if err != nil {
		return err
	}
	defer sub.Close()

	m := newMessageWithID()
	m.MessageType = pb.Message_PING
	if err := n.networkService.SendMessage(ctx, peer, m); err != nil {
		return err
	}

	select {
	case <-sub.Out():
		return nil
	case <-ctx.Done():
	case <-time.After(maxPongDelay):
	}
	return coreiface.ErrPeerUnreachable
}

func (n *MobazhaNode) handlePingMessage(from peer.ID, message *pb.Message) error {
	n.eventBus.Emit(&events.PingReceived{
		Peer: from,
	})
	m := newMessageWithID()
	m.MessageType = pb.Message_PONG
	return n.networkService.SendMessage(context.Background(), from, m)
}

func (n *MobazhaNode) handlePongMessage(from peer.ID, message *pb.Message) error {
	n.eventBus.Emit(&events.PongReceived{
		Peer: from,
	})
	return nil
}

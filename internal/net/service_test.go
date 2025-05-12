package net

import (
	"context"
	"testing"

	peer "github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
)

func TestNetworkService(t *testing.T) {
	mocknet, err := mocknet.FullMeshLinked(2)
	if err != nil {
		t.Fatal(err)
	}

	service1 := NewNetworkService("", mocknet.Hosts()[0], NewBanManager(nil, nil), true)
	service2 := NewNetworkService("", mocknet.Hosts()[1], NewBanManager(nil, nil), true)

	ms, err := service1.messageSenderForPeer(context.Background(), mocknet.Hosts()[1].ID())
	if err != nil {
		t.Fatal(err)
	}

	ch := make(chan struct{})
	service2.RegisterHandler(pb.Message_ACK, func(p peer.ID, msg *pb.Message) error {
		ch <- struct{}{}
		return nil
	})

	if err := ms.sendMessage(context.Background(), &pb.Message{}); err != nil {
		t.Error(err)
	}

	<-ch
}

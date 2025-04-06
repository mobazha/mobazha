package net

import (
	"context"
	"testing"

	pb "github.com/mobazha/mobazha3.0/internal/net/mbzpb"
	peer "github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

func TestMessageSender(t *testing.T) {
	// New network
	mocknet, err := mocknet.FullMeshLinked(2)
	if err != nil {
		t.Fatal(err)
	}

	service1 := NewNetworkService(mocknet.Hosts()[0], NewBanManager(nil, nil), true)
	service2 := NewNetworkService(mocknet.Hosts()[1], NewBanManager(nil, nil), true)

	// Open messageSenders and try to send messages
	ms1, err := service1.messageSenderForPeer(context.Background(), mocknet.Hosts()[1].ID())
	if err != nil {
		t.Fatal(err)
	}

	if err := ms1.sendMessage(context.Background(), &pb.Message{}); err != nil {
		t.Error(err)
	}

	ms2, err := service2.messageSenderForPeer(context.Background(), mocknet.Hosts()[0].ID())
	if err != nil {
		t.Fatal(err)
	}

	if err := ms2.sendMessage(context.Background(), &pb.Message{}); err != nil {
		t.Error(err)
	}

	// Make sure the context is respected on send.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = ms2.sendMessage(ctx, &pb.Message{})
	if err != ErrContextDone {
		t.Errorf("Expected ErrContextDone got %s", err)
	}

	// Make sure the context is respected on open.
	theirPid, err := peer.Decode("QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	_, err = service1.messageSenderForPeer(ctx, theirPid)
	if err != ErrContextDone {
		t.Errorf("Expected ErrContextDone got %s", err)
	}
}

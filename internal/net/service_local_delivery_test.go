package net

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	pb "github.com/mobazha/mobazha/pkg/net/mbzpb"
)

func TestNetworkServiceSendMessage_UsesLocalDeliverer(t *testing.T) {
	service, target := newLocalDeliveryTestService(t)

	var called atomic.Bool
	service.SetLocalDeliverer(localDelivererFunc(func(to peer.ID, from peer.ID, msg *pb.Message) (bool, error) {
		called.Store(true)
		if to != target {
			t.Fatalf("target = %s, want %s", to, target)
		}
		if from != service.host.ID() {
			t.Fatalf("from = %s, want %s", from, service.host.ID())
		}
		return true, nil
	}))

	if err := service.SendMessage(context.Background(), target, &pb.Message{MessageID: "msg-1"}); err != nil {
		t.Fatal(err)
	}
	if !called.Load() {
		t.Fatal("local deliverer was not called")
	}
}

func TestNetworkServiceSendMessage_ReturnsLocalDeliveryError(t *testing.T) {
	service, target := newLocalDeliveryTestService(t)
	wantErr := errors.New("local delivery failed")
	service.SetLocalDeliverer(localDelivererFunc(func(peer.ID, peer.ID, *pb.Message) (bool, error) {
		return true, wantErr
	}))

	if err := service.SendMessage(context.Background(), target, &pb.Message{MessageID: "msg-1"}); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

type localDelivererFunc func(peer.ID, peer.ID, *pb.Message) (bool, error)

func (f localDelivererFunc) DeliverToLocal(target peer.ID, from peer.ID, msg *pb.Message) (bool, error) {
	return f(target, from, msg)
}

func newLocalDeliveryTestService(t *testing.T) (*NetworkService, peer.ID) {
	t.Helper()

	mn := mocknet.New()
	priv, addr, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}
	h, err := mn.AddPeer(priv, addr)
	if err != nil {
		t.Fatal(err)
	}
	target := h.ID()
	return NewNetworkService("", h, NewBanManager(nil, nil), true), target
}

//go:build !private_distribution

package core

import (
	"testing"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestMobazhaNode_Follow(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}
	defer node.repo.DestroyRepo()

	p, err := peer.Decode("12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN")
	if err != nil {
		t.Fatal(err)
	}

	if err := node.Profile().SetProfile(&models.Profile{Name: "Ron Paul"}, nil); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	if err := node.Social().FollowNode(p, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	following, err := node.Social().GetMyFollowing()
	if err != nil {
		t.Fatal(err)
	}

	if following.Count() != 1 {
		t.Fatalf("Incorrect number of following returned. Expected %d, got %d", 1, following.Count())
	}

	if following[0] != p.String() {
		t.Errorf("Incorrect following peer returned. Expected %s, got %s", p.String(), following[0])
	}

	p2, err := peer.Decode("12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi")
	if err != nil {
		t.Fatal(err)
	}

	done = make(chan struct{})
	if err := node.Social().FollowNode(p2, done); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	following, err = node.Social().GetMyFollowing()
	if err != nil {
		t.Fatal(err)
	}

	if following.Count() != 2 {
		t.Fatalf("Incorrect number of following returned. Expected %d, got %d", 2, following.Count())
	}

	if following[1] != p2.String() {
		t.Errorf("Incorrect following peer returned. Expected %s, got %s", p2.String(), following[1])
	}

	stats, err := node.Profile().GetProfileStats()
	if err != nil {
		t.Fatal(err)
	}

	if stats == nil {
		t.Fatal("Profile stats is nil")
	}

	if stats.FollowingCount != uint32(following.Count()) {
		t.Errorf("Following count in profile incorrect. Expected %d, got %d", following.Count(), stats.FollowingCount)
	}
}

func TestMobazhaNode_GetFollowing(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}
	defer node.repo.DestroyRepo()

	p1, err := peer.Decode("12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := peer.Decode("12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi")
	if err != nil {
		t.Fatal(err)
	}

	if err := node.Profile().SetProfile(&models.Profile{Name: "Ron Paul"}, nil); err != nil {
		t.Fatal(err)
	}

	if err := node.Social().FollowNode(p1, nil); err != nil {
		t.Fatal(err)
	}

	following, err := node.Social().GetMyFollowing()
	if err != nil {
		t.Fatal(err)
	}
	if following.Count() != 1 {
		t.Fatalf("Incorrect number of following returned. Expected %d, got %d", 1, following.Count())
	}
	if following[0] != p1.String() {
		t.Errorf("Incorrect following peer returned. Expected %s, got %s", p1.String(), following[0])
	}

	if err := node.Social().FollowNode(p2, nil); err != nil {
		t.Fatal(err)
	}

	following, err = node.Social().GetMyFollowing()
	if err != nil {
		t.Fatal(err)
	}
	if following.Count() != 2 {
		t.Fatalf("Incorrect number of following returned. Expected %d, got %d", 2, following.Count())
	}

	var dbFollowing models.Following
	err = node.repo.DB().View(func(tx database.Tx) error {
		var e error
		dbFollowing, e = tx.GetFollowing()
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if dbFollowing.Count() != 2 {
		t.Fatalf("DB following count mismatch. Expected 2, got %d", dbFollowing.Count())
	}
}

func TestMobazhaNode_GetFollowers(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}
	defer node.repo.DestroyRepo()

	p1, err := peer.Decode("12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := peer.Decode("12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi")
	if err != nil {
		t.Fatal(err)
	}

	err = node.repo.DB().Update(func(tx database.Tx) error {
		return tx.SetFollowers(models.Followers{p1.String()})
	})
	if err != nil {
		t.Fatal(err)
	}

	followers, err := node.Social().GetMyFollowers()
	if err != nil {
		t.Fatal(err)
	}
	if followers.Count() != 1 {
		t.Fatalf("Incorrect number of followers returned. Expected %d, got %d", 1, followers.Count())
	}
	if followers[0] != p1.String() {
		t.Errorf("Incorrect follower peer returned. Expected %s, got %s", p1.String(), followers[0])
	}

	err = node.repo.DB().Update(func(tx database.Tx) error {
		return tx.SetFollowers(models.Followers{p1.String(), p2.String()})
	})
	if err != nil {
		t.Fatal(err)
	}

	followers, err = node.Social().GetMyFollowers()
	if err != nil {
		t.Fatal(err)
	}
	if followers.Count() != 2 {
		t.Fatalf("Incorrect number of followers returned. Expected %d, got %d", 2, followers.Count())
	}

	var dbFollowers models.Followers
	err = node.repo.DB().View(func(tx database.Tx) error {
		var e error
		dbFollowers, e = tx.GetFollowers()
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if dbFollowers.Count() != 2 {
		t.Fatalf("DB followers count mismatch. Expected 2, got %d", dbFollowers.Count())
	}
}

func Test_handleFollowAndUnfollow(t *testing.T) {
	mocknet, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}
	defer mocknet.TearDown()

	// Test follow
	sub, err := mocknet.Nodes()[1].eventBus.Subscribe(&events.Follow{})
	if err != nil {
		t.Fatal(err)
	}

	if err := mocknet.Nodes()[0].Social().FollowNode(mocknet.Nodes()[1].Identity(), nil); err != nil {
		t.Fatal(err)
	}

	var event interface{}
	select {
	case event = <-sub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	notif, ok := event.(*events.Follow)
	if !ok {
		t.Fatalf("Event type assertion failed")
	}

	if notif.PeerID != mocknet.Nodes()[0].Identity().String() {
		t.Errorf("Received incorrect peer ID in follow notification. Expected %s, got %s", mocknet.Nodes()[0].Identity(), notif.PeerID)
	}

	followers, err := mocknet.Nodes()[1].Social().GetMyFollowers()
	if err != nil {
		t.Fatal(err)
	}

	if followers.Count() != 1 {
		t.Fatalf("Incorrect number of followers returned. Expected %d, got %d", 1, followers.Count())
	}

	if followers[0] != mocknet.Nodes()[0].Identity().String() {
		t.Errorf("Incorrect follower ID. Expected %s, got %s", mocknet.Nodes()[0].Identity(), followers[0])
	}

	// Test unfollow
	sub2, err := mocknet.Nodes()[1].eventBus.Subscribe(&events.Unfollow{})
	if err != nil {
		t.Fatal(err)
	}

	if err := mocknet.Nodes()[0].Social().UnfollowNode(mocknet.Nodes()[1].Identity(), nil); err != nil {
		t.Fatal(err)
	}

	var event2 interface{}
	select {
	case event2 = <-sub2.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	notif2, ok := event2.(*events.Unfollow)
	if !ok {
		t.Fatalf("Event type assertion failed")
	}

	if notif2.PeerID != mocknet.Nodes()[0].Identity().String() {
		t.Errorf("Received incorrect peer ID in unfollow notification. Expected %s, got %s", mocknet.Nodes()[0].Identity(), notif2.PeerID)
	}

	followers, err = mocknet.Nodes()[1].Social().GetMyFollowers()
	if err != nil {
		t.Fatal(err)
	}

	if followers.Count() != 0 {
		t.Fatalf("Incorrect number of followers returned. Expected %d, got %d", 0, followers.Count())
	}
}

func TestMobazhaNode_FollowSequence(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}
	defer node.repo.DestroyRepo()

	p, err := peer.Decode("12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN")
	if err != nil {
		t.Fatal(err)
	}

	if err := node.Profile().SetProfile(&models.Profile{Name: "Ron Paul"}, nil); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	if err := node.Social().FollowNode(p, done); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	var seq models.FollowSequence
	err = node.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("peer_id = ?", p.String()).First(&seq).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if seq.Num != 1 {
		t.Errorf("Expected follow sequence number of 1, got %d", seq.Num)
	}

	done = make(chan struct{})
	if err := node.Social().UnfollowNode(p, done); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	err = node.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("peer_id = ?", p.String()).First(&seq).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if seq.Num != 2 {
		t.Errorf("Expected follow sequence number of 2, got %d", seq.Num)
	}
}

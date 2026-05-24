//go:build !private_distribution

package core

import (
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestMobazhaNode_PublishToFollowers(t *testing.T) {
	mocknet, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}

	defer mocknet.TearDown()

	// Start node - follower tracker
	mocknet.Nodes()[0].followerTracker.Start()

	storeSub, err := mocknet.Nodes()[1].SubscribeEvent(&events.MessageStore{})
	if err != nil {
		t.Fatal(err)
	}

	followSub, err := mocknet.Nodes()[0].SubscribeEvent(&events.TrackerFollow{})
	if err != nil {
		t.Fatal(err)
	}

	// Set profile node 0
	done1 := make(chan struct{})
	if err := mocknet.Nodes()[0].Profile().SetProfile(&models.Profile{Name: "Peter Griffin"}, done1); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done1:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	// Node 1 send follow
	done2 := make(chan struct{})
	if err := mocknet.Nodes()[1].Social().FollowNode(mocknet.Nodes()[0].Identity(), done2); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done2:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	select {
	case <-followSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	// Run the follower tracker to load node 1 as a follower in node 0.
	mocknet.Nodes()[0].followerTracker.tryConnectFollowers()

	// Set profile again with a new publish.
	done3 := make(chan struct{})
	if err := mocknet.Nodes()[0].Profile().SetProfile(&models.Profile{Name: "Peter Griffin2"}, done3); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done3:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	// Make sure 1 node is pinning the correct cids.
	select {
	case <-storeSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	// Verify the last published root CID was persisted in the DB.
	var lastPublishCID string
	err = mocknet.Nodes()[0].repo.DB().View(func(tx database.Tx) error {
		var event models.Event
		if err := tx.Read().Where("name = ?", "last_publish").First(&event).Error; err != nil {
			return err
		}
		lastPublishCID = event.Value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cid.Decode(lastPublishCID); err != nil {
		t.Fatalf("Invalid last_publish CID: %s", err)
	}
}

func TestMobazhaNode_republish(t *testing.T) {
	mocknet, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}

	defer mocknet.TearDown()

	sub, err := mocknet.Nodes()[0].SubscribeEvent(&events.PublishFinished{})
	if err != nil {
		t.Fatal(err)
	}

	mocknet.Nodes()[0].publishChan <- pubCloser{
		nil,
	}

	select {
	case <-sub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
}

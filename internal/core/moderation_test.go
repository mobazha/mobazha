package core

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/models"
	"github.com/mobazha/mobazha3.0/internal/models/factory"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

func TestOpenBazaarNode_SetAndRemoveSelfAsModerator(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	if err := node.SetProfile(&models.Profile{Name: "Ron Paul"}, nil); err != nil {
		t.Fatal(err)
	}

	modInfo := &models.ModeratorInfo{
		Fee: models.ModeratorFee{
			FeeType:    models.PercentageFee,
			Percentage: 10,
		},
	}

	done := make(chan struct{})
	if err := node.SetSelfAsModerator(context.Background(), modInfo, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	done2 := make(chan struct{})
	if err := node.RemoveSelfAsModerator(context.Background(), done2); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done2:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
}

func TestOpenBazaarNode_GetVerifiedModerators(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	mods := node.GetVerifiedModerators(context.Background())
	if len(mods) == 0 {
		t.Fatal("Expected moderators")
	}
}

func TestOpenBazaarNode_GetModerators(t *testing.T) {
	mocknet, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}

	defer mocknet.TearDown()

	done0 := make(chan struct{})
	originalProfile := &models.Profile{Name: "Ron Paul"}
	if err := mocknet.Nodes()[0].SetProfile(originalProfile, done0); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done0:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	modInfo := &models.ModeratorInfo{
		Fee: models.ModeratorFee{
			FeeType:    models.PercentageFee,
			Percentage: 10,
		},
	}

	done := make(chan struct{})
	if err := mocknet.Nodes()[0].SetSelfAsModerator(context.Background(), modInfo, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	mods := mocknet.Nodes()[1].GetModerators(context.Background())

	if len(mods) != 1 {
		t.Fatalf("Returned incorrect number of moderators. Expected %d, got %d", 1, len(mods))
	}

	if mods[0].String() != mocknet.Nodes()[0].Identity().String() {
		t.Errorf("Returned incorrect peer ID. Expected %s, got %s", mocknet.Nodes()[0].Identity(), mods[0])
	}

	ch := mocknet.Nodes()[1].GetModeratorsAsync(context.Background())

	mods = []peer.ID{}
	for mod := range ch {
		mods = append(mods, mod)
	}

	if len(mods) != 1 {
		t.Errorf("Returned incorrect number of moderators. Expected %d, got %d", 1, len(mods))
	}

	if mods[0].String() != mocknet.Nodes()[0].Identity().String() {
		t.Errorf("Returned incorrect peer ID. Expected %s, got %s", mocknet.Nodes()[0].Identity(), mods[0])
	}

	profile, err := mocknet.Nodes()[1].GetProfile(context.Background(), mods[0], false)
	if err != nil {
		t.Fatal(err)
	}

	if profile.Name != originalProfile.Name {
		t.Errorf("Returned incorrect profile name. Expected %s, got %s", originalProfile.Name, profile.Name)
	}

	if profile.ModeratorInfo == nil {
		t.Error("Profile moderator info is nil")
	}

	if !profile.Moderator {
		t.Error("Profile does not have moderator bool set")
	}

	if profile.ModeratorInfo.Fee.Percentage != modInfo.Fee.Percentage {
		t.Errorf("Returned incorrect moderator percentage. Expected %f, got %f", modInfo.Fee.Percentage, profile.ModeratorInfo.Fee.Percentage)
	}
}

func TestOpenBazaarNode_SetModeratorsOnListings(t *testing.T) {
	l1 := factory.NewPhysicalListing("tshirt")
	l1.Moderators = []string{}
	l2 := factory.NewPhysicalListing("shoes")
	l2.Moderators = []string{}

	n, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	if err := n.SaveListing(l1, nil); err != nil {
		t.Fatal(err)
	}

	if err := n.SaveListing(l2, nil); err != nil {
		t.Fatal(err)
	}

	modID := "12D3KooW9qYCthfQAwxnuW62ZTN8uoKBfRkt5a2bcKJWR5aDwta6"
	pid, err := peer.Decode(modID)
	if err != nil {
		t.Fatal(err)
	}

	mods := []peer.ID{pid}

	if err := n.SetModeratorsOnListings(mods, nil); err != nil {
		t.Fatal(err)
	}

	ls, err := n.GetMyListings()
	if err != nil {
		t.Fatal(err)
	}

	if len(ls) != 2 {
		t.Errorf("Expected 2 listings got %d", len(ls))
	}

	for _, l := range ls {
		if l.ModeratorIDs[0] != modID {
			t.Errorf("Expected mod ID %s, got %s", modID, l.ModeratorIDs[0])
		}
		listing, err := n.GetMyListingBySlug(l.Slug)
		if err != nil {
			t.Fatal(err)
		}
		if listing.Listing.Moderators[0] != modID {
			t.Errorf("Expected mod ID %s, got %s", modID, listing.Listing.Moderators[0])
		}
	}
}

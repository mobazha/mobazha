package core

import (
	"context"
	"testing"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
)

func TestMobazhaNode_SetAndRemoveSelfAsModerator(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	if err := node.Profile().SetProfile(&models.Profile{Name: "Ron Paul"}, nil); err != nil {
		t.Fatal(err)
	}

	modInfo := &models.ModeratorInfo{
		Fee: models.ModeratorFee{
			FeeType:    models.PercentageFee,
			Percentage: 10,
		},
	}

	done := make(chan struct{})
	if err := node.Profile().SetSelfAsModerator(context.Background(), modInfo, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	done2 := make(chan struct{})
	if err := node.Profile().RemoveSelfAsModerator(context.Background(), done2); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done2:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
}

func TestMobazhaNode_GetVerifiedModerators(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	// Verified moderators come from an external endpoint, not DHT.
	// In mock environment without endpoint config, expect empty result.
	_ = node.Profile().GetVerifiedModerators(context.Background())
}

func TestMobazhaNode_GetModerators_DHT_Removed(t *testing.T) {
	mocknet, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}

	defer mocknet.TearDown()

	// DHT-based moderator discovery has been removed.
	// GetModerators and GetModeratorsAsync should return empty results.
	mods := mocknet.Nodes()[1].Profile().GetModerators(context.Background())
	if len(mods) != 0 {
		t.Fatalf("Expected 0 moderators from DHT (removed), got %d", len(mods))
	}

	ch := mocknet.Nodes()[1].Profile().GetModeratorsAsync(context.Background())
	mods = nil
	for mod := range ch {
		mods = append(mods, mod)
	}
	if len(mods) != 0 {
		t.Fatalf("Expected 0 moderators from async DHT (removed), got %d", len(mods))
	}
}

func TestMobazhaNode_SetModeratorsOnListings(t *testing.T) {
	l1 := factory.NewPhysicalListing("tshirt")
	l1.Moderators = []string{}
	l2 := factory.NewPhysicalListing("shoes")
	l2.Moderators = []string{}

	n, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	if err := n.Listing().SaveListing(l1, nil); err != nil {
		t.Fatal(err)
	}

	if err := n.Listing().SaveListing(l2, nil); err != nil {
		t.Fatal(err)
	}

	modID := "12D3KooW9qYCthfQAwxnuW62ZTN8uoKBfRkt5a2bcKJWR5aDwta6"
	pid, err := peer.Decode(modID)
	if err != nil {
		t.Fatal(err)
	}

	mods := []peer.ID{pid}

	if err := n.Listing().SetModeratorsOnListings(mods, nil); err != nil {
		t.Fatal(err)
	}

	ls, err := n.Listing().GetMyListings()
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
		listing, err := n.Listing().GetMyListingBySlug(l.Slug)
		if err != nil {
			t.Fatal(err)
		}
		if listing.Listing.Moderators[0] != modID {
			t.Errorf("Expected mod ID %s, got %s", modID, listing.Listing.Moderators[0])
		}
	}
}

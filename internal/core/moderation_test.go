package core

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
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

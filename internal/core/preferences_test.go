package core

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/models/factory"
)

func TestMobazhaNode_SavePreferences(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}
	defer node.repo.DestroyRepo()

	listing := factory.NewPhysicalListing("ron-swanson-shirt")

	done := make(chan struct{})
	if err := node.Listing().SaveListing(listing, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	prefs := models.UserPreferences{
		RefundPolicy: "asdf",
	}

	if err := node.Preferences().SavePreferences(&prefs, nil); err != nil {
		t.Fatal(err)
	}

	var savedPrefs models.UserPreferences
	err = node.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().First(&savedPrefs).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if savedPrefs.RefundPolicy != prefs.RefundPolicy {
		t.Errorf("Expected refund policy %s, got %s", prefs.RefundPolicy, savedPrefs.RefundPolicy)
	}

	prefs = models.UserPreferences{
		Blocked: []byte(`["aasdf"]`),
	}

	if err := node.Preferences().SavePreferences(&prefs, nil); err == nil {
		t.Errorf("Expected error got nil")
	}

}

func TestMobazhaNode_BlockNode(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}
	defer node.repo.DestroyRepo()

	peerID := "12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN"
	addedToBlock, err := node.Preferences().BlockNode(peerID)
	if err != nil {
		t.Fatal(err)
	}
	if !addedToBlock {
		t.Error("addedToBlock flag is false ")
	}

	prefs, err := node.Preferences().GetPreferences()
	if err != nil {
		t.Fatal(err)
	}
	blockedNodes, err := prefs.BlockedNodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(blockedNodes) != 1 {
		t.Error("Incorrect blocked nodes size")
	}
	if blockedNodes[0].String() != peerID {
		t.Error("Blocked node peer ID not matched")
	}

	removeFromBlock, err := node.Preferences().UnblockNode(peerID)
	if err != nil {
		t.Fatal(err)
	}
	if !removeFromBlock {
		t.Error("removeFromBlock flag is false ")
	}
	prefs, err = node.Preferences().GetPreferences()
	if err != nil {
		t.Fatal(err)
	}
	blockedNodes, err = prefs.BlockedNodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(blockedNodes) != 0 {
		t.Error("Blocked nodes size is not 0")
	}
}

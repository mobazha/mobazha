package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

func newSharedLocalNode(t *testing.T) *MobazhaNode {
	t.Helper()

	dataDir := filepath.Join(t.TempDir(), "mobazha-test-node")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	node, err := NewNode(context.Background(), &repo.Config{
		DataDir: dataDir,
		Testnet: true,
	}, repo.DefaultNodeID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { node.DestroyNode() })

	// NewNode does not wire publish; listing save waits on initialBootstrapChan.
	// Mirror MockNode: start publishHandler and signal bootstrap ready.
	node.publishHandler()
	close(node.initialBootstrapChan)

	seedSharedShippingProfile(t, node)
	return node
}

func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting on done channel")
	}
}

func newSharedPrivateDistributionListing(t *testing.T, slug string) *pb.Listing {
	t.Helper()

	listing := factory.NewPhysicalListing(slug)
	listing.Metadata.PricingCurrency.Code = "EXTERNAL_PAYMENT"
	listing.Metadata.PricingCurrency.Divisibility = 12
	listing.Item.Price = "1000000000000"
	for _, sku := range listing.Item.Skus {
		sku.Price = "1000000000000"
	}
	if listing.ShippingProfile != nil {
		for _, group := range listing.ShippingProfile.LocationGroups {
			for _, zone := range group.Zones {
				for _, rate := range zone.Rates {
					rate.Currency = "EXTERNAL_PAYMENT"
					rate.Price = "500000000000"
				}
			}
		}
	}
	return listing
}

func seedSharedShippingProfile(t *testing.T, node *MobazhaNode) {
	t.Helper()
	if node.shippingService == nil {
		return
	}
	groups := []*models.LocationGroup{{
		ID: "default-lg",
		Zones: []*models.ShippingZone{{
			ID:      "zone-all",
			Name:    "Worldwide",
			Regions: []string{"ALL"},
			Rates: []*models.ShippingRate{{
				ID:       "rate-std",
				Name:     "Standard",
				Price:    "500",
				Currency: "USD",
			}},
		}},
	}}
	groupsJSON, err := json.Marshal(groups)
	if err != nil {
		t.Fatal(err)
	}
	if err := node.shippingService.CreateProfile(context.Background(), &models.ShippingProfileEntity{
		ID:                 "factory-default-profile",
		Name:               "Default Shipping",
		IsDefault:          true,
		LocationGroupsJSON: string(groupsJSON),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSharedNode_ProfileRoundTrip(t *testing.T) {
	node := newSharedLocalNode(t)

	name := "Ron Swanson"
	err := node.Profile().SetProfile(&models.Profile{Name: name}, nil)
	if err != nil {
		t.Fatal(err)
	}

	pro, err := node.Profile().GetMyProfile()
	if err != nil {
		t.Fatal(err)
	}
	if pro.Name != name {
		t.Fatalf("expected profile name %q, got %q", name, pro.Name)
	}
}

func TestSharedNode_LazyProfileStats(t *testing.T) {
	node := newSharedLocalNode(t)

	profile := &models.Profile{Name: "Ron Paul"}
	if err := node.Profile().SetProfile(profile, nil); err != nil {
		t.Fatal(err)
	}
	err := node.repo.DB().Update(func(tx database.Tx) error {
		if err := tx.SetFollowers(models.Followers{"QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub"}); err != nil {
			return err
		}
		return tx.SetFollowing(models.Following{"QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub"})
	})
	if err != nil {
		t.Fatal(err)
	}

	ret, err := node.Profile().GetMyProfile()
	if err != nil {
		t.Fatal(err)
	}
	if ret.Stats != nil {
		t.Fatal("GetMyProfile should not eagerly include stats")
	}

	stats, err := node.Profile().GetProfileStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.FollowerCount != 1 || stats.FollowingCount != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestSharedNode_SavePreferences(t *testing.T) {
	node := newSharedLocalNode(t)

	done := make(chan struct{})
	if err := node.Listing().SaveListing(newSharedPrivateDistributionListing(t, "ron-swanson-shirt"), done); err != nil {
		t.Fatal(err)
	}
	waitDone(t, done)

	prefs := models.UserPreferences{RefundPolicy: "asdf"}
	if err := node.Preferences().SavePreferences(&prefs, nil); err != nil {
		t.Fatal(err)
	}

	var savedPrefs models.UserPreferences
	err := node.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().First(&savedPrefs).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if savedPrefs.RefundPolicy != prefs.RefundPolicy {
		t.Fatalf("expected refund policy %q, got %q", prefs.RefundPolicy, savedPrefs.RefundPolicy)
	}
}

func TestSharedNode_BlockNode(t *testing.T) {
	node := newSharedLocalNode(t)

	peerID := "12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN"
	addedToBlock, err := node.Preferences().BlockNode(peerID)
	if err != nil {
		t.Fatal(err)
	}
	if !addedToBlock {
		t.Fatal("expected node to be added to block list")
	}

	prefs, err := node.Preferences().GetPreferences()
	if err != nil {
		t.Fatal(err)
	}
	blockedNodes, err := prefs.BlockedNodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(blockedNodes) != 1 || blockedNodes[0].String() != peerID {
		t.Fatalf("unexpected blocked nodes: %+v", blockedNodes)
	}
}

func TestSharedNode_SaveListing(t *testing.T) {
	node := newSharedLocalNode(t)

	done := make(chan struct{})
	if err := node.Listing().SaveListing(newSharedPrivateDistributionListing(t, "ron-swanson-shirt"), done); err != nil {
		t.Fatal(err)
	}
	waitDone(t, done)

	if _, err := node.Listing().GetMyListingBySlug("ron-swanson-shirt"); err != nil {
		t.Fatal(err)
	}

	index, err := node.Listing().GetMyListings()
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(index))
	}

	c, err := cid.Decode(index[0].CID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := node.Listing().GetMyListingByCID(c); err != nil {
		t.Fatal(err)
	}
}

func TestSharedNode_UpdateAndDeleteListing(t *testing.T) {
	node := newSharedLocalNode(t)

	listing1 := newSharedPrivateDistributionListing(t, "ron-swanson-shirt")
	listing2 := newSharedPrivateDistributionListing(t, "bag-of-shit")
	for _, listing := range []*pb.Listing{listing1, listing2} {
		done := make(chan struct{})
		if err := node.Listing().SaveListing(listing, done); err != nil {
			t.Fatal(err)
		}
		waitDone(t, done)
	}

	oldIndex, err := node.Listing().GetMyListings()
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	err = node.Listing().UpdateAllListings(func(listing *pb.Listing) (bool, error) {
		listing.Item.Title = "new title"
		return true, nil
	}, done)
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, done)

	newIndex, err := node.Listing().GetMyListings()
	if err != nil {
		t.Fatal(err)
	}
	if len(newIndex) != 2 {
		t.Fatalf("expected 2 listings, got %d", len(newIndex))
	}
	for _, updated := range newIndex {
		if updated.Title != "new title" {
			t.Fatalf("expected updated title, got %q", updated.Title)
		}
		for _, old := range oldIndex {
			if old.CID == updated.CID {
				t.Fatal("expected updated listing CID to change")
			}
		}
	}

	done = make(chan struct{})
	if err := node.Listing().DeleteListing(listing1.Slug, done); err != nil {
		t.Fatal(err)
	}
	waitDone(t, done)

	if _, err := node.Listing().GetMyListingBySlug(listing1.Slug); err == nil {
		t.Fatal("expected deleted listing lookup to fail")
	}
}

func TestSharedNode_GenerateListingSlug(t *testing.T) {
	node := newSharedLocalNode(t)

	done := make(chan struct{})
	if err := node.Listing().SaveListing(newSharedPrivateDistributionListing(t, "ron-swanson-shirt"), done); err != nil {
		t.Fatal(err)
	}
	waitDone(t, done)

	tests := []struct {
		title    string
		expected string
	}{
		{"test", "test"},
		{"test title", "test-title"},
		{"ron swanson shirt", "ron-swanson-shirt1"},
		{"💩💩💩", "and-x1f4a9-and-x1f4a9-and-x1f4a9"},
		{strings.Repeat("s", 65), strings.Repeat("s", 65)},
		{strings.Repeat("s", 66), strings.Repeat("s", 65)},
	}

	for _, test := range tests {
		err := node.repo.DB().View(func(dbtx database.Tx) error {
			slug, err := node.listingService.generateListingSlug(dbtx, test.title)
			if err != nil {
				return err
			}
			if slug != test.expected {
				t.Fatalf("expected slug %q, got %q", test.expected, slug)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestSharedNode_SetProfileMediaAvatar(t *testing.T) {
	node := newSharedLocalNode(t)

	done := make(chan struct{})
	if err := node.Profile().SetProfile(&models.Profile{Name: "Ron Paul"}, done); err != nil {
		t.Fatal(err)
	}
	waitDone(t, done)

	imgBytes := decodeMediaTestB64(t, mediaTestJPGImageB64)
	result, err := node.Media().SetProfileMedia(context.Background(), contracts.SlotAvatar, imgBytes)
	if err != nil {
		t.Fatal(err)
	}
	if result.Hashes == nil {
		t.Fatal("expected image hashes")
	}
	for label, h := range map[string]string{
		"Tiny": result.Hashes.Tiny, "Small": result.Hashes.Small, "Medium": result.Hashes.Medium,
		"Large": result.Hashes.Large, "Original": result.Hashes.Original,
	} {
		if h == "" {
			t.Fatalf("%s hash is empty", label)
		}
		if _, err := cid.Decode(h); err != nil {
			t.Fatalf("%s hash %q is not a valid CID: %v", label, h, err)
		}
	}
}

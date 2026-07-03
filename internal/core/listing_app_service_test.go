package core

import (
	"testing"

	peer "github.com/libp2p/go-libp2p/core/peer"
	mbznet "github.com/mobazha/mobazha/internal/net"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestListingAppService(t *testing.T, cfg ListingAppServiceConfig) *ListingAppService {
	t.Helper()
	if cfg.DB == nil {
		db, err := repo.MockDB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		cfg.DB = db
	}
	if cfg.BanChecker == nil {
		cfg.BanChecker = mbznet.NewBanManager(nil, nil)
	}
	if cfg.Publish == nil {
		cfg.Publish = noopPublish
	}
	if cfg.ContentStore == nil {
		cfg.ContentStore = &mockContentStore{}
	}
	if cfg.Signer == nil {
		cfg.Signer = newMockSigner()
	}
	if cfg.Keys == nil {
		cfg.Keys = &mockKeyProvider{}
	}
	if cfg.NodeID == "" {
		cfg.NodeID = mustPeerID(t, testVendorPeerID)
	}
	return NewListingAppService(cfg)
}

func TestListingAppService_GetMyListings_Empty(t *testing.T) {
	svc := newTestListingAppService(t, ListingAppServiceConfig{})
	index, err := svc.GetMyListings()
	require.NoError(t, err)
	assert.Empty(t, index)
}

func TestListingAppService_IsGlobalBanned_False(t *testing.T) {
	svc := newTestListingAppService(t, ListingAppServiceConfig{})
	pid := mustPeerID(t, testVendorPeerID)
	assert.False(t, svc.IsGlobalBanned(pid))
}

func TestListingAppService_IsGlobalBanned_True(t *testing.T) {
	bannedPeer := mustPeerID(t, testVendorPeerID)
	bm := mbznet.NewBanManager([]peer.ID{bannedPeer}, nil)
	svc := newTestListingAppService(t, ListingAppServiceConfig{
		BanChecker: bm,
	})
	assert.True(t, svc.IsGlobalBanned(bannedPeer))
}

func TestListingAppService_IsGlobalBanned_NotBanned(t *testing.T) {
	otherPeer, _ := peer.Decode("12D3KooWHHzSeKaY8xuZVzkLbKFfvNgPPeKhFBGrMbNbXRwuFCA5")
	bm := mbznet.NewBanManager([]peer.ID{otherPeer}, nil)
	svc := newTestListingAppService(t, ListingAppServiceConfig{
		BanChecker: bm,
	})
	queryPeer := mustPeerID(t, testVendorPeerID)
	assert.False(t, svc.IsGlobalBanned(queryPeer))
}

func TestListingAppService_BanChecker_BlockedPeer(t *testing.T) {
	blockedPeer := mustPeerID(t, testVendorPeerID)
	bm := mbznet.NewBanManager(nil, []peer.ID{blockedPeer})
	svc := newTestListingAppService(t, ListingAppServiceConfig{
		BanChecker: bm,
	})
	assert.False(t, svc.IsGlobalBanned(blockedPeer), "blocked is not the same as global-banned")
}

func TestListingAppService_NewBanManager_Empty(t *testing.T) {
	bm := mbznet.NewBanManager(nil, nil)
	svc := newTestListingAppService(t, ListingAppServiceConfig{
		BanChecker: bm,
	})
	pid := mustPeerID(t, testVendorPeerID)
	assert.False(t, svc.IsGlobalBanned(pid))
}

func TestValidateListingDraft_RejectsHTTPImageRefs(t *testing.T) {
	svc := newTestListingAppService(t, ListingAppServiceConfig{})
	sl := &pb.SignedListing{
		Listing: &pb.Listing{
			Slug:   "draft-with-http-image",
			Status: models.ListingStatusDraft,
			Item: &pb.Listing_Item{
				Title: "Draft listing",
				Images: []*pb.Image{{
					Filename: "hoodie.png",
					Tiny:     "https://cdn.example.com/tiny.png",
					Small:    "https://cdn.example.com/small.png",
					Medium:   "https://cdn.example.com/medium.png",
					Large:    "https://cdn.example.com/large.png",
					Original: "https://cdn.example.com/original.png",
				}},
			},
			Metadata: &pb.Listing_Metadata{},
			VendorID: &pb.ID{PeerID: testVendorPeerID},
		},
	}

	err := svc.validateListingDraft(sl)
	require.EqualError(t, err, "tiny image hashes must be properly formatted CID")
}

func TestValidateListingDraft_RejectsHTTPVariantImageRefs(t *testing.T) {
	svc := newTestListingAppService(t, ListingAppServiceConfig{})
	sl := &pb.SignedListing{
		Listing: &pb.Listing{
			Slug:   "draft-with-http-variant-image",
			Status: models.ListingStatusDraft,
			Item: &pb.Listing_Item{
				Title: "Draft listing",
				Options: []*pb.Listing_Item_Option{{
					Name: "size",
					Variants: []*pb.Listing_Item_Option_Variant{{
						Name: "M",
						Image: &pb.Image{
							Filename: "variant.png",
							Tiny:     "https://cdn.example.com/tiny.png",
							Small:    "https://cdn.example.com/small.png",
							Medium:   "https://cdn.example.com/medium.png",
							Large:    "https://cdn.example.com/large.png",
							Original: "https://cdn.example.com/original.png",
						},
					}},
				}},
			},
			Metadata: &pb.Listing_Metadata{},
			VendorID: &pb.ID{PeerID: testVendorPeerID},
		},
	}

	err := svc.validateListingDraft(sl)
	require.EqualError(t, err, "tiny image hashes must be properly formatted CID")
}

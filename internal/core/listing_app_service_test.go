package core

import (
	"testing"

	peer "github.com/libp2p/go-libp2p/core/peer"
	mbznet "github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/repo"
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

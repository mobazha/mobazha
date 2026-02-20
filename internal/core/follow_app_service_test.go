package core

import (
	"testing"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestFollowAppService(t *testing.T, cfg FollowAppServiceConfig) *FollowAppService {
	t.Helper()
	if cfg.DB == nil {
		db, err := repo.MockDB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		cfg.DB = db
	}
	if cfg.EventBus == nil {
		cfg.EventBus = events.NewBus()
	}
	if cfg.Messenger == nil {
		cfg.Messenger = &mockMessenger{}
	}
	if cfg.NodeID == "" {
		cfg.NodeID = "test-follow-svc"
	}
	if cfg.UpdateAndSaveProfile == nil {
		cfg.UpdateAndSaveProfile = func(tx database.Tx) error { return nil }
	}
	return NewFollowAppService(cfg)
}

func TestFollowAppService_GetMyFollowers_Empty(t *testing.T) {
	svc := newTestFollowAppService(t, FollowAppServiceConfig{})
	followers, err := svc.GetMyFollowers()
	require.NoError(t, err)
	assert.Empty(t, followers)
}

func TestFollowAppService_GetMyFollowing_Empty(t *testing.T) {
	svc := newTestFollowAppService(t, FollowAppServiceConfig{})
	following, err := svc.GetMyFollowing()
	require.NoError(t, err)
	assert.Empty(t, following)
}

func TestFollowAppService_FollowNode(t *testing.T) {
	msn := &mockMessenger{}
	svc := newTestFollowAppService(t, FollowAppServiceConfig{
		Messenger: msn,
	})

	pid := mustPeerID(t, testVendorPeerID)
	err := svc.FollowNode(pid, nil)
	require.NoError(t, err)

	following, err := svc.GetMyFollowing()
	require.NoError(t, err)
	assert.Contains(t, []string(following), pid.String())

	assert.Len(t, msn.sent, 1, "should have sent one follow message")
}

func TestFollowAppService_FollowNode_Duplicate(t *testing.T) {
	svc := newTestFollowAppService(t, FollowAppServiceConfig{})

	pid := mustPeerID(t, testVendorPeerID)
	err := svc.FollowNode(pid, nil)
	require.NoError(t, err)

	err = svc.FollowNode(pid, nil)
	assert.Error(t, err, "following the same peer twice should error")
	assert.Contains(t, err.Error(), "already following")
}

func TestFollowAppService_UnfollowNode(t *testing.T) {
	msn := &mockMessenger{}
	svc := newTestFollowAppService(t, FollowAppServiceConfig{
		Messenger: msn,
	})

	pid := mustPeerID(t, testVendorPeerID)
	err := svc.FollowNode(pid, nil)
	require.NoError(t, err)

	err = svc.UnfollowNode(pid, nil)
	require.NoError(t, err)

	following, err := svc.GetMyFollowing()
	require.NoError(t, err)
	assert.Empty(t, following)

	assert.Len(t, msn.sent, 2, "should have sent follow + unfollow messages")
}

func TestFollowAppService_UnfollowNode_NotFollowing(t *testing.T) {
	svc := newTestFollowAppService(t, FollowAppServiceConfig{})

	pid := mustPeerID(t, testVendorPeerID)
	err := svc.UnfollowNode(pid, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not following")
}

func TestFollowAppService_FollowsMe(t *testing.T) {
	svc := newTestFollowAppService(t, FollowAppServiceConfig{})

	pid := mustPeerID(t, testVendorPeerID)
	follows, err := svc.FollowsMe(pid)
	require.NoError(t, err)
	assert.False(t, follows)
}

func TestFollowAppService_FollowMultipleNodes(t *testing.T) {
	svc := newTestFollowAppService(t, FollowAppServiceConfig{})

	pid1 := mustPeerID(t, testVendorPeerID)
	pid2, err := peer.Decode("12D3KooWHHzSeKaY8xuZVzkLbKFfvNgPPeKhFBGrMbNbXRwuFCA5")
	require.NoError(t, err)

	require.NoError(t, svc.FollowNode(pid1, nil))
	require.NoError(t, svc.FollowNode(pid2, nil))

	following, err := svc.GetMyFollowing()
	require.NoError(t, err)
	assert.Len(t, following, 2)
}

func TestFollowAppService_FollowNode_DoneChannelClosed(t *testing.T) {
	svc := newTestFollowAppService(t, FollowAppServiceConfig{})

	pid := mustPeerID(t, testVendorPeerID)
	done := make(chan struct{})
	err := svc.FollowNode(pid, done)
	require.NoError(t, err)

	select {
	case <-done:
	default:
		t.Fatal("done channel should be closed after FollowNode")
	}
}

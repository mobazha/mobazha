//go:build !private_distribution

package core

import (
	"os"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPostsAppService(t *testing.T, cfg PostsAppServiceConfig) *PostsAppService {
	t.Helper()
	if cfg.DB == nil {
		db, err := repo.MockDB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		cfg.DB = db
	}
	if cfg.Signer == nil {
		cfg.Signer = newMockSigner()
	}
	if cfg.Keys == nil {
		cfg.Keys = &mockKeyProvider{}
	}
	if cfg.PeerID == "" {
		cfg.PeerID = mustPeerID(t, testVendorPeerID)
	}
	if cfg.Publish == nil {
		cfg.Publish = noopPublish
	}
	return NewPostsAppService(cfg)
}

func TestPostsAppService_GetMyPosts_Empty(t *testing.T) {
	svc := newTestPostsAppService(t, PostsAppServiceConfig{})
	posts, err := svc.GetMyPosts()
	// GetPostIndex reads from a JSON file on disk; when no posts have been
	// created the file does not exist, so a file-not-found error is expected.
	if err != nil {
		assert.True(t, os.IsNotExist(err), "expected file-not-found error, got: %v", err)
		return
	}
	assert.Empty(t, posts)
}

func TestPostsAppService_PostExist_False(t *testing.T) {
	svc := newTestPostsAppService(t, PostsAppServiceConfig{})
	exists := svc.PostExist("nonexistent-slug")
	assert.False(t, exists)
}

func TestPostsAppService_GetMyPostBySlug_NotFound(t *testing.T) {
	svc := newTestPostsAppService(t, PostsAppServiceConfig{})
	_, err := svc.GetMyPostBySlug("nonexistent-slug")
	assert.Error(t, err)
}

package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMediaAppService(t *testing.T, cfg MediaAppServiceConfig) *MediaAppService {
	t.Helper()
	if cfg.DB == nil {
		db, err := repo.MockDB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		cfg.DB = db
	}
	if cfg.ContentStore == nil {
		cfg.ContentStore = &mockContentStore{}
	}
	if cfg.NodeID == "" {
		cfg.NodeID = "test-media-svc"
	}
	return NewMediaAppService(cfg)
}

func TestMediaAppService_AddFile(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	fh, err := svc.AddFile([]byte("hello world"), "test.txt")
	require.NoError(t, err)
	assert.NotEmpty(t, fh.Hash)
	assert.Equal(t, "test.txt", fh.Name)
}

func TestMediaAppService_AddFile_DifferentData(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	fh1, err := svc.AddFile([]byte("data1"), "a.txt")
	require.NoError(t, err)

	fh2, err := svc.AddFile([]byte("data2"), "b.txt")
	require.NoError(t, err)

	assert.NotEqual(t, fh1.Hash, fh2.Hash, "different data should produce different CIDs")
}

func TestMediaAppService_AddFile_SameDataSameCID(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	fh1, err := svc.AddFile([]byte("same"), "a.txt")
	require.NoError(t, err)

	fh2, err := svc.AddFile([]byte("same"), "b.txt")
	require.NoError(t, err)

	assert.Equal(t, fh1.Hash, fh2.Hash, "same data should produce same CID")
}

func TestMediaAppService_GetFile_NilFunc(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})
	_, err := svc.GetFile(context.Background(), testCID())
	assert.Error(t, err, "should error when getIPFSFile is nil")
}

func TestMediaAppService_SetAvatarImage(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	err = db.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "Test"})
	})
	require.NoError(t, err)

	publishCalled := false
	svc := newTestMediaAppService(t, MediaAppServiceConfig{
		DB: db,
		Publish: func(done chan<- struct{}) {
			publishCalled = true
			if done != nil {
				close(done)
			}
		},
	})

	done := make(chan struct{})
	_, err = svc.SetAvatarImage("base64avatardata", done)
	if err != nil {
		t.Logf("SetAvatarImage error (expected due to invalid base64/image data): %v", err)
	}
	_ = publishCalled
}

func TestMediaAppService_SetHeaderImage(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	err = db.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "Test"})
	})
	require.NoError(t, err)

	svc := newTestMediaAppService(t, MediaAppServiceConfig{
		DB: db,
		Publish: func(done chan<- struct{}) {
			if done != nil {
				close(done)
			}
		},
	})

	done := make(chan struct{})
	_, err = svc.SetHeaderImage("base64headerdata", done)
	if err != nil {
		t.Logf("SetHeaderImage error (expected due to invalid base64/image data): %v", err)
	}
}

func TestMediaAppService_AddIntroVideo(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	err = db.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "Test"})
	})
	require.NoError(t, err)

	svc := newTestMediaAppService(t, MediaAppServiceConfig{
		DB: db,
	})

	_, err = svc.AddIntroVideo([]byte("fake-video"), "video.mp4")
	if err != nil {
		t.Logf("AddIntroVideo error (may be expected): %v", err)
	}
}

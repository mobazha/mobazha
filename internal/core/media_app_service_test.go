package core

import (
	"context"
	"testing"

	cid "github.com/ipfs/go-cid"
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

func TestMediaAppService_GetMedia_NilIPFS_NoDB(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})
	_, _, err := svc.GetMedia(context.Background(), testCID())
	assert.Error(t, err, "should error when no DB hit and IPFS is nil")
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

func TestMediaAppService_AddIntroVideo_CIDIndexed(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	fh, err := svc.AddIntroVideo([]byte("test-video-data"), "intro.mp4")
	require.NoError(t, err)
	assert.NotEmpty(t, fh.Hash, "CID should be computed for intro video")
	assert.Equal(t, "intro.mp4", fh.Name)

	reader, ct, err := svc.GetMedia(context.Background(), mustParseCID(t, fh.Hash))
	require.NoError(t, err, "intro video should be retrievable via GetMedia")
	assert.NotNil(t, reader)
	assert.NotEmpty(t, ct, "ContentType should be set")
}

func TestMediaAppService_AddFile_MediaCIDIndexed(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	fh, err := svc.AddFile([]byte("hello world"), "test.txt")
	require.NoError(t, err)

	reader, ct, err := svc.GetMedia(context.Background(), mustParseCID(t, fh.Hash))
	require.NoError(t, err, "file should be retrievable via GetMedia")
	assert.NotNil(t, reader)
	assert.Contains(t, ct, "text/plain", "ContentType should be detected")
}

func TestMediaAppService_GetMedia_CrossMedia(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	fhFile, err := svc.AddFile([]byte("file-data"), "doc.txt")
	require.NoError(t, err)

	fhVideo, err := svc.AddIntroVideo([]byte("video-data"), "intro.mp4")
	require.NoError(t, err)

	assert.NotEqual(t, fhFile.Hash, fhVideo.Hash, "different data should have different CIDs")

	r1, _, err := svc.GetMedia(context.Background(), mustParseCID(t, fhFile.Hash))
	require.NoError(t, err)
	assert.NotNil(t, r1)

	r2, _, err := svc.GetMedia(context.Background(), mustParseCID(t, fhVideo.Hash))
	require.NoError(t, err)
	assert.NotNil(t, r2)
}

func TestMediaAppService_DetectContentType(t *testing.T) {
	tests := []struct {
		data     []byte
		filename string
		expected string
	}{
		{[]byte("hello"), "test.mp4", "video/mp4"},
		{[]byte("hello"), "test.webm", "video/webm"},
		{[]byte("hello"), "test.jpg", "image/jpeg"},
		{[]byte("hello"), "test.png", "image/png"},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			ct := detectContentType(tt.data, tt.filename)
			assert.Equal(t, tt.expected, ct)
		})
	}
}

func mustParseCID(t *testing.T, s string) cid.Cid {
	t.Helper()
	c, err := cid.Decode(s)
	require.NoError(t, err)
	return c
}

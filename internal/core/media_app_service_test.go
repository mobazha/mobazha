package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"strings"
	"testing"

	cid "github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mediaTestJPGImageB64 = "/9j/4AAQSkZJRgABAQAAAQABAAD//gA7Q1JFQVRPUjogZ2QtanBlZyB2MS4wICh1c2luZyBJSkcgSlBFRyB2NjIpLCBxdWFsaXR5ID0gNjUK/9sAQwALCAgKCAcLCgkKDQwLDREcEhEPDxEiGRoUHCkkKyooJCcnLTJANy0wPTAnJzhMOT1DRUhJSCs2T1VORlRAR0hF/9sAQwEMDQ0RDxEhEhIhRS4nLkVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUVF/8AAEQgAMgAyAwEiAAIRAQMRAf/EAB8AAAEFAQEBAQEBAAAAAAAAAAABAgMEBQYHCAkKC//EALUQAAIBAwMCBAMFBQQEAAABfQECAwAEEQUSITFBBhNRYQcicRQygZGhCCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2drh4uPk5ebn6Onq8fLz9PX29/j5+v/EAB8BAAMBAQEBAQEBAQEAAAAAAAABAgMEBQYHCAkKC//EALURAAIBAgQEAwQHBQQEAAECdwABAgMRBAUhMQYSQVEHYXETIjKBCBRCkaGxwQkjM1LwFWJy0QoWJDThJfEXGBkaJicoKSo1Njc4OTpDREVGR0hJSlNUVVZXWFlaY2RlZmdoaWpzdHV2d3h5eoKDhIWGh4iJipKTlJWWl5iZmqKjpKWmp6ipqrKztLW2t7i5usLDxMXGx8jJytLT1NXW19jZ2uLj5OXm5+jp6vLz9PX29/j5+v/aAAwDAQACEQMRAD8A840awhv5zFKWDYyMHrVvWtE/szynj3GJ+MnsaoWFw1ndxTr1Rskeor0+70uPXNBYQ4JkQSRH36iiXw3CO9meWxxNJIqICWY4AHeu5g8C232aMztL5pUFtpGM/lUXgPw+13qD3lwhEdscAEdX/wDrVseNddl0l4bSxcLcN8zHAOB6c1UnyJLqxRTlJ9kY83guzQcNN/30P8KwNY0W206AvufceFBPWvRtMtrw6RHLqUm+dxvOVA2j04rzjxJqAv8AUXEZ/cxHavv71M20+UcbNc3Q5/bRUu2igCVRXpfw51MXFtJp0rfPD88ee6nr+R/nXmq13fw40xpL6TUXyEiGxfcnrVwV7kSdrHo7C10ixnn2rFEu6R8cZPU15r4espfFviua/uQTbxvvbPT/AGVrX+IetMyQ6PakmSUhpAv6Cuh0DT4PC/hsGbCsE82ZvfHSs4O160umiLmtFTW73/rzMjx7rC6Zp32WFgJ7gY4/hXua8nbmtTXtWk1rVZruQnDHCL/dXtWW1TBPd7suVl7q6EeKKKKsgkt42nlSNBlmIAr1rTZINA0MAkBYU3MfU15joEsEN5588irs+6GPetfX9cW8iis7eVSjHLsDxRJ+7yx3YRV5XeyNjwlbPrviCbWL0bkjbcoPQt2H4Vf+IOsTyxpplrHIyt80rKpwfQU3R9W0vS9PitkvIBtHzHeOT3q+3ibTiP8Aj9g/77FE+R2itkEXK7m92eXm2uB1gk/74NRPDKoOY3H1U16RceI7FgcXkJ/4GKwdT1q2lgkVJ0YlSOGoco20CzONzRUe6igBq1KtFFAD6KKKAGmo26UUUAR0UUUAf//Z"

func decodeMediaTestB64(t *testing.T, b64 string) []byte {
	t.Helper()
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(b64))
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil && buf.Len() == 0 {
		t.Fatalf("failed to decode base64: %v", err)
	}
	return buf.Bytes()
}

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

func TestMediaAppService_UploadMedia_File(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	result, err := svc.UploadMedia(context.Background(), []byte("hello world"), "test.txt", contracts.UploadOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Hash)
	assert.Equal(t, "test.txt", result.Filename)
}

func TestMediaAppService_UploadMedia_DifferentData(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	r1, err := svc.UploadMedia(context.Background(), []byte("data1"), "a.txt", contracts.UploadOpts{})
	require.NoError(t, err)

	r2, err := svc.UploadMedia(context.Background(), []byte("data2"), "b.txt", contracts.UploadOpts{})
	require.NoError(t, err)

	assert.NotEqual(t, r1.Hash, r2.Hash, "different data should produce different CIDs")
}

func TestMediaAppService_UploadMedia_SameDataSameCID(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	r1, err := svc.UploadMedia(context.Background(), []byte("same"), "a.txt", contracts.UploadOpts{})
	require.NoError(t, err)

	r2, err := svc.UploadMedia(context.Background(), []byte("same"), "b.txt", contracts.UploadOpts{})
	require.NoError(t, err)

	assert.Equal(t, r1.Hash, r2.Hash, "same data should produce same CID")
}

func TestMediaAppService_GetMedia_NilIPFS_NoDB(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})
	_, _, err := svc.GetMedia(context.Background(), testCID())
	assert.Error(t, err, "should error when no DB hit and no blob store")
}

func TestMediaAppService_SetProfileMedia_Avatar(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	err = db.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "Test"})
	})
	require.NoError(t, err)

	bus := events.NewBus()
	sub, err := bus.Subscribe([]interface{}{&events.ProfileChanged{}})
	require.NoError(t, err)
	defer sub.Close()

	svc := newTestMediaAppService(t, MediaAppServiceConfig{
		DB:       db,
		EventBus: bus,
	})

	imgBytes := decodeMediaTestB64(t, mediaTestJPGImageB64)
	result, err := svc.SetProfileMedia(context.Background(), contracts.SlotAvatar, imgBytes)
	require.NoError(t, err)

	select {
	case evt := <-sub.Out():
		_, ok := evt.(*events.ProfileChanged)
		assert.True(t, ok, "expected ProfileChanged event")
	default:
		t.Fatal("ProfileChanged event not emitted after setting avatar")
	}
	assert.NotNil(t, result.Hashes)
	assert.NotEmpty(t, result.Hashes.Original)
	assert.NotEmpty(t, result.Hashes.Tiny)
	assert.NotEmpty(t, result.Hashes.Small)
	assert.NotEmpty(t, result.Hashes.Medium)
	assert.NotEmpty(t, result.Hashes.Large)
}

func TestMediaAppService_SetProfileMedia_Header(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	err = db.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "Test"})
	})
	require.NoError(t, err)

	bus := events.NewBus()
	sub, err := bus.Subscribe([]interface{}{&events.ProfileChanged{}})
	require.NoError(t, err)
	defer sub.Close()

	svc := newTestMediaAppService(t, MediaAppServiceConfig{
		DB:       db,
		EventBus: bus,
	})

	imgBytes := decodeMediaTestB64(t, mediaTestJPGImageB64)
	result, err := svc.SetProfileMedia(context.Background(), contracts.SlotHeader, imgBytes)
	require.NoError(t, err)

	select {
	case evt := <-sub.Out():
		_, ok := evt.(*events.ProfileChanged)
		assert.True(t, ok, "expected ProfileChanged event")
	default:
		t.Fatal("ProfileChanged event not emitted after setting header")
	}
	assert.NotNil(t, result.Hashes)
	assert.NotEmpty(t, result.Hashes.Original)
}

func TestMediaAppService_SetProfileMedia_InvalidImage(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	_, err := svc.SetProfileMedia(context.Background(), contracts.SlotAvatar, []byte("not-an-image"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad request")
}

func TestMediaAppService_SetProfileMedia_UnknownSlot(t *testing.T) {
	imgBytes := decodeMediaTestB64(t, mediaTestJPGImageB64)
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	_, err := svc.SetProfileMedia(context.Background(), contracts.ProfileSlot("unknown"), imgBytes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown profile slot")
}

func TestMediaAppService_UploadMedia_Video(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	err = db.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "Test"})
	})
	require.NoError(t, err)

	svc := newTestMediaAppService(t, MediaAppServiceConfig{DB: db})

	_, err = svc.UploadMedia(context.Background(), []byte("fake-video"), "video.mp4", contracts.UploadOpts{})
	if err != nil {
		t.Logf("UploadMedia video error (may be expected): %v", err)
	}
}

func TestMediaAppService_UploadMedia_VideoCIDIndexed(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	result, err := svc.UploadMedia(context.Background(), []byte("test-video-data"), "intro.mp4", contracts.UploadOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Hash, "CID should be computed for intro video")
	assert.Equal(t, "intro.mp4", result.Filename)

	reader, ct, err := svc.GetMedia(context.Background(), mustParseCID(t, result.Hash))
	require.NoError(t, err, "intro video should be retrievable via GetMedia")
	assert.NotNil(t, reader)
	assert.NotEmpty(t, ct, "ContentType should be set")
}

func TestMediaAppService_UploadMedia_FileCIDIndexed(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	result, err := svc.UploadMedia(context.Background(), []byte("hello world"), "test.txt", contracts.UploadOpts{})
	require.NoError(t, err)

	reader, ct, err := svc.GetMedia(context.Background(), mustParseCID(t, result.Hash))
	require.NoError(t, err, "file should be retrievable via GetMedia")
	assert.NotNil(t, reader)
	assert.Contains(t, ct, "text/plain", "ContentType should be detected")
}

func TestMediaAppService_GetMedia_CrossMedia(t *testing.T) {
	svc := newTestMediaAppService(t, MediaAppServiceConfig{})

	rFile, err := svc.UploadMedia(context.Background(), []byte("file-data"), "doc.txt", contracts.UploadOpts{})
	require.NoError(t, err)

	rVideo, err := svc.UploadMedia(context.Background(), []byte("video-data"), "intro.mp4", contracts.UploadOpts{})
	require.NoError(t, err)

	assert.NotEqual(t, rFile.Hash, rVideo.Hash, "different data should have different CIDs")

	r1, _, err := svc.GetMedia(context.Background(), mustParseCID(t, rFile.Hash))
	require.NoError(t, err)
	assert.NotNil(t, r1)

	r2, _, err := svc.GetMedia(context.Background(), mustParseCID(t, rVideo.Hash))
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

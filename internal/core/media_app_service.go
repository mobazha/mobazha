package core

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// ── Callback Types ──────────────────────────────────────────────

type PublishFunc func(done chan<- struct{})

type PublishFileFunc func(ctx context.Context, c cid.Cid, done chan<- struct{})

const defaultMaxUploadBytes = 10 << 20 // 10 MB — client should resize to ≤2048px before upload

// ── MediaAppService ─────────────────────────────────────────────

type MediaAppService struct {
	db           database.Database
	contentStore contracts.ContentStore
	blobStore    contracts.BlobStore
	nodeID       string

	publish     PublishFunc
	publishFile PublishFileFunc
	eventBus    events.Bus
}

type MediaAppServiceConfig struct {
	DB           database.Database
	ContentStore contracts.ContentStore
	BlobStore    contracts.BlobStore
	NodeID       string

	Publish     PublishFunc
	PublishFile PublishFileFunc
	EventBus    events.Bus
}

func NewMediaAppService(cfg MediaAppServiceConfig) *MediaAppService {
	return &MediaAppService{
		db:           cfg.DB,
		contentStore: cfg.ContentStore,
		blobStore:    cfg.BlobStore,
		nodeID:       cfg.NodeID,
		publish:      cfg.Publish,
		publishFile:  cfg.PublishFile,
		eventBus:     cfg.EventBus,
	}
}

// ── UploadMedia ─────────────────────────────────────────────────

func (s *MediaAppService) UploadMedia(ctx context.Context, data []byte, filename string, opts contracts.UploadOpts) (*contracts.UploadResult, error) {
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxUploadBytes
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: file exceeds %d bytes", coreiface.ErrBadRequest, maxBytes)
	}

	if opts.Variants {
		return s.uploadWithVariants(ctx, data, filename)
	}
	return s.uploadSingle(ctx, data, filename)
}

func (s *MediaAppService) uploadSingle(ctx context.Context, data []byte, filename string) (*contracts.UploadResult, error) {
	c, err := s.contentStore.ComputeCID(data)
	if err != nil {
		return nil, err
	}
	key := contracts.CanonicalCID(c)
	ct := detectContentType(data, filename)

	if s.blobStore != nil {
		if err := s.blobStore.Put(ctx, key, data, ct); err != nil {
			return nil, fmt.Errorf("blobstore put: %w", err)
		}
		if err := s.saveMetadataOnly(c.String(), "file", "", filename, ct); err != nil {
			return nil, err
		}
	} else {
		if err := s.db.Update(func(dbtx database.Tx) error {
			if err := dbtx.SetUploadedFile(models.UploadedFile{
				Name:      filename,
				FileBytes: data,
			}); err != nil {
				return err
			}
			return dbtx.IndexMediaCID(c.String(), "file", "", filename, ct)
		}); err != nil {
			return nil, err
		}
	}

	if s.publishFile != nil {
		s.publishFile(context.Background(), c, nil)
	}

	return &contracts.UploadResult{
		Hash:     c.String(),
		Filename: filename,
		CDNURL:   s.publicURL(key),
	}, nil
}

func (s *MediaAppService) uploadWithVariants(ctx context.Context, data []byte, filename string) (*contracts.UploadResult, error) {
	img, err := decodeImageBytes(data)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid image: %s", coreiface.ErrBadRequest, err.Error())
	}

	const baseWidth, baseHeight = 120, 120

	t, err := s.storeResizedImage(ctx, img, 1*baseWidth, 1*baseHeight, filename, models.ImageSizeTiny)
	if err != nil {
		return nil, err
	}
	sm, err := s.storeResizedImage(ctx, img, 2*baseWidth, 2*baseHeight, filename, models.ImageSizeSmall)
	if err != nil {
		return nil, err
	}
	m, err := s.storeResizedImage(ctx, img, 4*baseWidth, 4*baseHeight, filename, models.ImageSizeMedium)
	if err != nil {
		return nil, err
	}
	l, err := s.storeResizedImage(ctx, img, 8*baseWidth, 8*baseHeight, filename, models.ImageSizeLarge)
	if err != nil {
		return nil, err
	}

	var origBuf bytes.Buffer
	if err := jpeg.Encode(&origBuf, img, nil); err != nil {
		return nil, err
	}
	o, err := s.storeImageBytes(ctx, origBuf.Bytes(), filename, models.ImageSizeOriginal)
	if err != nil {
		return nil, err
	}

	hashes := &models.ImageHashes{
		Tiny: t.String(), Small: sm.String(), Medium: m.String(),
		Large: l.String(), Original: o.String(), Filename: filename,
	}
	return &contracts.UploadResult{
		Hash:     o.String(),
		Filename: filename,
		Hashes:   hashes,
		CDNURL:   s.publicURL(contracts.CanonicalCID(o)),
	}, nil
}

// ── GetMedia ────────────────────────────────────────────────────

func (s *MediaAppService) GetMedia(ctx context.Context, c cid.Cid) (io.ReadSeeker, string, error) {
	// Level 1: BlobStore
	if s.blobStore != nil {
		key := contracts.CanonicalCID(c)
		rc, ct, err := s.blobStore.Get(ctx, key)
		if err == nil {
			data, readErr := io.ReadAll(rc)
			rc.Close()
			if readErr == nil {
				return bytes.NewReader(data), ct, nil
			}
			log.Warningf("BlobStore read error for %s, falling back to DB: %v", key, readErr)
		}
	}

	// Level 2: DB (legacy)
	var data []byte
	var contentType string
	dbErr := s.db.View(func(tx database.Tx) error {
		var err error
		data, contentType, err = tx.GetMediaByCID(c.String())
		return err
	})
	if dbErr == nil && len(data) > 0 {
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		// Async backfill to BlobStore for future hits
		if s.blobStore != nil {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				key := contracts.CanonicalCID(c)
				if err := s.blobStore.Put(ctx, key, data, contentType); err != nil {
					log.Warningf("BlobStore backfill failed for %s: %v", key, err)
				}
			}()
		}
		return bytes.NewReader(data), contentType, nil
	}

	if dbErr != nil {
		return nil, "", fmt.Errorf("media not found (db error: %w)", dbErr)
	}
	return nil, "", fmt.Errorf("%w: media %s not found in BlobStore or DB", coreiface.ErrNotFound, c)
}

// ── SetProfileMedia ─────────────────────────────────────────────

func (s *MediaAppService) SetProfileMedia(ctx context.Context, slot contracts.ProfileSlot, imageData []byte) (*contracts.UploadResult, error) {
	img, err := decodeImageBytes(imageData)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid image: %s", coreiface.ErrBadRequest, err.Error())
	}

	var baseW, baseH int
	switch slot {
	case contracts.SlotAvatar:
		baseW, baseH = 60, 60
	case contracts.SlotHeader:
		baseW, baseH = 315, 90
	default:
		return nil, fmt.Errorf("%w: unknown profile slot: %s", coreiface.ErrBadRequest, slot)
	}

	name := string(slot)

	var hashes models.ImageHashes
	var profileUpdated bool

	err = s.db.Update(func(tx database.Tx) error {
		var innerErr error
		hashes, innerErr = s.resizeAndStoreProfileImage(ctx, tx, img, name, baseW, baseH)
		if innerErr != nil {
			return innerErr
		}
		profile, pErr := tx.GetProfile()
		if pErr != nil {
			// Profile may not exist yet (initial setup). Images are stored;
			// profile association will happen on next upload after profile creation.
			return nil
		}
		switch slot {
		case contracts.SlotAvatar:
			profile.AvatarHashes = hashes
		case contracts.SlotHeader:
			profile.HeaderHashes = hashes
		}
		profileUpdated = true
		return tx.SetProfile(profile)
	})
	if err != nil {
		return nil, err
	}

	if profileUpdated {
		if s.publish != nil {
			s.publish(nil)
		}
		if s.eventBus != nil {
			s.eventBus.Emit(&events.ProfileChanged{})
		}
	}

	return &contracts.UploadResult{
		Hash:     hashes.Original,
		Filename: name,
		Hashes:   &hashes,
		CDNURL:   s.publicURL(hashes.Original),
	}, nil
}

// ── GetProfileMedia ─────────────────────────────────────────────

func (s *MediaAppService) GetProfileMedia(_ context.Context, peerID peer.ID, slot contracts.ProfileSlot, size models.ImageSize, _ bool) (io.ReadSeeker, error) {
	name := string(slot)
	if peerID.String() == s.nodeID {
		return s.getLocalImageByName(size, name)
	}
	return nil, fmt.Errorf("%w: remote profile media not available (IPFS retired)", coreiface.ErrNotFound)
}

// ── Internal Helpers ────────────────────────────────────────────

func (s *MediaAppService) getLocalImageByName(size models.ImageSize, name string) (io.ReadSeeker, error) {
	var data []byte
	err := s.db.View(func(tx database.Tx) error {
		var dbErr error
		data, dbErr = tx.GetImageByName(size, name)
		return dbErr
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %s not found", coreiface.ErrNotFound, name)
	}
	return bytes.NewReader(data), nil
}

func (s *MediaAppService) resizeAndStoreProfileImage(ctx context.Context, dbtx database.Tx, img image.Image, name string, baseW, baseH int) (models.ImageHashes, error) {
	t, err := s.storeResizedProfileImage(ctx, dbtx, img, 1*baseW, 1*baseH, name, models.ImageSizeTiny)
	if err != nil {
		return models.ImageHashes{}, err
	}
	sm, err := s.storeResizedProfileImage(ctx, dbtx, img, 2*baseW, 2*baseH, name, models.ImageSizeSmall)
	if err != nil {
		return models.ImageHashes{}, err
	}
	m, err := s.storeResizedProfileImage(ctx, dbtx, img, 4*baseW, 4*baseH, name, models.ImageSizeMedium)
	if err != nil {
		return models.ImageHashes{}, err
	}
	l, err := s.storeResizedProfileImage(ctx, dbtx, img, 8*baseW, 8*baseH, name, models.ImageSizeLarge)
	if err != nil {
		return models.ImageHashes{}, err
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		return models.ImageHashes{}, err
	}
	o, err := s.storeProfileImageBytes(ctx, dbtx, buf.Bytes(), name, models.ImageSizeOriginal)
	if err != nil {
		return models.ImageHashes{}, err
	}

	return models.ImageHashes{
		Tiny: t.String(), Small: sm.String(), Medium: m.String(),
		Large: l.String(), Original: o.String(), Filename: name,
	}, nil
}

// storeResizedImage resizes an image and stores it (BlobStore path, no dbtx needed).
func (s *MediaAppService) storeResizedImage(ctx context.Context, img image.Image, w, h int, name string, size models.ImageSize) (cid.Cid, error) {
	width, height := getImageAttributes(w, h, img.Bounds().Max.X, img.Bounds().Max.Y)
	newImg := imaging.Resize(img, width, height, imaging.Lanczos)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, newImg, &jpeg.Options{Quality: 100}); err != nil {
		return cid.Cid{}, err
	}
	return s.storeImageBytes(ctx, buf.Bytes(), name, size)
}

// storeImageBytes computes CID and stores via BlobStore or DB.
func (s *MediaAppService) storeImageBytes(ctx context.Context, data []byte, name string, size models.ImageSize) (cid.Cid, error) {
	c, err := s.contentStore.ComputeCID(data)
	if err != nil {
		return cid.Cid{}, err
	}

	if s.blobStore != nil {
		key := contracts.CanonicalCID(c)
		if err := s.blobStore.Put(ctx, key, data, "image/jpeg"); err != nil {
			return cid.Cid{}, fmt.Errorf("blobstore put: %w", err)
		}
		if err := s.saveMetadataOnly(c.String(), "image", string(size), name, "image/jpeg"); err != nil {
			return cid.Cid{}, err
		}
	} else {
		if err := s.db.Update(func(dbtx database.Tx) error {
			if err := dbtx.SetImage(models.Image{
				Name:       name,
				Size:       size,
				ImageBytes: data,
			}); err != nil {
				return err
			}
			return dbtx.IndexMediaCID(c.String(), "image", string(size), name, "image/jpeg")
		}); err != nil {
			return cid.Cid{}, err
		}
	}
	return c, nil
}

// storeResizedProfileImage resizes and stores with a DB transaction (for profile updates).
func (s *MediaAppService) storeResizedProfileImage(ctx context.Context, dbtx database.Tx, img image.Image, w, h int, name string, size models.ImageSize) (cid.Cid, error) {
	width, height := getImageAttributes(w, h, img.Bounds().Max.X, img.Bounds().Max.Y)
	newImg := imaging.Resize(img, width, height, imaging.Lanczos)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, newImg, &jpeg.Options{Quality: 100}); err != nil {
		return cid.Cid{}, err
	}
	return s.storeProfileImageBytes(ctx, dbtx, buf.Bytes(), name, size)
}

// storeProfileImageBytes stores a profile image variant within a DB transaction.
func (s *MediaAppService) storeProfileImageBytes(ctx context.Context, dbtx database.Tx, data []byte, name string, size models.ImageSize) (cid.Cid, error) {
	c, err := s.contentStore.ComputeCID(data)
	if err != nil {
		return cid.Cid{}, err
	}

	if s.blobStore != nil {
		key := contracts.CanonicalCID(c)
		if err := s.blobStore.Put(ctx, key, data, "image/jpeg"); err != nil {
			return cid.Cid{}, fmt.Errorf("blobstore put: %w", err)
		}
	}

	// Profile images always write to DB too (for local GetProfileMedia reads).
	if err := dbtx.SetImage(models.Image{
		Name:       name,
		Size:       size,
		ImageBytes: data,
	}); err != nil {
		return cid.Cid{}, err
	}
	if err := dbtx.IndexMediaCID(c.String(), "image", string(size), name, "image/jpeg"); err != nil {
		return cid.Cid{}, err
	}
	return c, nil
}

func (s *MediaAppService) saveMetadataOnly(cidStr, mediaType, sizeStr, name, contentType string) error {
	return s.db.Update(func(dbtx database.Tx) error {
		return dbtx.IndexMediaCID(cidStr, mediaType, sizeStr, name, contentType)
	})
}

func (s *MediaAppService) publicURL(key string) string {
	if s.blobStore != nil {
		return s.blobStore.PublicURL(key)
	}
	return ""
}

// ── Utility Functions ───────────────────────────────────────────

func decodeImageBytes(data []byte) (image.Image, error) {
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}
	return img, nil
}

func getImageAttributes(targetWidth, targetHeight, imgWidth, imgHeight int) (width, height int) {
	targetRatio := float32(targetWidth) / float32(targetHeight)
	imageRatio := float32(imgWidth) / float32(imgHeight)
	var h, w float32
	if imageRatio > targetRatio {
		h = float32(targetHeight)
		w = float32(targetHeight) * imageRatio
	} else {
		w = float32(targetWidth)
		h = float32(targetWidth) * (float32(imgHeight) / float32(imgWidth))
	}
	return int(w), int(h)
}

func detectContentType(data []byte, filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(lower, ".webm"):
		return "video/webm"
	case strings.HasSuffix(lower, ".mov"):
		return "video/quicktime"
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	}
	return http.DetectContentType(data)
}

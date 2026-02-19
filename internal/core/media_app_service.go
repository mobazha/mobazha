package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"strings"

	"github.com/disintegration/imaging"
	ipath "github.com/ipfs/boxo/path"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// ── IPFS Callback Types ─────────────────────────────────────────

type GetIPFSFileFunc func(ctx context.Context, path ipath.Path) (io.ReadSeeker, error)

type FetchIPNSRecordFunc func(ctx context.Context, pid peer.ID, useCache bool) (*ipns.Record, error)

type PublishFunc func(done chan<- struct{})

type PublishFileFunc func(ctx context.Context, c cid.Cid, done chan<- struct{})

// ── MediaAppService ─────────────────────────────────────────────

type MediaAppService struct {
	db           database.Database
	contentStore contracts.ContentStore
	nodeID       string

	getIPFSFile      GetIPFSFileFunc
	fetchIPNSRecord  FetchIPNSRecordFunc
	publish          PublishFunc
	publishFile      PublishFileFunc
}

type MediaAppServiceConfig struct {
	DB           database.Database
	ContentStore contracts.ContentStore
	NodeID       string

	GetIPFSFile     GetIPFSFileFunc
	FetchIPNSRecord FetchIPNSRecordFunc
	Publish         PublishFunc
	PublishFile     PublishFileFunc
}

func NewMediaAppService(cfg MediaAppServiceConfig) *MediaAppService {
	return &MediaAppService{
		db:              cfg.DB,
		contentStore:    cfg.ContentStore,
		nodeID:          cfg.NodeID,
		getIPFSFile:     cfg.GetIPFSFile,
		fetchIPNSRecord: cfg.FetchIPNSRecord,
		publish:         cfg.Publish,
		publishFile:     cfg.PublishFile,
	}
}

// ── File Operations ─────────────────────────────────────────────

func (s *MediaAppService) GetFile(ctx context.Context, c cid.Cid) (io.ReadSeeker, error) {
	if s.getIPFSFile == nil {
		return nil, fmt.Errorf("IPFS file reader not available")
	}
	pth := ipath.FromCid(c)
	return s.getIPFSFile(ctx, pth)
}

func (s *MediaAppService) AddFile(fileData []byte, filename string) (models.FileHash, error) {
	c, err := s.contentStore.ComputeCID(fileData)
	if err != nil {
		return models.FileHash{}, err
	}
	if s.publishFile != nil {
		s.publishFile(context.Background(), c, nil)
	}
	return models.FileHash{Hash: c.String(), Name: filename}, nil
}

func (s *MediaAppService) AddIntroVideo(fileData []byte, filename string) (models.FileHash, error) {
	err := s.db.Update(func(dbtx database.Tx) error {
		return dbtx.SetIntroVideo(models.IntroVideo{
			VideoBytes: fileData,
			Name:       filename,
		})
	})
	if err != nil {
		return models.FileHash{}, err
	}
	c, err := s.contentStore.ComputeCID(fileData)
	return models.FileHash{Hash: c.String(), Name: filename}, err
}

// ── Image Read Operations ───────────────────────────────────────

func (s *MediaAppService) GetImage(ctx context.Context, c cid.Cid) (io.ReadSeeker, error) {
	return s.GetFile(ctx, c)
}

func (s *MediaAppService) GetAvatar(ctx context.Context, peerID peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
	return s.getIPFSImageByName(ctx, peerID, size, "avatar", useCache)
}

func (s *MediaAppService) GetHeader(ctx context.Context, peerID peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
	return s.getIPFSImageByName(ctx, peerID, size, "header", useCache)
}

func (s *MediaAppService) getIPFSImageByName(ctx context.Context, peerID peer.ID, size models.ImageSize, name string, useCache bool) (io.ReadSeeker, error) {
	if s.fetchIPNSRecord == nil || s.getIPFSFile == nil {
		return nil, fmt.Errorf("IPFS infrastructure not available")
	}
	record, err := s.fetchIPNSRecord(ctx, peerID, useCache)
	if err != nil {
		return nil, err
	}
	pth, err := record.Value()
	if err != nil {
		return nil, err
	}
	pth1, err := ipath.Join(pth, "images", string(size), name)
	if err != nil {
		return nil, err
	}
	nd, err := s.getIPFSFile(ctx, pth1)
	if err != nil {
		return nil, fmt.Errorf("%w: %s not found", coreiface.ErrNotFound, name)
	}
	return nd, nil
}

// ── Image Write Operations ──────────────────────────────────────

func (s *MediaAppService) SetAvatarImage(base64ImageData string, done chan struct{}) (models.ImageHashes, error) {
	var (
		hashes         models.ImageHashes
		profileUpdated bool
		err            error
	)
	err = s.db.Update(func(tx database.Tx) error {
		hashes, err = s.resizeAndAddImage(tx, base64ImageData, "avatar", 60, 60)
		if err != nil {
			return err
		}
		profile, err := tx.GetProfile()
		if err == nil {
			profile.AvatarHashes = hashes
			profileUpdated = true
			return tx.SetProfile(profile)
		}
		return nil
	})
	if err != nil {
		maybeCloseDone(done)
		return models.ImageHashes{}, err
	}
	if profileUpdated && s.publish != nil {
		s.publish(done)
	}
	return hashes, nil
}

func (s *MediaAppService) SetHeaderImage(base64ImageData string, done chan struct{}) (models.ImageHashes, error) {
	var (
		hashes         models.ImageHashes
		err            error
		profileUpdated bool
	)
	err = s.db.Update(func(tx database.Tx) error {
		hashes, err = s.resizeAndAddImage(tx, base64ImageData, "header", 315, 90)
		if err != nil {
			return err
		}
		profile, err := tx.GetProfile()
		if err == nil {
			profile.HeaderHashes = hashes
			profileUpdated = true
			return tx.SetProfile(profile)
		}
		return nil
	})
	if err != nil {
		maybeCloseDone(done)
		return models.ImageHashes{}, err
	}
	if profileUpdated && s.publish != nil {
		s.publish(done)
	}
	return hashes, nil
}

func (s *MediaAppService) SetImage(base64ImageData string, filename string) (models.FileHash, error) {
	img, err := decodeImageData(base64ImageData)
	if err != nil {
		return models.FileHash{}, fmt.Errorf("%w: invalid image: %s", coreiface.ErrBadRequest, err.Error())
	}
	var buf bytes.Buffer
	const maxImageSize = 10 * 1000 * 1000
	if err := imageToJpeg(&buf, img, maxImageSize); err != nil {
		return models.FileHash{}, err
	}
	return s.AddFile(buf.Bytes(), filename)
}

func (s *MediaAppService) SetProductImage(base64ImageData string, filename string) (models.ImageHashes, error) {
	var (
		hashes models.ImageHashes
		err    error
	)
	err = s.db.Update(func(tx database.Tx) error {
		hashes, err = s.resizeAndAddImage(tx, base64ImageData, filename, 120, 120)
		return err
	})
	return hashes, err
}

// ── Internal Helpers ────────────────────────────────────────────

func (s *MediaAppService) resizeAndAddImage(dbtx database.Tx, base64ImageData, filename string, baseWidth, baseHeight int) (models.ImageHashes, error) {
	img, err := decodeImageData(base64ImageData)
	if err != nil {
		return models.ImageHashes{}, fmt.Errorf("%w: invalid image: %s", coreiface.ErrBadRequest, err.Error())
	}

	t, err := s.addResizedImage(dbtx, img, 1*baseWidth, 1*baseHeight, filename, models.ImageSizeTiny)
	if err != nil {
		return models.ImageHashes{}, err
	}
	sm, err := s.addResizedImage(dbtx, img, 2*baseWidth, 2*baseHeight, filename, models.ImageSizeSmall)
	if err != nil {
		return models.ImageHashes{}, err
	}
	m, err := s.addResizedImage(dbtx, img, 4*baseWidth, 4*baseHeight, filename, models.ImageSizeMedium)
	if err != nil {
		return models.ImageHashes{}, err
	}
	l, err := s.addResizedImage(dbtx, img, 8*baseWidth, 8*baseHeight, filename, models.ImageSizeLarge)
	if err != nil {
		return models.ImageHashes{}, err
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		return models.ImageHashes{}, err
	}

	o, err := s.addImage(dbtx, models.Image{
		Name:       filename,
		Size:       models.ImageSizeOriginal,
		ImageBytes: buf.Bytes(),
	})
	if err != nil {
		return models.ImageHashes{}, err
	}

	return models.ImageHashes{Tiny: t.String(), Small: sm.String(), Medium: m.String(), Large: l.String(), Original: o.String(), Filename: filename}, nil
}

func (s *MediaAppService) addImage(dbtx database.Tx, img models.Image) (cid.Cid, error) {
	if err := dbtx.SetImage(img); err != nil {
		return cid.Cid{}, err
	}
	return s.contentStore.ComputeCID(img.ImageBytes)
}

func (s *MediaAppService) addResizedImage(dbtx database.Tx, img image.Image, w, h int, name string, size models.ImageSize) (cid.Cid, error) {
	width, height := getImageAttributes(w, h, img.Bounds().Max.X, img.Bounds().Max.Y)
	newImg := imaging.Resize(img, width, height, imaging.Lanczos)

	var buf bytes.Buffer
	q := &jpeg.Options{Quality: 100}
	if err := jpeg.Encode(&buf, newImg, q); err != nil {
		return cid.Cid{}, err
	}

	return s.addImage(dbtx, models.Image{
		ImageBytes: buf.Bytes(),
		Size:       size,
		Name:       name,
	})
}

func decodeImageData(base64ImageData string) (image.Image, error) {
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
	img, err := imaging.Decode(reader, imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}
	return img, err
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


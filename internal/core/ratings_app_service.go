package core

import (
	"context"
	"encoding/json"
	"fmt"

	ipath "github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/database/ffsqlite"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/request"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/encoding/protojson"
)

type GetRatingIndexFromNetDBFunc func(peerID string, reqCtx *request.Context) (models.RatingIndex, error)

type RatingsAppService struct {
	db              database.Database
	contentStore    contracts.ContentStore
	fetchIPNSRecord FetchIPNSRecordFunc
	getRatingIndex  GetRatingIndexFromNetDBFunc
}

type RatingsAppServiceConfig struct {
	DB              database.Database
	ContentStore    contracts.ContentStore
	FetchIPNSRecord FetchIPNSRecordFunc
	GetRatingIndex  GetRatingIndexFromNetDBFunc
}

func NewRatingsAppService(cfg RatingsAppServiceConfig) *RatingsAppService {
	return &RatingsAppService{
		db:              cfg.DB,
		contentStore:    cfg.ContentStore,
		fetchIPNSRecord: cfg.FetchIPNSRecord,
		getRatingIndex:  cfg.GetRatingIndex,
	}
}

func (s *RatingsAppService) GetMyRatings() (models.RatingIndex, error) {
	var (
		index models.RatingIndex
		err   error
	)
	err = s.db.View(func(tx database.Tx) error {
		index, err = tx.GetRatingIndex()
		if err != nil {
			return fmt.Errorf("%w: rating index not found", coreiface.ErrNotFound)
		}
		return nil
	})
	return index, err
}

func (s *RatingsAppService) GetRatings(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (models.RatingIndex, error) {
	getDataFromIPNS := func() (models.RatingIndex, error) {
		if s.fetchIPNSRecord == nil {
			return nil, fmt.Errorf("IPNS resolution not available")
		}
		record, err := s.fetchIPNSRecord(ctx, peerID, useCache)
		if err != nil {
			return nil, err
		}
		pth, err := record.Value()
		if err != nil {
			return nil, err
		}
		pth1, err := ipath.Join(pth, ffsqlite.RatingIndexFile)
		if err != nil {
			return nil, err
		}
		indexBytes, err := s.contentStore.Cat(ctx, pth1.String())
		if err != nil {
			return nil, err
		}
		var index models.RatingIndex
		if err := json.Unmarshal(indexBytes, &index); err != nil {
			return nil, err
		}
		return index, nil
	}

	if preferIPNS || s.getRatingIndex == nil {
		index, err := getDataFromIPNS()
		if err != nil && s.getRatingIndex != nil {
			return s.getRatingIndex(peerID.String(), reqCtx)
		}
		return index, err
	}

	index, err := s.getRatingIndex(peerID.String(), reqCtx)
	if err != nil {
		return getDataFromIPNS()
	}
	return index, err
}

func (s *RatingsAppService) GetRating(ctx context.Context, c cid.Cid) (*pb.Rating, error) {
	ratingBytes, err := s.contentStore.Cat(ctx, ipath.FromCid(c).String())
	if err != nil {
		return nil, err
	}
	var rating pb.Rating
	if err := protojson.Unmarshal(ratingBytes, &rating); err != nil {
		return nil, fmt.Errorf("%w: %s", coreiface.ErrNotFound, err)
	}
	if err := utils.ValidateRating(&rating); err != nil {
		return nil, fmt.Errorf("%w: %s", coreiface.ErrNotFound, err)
	}
	return &rating, nil
}

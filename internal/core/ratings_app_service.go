package core

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/request"
)

type GetRatingIndexFromNetDBFunc func(peerID string, reqCtx *request.Context) (models.RatingIndex, error)

type RatingsAppService struct {
	db                 database.Database
	getRatingIndex     GetRatingIndexFromNetDBFunc
	coTenantPublicData contracts.CoTenantPublicDataFn
}

type RatingsAppServiceConfig struct {
	DB                 database.Database
	GetRatingIndex     GetRatingIndexFromNetDBFunc
	CoTenantPublicData contracts.CoTenantPublicDataFn
}

func NewRatingsAppService(cfg RatingsAppServiceConfig) *RatingsAppService {
	return &RatingsAppService{
		db:                 cfg.DB,
		getRatingIndex:     cfg.GetRatingIndex,
		coTenantPublicData: cfg.CoTenantPublicData,
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

func (s *RatingsAppService) GetRatings(_ context.Context, peerID peer.ID, reqCtx *request.Context, _ bool) (models.RatingIndex, error) {
	if s.coTenantPublicData != nil {
		if pd, err := s.coTenantPublicData(peerID); err == nil {
			if index, err := pd.GetRatingIndex(); err == nil {
				return index, nil
			}
		}
	}

	if s.getRatingIndex != nil {
		return s.getRatingIndex(peerID.String(), reqCtx)
	}

	return nil, fmt.Errorf("rating data not available for remote peer %s", peerID)
}

func (s *RatingsAppService) GetRating(_ context.Context, c cid.Cid) (*pb.Rating, error) {
	return nil, fmt.Errorf("rating retrieval by CID %s not available (IPFS retired)", c)
}

package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestModerationAppService(t *testing.T, cfg ModerationAppServiceConfig) *ModerationAppService {
	t.Helper()
	if cfg.DB == nil {
		db, err := repo.MockDB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		cfg.DB = db
	}
	if cfg.Publish == nil {
		cfg.Publish = noopPublish
	}
	if cfg.NodeID == "" {
		cfg.NodeID = "test-moderation-svc"
	}
	return NewModerationAppService(cfg)
}

func seedProfile(t *testing.T, db database.Database, profile *models.Profile) {
	t.Helper()
	err := db.Update(func(tx database.Tx) error {
		return tx.SetProfile(profile)
	})
	require.NoError(t, err)
}

func TestModerationAppService_IsModerator_False(t *testing.T) {
	svc := newTestModerationAppService(t, ModerationAppServiceConfig{
		GetMyProfile: func() (*models.Profile, error) {
			return &models.Profile{Name: "Test", Moderator: false}, nil
		},
	})
	assert.False(t, svc.IsModerator())
}

func TestModerationAppService_IsModerator_True(t *testing.T) {
	svc := newTestModerationAppService(t, ModerationAppServiceConfig{
		GetMyProfile: func() (*models.Profile, error) {
			return &models.Profile{Name: "Test", Moderator: true}, nil
		},
	})
	assert.True(t, svc.IsModerator())
}

func TestModerationAppService_SetSelfAsModerator(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	seedProfile(t, db, &models.Profile{
		Name:       "Test Store",
		Currencies: []string{"BTC", "ETH"},
	})

	svc := newTestModerationAppService(t, ModerationAppServiceConfig{
		DB: db,
		GetAcceptedCurrencies: func() ([]string, error) {
			return []string{"BTC", "ETH"}, nil
		},
	})

	modInfo := &models.ModeratorInfo{
		Fee: models.ModeratorFee{
			FeeType:    1,
			Percentage: 5.0,
		},
	}

	done := make(chan struct{})
	err = svc.SetSelfAsModerator(context.Background(), modInfo, done)
	require.NoError(t, err)

	<-done

	var profile *models.Profile
	err = db.View(func(tx database.Tx) error {
		profile, err = tx.GetProfile()
		return err
	})
	require.NoError(t, err)
	assert.True(t, profile.Moderator)
	assert.NotNil(t, profile.ModeratorInfo)
}

func TestModerationAppService_SetSelfAsModerator_FixedFeeRequired(t *testing.T) {
	svc := newTestModerationAppService(t, ModerationAppServiceConfig{})

	modInfo := &models.ModeratorInfo{
		Fee: models.ModeratorFee{
			FeeType: 0,
		},
	}

	err := svc.SetSelfAsModerator(context.Background(), modInfo, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fixed fee must be set")
}

func TestModerationAppService_RemoveSelfAsModerator(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	seedProfile(t, db, &models.Profile{
		Name:      "Test Store",
		Moderator: true,
	})

	svc := newTestModerationAppService(t, ModerationAppServiceConfig{
		DB: db,
	})

	done := make(chan struct{})
	err = svc.RemoveSelfAsModerator(context.Background(), done)
	require.NoError(t, err)

	<-done

	var profile *models.Profile
	err = db.View(func(tx database.Tx) error {
		profile, err = tx.GetProfile()
		return err
	})
	require.NoError(t, err)
	assert.False(t, profile.Moderator)
}

func TestModerationAppService_SetModeratorsOnListings(t *testing.T) {
	updateCalled := false
	svc := newTestModerationAppService(t, ModerationAppServiceConfig{
		UpdateAllListings: func(updateFunc func(l *pb.Listing) (bool, error), done chan<- struct{}) error {
			updateCalled = true
			maybeCloseDone(done)
			return nil
		},
	})

	done := make(chan struct{})
	err := svc.SetModeratorsOnListings(nil, done)
	require.NoError(t, err)
	<-done
	assert.True(t, updateCalled, "updateAllListings should be called")
}

func TestModerationAppService_GetVerifiedModerators_EmptyEndpoint(t *testing.T) {
	svc := newTestModerationAppService(t, ModerationAppServiceConfig{
		VerifiedModEndpoint: "",
	})

	mods := svc.GetVerifiedModerators(context.Background())
	assert.Empty(t, mods)
}

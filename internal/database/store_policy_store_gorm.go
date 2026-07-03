package database

import (
	"context"
	"errors"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	pkgdb "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
)

var (
	ErrStorePolicyConflict = errors.New("store policy revision conflict")
	ErrStorePolicyNotFound = errors.New("store policy not found")
)

type GormStorePolicyStore struct {
	db pkgdb.Database
}

var _ contracts.StorePolicyStore = (*GormStorePolicyStore)(nil)

func NewGormStorePolicyStore(db pkgdb.Database) *GormStorePolicyStore {
	return &GormStorePolicyStore{db: db}
}

func MigrateStorePolicyModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		if err := tx.Migrate(&models.StorePolicy{}); err != nil {
			return err
		}
		return tx.Migrate(&models.StoreModerator{})
	})
}

func (s *GormStorePolicyStore) GetPolicy(_ context.Context) (*models.StorePolicy, error) {
	var policy models.StorePolicy
	err := s.db.View(func(tx pkgdb.Tx) error {
		if err := tx.Read().First(&policy).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				policy = models.StorePolicy{}
				return nil
			}
			return err
		}
		return tx.Read().
			Order("position ASC, created_at ASC").
			Find(&policy.Moderators).Error
	})
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func (s *GormStorePolicyStore) ReplaceModerators(_ context.Context, expectedRevision *uint64, moderators []models.StoreModerator) (*models.StorePolicy, error) {
	var policy models.StorePolicy
	now := time.Now()

	err := s.db.Update(func(tx pkgdb.Tx) error {
		err := tx.Read().First(&policy).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			policy = models.StorePolicy{
				CreatedAt: now,
			}
			if err := tx.Save(&policy); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		if expectedRevision != nil && policy.Revision != *expectedRevision {
			return ErrStorePolicyConflict
		}

		if err := tx.DeleteAll(&models.StoreModerator{}); err != nil {
			return err
		}
		for i := range moderators {
			moderators[i].Position = i
			if err := tx.Save(&moderators[i]); err != nil {
				return err
			}
		}

		policy.Revision++
		policy.UpdatedAt = now
		if err := tx.Save(&policy); err != nil {
			return err
		}
		policy.Moderators = moderators
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

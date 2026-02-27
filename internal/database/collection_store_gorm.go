package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

var (
	ErrCollectionNotFound         = errors.New("collection not found")
	ErrDuplicateCollectionProduct = errors.New("product already in collection")
	ErrCollectionTitleRequired    = errors.New("collection title is required")
	ErrCollectionMaxReached       = errors.New("maximum collections reached")
	ErrCollectionProductMaxExceeded = errors.New("maximum products per collection would be exceeded")
	ErrCollectionProductRequired  = errors.New("at least one product slug is required")
)

type GormCollectionStore struct {
	db pkgdb.Database
}

var _ contracts.CollectionStore = (*GormCollectionStore)(nil)

func NewGormCollectionStore(db pkgdb.Database) *GormCollectionStore {
	return &GormCollectionStore{db: db}
}

func MigrateCollectionModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		if err := tx.Migrate(&models.Collection{}); err != nil {
			return err
		}
		return tx.Migrate(&models.CollectionProduct{})
	})
}

func (s *GormCollectionStore) CreateCollection(_ context.Context, c *models.Collection) error {
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now

	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(c)
	})
}

func (s *GormCollectionStore) GetCollection(_ context.Context, id string) (*models.Collection, error) {
	var c models.Collection
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("id = ? AND deleted_at IS NULL", id).
			Preload("Products", func(db *gorm.DB) *gorm.DB {
				return db.Order("position ASC")
			}).First(&c).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCollectionNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (s *GormCollectionStore) ListCollections(_ context.Context, page, pageSize int, publishedOnly bool) ([]*models.Collection, int64, error) {
	var collections []*models.Collection
	var total int64

	err := s.db.View(func(tx pkgdb.Tx) error {
		q := tx.Read().Model(&models.Collection{}).Where("deleted_at IS NULL")
		if publishedOnly {
			q = q.Where("published = ?", true)
		}

		if err := q.Count(&total).Error; err != nil {
			return err
		}

		offset := (page - 1) * pageSize
		return q.Order("created_at DESC").
			Offset(offset).Limit(pageSize).
			Preload("Products", func(db *gorm.DB) *gorm.DB {
				return db.Order("position ASC")
			}).Find(&collections).Error
	})
	if err != nil {
		return nil, 0, err
	}
	return collections, total, nil
}

func (s *GormCollectionStore) UpdateCollection(_ context.Context, c *models.Collection) error {
	c.UpdatedAt = time.Now()
	return s.db.Update(func(tx pkgdb.Tx) error {
		result := tx.Read().Model(&models.Collection{}).
			Where("id = ? AND deleted_at IS NULL", c.ID).
			Updates(map[string]interface{}{
				"title":       c.Title,
				"description": c.Description,
				"image":       c.Image,
				"sort_order":  c.SortOrder,
				"published":   c.Published,
				"updated_at":  c.UpdatedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrCollectionNotFound
		}
		return nil
	})
}

func (s *GormCollectionStore) DeleteCollection(_ context.Context, id string) error {
	now := time.Now()
	return s.db.Update(func(tx pkgdb.Tx) error {
		result := tx.Read().Model(&models.Collection{}).
			Where("id = ? AND deleted_at IS NULL", id).
			Update("deleted_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrCollectionNotFound
		}
		return nil
	})
}

func (s *GormCollectionStore) AddProducts(_ context.Context, collectionID string, slugs []string) error {
	return s.db.Update(func(tx pkgdb.Tx) error {
		var maxPos int
		tx.Read().Model(&models.CollectionProduct{}).
			Where("collection_id = ?", collectionID).
			Select("COALESCE(MAX(position), -1)").Scan(&maxPos)

		for i, slug := range slugs {
			cp := &models.CollectionProduct{
				CollectionID: collectionID,
				ListingSlug:  slug,
				Position:     maxPos + 1 + i,
			}
			if err := tx.Save(cp); err != nil {
				if isUniqueConstraintErr(err) {
					return fmt.Errorf("%w: %s", ErrDuplicateCollectionProduct, slug)
				}
				return err
			}
		}
		return nil
	})
}

func isUniqueConstraintErr(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key value violates unique constraint")
}

func (s *GormCollectionStore) RemoveProduct(_ context.Context, collectionID, slug string) error {
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Read().Where("collection_id = ? AND listing_slug = ?", collectionID, slug).
			Delete(&models.CollectionProduct{}).Error
	})
}

func (s *GormCollectionStore) ReorderProducts(_ context.Context, collectionID string, orderedSlugs []string) error {
	return s.db.Update(func(tx pkgdb.Tx) error {
		for i, slug := range orderedSlugs {
			result := tx.Read().Model(&models.CollectionProduct{}).
				Where("collection_id = ? AND listing_slug = ?", collectionID, slug).
				Update("position", i)
			if result.Error != nil {
				return result.Error
			}
		}
		return nil
	})
}

func (s *GormCollectionStore) IsProductInCollections(_ context.Context, collectionIDs []string, slug string) (bool, error) {
	var count int64
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Model(&models.CollectionProduct{}).
			Where("collection_id IN ? AND listing_slug = ?", collectionIDs, slug).
			Count(&count).Error
	})
	return count > 0, err
}

func (s *GormCollectionStore) RemoveProductFromAllCollections(_ context.Context, slug string) error {
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Read().Where("listing_slug = ?", slug).
			Delete(&models.CollectionProduct{}).Error
	})
}

func (s *GormCollectionStore) CountCollections(_ context.Context) (int64, error) {
	var count int64
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Model(&models.Collection{}).
			Where("deleted_at IS NULL").Count(&count).Error
	})
	return count, err
}

func (s *GormCollectionStore) CountCollectionProducts(_ context.Context, collectionID string) (int64, error) {
	var count int64
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Model(&models.CollectionProduct{}).
			Where("collection_id = ?", collectionID).Count(&count).Error
	})
	return count, err
}

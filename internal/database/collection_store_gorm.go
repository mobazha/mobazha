package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	pkgdb "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
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
		if err := tx.Read().Where("id = ? AND deleted_at IS NULL", id).First(&c).Error; err != nil {
			return err
		}
		return tx.Read().Where("collection_id = ?", id).
			Order("position ASC").Find(&c.Products).Error
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
		countQ := tx.Read().Model(&models.Collection{}).Where("deleted_at IS NULL")
		if publishedOnly {
			countQ = countQ.Where("published = ?", true)
		}
		if err := countQ.Count(&total).Error; err != nil {
			return err
		}

		findQ := tx.Read().Where("deleted_at IS NULL")
		if publishedOnly {
			findQ = findQ.Where("published = ?", true)
		}
		offset := (page - 1) * pageSize
		if err := findQ.Order("created_at DESC").
			Offset(offset).Limit(pageSize).
			Find(&collections).Error; err != nil {
			return err
		}

		if len(collections) == 0 {
			return nil
		}

		ids := make([]string, len(collections))
		collMap := make(map[string]*models.Collection, len(collections))
		for i, c := range collections {
			ids[i] = c.ID
			collMap[c.ID] = c
		}

		var products []models.CollectionProduct
		if err := tx.Read().Where("collection_id IN ?", ids).
			Order("position ASC").Find(&products).Error; err != nil {
			return err
		}
		for i := range products {
			if c, ok := collMap[products[i].CollectionID]; ok {
				c.Products = append(c.Products, products[i])
			}
		}

		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return collections, total, nil
}

func (s *GormCollectionStore) UpdateCollection(_ context.Context, c *models.Collection) error {
	c.UpdatedAt = time.Now()
	return s.db.Update(func(tx pkgdb.Tx) error {
		var existing models.Collection
		if err := tx.Read().Where("id = ? AND deleted_at IS NULL", c.ID).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCollectionNotFound
			}
			return err
		}
		existing.Title = c.Title
		existing.Description = c.Description
		existing.Image = c.Image
		existing.SortOrder = c.SortOrder
		existing.Published = c.Published
		existing.UpdatedAt = c.UpdatedAt
		return tx.Save(&existing)
	})
}

func (s *GormCollectionStore) DeleteCollection(_ context.Context, id string) error {
	now := time.Now()
	return s.db.Update(func(tx pkgdb.Tx) error {
		var existing models.Collection
		if err := tx.Read().Where("id = ? AND deleted_at IS NULL", id).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCollectionNotFound
			}
			return err
		}
		existing.DeletedAt = &now
		return tx.Save(&existing)
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
		return tx.Delete("collection_id", collectionID, map[string]interface{}{
			"listing_slug = ?": slug,
		}, &models.CollectionProduct{})
	})
}

func (s *GormCollectionStore) ReorderProducts(_ context.Context, collectionID string, orderedSlugs []string) error {
	return s.db.Update(func(tx pkgdb.Tx) error {
		for i, slug := range orderedSlugs {
			if err := tx.Update("position", i, map[string]interface{}{
				"collection_id = ?": collectionID,
				"listing_slug = ?":  slug,
			}, &models.CollectionProduct{}); err != nil {
				return err
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
		return tx.Delete("listing_slug", slug, nil, &models.CollectionProduct{})
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

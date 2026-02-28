package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

var (
	ErrShippingProfileNotFound = errors.New("shipping profile not found")
	ErrShippingLocationNotFound = errors.New("shipping location not found")
	ErrListingRefNotFound       = errors.New("listing shipping ref not found")
	ErrProfileHasListings       = errors.New("shipping profile has associated listings")
	ErrVersionConflict          = errors.New("version conflict")
)

type GormShippingStore struct {
	db pkgdb.Database
}

var _ contracts.ShippingStore = (*GormShippingStore)(nil)

func NewGormShippingStore(db pkgdb.Database) *GormShippingStore {
	return &GormShippingStore{db: db}
}

// MigrateShippingModels creates/updates shipping tables. Call during repo init.
func MigrateShippingModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		if err := tx.Migrate(&models.ShippingLocationEntity{}); err != nil {
			return err
		}
		if err := tx.Migrate(&models.ShippingProfileEntity{}); err != nil {
			return err
		}
		return tx.Migrate(&models.ListingShippingRef{})
	})
}

// --- Profile CRUD ---

func (s *GormShippingStore) CreateProfile(_ context.Context, profile *models.ShippingProfileEntity) error {
	if profile.ID == "" {
		profile.ID = uuid.New().String()
	}
	now := time.Now()
	profile.CreatedAt = now
	profile.UpdatedAt = now
	if profile.Version == 0 {
		profile.Version = 1
	}
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(profile)
	})
}

func (s *GormShippingStore) GetProfile(_ context.Context, id string) (*models.ShippingProfileEntity, error) {
	var p models.ShippingProfileEntity
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("id = ?", id).First(&p).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrShippingProfileNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *GormShippingStore) GetDefaultProfile(_ context.Context) (*models.ShippingProfileEntity, error) {
	var p models.ShippingProfileEntity
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("is_default = ?", true).First(&p).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrShippingProfileNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *GormShippingStore) ListProfiles(_ context.Context) ([]*models.ShippingProfileEntity, error) {
	var profiles []*models.ShippingProfileEntity
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Order("created_at ASC").Find(&profiles).Error
	})
	if err != nil {
		return nil, err
	}
	return profiles, nil
}

func (s *GormShippingStore) UpdateProfile(_ context.Context, profile *models.ShippingProfileEntity) error {
	profile.UpdatedAt = time.Now()
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(profile)
	})
}

func (s *GormShippingStore) DeleteProfile(_ context.Context, id string) error {
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Delete("id", id, nil, &models.ShippingProfileEntity{})
	})
}

func (s *GormShippingStore) SetDefaultProfile(_ context.Context, id string) error {
	return s.db.Update(func(tx pkgdb.Tx) error {
		if err := tx.Read().Model(&models.ShippingProfileEntity{}).
			Where("is_default = ?", true).
			UpdateColumn("is_default", false).Error; err != nil {
			return err
		}
		result := tx.Read().Model(&models.ShippingProfileEntity{}).
			Where("id = ?", id).
			UpdateColumn("is_default", true)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrShippingProfileNotFound
		}
		return nil
	})
}

// --- Location CRUD ---

func (s *GormShippingStore) CreateLocation(_ context.Context, loc *models.ShippingLocationEntity) error {
	if loc.ID == "" {
		loc.ID = uuid.New().String()
	}
	now := time.Now()
	loc.CreatedAt = now
	loc.UpdatedAt = now
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(loc)
	})
}

func (s *GormShippingStore) GetLocation(_ context.Context, id string) (*models.ShippingLocationEntity, error) {
	var loc models.ShippingLocationEntity
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("id = ?", id).First(&loc).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrShippingLocationNotFound
		}
		return nil, err
	}
	return &loc, nil
}

func (s *GormShippingStore) ListLocations(_ context.Context) ([]*models.ShippingLocationEntity, error) {
	var locs []*models.ShippingLocationEntity
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Order("created_at ASC").Find(&locs).Error
	})
	if err != nil {
		return nil, err
	}
	return locs, nil
}

func (s *GormShippingStore) UpdateLocation(_ context.Context, loc *models.ShippingLocationEntity) error {
	loc.UpdatedAt = time.Now()
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(loc)
	})
}

func (s *GormShippingStore) DeleteLocation(_ context.Context, id string) error {
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Delete("id", id, nil, &models.ShippingLocationEntity{})
	})
}

// --- Listing-Profile references ---

func (s *GormShippingStore) UpsertListingRef(_ context.Context, ref *models.ListingShippingRef) error {
	if ref.ID == "" {
		ref.ID = uuid.New().String()
	}
	now := time.Now()
	ref.UpdatedAt = now
	return s.db.Update(func(tx pkgdb.Tx) error {
		var existing models.ListingShippingRef
		err := tx.Read().Where("listing_slug = ?", ref.ListingSlug).First(&existing).Error
		if err == nil {
			existing.ShippingProfileID = ref.ShippingProfileID
			existing.SnapshotVersion = ref.SnapshotVersion
			existing.IsStale = ref.IsStale
			existing.UpdatedAt = now
			return tx.Save(&existing)
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ref.CreatedAt = now
			return tx.Save(ref)
		}
		return err
	})
}

func (s *GormShippingStore) GetListingRef(_ context.Context, listingSlug string) (*models.ListingShippingRef, error) {
	var ref models.ListingShippingRef
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("listing_slug = ?", listingSlug).First(&ref).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ref, nil
}

func (s *GormShippingStore) DeleteListingRef(_ context.Context, listingSlug string) error {
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Delete("listing_slug", listingSlug, nil, &models.ListingShippingRef{})
	})
}

func (s *GormShippingStore) ListRefsByProfile(_ context.Context, profileID string, page, pageSize int) ([]*models.ListingShippingRef, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	var refs []*models.ListingShippingRef
	var total int64

	err := s.db.View(func(tx pkgdb.Tx) error {
		q := tx.Read().Model(&models.ListingShippingRef{}).
			Where("shipping_profile_id = ?", profileID)
		if err := q.Count(&total).Error; err != nil {
			return err
		}
		offset := (page - 1) * pageSize
		return q.Order("created_at ASC").Offset(offset).Limit(pageSize).Find(&refs).Error
	})
	if err != nil {
		return nil, 0, err
	}
	return refs, int(total), nil
}

// MigrateRefs moves all refs from one profile to another and marks them stale.
//
// NOTE: Uses tx.Read() for batch UPDATE because database.Tx.Update() is per-field
// and cannot express multi-column SET in one statement. Tenant isolation is preserved
// because tx.Read() scopes with WHERE tenant_id = ?.
func (s *GormShippingStore) MigrateRefs(_ context.Context, fromProfileID, toProfileID string) (int, error) {
	var count int64
	err := s.db.Update(func(tx pkgdb.Tx) error {
		result := tx.Read().Model(&models.ListingShippingRef{}).
			Where("shipping_profile_id = ?", fromProfileID).
			Updates(map[string]interface{}{
				"shipping_profile_id": toProfileID,
				"is_stale":           true,
				"updated_at":         time.Now(),
			})
		if result.Error != nil {
			return result.Error
		}
		count = result.RowsAffected
		return nil
	})
	return int(count), err
}

// MarkProfileStale sets is_stale=true on all refs for the given profile.
//
// NOTE: Uses tx.Read() for batch UPDATE. See MigrateRefs comment for rationale.
func (s *GormShippingStore) MarkProfileStale(_ context.Context, profileID string) error {
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Read().Model(&models.ListingShippingRef{}).
			Where("shipping_profile_id = ?", profileID).
			Updates(map[string]interface{}{
				"is_stale":   true,
				"updated_at": time.Now(),
			}).Error
	})
}

func (s *GormShippingStore) ListStaleRefs(_ context.Context, page, pageSize int) ([]*models.ListingShippingRef, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	var refs []*models.ListingShippingRef
	var total int64

	err := s.db.View(func(tx pkgdb.Tx) error {
		q := tx.Read().Model(&models.ListingShippingRef{}).Where("is_stale = ?", true)
		if err := q.Count(&total).Error; err != nil {
			return err
		}
		offset := (page - 1) * pageSize
		return q.Order("updated_at ASC").Offset(offset).Limit(pageSize).Find(&refs).Error
	})
	if err != nil {
		return nil, 0, err
	}
	return refs, int(total), nil
}

func (s *GormShippingStore) CountListingsByProfile(_ context.Context, profileID string) (int, error) {
	var count int64
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Model(&models.ListingShippingRef{}).
			Where("shipping_profile_id = ?", profileID).
			Count(&count).Error
	})
	return int(count), err
}

// Sentinel error helpers.
func IsShippingProfileNotFound(err error) bool  { return errors.Is(err, ErrShippingProfileNotFound) }
func IsShippingLocationNotFound(err error) bool { return errors.Is(err, ErrShippingLocationNotFound) }
func IsVersionConflict(err error) bool          { return errors.Is(err, ErrVersionConflict) }

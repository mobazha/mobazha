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
	ErrUsageLimitReached = errors.New("usage limit reached")
	ErrDiscountNotFound  = errors.New("discount not found")
)

// GormDiscountStore implements contracts.DiscountStore using the node's database.Database.
// Tenant scoping is handled internally by database.Tx on writes and read-scoped queries.
type GormDiscountStore struct {
	db pkgdb.Database
}

var _ contracts.DiscountStore = (*GormDiscountStore)(nil)

func NewGormDiscountStore(db pkgdb.Database) *GormDiscountStore {
	return &GormDiscountStore{db: db}
}

// MigrateDiscountModels creates/updates discount tables. Call during repo init.
func MigrateDiscountModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		if err := tx.Migrate(&models.Discount{}); err != nil {
			return err
		}
		if err := tx.Migrate(&models.DiscountCode{}); err != nil {
			return err
		}
		return tx.Migrate(&models.DiscountRedemption{})
	})
}

func (s *GormDiscountStore) CreateDiscount(_ context.Context, d *models.Discount) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	now := time.Now()
	d.CreatedAt = now
	d.UpdatedAt = now

	return s.db.Update(func(tx pkgdb.Tx) error {
		if err := tx.Save(d); err != nil {
			return err
		}
		for i := range d.Codes {
			if d.Codes[i].ID == "" {
				d.Codes[i].ID = uuid.New().String()
			}
			d.Codes[i].DiscountID = d.ID
			if err := tx.Save(&d.Codes[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *GormDiscountStore) GetDiscount(_ context.Context, id string) (*models.Discount, error) {
	var d models.Discount
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("id = ? AND deleted_at IS NULL", id).
			Preload("Codes").First(&d).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDiscountNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (s *GormDiscountStore) ListDiscounts(_ context.Context, filter contracts.DiscountFilter) ([]models.Discount, int64, error) {
	var discounts []models.Discount
	var total int64

	err := s.db.View(func(tx pkgdb.Tx) error {
		q := tx.Read().Model(&models.Discount{})
		if !filter.IncludeExpired {
			q = q.Where("deleted_at IS NULL")
		}
		if filter.Status != nil {
			q = q.Where("status = ?", *filter.Status)
		}
		if filter.Method != nil {
			q = q.Where("method = ?", *filter.Method)
		}
		if filter.SearchTerm != "" {
			q = q.Where("title LIKE ?", "%"+filter.SearchTerm+"%")
		}
		if err := q.Count(&total).Error; err != nil {
			return err
		}

		page := filter.Page
		if page < 1 {
			page = 1
		}
		pageSize := filter.PageSize
		if pageSize < 1 {
			pageSize = 20
		}
		offset := (page - 1) * pageSize

		return q.Order("created_at DESC").
			Offset(offset).Limit(pageSize).
			Preload("Codes").
			Find(&discounts).Error
	})
	if err != nil {
		return nil, 0, err
	}
	return discounts, total, nil
}

func (s *GormDiscountStore) UpdateDiscount(_ context.Context, d *models.Discount) error {
	d.UpdatedAt = time.Now()
	return s.db.Update(func(tx pkgdb.Tx) error {
		var existing models.Discount
		if err := tx.Read().Where("id = ? AND deleted_at IS NULL", d.ID).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrDiscountNotFound
			}
			return err
		}
		d.UsageCount = existing.UsageCount
		return tx.Save(d)
	})
}

func (s *GormDiscountStore) SoftDeleteDiscount(_ context.Context, id string) error {
	now := time.Now()
	return s.db.Update(func(tx pkgdb.Tx) error {
		var d models.Discount
		if err := tx.Read().Where("id = ? AND deleted_at IS NULL", id).First(&d).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrDiscountNotFound
			}
			return err
		}
		d.DeletedAt = &now
		d.UpdatedAt = now
		return tx.Save(&d)
	})
}

func (s *GormDiscountStore) CreateCodes(_ context.Context, codes []models.DiscountCode) error {
	if len(codes) == 0 {
		return nil
	}
	return s.db.Update(func(tx pkgdb.Tx) error {
		for i := range codes {
			if codes[i].ID == "" {
				codes[i].ID = uuid.New().String()
			}
			if err := tx.Save(&codes[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *GormDiscountStore) ListCodes(_ context.Context, discountID string) ([]models.DiscountCode, error) {
	var codes []models.DiscountCode
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("discount_id = ?", discountID).
			Order("created_at DESC").Find(&codes).Error
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *GormDiscountStore) DeleteCode(_ context.Context, codeID string) error {
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Delete("id", codeID, nil, &models.DiscountCode{})
	})
}

func (s *GormDiscountStore) FindCodeByHash(_ context.Context, codeHash string) (*models.DiscountCode, error) {
	var code models.DiscountCode
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("code_hash = ?", codeHash).First(&code).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &code, nil
}

// IncrementUsageWithCheck atomically increments usage counts using
// UPDATE ... WHERE (usage_limit = 0 OR usage_count < usage_limit).
//
// NOTE: This intentionally uses tx.Read().UpdateColumn() instead of tx.Save()
// because atomic conditional increment (UPDATE ... WHERE count < limit) cannot
// be expressed through the Tx.Save() API. Tenant isolation is preserved because
// tx.Read() already scopes the query with WHERE tenant_id = ?.
// See db-transaction-rules.mdc — this is a documented exception.
func (s *GormDiscountStore) IncrementUsageWithCheck(_ context.Context, discountID string, codeID *string) error {
	return s.db.Update(func(tx pkgdb.Tx) error {
		result := tx.Read().Model(&models.Discount{}).
			Where("id = ? AND deleted_at IS NULL AND (usage_limit = 0 OR usage_count < usage_limit)", discountID).
			UpdateColumn("usage_count", gorm.Expr("usage_count + 1"))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrUsageLimitReached
		}

		if codeID != nil && *codeID != "" {
			codeResult := tx.Read().Model(&models.DiscountCode{}).
				Where("id = ? AND (usage_limit = 0 OR usage_count < usage_limit)", *codeID).
				UpdateColumn("usage_count", gorm.Expr("usage_count + 1"))
			if codeResult.Error != nil {
				return codeResult.Error
			}
			if codeResult.RowsAffected == 0 {
				return ErrUsageLimitReached
			}
		}
		return nil
	})
}

func (s *GormDiscountStore) CountCustomerRedemptions(_ context.Context, discountID, customerPeerID string) (int64, error) {
	var count int64
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Model(&models.DiscountRedemption{}).
			Where("discount_id = ? AND customer_peer_id = ?", discountID, customerPeerID).
			Count(&count).Error
	})
	return count, err
}

func (s *GormDiscountStore) CreateRedemption(_ context.Context, r *models.DiscountRedemption) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(r)
	})
}

func (s *GormDiscountStore) ListRedemptions(_ context.Context, discountID string, page, pageSize int) ([]models.DiscountRedemption, int64, error) {
	var redemptions []models.DiscountRedemption
	var total int64

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	err := s.db.View(func(tx pkgdb.Tx) error {
		q := tx.Read().Model(&models.DiscountRedemption{}).
			Where("discount_id = ?", discountID)
		if err := q.Count(&total).Error; err != nil {
			return err
		}
		offset := (page - 1) * pageSize
		return q.Order("redeemed_at DESC").Offset(offset).Limit(pageSize).Find(&redemptions).Error
	})
	if err != nil {
		return nil, 0, err
	}
	return redemptions, total, nil
}

func (s *GormDiscountStore) GetApplicableDiscounts(_ context.Context, productIDs []string) ([]models.Discount, error) {
	var discounts []models.Discount
	now := time.Now()

	err := s.db.View(func(tx pkgdb.Tx) error {
		q := tx.Read().Where(
			"method = ? AND status = ? AND deleted_at IS NULL AND starts_at <= ? AND (ends_at IS NULL OR ends_at > ?)",
			models.DiscountMethodAutomatic, models.DiscountStatusActive, now, now,
		)

		return q.Order("created_at DESC").Find(&discounts).Error
	})
	if err != nil {
		return nil, err
	}

	if len(productIDs) == 0 {
		return discounts, nil
	}

	var applicable []models.Discount
	for _, d := range discounts {
		switch d.AppliesTo {
		case models.DiscountAppliesToAll:
			applicable = append(applicable, d)
		case models.DiscountAppliesToSpecificProducts:
			if hasOverlap(d.ProductIDs, productIDs) {
				applicable = append(applicable, d)
			}
		case models.DiscountAppliesToSpecificCollections:
			applicable = append(applicable, d)
		}
	}
	return applicable, nil
}

func (s *GormDiscountStore) CountDiscounts(_ context.Context) (int64, error) {
	var count int64
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Model(&models.Discount{}).
			Where("deleted_at IS NULL").Count(&count).Error
	})
	return count, err
}

func hasOverlap(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, v := range a {
		set[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := set[v]; ok {
			return true
		}
	}
	return false
}

// Sentinel errors for discount operations.
func IsUsageLimitReached(err error) bool { return errors.Is(err, ErrUsageLimitReached) }
func IsDiscountNotFound(err error) bool  { return errors.Is(err, ErrDiscountNotFound) }

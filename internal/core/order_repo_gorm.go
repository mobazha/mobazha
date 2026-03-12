package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

var _ contracts.OrderRepo = (*GormOrderRepo)(nil)

// GormOrderRepo implements contracts.OrderRepo using the existing
// database.Database abstraction. Read operations use db.View();
// write operations use db.Update() to ensure TenantID injection
// in multi-tenant mode.
type GormOrderRepo struct {
	db database.Database
}

// NewGormOrderRepo creates a new GormOrderRepo backed by the given database.
func NewGormOrderRepo(db database.Database) *GormOrderRepo {
	return &GormOrderRepo{db: db}
}

func (r *GormOrderRepo) FindByID(_ context.Context, orderID string) (*models.Order, error) {
	var order models.Order
	err := r.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("order %s not found: %w", orderID, err)
		}
		return nil, err
	}
	return &order, nil
}

func (r *GormOrderRepo) FindPurchases(_ context.Context, filter contracts.OrderFilter) ([]models.Order, int64, error) {
	return r.findOrders("buyer", filter)
}

func (r *GormOrderRepo) FindSales(_ context.Context, filter contracts.OrderFilter) ([]models.Order, int64, error) {
	return r.findOrders("vendor", filter)
}

func (r *GormOrderRepo) findOrders(role string, filter contracts.OrderFilter) ([]models.Order, int64, error) {
	var orders []models.Order
	var total int64

	stm, args := buildFilterClause(filter)
	if len(stm) > 0 {
		stm += " and my_role = ?"
	} else {
		stm = "my_role = ?"
	}
	args = append(args, role)

	err := r.db.View(func(tx database.Tx) error {
		base := tx.Read().Model(&models.Order{}).Where(stm, args...)
		if err := base.Count(&total).Error; err != nil {
			return err
		}

		q := tx.Read().Where(stm, args...)
		q = applySortOrder(q, filter)
		if filter.Limit > 0 {
			q = q.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			q = q.Offset(filter.Offset)
		}
		return q.Find(&orders).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *GormOrderRepo) FindUnverifiedPaymentOrders(_ context.Context) ([]models.Order, error) {
	var orders []models.Order
	err := r.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("serialized_payment_sent IS NOT NULL AND payment_verified = ? AND open = ? AND my_role = ?",
				false, true, string(models.RoleVendor)).
			Find(&orders).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return orders, nil
}

func (r *GormOrderRepo) Save(_ context.Context, order *models.Order) error {
	return r.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	})
}

func (r *GormOrderRepo) MarkAsRead(_ context.Context, orderID string) error {
	return r.db.Update(func(tx database.Tx) error {
		return tx.Update("read", true,
			map[string]interface{}{"id = ?": orderID, "read = ?": false},
			&models.Order{})
	})
}

func (r *GormOrderRepo) UpdateState(_ context.Context, orderID string, state models.OrderState) error {
	return r.db.Update(func(tx database.Tx) error {
		return tx.Update("state", state,
			map[string]interface{}{"id = ?": orderID},
			&models.Order{})
	})
}

func (r *GormOrderRepo) UpdateLastCheckTime(_ context.Context, orderID string, t time.Time) error {
	return r.db.Update(func(tx database.Tx) error {
		return tx.Update("last_check_for_payments", t,
			map[string]interface{}{"id = ?": orderID},
			&models.Order{})
	})
}

func (r *GormOrderRepo) ExpirePaymentVerification(_ context.Context, orderID string, marker time.Time) error {
	return r.db.Update(func(tx database.Tx) error {
		if err := tx.Update("open", false,
			map[string]interface{}{"id = ?": orderID},
			&models.Order{}); err != nil {
			return err
		}
		return tx.Update("last_check_for_payments", marker,
			map[string]interface{}{"id = ?": orderID},
			&models.Order{})
	})
}

func (r *GormOrderRepo) FindByPaymentTransactionID(_ context.Context, txID string) (*models.Order, error) {
	var order models.Order
	err := r.db.View(func(tx database.Tx) error {
		return tx.Read().Where("payment_transaction_id = ?", txID).First(&order).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("order with payment tx %s not found: %w", txID, err)
		}
		return nil, err
	}
	return &order, nil
}

func (r *GormOrderRepo) SetPaymentTransactionID(_ context.Context, orderID string, txID string) error {
	return r.db.Update(func(tx database.Tx) error {
		return tx.Update("payment_transaction_id", txID,
			map[string]interface{}{"id = ?": orderID},
			&models.Order{})
	})
}

func (r *GormOrderRepo) MergeFiatMetadata(ctx context.Context, orderID string, kv map[string]string) error {
	order, err := r.FindByID(ctx, orderID)
	if err != nil {
		return err
	}
	if err := order.MergeFiatMetadata(kv); err != nil {
		return err
	}
	return r.db.Update(func(tx database.Tx) error {
		return tx.Update("fiat_metadata", order.FiatMetadata,
			map[string]interface{}{"id = ?": orderID},
			&models.Order{})
	})
}

// ── Query building helpers ──────────────────────────────────────

func buildFilterClause(f contracts.OrderFilter) (string, []interface{}) {
	var stm string
	var args []interface{}

	if len(f.StateFilter) > 0 {
		stm = "state in ?"
		args = append(args, f.StateFilter)
	}

	if f.SearchTerm != "" && len(f.SearchColumns) > 0 {
		searchClause := "LOWER("
		for i, col := range f.SearchColumns {
			searchClause += col
			if i < len(f.SearchColumns)-1 {
				searchClause += " || "
			}
		}
		searchClause += ") LIKE LOWER(?)"

		if stm != "" {
			stm += " and " + searchClause
		} else {
			stm = searchClause
		}
		args = append(args, "%"+f.SearchTerm+"%")
	}

	return stm, args
}

func applySortOrder(q *gorm.DB, f contracts.OrderFilter) *gorm.DB {
	if !f.SortByRead && !f.SortAscending {
		q = q.Order("read asc")
	}
	if f.SortByRead {
		q = q.Order("read asc")
	}
	if f.SortAscending {
		q = q.Order("created_at asc")
	} else {
		q = q.Order("created_at desc")
	}
	return q
}

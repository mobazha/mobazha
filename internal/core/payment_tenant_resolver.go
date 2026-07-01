package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corepayment "github.com/mobazha/mobazha3.0/internal/core/payment"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

// paymentOrderTenantResolver keeps observation routing in Open Core. It is
// chain- and provider-neutral: commercial monitors submit evidence through the
// public funding sink and never receive database access.
type paymentOrderTenantResolver struct {
	db database.Database
}

func (r *paymentOrderTenantResolver) ResolveTenant(_ context.Context, orderID string) (string, error) {
	if strings.HasPrefix(orderID, corepayment.GuestOrderTokenPrefix) {
		var guest models.GuestOrder
		err := r.db.View(func(tx database.Tx) error {
			return tx.Read().Where("order_token = ?", orderID).First(&guest).Error
		})
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "", corepayment.ErrUnknownOrder
			}
			return "", err
		}
		if guest.TenantID == "" {
			return database.StandaloneTenantID, nil
		}
		return guest.TenantID, nil
	}

	var order models.Order
	err := r.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", corepayment.ErrUnknownOrder
		}
		return "", err
	}
	if order.TenantID == "" {
		return "", corepayment.ErrUnknownOrder
	}
	return order.TenantID, nil
}

func (r *paymentOrderTenantResolver) ResolveTenants(ctx context.Context, orderID string) ([]string, error) {
	if strings.HasPrefix(orderID, corepayment.GuestOrderTokenPrefix) {
		tenantID, err := r.ResolveTenant(ctx, orderID)
		if err != nil {
			return nil, err
		}
		return []string{tenantID}, nil
	}

	var tenantIDs []string
	if rawProvider, ok := r.db.(interface{ RawDB() *gorm.DB }); ok {
		raw := rawProvider.RawDB()
		if raw == nil {
			return nil, fmt.Errorf("raw DB unavailable")
		}
		if err := raw.Model(&models.Order{}).
			Where("id = ? AND tenant_id <> ''", orderID).
			Distinct("tenant_id").Pluck("tenant_id", &tenantIDs).Error; err != nil {
			return nil, err
		}
	} else if err := r.db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.Order{}).
			Where("id = ? AND tenant_id <> ''", orderID).
			Distinct("tenant_id").Pluck("tenant_id", &tenantIDs).Error
	}); err != nil {
		return nil, err
	}
	if len(tenantIDs) == 0 {
		return nil, corepayment.ErrUnknownOrder
	}
	return tenantIDs, nil
}

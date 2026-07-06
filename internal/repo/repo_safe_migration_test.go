package repo

import (
	"testing"

	"github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestAutoMigrateDatabaseSafe_IncludesPaymentSelectionQuotes(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, autoMigrateDatabaseSafe(db))
	require.NoError(t, db.View(func(tx database.Tx) error {
		require.True(t, tx.Read().Migrator().HasTable(&models.PaymentSelectionQuote{}))
		return nil
	}))
}

func TestAutoMigrateDatabaseSafe_PurchaseRequestCorrelationIsTenantScoped(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, autoMigrateDatabaseSafe(db))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		for _, correlation := range []*models.PurchaseRequestCorrelation{
			{
				TenantMixin:       models.TenantMixin{TenantID: "tenant-a"},
				PurchaseRequestID: "request-1",
				OrderID:           "order-a",
			},
			{
				TenantMixin:       models.TenantMixin{TenantID: "tenant-b"},
				PurchaseRequestID: "request-1",
				OrderID:           "order-b",
			},
		} {
			if err := tx.Read().Create(correlation).Error; err != nil {
				return err
			}
		}
		return nil
	}))
	require.Error(t, db.Update(func(tx database.Tx) error {
		return tx.Read().Create(&models.PurchaseRequestCorrelation{
			TenantMixin:       models.TenantMixin{TenantID: "tenant-a"},
			PurchaseRequestID: "request-1",
			OrderID:           "order-a-duplicate",
		}).Error
	}))
}

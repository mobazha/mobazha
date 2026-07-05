package database_test

import (
	"testing"

	dbgorm "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestMigrateFiatModels_RemovesLegacyConfigWithoutCredentialReference(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.FiatProviderConfig{}); err != nil {
			return err
		}
		return tx.Save(&models.FiatProviderConfig{
			ProviderID: "stripe", AccountID: "acct_legacy", PublicKey: "pk_legacy", IsActive: true,
		})
	}))

	require.NoError(t, dbgorm.MigrateFiatModels(db))
	var count int64
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.FiatProviderConfig{}).Count(&count).Error
	}))
	require.Zero(t, count)
}

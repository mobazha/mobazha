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

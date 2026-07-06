//go:build integration

// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package database_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	dbgorm "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMigrateFiatModels_PostgresBackfillsPaymentAttemptKind(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MOBAZHA_TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("MOBAZHA_TEST_POSTGRES_DSN is not set")
	}

	schema := fmt.Sprintf("payment_attempt_migration_%d", time.Now().UnixNano())
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		require.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error)
		sqlDB, dbErr := admin.DB()
		require.NoError(t, dbErr)
		require.NoError(t, sqlDB.Close())
	})

	shared, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := shared.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	db, err := dbstore.NewTenantDBWithPublicData(shared, "migration-tenant", nil)
	require.NoError(t, err)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&legacyPaymentAttempt{}); err != nil {
			return err
		}
		return tx.Create(&legacyPaymentAttempt{
			AttemptID: "pa_postgres_existing", PaymentSessionID: "ps_postgres_existing",
			OrderID: "order_postgres_existing", ProviderID: "stripe", Amount: 4200,
			Currency: "USD", RouteBindingID: "prb_postgres_existing",
			IdempotencyKey: "mbz_postgres_existing", State: models.PaymentAttemptExternalCreated,
			ExternalReference: "pi_postgres_existing",
		})
	}))

	require.NoError(t, dbgorm.MigrateFiatModels(db))
	var attempt models.PaymentAttempt
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("attempt_id = ?", "pa_postgres_existing").First(&attempt).Error
	}))
	require.Equal(t, models.PaymentAttemptKindProviderSession, attempt.Kind)
	require.Equal(t, int64(4200), attempt.Amount)
	require.Equal(t, "pi_postgres_existing", attempt.ExternalReference)
}

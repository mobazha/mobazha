package core

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestFiatService_ClaimProviderAction_TenantIsolation(t *testing.T) {
	shared, err := gorm.Open(sqlitedialect.Open(filepath.Join(t.TempDir(), "provider-actions.db")+"?_busy_timeout=5000"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, shared.AutoMigrate(&models.PaymentProviderAction{}))
	t.Cleanup(func() {
		sqlDB, dbErr := shared.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	tenantA, err := dbstore.NewTenantDBWithPublicData(shared, "tenant-a", nil)
	require.NoError(t, err)
	tenantB, err := dbstore.NewTenantDBWithPublicData(shared, "tenant-b", nil)
	require.NoError(t, err)
	actionA := tenantProviderActionFixture()
	actionB := tenantProviderActionFixture()
	require.NoError(t, tenantA.Update(func(tx database.Tx) error { return tx.Create(&actionA) }))
	require.NoError(t, tenantB.Update(func(tx database.Tx) error { return tx.Create(&actionB) }))

	serviceA := NewFiatPaymentAppService(nil, tenantA, "worker-a", false)
	serviceB := NewFiatPaymentAppService(nil, tenantB, "worker-b", false)
	claimedA, claimedActionA, err := serviceA.claimProviderAction(actionA, time.Now().UTC())
	require.NoError(t, err)
	require.True(t, claimedA)
	claimedB, claimedActionB, err := serviceB.claimProviderAction(actionB, time.Now().UTC())
	require.NoError(t, err)
	require.True(t, claimedB)

	assert.Contains(t, claimedActionA.LeaseOwner, "worker-a:")
	assert.Contains(t, claimedActionB.LeaseOwner, "worker-b:")
	var rowA, rowB models.PaymentProviderAction
	require.NoError(t, tenantA.View(func(tx database.Tx) error { return tx.Read().First(&rowA).Error }))
	require.NoError(t, tenantB.View(func(tx database.Tx) error { return tx.Read().First(&rowB).Error }))
	assert.Equal(t, "tenant-a", rowA.TenantID)
	assert.Equal(t, "tenant-b", rowB.TenantID)
	assert.Equal(t, claimedActionA.LeaseOwner, rowA.LeaseOwner)
	assert.Equal(t, claimedActionB.LeaseOwner, rowB.LeaseOwner)
}

func tenantProviderActionFixture() models.PaymentProviderAction {
	return models.PaymentProviderAction{
		ActionID: "fpa_same", ActionKind: models.PaymentProviderActionRefund, ProviderID: "stripe",
		AttemptID: "attempt_same", RouteBindingID: "route_same", ProviderBindingID: "binding_same",
		ExternalReference: "pi_same", IdempotencyKey: "mbza_same", IntentFingerprint: "fingerprint_same",
		IntentPayload: []byte(`{"params":{"PaymentID":"pi_same"}}`), State: models.PaymentProviderActionPendingExternal,
	}
}

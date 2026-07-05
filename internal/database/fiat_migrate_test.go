package database_test

import (
	"testing"
	"time"

	dbgorm "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

type legacyPaymentProviderAction struct {
	TenantID          string `gorm:"column:tenant_id;primaryKey;default:''"`
	ActionID          string `gorm:"column:action_id;primaryKey;size:64"`
	ActionKind        string `gorm:"column:action_kind;size:32;not null"`
	ProviderID        string `gorm:"column:provider_id;size:64;not null"`
	AttemptID         string `gorm:"column:attempt_id;size:64;not null"`
	RouteBindingID    string `gorm:"column:route_binding_id;size:64;not null"`
	ProviderBindingID string `gorm:"column:provider_binding_id;size:128;not null"`
	ExternalReference string `gorm:"column:external_reference;size:255;not null"`
	IdempotencyKey    string `gorm:"column:idempotency_key;size:128;not null"`
	IntentFingerprint string `gorm:"column:intent_fingerprint;size:64;not null"`
	IntentPayload     []byte `gorm:"column:intent_payload;not null"`
	ResultPayload     []byte `gorm:"column:result_payload"`
	State             string `gorm:"column:state;size:32;not null"`
	Attempts          int    `gorm:"column:attempts;not null;default:0"`
	LastError         string `gorm:"column:last_error;size:2048"`
	NextAttemptAt     *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (legacyPaymentProviderAction) TableName() string { return "payment_provider_actions" }

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

func TestMigrateFiatModels_AddsProviderActionLeaseColumnsToExistingRows(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&legacyPaymentProviderAction{}); err != nil {
			return err
		}
		return tx.Create(&legacyPaymentProviderAction{
			ActionID: "fpa_existing", ActionKind: models.PaymentProviderActionRefund, ProviderID: "stripe",
			AttemptID: "attempt_existing", RouteBindingID: "route_existing", ProviderBindingID: "binding_existing",
			ExternalReference: "pi_existing", IdempotencyKey: "mbza_existing", IntentFingerprint: "fingerprint",
			IntentPayload: []byte(`{"paymentID":"pi_existing"}`), State: models.PaymentProviderActionCompleted,
		})
	}))

	require.NoError(t, dbgorm.MigrateFiatModels(db))
	var action models.PaymentProviderAction
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", "fpa_existing").First(&action).Error
	}))
	require.Empty(t, action.LeaseOwner)
	require.Nil(t, action.LeaseExpiresAt)
	require.Nil(t, action.LastAttemptAt)
	require.Nil(t, action.CompletedAt)
}

package payment

import (
	"testing"

	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func newStorePaymentSettingsDB(t *testing.T) *vTestDB {
	t.Helper()
	db := newVerifierTestDB(t)
	require.NoError(t, db.gormDB.AutoMigrate(&models.StorePaymentSettings{}))
	return db
}

func TestSaveStorePaymentSettings_PersistsPolicy(t *testing.T) {
	db := newStorePaymentSettingsDB(t)

	cfg, err := SaveStorePaymentSettings(db, models.PaymentConfirmationPolicyMempoolAccepted)
	require.NoError(t, err)
	require.Equal(t, models.PaymentConfirmationPolicyMempoolAccepted, cfg.UtxoConfirmationPolicy)

	got, err := GetStorePaymentSettings(db)
	require.NoError(t, err)
	require.Equal(t, models.PaymentConfirmationPolicyMempoolAccepted, got.UtxoConfirmationPolicy)
}

func TestResolveUtxoConfirmationPolicy_DefaultsToChainConfirmed(t *testing.T) {
	db := newStorePaymentSettingsDB(t)
	require.Equal(t, models.PaymentConfirmationPolicyChainConfirmed, ResolveUtxoConfirmationPolicy(db))
}

func TestGetStorePaymentSettings_DefaultsWhenMissing(t *testing.T) {
	db := newStorePaymentSettingsDB(t)

	got, err := GetStorePaymentSettings(db)
	require.NoError(t, err)
	require.Equal(t, models.PaymentConfirmationPolicyChainConfirmed, got.UtxoConfirmationPolicy)
}

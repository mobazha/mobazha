package core

import (
	"encoding/json"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPreferencesAppService(t *testing.T, cfg ...PreferencesAppServiceConfig) *PreferencesAppService {
	t.Helper()
	var c PreferencesAppServiceConfig
	if len(cfg) > 0 {
		c = cfg[0]
	}
	if c.DB == nil {
		db, err := repo.MockDB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		c.DB = db
	}
	return NewPreferencesAppService(c)
}

func seedPreferences(t *testing.T, db database.Database, prefs *models.UserPreferences) {
	t.Helper()
	prefs.ID = 1
	err := db.Update(func(tx database.Tx) error { return tx.Save(prefs) })
	require.NoError(t, err)
}

// ── GetPreferences ──────────────────────────────────────────────

func TestPreferencesAppService_GetPreferences_NotFound(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	_, err := svc.GetPreferences()
	assert.Error(t, err)
}

func TestPreferencesAppService_GetPreferences_Found(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	seedPreferences(t, svc.db, &models.UserPreferences{
		UserAgent:     "test-agent",
		LocalCurrency: "USD",
		Country:       "US",
	})

	prefs, err := svc.GetPreferences()
	require.NoError(t, err)
	assert.Equal(t, "test-agent", prefs.UserAgent)
	assert.Equal(t, "USD", prefs.LocalCurrency)
	assert.Equal(t, "US", prefs.Country)
}

// ── SavePreferences ─────────────────────────────────────────────

func TestPreferencesAppService_SavePreferences_Basic(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	prefs := &models.UserPreferences{
		UserAgent:          "test-agent",
		LocalCurrency:      "USD",
		ShowNotifications:  true,
		PaymentDataInQR:    true,
		MisPaymentBuffer:   0.01,
		TermsAndConditions: "Standard T&C",
	}

	require.NoError(t, svc.SavePreferences(prefs, nil))

	saved, err := svc.GetPreferences()
	require.NoError(t, err)
	assert.Equal(t, "test-agent", saved.UserAgent)
	assert.Equal(t, "USD", saved.LocalCurrency)
	assert.True(t, saved.ShowNotifications)
	assert.Equal(t, "Standard T&C", saved.TermsAndConditions)
}

func TestPreferencesAppService_SavePreferences_UpdateExisting(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	seedPreferences(t, svc.db, &models.UserPreferences{
		UserAgent:     "old-agent",
		LocalCurrency: "EUR",
	})

	prefs := &models.UserPreferences{
		UserAgent:     "new-agent",
		LocalCurrency: "USD",
	}
	require.NoError(t, svc.SavePreferences(prefs, nil))

	saved, err := svc.GetPreferences()
	require.NoError(t, err)
	assert.Equal(t, "new-agent", saved.UserAgent)
	assert.Equal(t, "USD", saved.LocalCurrency)
}

func TestPreferencesAppService_SavePreferences_InvalidCurrency(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	currencies, _ := json.Marshal([]string{"INVALID_COIN"})
	prefs := &models.UserPreferences{
		PrefCurrencies: currencies,
	}
	err := svc.SavePreferences(prefs, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not valid")
}

// ── SavePreferences triggers UpdateAllListings on mod change ────

func TestPreferencesAppService_SavePreferences_ModeratorChangeTriggersUpdate(t *testing.T) {
	updateCalled := false
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	svc := newTestPreferencesAppService(t, PreferencesAppServiceConfig{
		DB: db,
		UpdateAllListingsFunc: func(updateFunc func(l *pb.Listing) (bool, error), done chan<- struct{}) error {
			updateCalled = true
			maybeCloseDone(done)
			return nil
		},
	})
	seedPreferences(t, db, &models.UserPreferences{})

	mods, _ := json.Marshal([]string{testVendorPeerID})
	prefs := &models.UserPreferences{
		Mods: mods,
	}
	require.NoError(t, svc.SavePreferences(prefs, nil))
	assert.True(t, updateCalled)
}

// ── BlockNode / UnblockNode ─────────────────────────────────────

func TestPreferencesAppService_BlockNode(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	seedPreferences(t, svc.db, &models.UserPreferences{})

	added, err := svc.BlockNode(testVendorPeerID)
	require.NoError(t, err)
	assert.True(t, added)

	prefs, err := svc.GetPreferences()
	require.NoError(t, err)
	blocked, err := prefs.BlockedNodes()
	require.NoError(t, err)
	assert.Len(t, blocked, 1)
}

func TestPreferencesAppService_BlockNode_Duplicate(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	seedPreferences(t, svc.db, &models.UserPreferences{})

	added1, err := svc.BlockNode(testVendorPeerID)
	require.NoError(t, err)
	assert.True(t, added1)

	added2, err := svc.BlockNode(testVendorPeerID)
	require.NoError(t, err)
	assert.False(t, added2)
}

func TestPreferencesAppService_BlockNode_InvalidPeerID(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	_, err := svc.BlockNode("invalid-peer-id")
	assert.Error(t, err)
}

func TestPreferencesAppService_UnblockNode(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	seedPreferences(t, svc.db, &models.UserPreferences{})

	_, err := svc.BlockNode(testVendorPeerID)
	require.NoError(t, err)

	removed, err := svc.UnblockNode(testVendorPeerID)
	require.NoError(t, err)
	assert.True(t, removed)

	prefs, err := svc.GetPreferences()
	require.NoError(t, err)
	blocked, err := prefs.BlockedNodes()
	require.NoError(t, err)
	assert.Empty(t, blocked)
}

func TestPreferencesAppService_UnblockNode_NotBlocked(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	seedPreferences(t, svc.db, &models.UserPreferences{})

	removed, err := svc.UnblockNode(testVendorPeerID)
	require.NoError(t, err)
	assert.False(t, removed)
}

// ── CheckAndMigrateShippingProfiles ─────────────────────────────

func TestPreferencesAppService_CheckAndMigrateShippingProfiles_NoPrefs(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	require.NoError(t, svc.CheckAndMigrateShippingProfiles())
}

func TestPreferencesAppService_CheckAndMigrateShippingProfiles_NoMigrationNeeded(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	seedPreferences(t, svc.db, &models.UserPreferences{})
	require.NoError(t, svc.CheckAndMigrateShippingProfiles())
}

// ── countPhysicalGoods blocks shipping removal ──────────────────

func TestPreferencesAppService_SavePreferences_RejectRemoveShippingWithPhysicalGoods(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	svc := newTestPreferencesAppService(t, PreferencesAppServiceConfig{
		DB: db,
		GetMyListingsFunc: func() (models.ListingIndex, error) {
			return models.ListingIndex{
				{ContractType: "PHYSICAL_GOOD"},
			}, nil
		},
	})

	shippingOpts, _ := json.Marshal([]models.ShippingOption{
		{ID: 1, Name: "Standard"},
	})
	seedPreferences(t, db, &models.UserPreferences{ShippingOptions: shippingOpts})

	prefs := &models.UserPreferences{}
	err = svc.SavePreferences(prefs, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot remove all shipping")
}

package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mobazha/mobazha3.0/pkg/database/sqlitedialect"
)

// newFeatureOverrideTestDB builds an in-memory SQLite DB with the
// feature_overrides table migrated. It reuses testDatabase defined in
// order_repo_gorm_test.go.
//
// Note: testTx.Save is a minimal gorm passthrough that does NOT inject
// TenantID — that is fine here because the production TenantDB wrapper
// handles injection and we exercise the store's CRUD semantics against
// a single (empty) tenant scope. See feature_override_store.go for the
// design rationale.
func newFeatureOverrideTestDB(t *testing.T) *testDatabase {
	t.Helper()
	db, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.FeatureOverride{}))
	return &testDatabase{gormDB: db}
}

func TestFeatureOverrideStore_GetMissingReturnsConfiguredFalse(t *testing.T) {
	store := NewFeatureOverrideStore(newFeatureOverrideTestDB(t))
	value, configured, err := store.Get(context.Background(), "_default", "guestCheckout")
	require.NoError(t, err)
	assert.False(t, configured, "no row → configured must be false so resolver falls back to DefaultValue")
	assert.False(t, value, "value is meaningless when configured=false")
}

func TestFeatureOverrideStore_SetThenGetReturnsEnabled(t *testing.T) {
	store := NewFeatureOverrideStore(newFeatureOverrideTestDB(t))
	ctx := context.Background()

	require.NoError(t, store.Set(ctx, "_default", "guestCheckout", true, "alice"))

	value, configured, err := store.Get(ctx, "_default", "guestCheckout")
	require.NoError(t, err)
	assert.True(t, configured)
	assert.True(t, value)
}

func TestFeatureOverrideStore_SetIsUpsert(t *testing.T) {
	store := NewFeatureOverrideStore(newFeatureOverrideTestDB(t))
	ctx := context.Background()

	require.NoError(t, store.Set(ctx, "_default", "guestCheckout", true, "alice"))
	// Second Set on the same composite key (tenant_id, feature_key)
	// must update in place, not create a duplicate row.
	require.NoError(t, store.Set(ctx, "_default", "guestCheckout", false, "bob"))

	value, configured, err := store.Get(ctx, "_default", "guestCheckout")
	require.NoError(t, err)
	assert.True(t, configured)
	assert.False(t, value, "second Set must overwrite the first value")

	list, err := store.List(ctx, "_default")
	require.NoError(t, err)
	assert.Len(t, list, 1, "upsert must not produce duplicate rows")
}

func TestFeatureOverrideStore_ListReturnsAllConfigured(t *testing.T) {
	store := NewFeatureOverrideStore(newFeatureOverrideTestDB(t))
	ctx := context.Background()

	require.NoError(t, store.Set(ctx, "_default", "guestCheckout", true, "alice"))
	require.NoError(t, store.Set(ctx, "_default", "localEncryptedStorage", false, "alice"))

	list, err := store.List(ctx, "_default")
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{
		"guestCheckout":         true,
		"localEncryptedStorage": false,
	}, list)
}

func TestFeatureOverrideStore_ListEmptyReturnsEmptyMap(t *testing.T) {
	store := NewFeatureOverrideStore(newFeatureOverrideTestDB(t))
	list, err := store.List(context.Background(), "_default")
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestFeatureOverrideStore_SetRejectsEmptyKey(t *testing.T) {
	store := NewFeatureOverrideStore(newFeatureOverrideTestDB(t))
	err := store.Set(context.Background(), "_default", "", true, "alice")
	assert.Error(t, err)
}

func TestFeatureOverrideStore_NilReceiverIsManagedEscrow(t *testing.T) {
	var store *FeatureOverrideStore

	value, configured, err := store.Get(context.Background(), "_default", "guestCheckout")
	require.NoError(t, err)
	assert.False(t, configured)
	assert.False(t, value)

	list, err := store.List(context.Background(), "_default")
	require.NoError(t, err)
	assert.Empty(t, list)

	assert.Error(t, store.Set(context.Background(), "_default", "guestCheckout", true, "alice"))
	assert.Error(t, store.Migrate())
}

package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestAuditDB returns an in-memory database with the audit-log
// table pre-migrated. Reuses testDatabase from order_repo_gorm_test.go.
func newTestAuditDB(t *testing.T) *testDatabase {
	t.Helper()
	db := newTestDatabase(t)
	require.NoError(t, db.gormDB.AutoMigrate(&models.FeatureFlagAuditLog{}))
	return db
}

func TestFeatureAuditLogStore_AppendAudit_PlatformScope(t *testing.T) {
	db := newTestAuditDB(t)
	store := NewFeatureAuditLogStore(db)

	oldValue := false
	entry := &models.FeatureFlagAuditLog{
		Scope:      "platform_global",
		FeatureKey: "guestCheckout",
		OldValue:   &oldValue,
		NewValue:   true,
		Actor:      "admin-1",
		Reason:     "enable for beta rollout",
		IPAddress:  "127.0.0.1",
		UserAgent:  "curl/8",
	}
	require.NoError(t, store.AppendAudit(context.Background(), entry))

	var rows []models.FeatureFlagAuditLog
	require.NoError(t, db.gormDB.Find(&rows).Error)
	require.Len(t, rows, 1)
	got := rows[0]
	assert.Equal(t, "platform_global", got.Scope)
	assert.Equal(t, "guestCheckout", got.FeatureKey)
	assert.Equal(t, "", got.TenantID, "platform_global entries should carry empty TenantID")
	assert.True(t, got.NewValue)
	require.NotNil(t, got.OldValue)
	assert.False(t, *got.OldValue)
	assert.Equal(t, "admin-1", got.Actor)
	assert.Equal(t, "enable for beta rollout", got.Reason)
	assert.False(t, got.CreatedAt.IsZero(), "CreatedAt should be set")
}

func TestFeatureAuditLogStore_AppendAudit_TenantScope(t *testing.T) {
	db := newTestAuditDB(t)
	store := NewFeatureAuditLogStore(db)

	entry := &models.FeatureFlagAuditLog{
		Scope:      "tenant",
		TenantID:   "tenant-abc",
		FeatureKey: "orderAutoComplete",
		OldValue:   nil, // never-set → NULL
		NewValue:   true,
		Actor:      "seller-1",
	}
	require.NoError(t, store.AppendAudit(context.Background(), entry))

	var rows []models.FeatureFlagAuditLog
	require.NoError(t, db.gormDB.Find(&rows).Error)
	require.Len(t, rows, 1)
	got := rows[0]
	assert.Equal(t, "tenant", got.Scope)
	assert.Equal(t, "tenant-abc", got.TenantID)
	assert.Nil(t, got.OldValue, "OldValue=nil preserved as NULL")
}

func TestFeatureAuditLogStore_AppendAudit_ValidatesRequiredFields(t *testing.T) {
	db := newTestAuditDB(t)
	store := NewFeatureAuditLogStore(db)

	tests := []struct {
		name  string
		entry *models.FeatureFlagAuditLog
	}{
		{"nil entry", nil},
		{"missing scope", &models.FeatureFlagAuditLog{FeatureKey: "x", Actor: "a"}},
		{"missing feature key", &models.FeatureFlagAuditLog{Scope: "tenant", Actor: "a"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := store.AppendAudit(context.Background(), tc.entry)
			assert.Error(t, err)
		})
	}
}

func TestFeatureAuditLogStore_List_OrdersByCreatedAtDesc(t *testing.T) {
	db := newTestAuditDB(t)
	store := NewFeatureAuditLogStore(db)
	ctx := context.Background()

	for i, key := range []string{"first", "second", "third"} {
		_ = i
		require.NoError(t, store.AppendAudit(ctx, &models.FeatureFlagAuditLog{
			Scope:      "platform_global",
			FeatureKey: key,
			NewValue:   true,
			Actor:      "admin",
		}))
	}

	rows, err := store.List(ctx, "platform_global", "", 0)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	// Newest first.
	for i := 1; i < len(rows); i++ {
		assert.True(t,
			!rows[i-1].CreatedAt.Before(rows[i].CreatedAt),
			"rows[%d].CreatedAt should be >= rows[%d].CreatedAt", i-1, i,
		)
	}
}

func TestFeatureAuditLogStore_List_FilterByScopeAndTenant(t *testing.T) {
	db := newTestAuditDB(t)
	store := NewFeatureAuditLogStore(db)
	ctx := context.Background()

	require.NoError(t, store.AppendAudit(ctx, &models.FeatureFlagAuditLog{
		Scope: "platform_global", FeatureKey: "a", NewValue: true, Actor: "admin",
	}))
	require.NoError(t, store.AppendAudit(ctx, &models.FeatureFlagAuditLog{
		Scope: "tenant", TenantID: "t1", FeatureKey: "a", NewValue: true, Actor: "seller",
	}))
	require.NoError(t, store.AppendAudit(ctx, &models.FeatureFlagAuditLog{
		Scope: "tenant", TenantID: "t2", FeatureKey: "a", NewValue: true, Actor: "seller",
	}))

	platform, err := store.List(ctx, "platform_global", "", 0)
	require.NoError(t, err)
	assert.Len(t, platform, 1)

	t1, err := store.List(ctx, "tenant", "t1", 0)
	require.NoError(t, err)
	assert.Len(t, t1, 1)
	assert.Equal(t, "t1", t1[0].TenantID)

	allTenant, err := store.List(ctx, "tenant", "", 0)
	require.NoError(t, err)
	assert.Len(t, allTenant, 2)
}

func TestFeatureAuditLogStore_List_RespectsLimit(t *testing.T) {
	db := newTestAuditDB(t)
	store := NewFeatureAuditLogStore(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		require.NoError(t, store.AppendAudit(ctx, &models.FeatureFlagAuditLog{
			Scope: "platform_global", FeatureKey: "a", NewValue: true, Actor: "admin",
		}))
	}

	rows, err := store.List(ctx, "", "", 3)
	require.NoError(t, err)
	assert.Len(t, rows, 3)
}

func TestFeatureAuditLogStore_Nil_ManagedEscrow(t *testing.T) {
	var s *FeatureAuditLogStore
	// Nil receiver: AppendAudit should return error, not panic.
	err := s.AppendAudit(context.Background(), &models.FeatureFlagAuditLog{
		Scope: "tenant", FeatureKey: "a", NewValue: true, Actor: "a",
	})
	assert.Error(t, err)
	// List should return nil, no error.
	rows, err := s.List(context.Background(), "", "", 0)
	assert.NoError(t, err)
	assert.Nil(t, rows)
}

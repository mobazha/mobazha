package dbstore

import (
	"errors"
	"fmt"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newTestSharedDB creates a shared *gorm.DB backed by a temp file for testing.
// We use a file-based SQLite (not :memory:) because :memory: gives each
// connection its own database, breaking cross-transaction visibility.
func newTestSharedDB(t *testing.T) *gorm.DB {
	t.Helper()
	tmpDir := path.Join(os.TempDir(), "mobazha-test", fmt.Sprintf("shareddb-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	dbPath := path.Join(tmpDir, "shared.db")
	dsn := dbPath + "?_busy_timeout=5000"
	db, err := gorm.Open(sqlitedialect.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	// Enable WAL mode for concurrent readers
	db.Exec("PRAGMA journal_mode=WAL")
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	return db
}

// newTestTenantDB creates a TenantDB for the given tenant using the shared DB.
func newTestTenantDB(t *testing.T, sharedDB *gorm.DB, tenantID string) database.Database {
	t.Helper()
	if err := sharedDB.AutoMigrate(&PublicDataRecord{}, &PublicMediaRecord{}); err != nil {
		t.Fatalf("failed to migrate public data tables: %v", err)
	}
	pd := NewDBPublicData(sharedDB, tenantID)
	db, err := NewTenantDBWithPublicData(sharedDB, tenantID, pd)
	if err != nil {
		t.Fatalf("failed to create TenantDB for %s: %v", tenantID, err)
	}
	return db
}

func TestNewTenantDB_RejectsEmptyTenantID(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	pd := NewDBPublicData(sharedDB, "dummy")
	_, err := NewTenantDBWithPublicData(sharedDB, "", pd)
	if err == nil {
		t.Fatal("expected error for empty tenantID")
	}
}

func TestTenantDB_SaveAndRead(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	db := newTestTenantDB(t, sharedDB, "tenant-A")

	// Migrate
	err := db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.Key{})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Save
	err = db.Update(func(tx database.Tx) error {
		return tx.Save(&models.Key{Name: "identity", Value: []byte("key-A")})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Read back
	var key models.Key
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&key).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if string(key.Value) != "key-A" {
		t.Errorf("expected key-A, got %s", string(key.Value))
	}
	if key.TenantID != "tenant-A" {
		t.Errorf("expected tenant-A, got %s", key.TenantID)
	}
}

func TestTenantDB_Isolation(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	dbA := newTestTenantDB(t, sharedDB, "tenant-A")
	dbB := newTestTenantDB(t, sharedDB, "tenant-B")

	// Migrate on one tenant (schema is global)
	err := dbA.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.Key{})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Save different keys for each tenant (same name = "identity")
	err = dbA.Update(func(tx database.Tx) error {
		return tx.Save(&models.Key{Name: "identity", Value: []byte("key-A")})
	})
	if err != nil {
		t.Fatal(err)
	}

	err = dbB.Update(func(tx database.Tx) error {
		return tx.Save(&models.Key{Name: "identity", Value: []byte("key-B")})
	})
	if err != nil {
		t.Fatalf("tenant-B should be able to save 'identity' key (composite PK): %v", err)
	}

	// Tenant A should only see its own key
	var keyA models.Key
	err = dbA.View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&keyA).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(keyA.Value) != "key-A" {
		t.Errorf("tenant-A saw wrong key: %s", string(keyA.Value))
	}

	// Tenant B should only see its own key
	var keyB models.Key
	err = dbB.View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&keyB).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(keyB.Value) != "key-B" {
		t.Errorf("tenant-B saw wrong key: %s", string(keyB.Value))
	}

	// Verify total rows in shared DB (both tenants' data)
	var count int64
	sharedDB.Model(&models.Key{}).Count(&count)
	if count != 2 {
		t.Errorf("expected 2 total rows in shared DB, got %d", count)
	}
}

func TestTenantDB_Isolation_Messages(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	dbA := newTestTenantDB(t, sharedDB, "tenant-A")
	dbB := newTestTenantDB(t, sharedDB, "tenant-B")

	// Migrate
	err := dbA.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.OutgoingMessage{})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Each tenant saves a message with a globally unique ID
	err = dbA.Update(func(tx database.Tx) error {
		return tx.Save(&models.OutgoingMessage{ID: "msg-A-1"})
	})
	if err != nil {
		t.Fatal(err)
	}

	err = dbB.Update(func(tx database.Tx) error {
		return tx.Save(&models.OutgoingMessage{ID: "msg-B-1"})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Tenant A sees only its own
	var msgsA []models.OutgoingMessage
	err = dbA.View(func(tx database.Tx) error {
		return tx.Read().Find(&msgsA).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgsA) != 1 || msgsA[0].ID != "msg-A-1" {
		t.Errorf("tenant-A expected [msg-A-1], got %v", msgsA)
	}

	// Tenant B sees only its own
	var msgsB []models.OutgoingMessage
	err = dbB.View(func(tx database.Tx) error {
		return tx.Read().Find(&msgsB).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgsB) != 1 || msgsB[0].ID != "msg-B-1" {
		t.Errorf("tenant-B expected [msg-B-1], got %v", msgsB)
	}
}

func TestTenantDB_Update(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	db := newTestTenantDB(t, sharedDB, "tenant-C")

	err := db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.NotificationRecord{}); err != nil {
			return err
		}
		return tx.Save(&models.NotificationRecord{
			ID:           "notif-1",
			Timestamp:    time.Now(),
			Read:         false,
			Type:         "order",
			Notification: []byte(`{"orderID":"ord-1"}`),
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.Update(func(tx database.Tx) error {
		return tx.Update("read", true, map[string]interface{}{"id = ?": "notif-1"}, &models.NotificationRecord{})
	})
	if err != nil {
		t.Fatal(err)
	}

	var rec models.NotificationRecord
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", "notif-1").First(&rec).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rec.Read {
		t.Error("notification should be marked as read")
	}
}

func TestTenantDB_Delete(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	db := newTestTenantDB(t, sharedDB, "tenant-D")

	err := db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.OutgoingMessage{}); err != nil {
			return err
		}
		if err := tx.Save(&models.OutgoingMessage{ID: "msg-1"}); err != nil {
			return err
		}
		return tx.Save(&models.OutgoingMessage{ID: "msg-2"})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify messages are saved
	var countBefore int64
	sharedDB.Model(&models.OutgoingMessage{}).Where("tenant_id = ?", "tenant-D").Count(&countBefore)
	t.Logf("Before delete: %d messages in shared DB for tenant-D", countBefore)

	// Delete one
	err = db.Update(func(tx database.Tx) error {
		return tx.Delete("id", "msg-1", nil, &models.OutgoingMessage{})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check raw count after delete
	var countAfter int64
	sharedDB.Model(&models.OutgoingMessage{}).Where("tenant_id = ?", "tenant-D").Count(&countAfter)
	t.Logf("After delete: %d messages in shared DB for tenant-D", countAfter)

	var messages []models.OutgoingMessage
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Find(&messages).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("View returned %d messages", len(messages))
	if len(messages) != 1 {
		t.Errorf("expected 1 message remaining, got %d", len(messages))
	} else if messages[0].ID != "msg-2" {
		t.Errorf("expected msg-2, got %s", messages[0].ID)
	}
}

func TestTenantDB_DeleteConditionExpression(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	db := newTestTenantDB(t, sharedDB, "tenant-D")

	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now()
	err := db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.OutboxEvent{}); err != nil {
			return err
		}
		if err := tx.Save(&models.OutboxEvent{
			EventName:   "order.expired",
			Payload:     []byte(`{}`),
			DeliveredAt: &old,
		}); err != nil {
			return err
		}
		return tx.Save(&models.OutboxEvent{
			EventName:   "order.expired",
			Payload:     []byte(`{}`),
			DeliveredAt: &recent,
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.Update(func(tx database.Tx) error {
		return tx.Delete("delivered_at < ?", recent, nil, &models.OutboxEvent{})
	})
	if err != nil {
		t.Fatal(err)
	}

	var remaining []models.OutboxEvent
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Find(&remaining).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 event remaining, got %d", len(remaining))
	}
	if remaining[0].DeliveredAt == nil || !remaining[0].DeliveredAt.Equal(recent) {
		t.Fatalf("expected recent event to remain, got %#v", remaining[0].DeliveredAt)
	}
}

func TestTenantDB_DeleteAll(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	dbA := newTestTenantDB(t, sharedDB, "tenant-E")
	dbB := newTestTenantDB(t, sharedDB, "tenant-F")

	// Migrate
	err := dbA.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.OutgoingMessage{})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Each tenant saves a message with globally unique ID
	err = dbA.Update(func(tx database.Tx) error {
		return tx.Save(&models.OutgoingMessage{ID: "msg-E-1"})
	})
	if err != nil {
		t.Fatal(err)
	}

	err = dbB.Update(func(tx database.Tx) error {
		return tx.Save(&models.OutgoingMessage{ID: "msg-F-1"})
	})
	if err != nil {
		t.Fatal(err)
	}

	// DeleteAll on tenant-E should not affect tenant-F
	err = dbA.Update(func(tx database.Tx) error {
		return tx.DeleteAll(&models.OutgoingMessage{})
	})
	if err != nil {
		t.Fatal(err)
	}

	// tenant-E: should be empty
	var msgsA []models.OutgoingMessage
	err = dbA.View(func(tx database.Tx) error {
		return tx.Read().Find(&msgsA).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgsA) != 0 {
		t.Error("tenant-E should have 0 messages after DeleteAll")
	}

	// tenant-F: should still have its message
	var msgsB []models.OutgoingMessage
	err = dbB.View(func(tx database.Tx) error {
		return tx.Read().Find(&msgsB).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgsB) != 1 {
		t.Errorf("tenant-F should still have 1 message after tenant-E's DeleteAll, got %d", len(msgsB))
	}
}

func TestTenantDB_Rollback(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	db := newTestTenantDB(t, sharedDB, "tenant-G")

	err := db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.OutgoingMessage{})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Attempt a save that is rolled back
	err = db.Update(func(tx database.Tx) error {
		if err := tx.Save(&models.OutgoingMessage{ID: "should-rollback"}); err != nil {
			return err
		}
		return errors.New("simulated failure")
	})
	if err == nil {
		t.Fatal("expected error from Update")
	}

	// Verify rollback: no messages should exist
	var messages []models.OutgoingMessage
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Find(&messages).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages after rollback, got %d", len(messages))
	}
}

func TestTenantDB_ReadOnlyCannotWrite(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	db := newTestTenantDB(t, sharedDB, "tenant-H")

	err := db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.OutgoingMessage{})
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.View(func(tx database.Tx) error {
		return tx.Save(&models.OutgoingMessage{ID: "should-fail"})
	})
	if err == nil {
		t.Fatal("expected error when writing in View (read-only)")
	}
}

func TestTenantDB_PublicDataProfile(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	db := newTestTenantDB(t, sharedDB, "tenant-I")

	name := "Test Store"
	err := db.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: name})
	})
	if err != nil {
		t.Fatal(err)
	}

	var profile *models.Profile
	err = db.View(func(tx database.Tx) error {
		var e error
		profile, e = tx.GetProfile()
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Name != name {
		t.Errorf("expected profile name %s, got %s", name, profile.Name)
	}
}

func TestTenantDB_PublicDataIsolation(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	dbA := newTestTenantDB(t, sharedDB, "tenant-J")
	dbB := newTestTenantDB(t, sharedDB, "tenant-K")

	// Each tenant sets a different profile
	err := dbA.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "Store A"})
	})
	if err != nil {
		t.Fatal(err)
	}

	err = dbB.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "Store B"})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Each tenant sees only its own profile
	var profileA *models.Profile
	err = dbA.View(func(tx database.Tx) error {
		var e error
		profileA, e = tx.GetProfile()
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if profileA.Name != "Store A" {
		t.Errorf("expected 'Store A', got '%s'", profileA.Name)
	}

	var profileB *models.Profile
	err = dbB.View(func(tx database.Tx) error {
		var e error
		profileB, e = tx.GetProfile()
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if profileB.Name != "Store B" {
		t.Errorf("expected 'Store B', got '%s'", profileB.Name)
	}
}

func TestSetTenantID_EmbeddedMixin(t *testing.T) {
	msg := &models.OutgoingMessage{ID: "test"}
	setTenantID(msg, "my-tenant")

	if msg.TenantID != "my-tenant" {
		t.Errorf("expected TenantID 'my-tenant' via TenantMixin, got '%s'", msg.TenantID)
	}
}

func TestSetTenantID_DirectField(t *testing.T) {
	// Key uses direct TenantID field (not TenantMixin embedding)
	key := &models.Key{Name: "test", Value: []byte("value")}
	setTenantID(key, "my-tenant")

	if key.TenantID != "my-tenant" {
		t.Errorf("expected TenantID 'my-tenant' via direct field, got '%s'", key.TenantID)
	}
}

func TestSetTenantID_StoreCartRecord(t *testing.T) {
	cart := &models.StoreCartRecord{VendorID: "vendor-1", Items: []byte("[]")}
	setTenantID(cart, "tenant-X")

	if cart.TenantID != "tenant-X" {
		t.Errorf("expected TenantID 'tenant-X', got '%s'", cart.TenantID)
	}
}

func TestSetTenantID_NonPointer(t *testing.T) {
	// Non-pointer values can't be modified
	key := models.Key{Name: "test"}
	setTenantID(key, "my-tenant")
	if key.TenantID != "" {
		t.Error("setTenantID should not modify value types")
	}
}

func TestSetTenantID_NonStruct(t *testing.T) {
	// Non-struct should not panic
	var str = "hello"
	setTenantID(&str, "my-tenant") // no-op, should not panic
}

// TestTenantDB_MixedModelView verifies that querying multiple different models
// within the same View callback does not cause GORM schema/session leakage.
// This reproduces the exact bug in builder.go where First(&prefs) followed by
// GetKeysFromDB (which queries Key model) failed with "record not found" because
// the GORM Statement.Schema was polluted by the first query.
// TestTenantDB_SaveInjectsTenantID verifies that tx.Save() automatically
// injects the correct TenantID into the model. This is the SAFE path.
func TestTenantDB_SaveInjectsTenantID(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	db := newTestTenantDB(t, sharedDB, "tenant-save-test")

	err := db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.OutgoingMessage{})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Save a message WITHOUT manually setting TenantID.
	// tx.Save() should inject it automatically.
	msg := &models.OutgoingMessage{ID: "msg-auto-tenant"}
	err = db.Update(func(tx database.Tx) error {
		return tx.Save(msg)
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify the model's TenantID was set by Save()
	if msg.TenantID != "tenant-save-test" {
		t.Errorf("tx.Save() should inject TenantID into model: expected 'tenant-save-test', got '%s'", msg.TenantID)
	}

	// Verify in the raw shared DB that tenant_id was persisted correctly
	var raw models.OutgoingMessage
	if err := sharedDB.Where("id = ?", "msg-auto-tenant").First(&raw).Error; err != nil {
		t.Fatalf("failed to read from shared DB: %v", err)
	}
	if raw.TenantID != "tenant-save-test" {
		t.Errorf("tenant_id in DB should be 'tenant-save-test', got '%s'", raw.TenantID)
	}
}

// TestTenantDB_ReadSaveBypassesTenantID verifies that tx.Read().Save() does NOT
// inject TenantID — this is the DANGEROUS pattern that must be avoided.
// This test documents the known-bad behavior so regressions are caught.
func TestTenantDB_ReadSaveBypassesTenantID(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	db := newTestTenantDB(t, sharedDB, "tenant-readsave-test")

	err := db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.OutgoingMessage{})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Use tx.Read().Save() — this BYPASSES TenantID injection.
	// The message will be saved with an empty TenantID.
	msg := &models.OutgoingMessage{ID: "msg-no-tenant"}
	err = db.Update(func(tx database.Tx) error {
		return tx.Read().Save(msg).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	// The model's TenantID was NOT set by Read().Save()
	if msg.TenantID != "" {
		t.Errorf("tx.Read().Save() should NOT inject TenantID, but got '%s'", msg.TenantID)
	}

	// The record exists in the shared DB with empty tenant_id
	var raw models.OutgoingMessage
	if err := sharedDB.Where("id = ?", "msg-no-tenant").First(&raw).Error; err != nil {
		t.Fatalf("failed to read from shared DB: %v", err)
	}
	if raw.TenantID != "" {
		t.Errorf("tx.Read().Save() should leave tenant_id empty, got '%s'", raw.TenantID)
	}

	// The record is INVISIBLE to the tenant's scoped view (the real bug).
	var msgs []models.OutgoingMessage
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", "msg-no-tenant").Find(&msgs).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Errorf("Records saved via tx.Read().Save() should be invisible to tenant queries, got %d results", len(msgs))
	}
}

// TestTenantDB_CrossTenantReadIsolation ensures that a tenant cannot read
// another tenant's data, even by constructing queries that try to bypass scoping.
func TestTenantDB_CrossTenantReadIsolation(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	dbA := newTestTenantDB(t, sharedDB, "tenant-X")
	dbB := newTestTenantDB(t, sharedDB, "tenant-Y")

	// Migrate
	err := dbA.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.Key{})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Tenant X saves secret data
	err = dbA.Update(func(tx database.Tx) error {
		return tx.Save(&models.Key{Name: "secret", Value: []byte("X-secret-value")})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Tenant Y tries to read Tenant X's data by querying for name = "secret"
	var keys []models.Key
	err = dbB.View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "secret").Find(&keys).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Errorf("Tenant Y should not see Tenant X's data, got %d keys", len(keys))
	}

	// Tenant Y tries to read ALL keys (unfiltered)
	var allKeys []models.Key
	err = dbB.View(func(tx database.Tx) error {
		return tx.Read().Find(&allKeys).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(allKeys) != 0 {
		t.Errorf("Tenant Y should see 0 keys (no data of its own), got %d", len(allKeys))
	}

	// Verify the shared DB has the data (it exists, just scoped)
	var total int64
	sharedDB.Model(&models.Key{}).Count(&total)
	if total != 1 {
		t.Errorf("shared DB should have 1 key from Tenant X, got %d", total)
	}
}

func TestTenantDB_MixedModelView(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	db := newTestTenantDB(t, sharedDB, "tenant-mixed")

	// Migrate both models
	err := db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.UserPreferences{}); err != nil {
			return err
		}
		return tx.Migrate(&models.Key{})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Save test data
	err = db.Update(func(tx database.Tx) error {
		if err := tx.Save(&models.UserPreferences{}); err != nil {
			return fmt.Errorf("save prefs: %w", err)
		}
		if err := tx.Save(&models.Key{Name: "escrow", Value: []byte("escrow-key")}); err != nil {
			return fmt.Errorf("save escrow key: %w", err)
		}
		if err := tx.Save(&models.Key{Name: "bip44", Value: []byte("bip44-key")}); err != nil {
			return fmt.Errorf("save bip44 key: %w", err)
		}
		if err := tx.Save(&models.Key{Name: "mnemonic", Value: []byte("test mnemonic")}); err != nil {
			return fmt.Errorf("save mnemonic key: %w", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// The critical test: query UserPreferences then Key in the SAME View callback.
	// Before the fix, the second query would fail because GORM's Statement.Schema
	// was still set to UserPreferences from the first query.
	var prefs models.UserPreferences
	var escrowKey, bip44Key, mnemonicKey models.Key
	err = db.View(func(tx database.Tx) error {
		// Step 1: Query UserPreferences (sets GORM schema to user_preferences table)
		if err := tx.Read().First(&prefs).Error; err != nil {
			return fmt.Errorf("read prefs: %w", err)
		}

		// Step 2: Query Key model (must use keys table, not user_preferences)
		if err := tx.Read().Where("name = ?", "escrow").First(&escrowKey).Error; err != nil {
			return fmt.Errorf("read escrow key: %w", err)
		}
		if err := tx.Read().Where("name = ?", "bip44").First(&bip44Key).Error; err != nil {
			return fmt.Errorf("read bip44 key: %w", err)
		}
		if err := tx.Read().Where("name = ?", "mnemonic").First(&mnemonicKey).Error; err != nil {
			return fmt.Errorf("read mnemonic key: %w", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Mixed model View failed: %v", err)
	}

	// Verify results
	if string(escrowKey.Value) != "escrow-key" {
		t.Errorf("expected escrow-key, got %s", string(escrowKey.Value))
	}
	if string(bip44Key.Value) != "bip44-key" {
		t.Errorf("expected bip44-key, got %s", string(bip44Key.Value))
	}
	if string(mnemonicKey.Value) != "test mnemonic" {
		t.Errorf("expected test mnemonic, got %s", string(mnemonicKey.Value))
	}
	if escrowKey.TenantID != "tenant-mixed" {
		t.Errorf("expected tenant-mixed, got %s", escrowKey.TenantID)
	}
}

// TestTenantDB_MixedModelUpdate verifies the same mixed-model scenario in Update callbacks.
func TestTenantDB_MixedModelUpdate(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	db := newTestTenantDB(t, sharedDB, "tenant-mixed-update")

	// Migrate
	err := db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.UserPreferences{}); err != nil {
			return err
		}
		return tx.Migrate(&models.Key{})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Save initial data
	err = db.Update(func(tx database.Tx) error {
		if err := tx.Save(&models.UserPreferences{}); err != nil {
			return err
		}
		if err := tx.Save(&models.Key{Name: "mnemonic", Value: []byte("test seed")}); err != nil {
			return err
		}
		return tx.Save(&models.Key{Name: "escrow", Value: []byte("old-escrow")})
	})
	if err != nil {
		t.Fatal(err)
	}

	// In a single Update: read prefs, then read key, then save updated key
	err = db.Update(func(tx database.Tx) error {
		var prefs models.UserPreferences
		if err := tx.Read().First(&prefs).Error; err != nil {
			return fmt.Errorf("read prefs in update: %w", err)
		}

		var mnemonic models.Key
		if err := tx.Read().Where("name = ?", "mnemonic").First(&mnemonic).Error; err != nil {
			return fmt.Errorf("read mnemonic in update: %w", err)
		}

		// Save a new key based on what we read
		return tx.Save(&models.Key{Name: "escrow", Value: []byte("new-escrow-from-" + string(mnemonic.Value))})
	})
	if err != nil {
		t.Fatalf("Mixed model Update failed: %v", err)
	}

	// Verify the key was updated correctly
	var key models.Key
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "escrow").First(&key).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(key.Value) != "new-escrow-from-test seed" {
		t.Errorf("expected 'new-escrow-from-test seed', got '%s'", string(key.Value))
	}
}

func TestTenantDB_Close_IsNoop(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	db := newTestTenantDB(t, sharedDB, "tenant-Z")

	err := db.Close()
	if err != nil {
		t.Errorf("TenantDB.Close should return nil, got %v", err)
	}

	// After Close, shared DB should still work (it's not closed by TenantDB)
	err = sharedDB.Exec("SELECT 1").Error
	if err != nil {
		t.Errorf("shared DB should still work after TenantDB.Close: %v", err)
	}
}

func TestTenantDB_Concurrent(t *testing.T) {
	sharedDB := newTestSharedDB(t)

	// Migrate
	dbInit := newTestTenantDB(t, sharedDB, "init")
	err := dbInit.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.Key{}); err != nil {
			return err
		}
		return tx.Migrate(&models.OutgoingMessage{})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Concurrent writes from multiple tenants
	var wg sync.WaitGroup
	errCh := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tenantID := fmt.Sprintf("tenant-%d", idx)
			db := newTestTenantDB(t, sharedDB, tenantID)
			if e := db.Update(func(tx database.Tx) error {
				return tx.Save(&models.Key{Name: "identity", Value: []byte(tenantID)})
			}); e != nil {
				errCh <- fmt.Errorf("tenant %s save failed: %w", tenantID, e)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}

	// Verify each tenant can read its own key
	for i := 0; i < 10; i++ {
		tenantID := fmt.Sprintf("tenant-%d", i)
		db := newTestTenantDB(t, sharedDB, tenantID)
		var key models.Key
		err := db.View(func(tx database.Tx) error {
			return tx.Read().Where("name = ?", "identity").First(&key).Error
		})
		if err != nil {
			t.Errorf("tenant %s failed to read: %v", tenantID, err)
			continue
		}
		if string(key.Value) != tenantID {
			t.Errorf("tenant %s got wrong value: %s", tenantID, string(key.Value))
		}
	}
}

// TestTenantDB_MixedSQLAndPublicData_CommitNoDeadlock verifies that writing
// both SQL models (via tx.Save) and public data (via tx.SetFollowing) in the
// same transaction does not cause SQLite lock contention during commit.
func TestTenantDB_MixedSQLAndPublicData_CommitNoDeadlock(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	if err := sharedDB.AutoMigrate(&models.FollowSequence{}); err != nil {
		t.Fatal(err)
	}
	db := newTestTenantDB(t, sharedDB, "tenant-commit-lock")

	err := db.Update(func(tx database.Tx) error {
		seq := &models.FollowSequence{PeerID: "QmTest123", Num: 1}
		if err := tx.Save(seq); err != nil {
			return err
		}
		return tx.SetFollowing(models.Following{"QmTest123"})
	})
	if err != nil {
		t.Fatalf("mixed SQL+public-data commit should not deadlock: %v", err)
	}

	err = db.View(func(tx database.Tx) error {
		following, e := tx.GetFollowing()
		if e != nil {
			return e
		}
		if len(following) != 1 || following[0] != "QmTest123" {
			t.Errorf("expected [QmTest123], got %v", following)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestTenantDB_RollbackAfterError_NoDeadlock verifies that when fn returns
// an error, the rollback path does not deadlock on SQLite. The rollback
// cache tries to restore old values via publicData, which must use the
// transaction-scoped DB to avoid lock contention with the active GORM tx.
func TestTenantDB_RollbackAfterError_NoDeadlock(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	if err := sharedDB.AutoMigrate(&models.FollowSequence{}); err != nil {
		t.Fatal(err)
	}
	db := newTestTenantDB(t, sharedDB, "tenant-rollback-lock")

	err := db.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "original"})
	})
	if err != nil {
		t.Fatal(err)
	}

	intentionalErr := errors.New("intentional rollback")
	err = db.Update(func(tx database.Tx) error {
		if e := tx.SetProfile(&models.Profile{Name: "modified"}); e != nil {
			return e
		}
		seq := &models.FollowSequence{PeerID: "QmFail", Num: 1}
		if e := tx.Save(seq); e != nil {
			return e
		}
		return intentionalErr
	})
	if !errors.Is(err, intentionalErr) {
		t.Fatalf("expected intentional error, got: %v", err)
	}

	err = db.View(func(tx database.Tx) error {
		profile, e := tx.GetProfile()
		if e != nil {
			return e
		}
		if profile.Name != "original" {
			t.Errorf("profile should be unchanged after rollback, got: %s", profile.Name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestTenantDB_PublicDataAtomicWithSQL verifies that public data writes are
// atomic with SQL writes — if the transaction is rolled back, public data
// changes are also reverted.
func TestTenantDB_PublicDataAtomicWithSQL(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	db := newTestTenantDB(t, sharedDB, "tenant-atomic")

	err := db.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "initial"})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a commit failure by creating a conflicting SQL state.
	// We set public data + SQL, but the fn returns an error.
	err = db.Update(func(tx database.Tx) error {
		if e := tx.SetProfile(&models.Profile{Name: "should-not-persist"}); e != nil {
			return e
		}
		if e := tx.SetFollowing(models.Following{"QmShouldNotPersist"}); e != nil {
			return e
		}
		return errors.New("abort")
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// Verify public data is still at original values
	err = db.View(func(tx database.Tx) error {
		profile, e := tx.GetProfile()
		if e != nil {
			return e
		}
		if profile.Name != "initial" {
			t.Errorf("profile should be 'initial' after rollback, got: %s", profile.Name)
		}

		_, e = tx.GetFollowing()
		if !os.IsNotExist(e) {
			t.Errorf("following should not exist after rollback, got err: %v", e)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

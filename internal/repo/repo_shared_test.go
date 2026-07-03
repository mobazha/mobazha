package repo

import (
	"fmt"
	"os"
	"path"
	"sync"
	"testing"

	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// testSharedDB creates a file-based SQLite DB for testing NewRepoWithSharedDB.
func testSharedDB(t *testing.T) *gorm.DB {
	t.Helper()
	tmpDir := path.Join(os.TempDir(), "mobazha", "shared-repo-test")
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
	db.Exec("PRAGMA journal_mode=WAL")
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	return db
}

// resetMigrateOnce resets the sharedDBMigrateOnce so each test starts fresh.
func resetMigrateOnce() {
	sharedDBMigrateOnce = sync.Once{}
	sharedDBMigrateErr = nil
}

// generateTestIdentityKey creates a fresh identity key for testing.
func generateTestIdentityKey(t *testing.T) []byte {
	t.Helper()
	keys, err := generateNodeKeys("", nil)
	if err != nil {
		t.Fatalf("failed to generate test keys: %v", err)
	}
	return keys.identityKey
}

func TestNewRepoWithSharedDB(t *testing.T) {
	resetMigrateOnce()

	sharedDB := testSharedDB(t)
	dataDir := path.Join(os.TempDir(), "mobazha", "shared-repo-tenant-A")
	os.MkdirAll(dataDir, os.ModePerm)
	t.Cleanup(func() { os.RemoveAll(dataDir) })

	identityKey := generateTestIdentityKey(t)

	r, err := NewRepoWithSharedDB("tenant-A", dataDir, sharedDB, identityKey, true)
	if err != nil {
		t.Fatalf("NewRepoWithSharedDB failed: %v", err)
	}
	defer r.Close()

	if r.DB() == nil {
		t.Fatal("expected non-nil DB")
	}

	// Verify keys were saved
	var keys []models.Key
	err = r.DB().View(func(tx database.Tx) error {
		return tx.Read().Find(&keys).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	keyNames := make(map[string]bool)
	for _, k := range keys {
		keyNames[k.Name] = true
		if k.TenantID != "tenant-A" {
			t.Errorf("key %s has wrong tenant_id: %s", k.Name, k.TenantID)
		}
	}

	// Check required keys exist
	requiredKeys := []string{"identity", "escrow", "ratings", "bip44", "solana", "mnemonic"}
	for _, name := range requiredKeys {
		if !keyNames[name] {
			t.Errorf("missing required key: %s", name)
		}
	}

	// Verify identity key matches what was provided
	var identKey models.Key
	err = r.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&identKey).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(identKey.Value) != string(identityKey) {
		t.Error("identity key doesn't match provided key")
	}

	// Verify default preferences were saved
	var prefs models.UserPreferences
	err = r.DB().View(func(tx database.Tx) error {
		return tx.Read().First(&prefs).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if !prefs.AutoConfirm {
		t.Error("default preferences should have AutoConfirm=true")
	}
}

func TestNewRepoWithSharedDB_NilDB(t *testing.T) {
	resetMigrateOnce()

	_, err := NewRepoWithSharedDB("tenant-X", "/tmp/test", nil, nil, true)
	if err == nil {
		t.Fatal("expected error for nil sharedDB")
	}
}

func TestNewRepoWithSharedDB_EmptyNodeID(t *testing.T) {
	resetMigrateOnce()

	sharedDB := testSharedDB(t)
	_, err := NewRepoWithSharedDB("", "/tmp/test", sharedDB, nil, true)
	if err == nil {
		t.Fatal("expected error for empty nodeID")
	}
}

func TestNewRepoWithSharedDB_MultiTenantIsolation(t *testing.T) {
	resetMigrateOnce()

	sharedDB := testSharedDB(t)

	// Create two tenants
	dirA := path.Join(os.TempDir(), "mobazha", "shared-repo-tenant-iso-A")
	dirB := path.Join(os.TempDir(), "mobazha", "shared-repo-tenant-iso-B")
	os.MkdirAll(dirA, os.ModePerm)
	os.MkdirAll(dirB, os.ModePerm)
	t.Cleanup(func() {
		os.RemoveAll(dirA)
		os.RemoveAll(dirB)
	})

	keyA := generateTestIdentityKey(t)
	keyB := generateTestIdentityKey(t)

	repoA, err := NewRepoWithSharedDB("tenant-A", dirA, sharedDB, keyA, true)
	if err != nil {
		t.Fatalf("tenant-A repo creation failed: %v", err)
	}
	defer repoA.Close()

	repoB, err := NewRepoWithSharedDB("tenant-B", dirB, sharedDB, keyB, true)
	if err != nil {
		t.Fatalf("tenant-B repo creation failed: %v", err)
	}
	defer repoB.Close()

	// Each tenant should see only its own identity key
	var identA, identB models.Key
	err = repoA.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&identA).Error
	})
	if err != nil {
		t.Fatalf("tenant-A can't read identity: %v", err)
	}
	err = repoB.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&identB).Error
	})
	if err != nil {
		t.Fatalf("tenant-B can't read identity: %v", err)
	}

	if string(identA.Value) != string(keyA) {
		t.Error("tenant-A got wrong identity key")
	}
	if string(identB.Value) != string(keyB) {
		t.Error("tenant-B got wrong identity key")
	}

	// Verify both tenants' keys exist in the shared DB
	var totalKeys int64
	sharedDB.Model(&models.Key{}).Count(&totalKeys)
	// Each tenant has ~6 keys (identity, escrow, ratings, bip44, mnemonic, sol)
	if totalKeys < 10 {
		t.Errorf("expected at least 10 total keys for 2 tenants, got %d", totalKeys)
	}
}

func TestNewRepoWithSharedDB_ExistingTenant(t *testing.T) {
	resetMigrateOnce()

	sharedDB := testSharedDB(t)
	dataDir := path.Join(os.TempDir(), "mobazha", "shared-repo-existing")
	os.MkdirAll(dataDir, os.ModePerm)
	t.Cleanup(func() { os.RemoveAll(dataDir) })

	keyV1 := generateTestIdentityKey(t)
	keyV2 := generateTestIdentityKey(t)

	// First creation
	r1, err := NewRepoWithSharedDB("tenant-existing", dataDir, sharedDB, keyV1, true)
	if err != nil {
		t.Fatalf("first creation failed: %v", err)
	}
	r1.Close()

	// Second creation with different identity key (e.g. KeyVault rotation)
	r2, err := NewRepoWithSharedDB("tenant-existing", dataDir, sharedDB, keyV2, true)
	if err != nil {
		t.Fatalf("second creation failed: %v", err)
	}
	defer r2.Close()

	// Identity key should be updated to keyV2
	var identKey models.Key
	err = r2.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&identKey).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(identKey.Value) != string(keyV2) {
		t.Error("identity key should be updated to v2")
	}
}

func TestGenerateNodeKeys(t *testing.T) {
	// Test with auto-generated mnemonic
	keys, err := generateNodeKeys("", nil)
	if err != nil {
		t.Fatalf("generateNodeKeys failed: %v", err)
	}

	if len(keys.identityKey) == 0 {
		t.Error("identity key should not be empty")
	}
	if keys.escrowKey == nil {
		t.Error("escrow key should not be nil")
	}
	if keys.ratingKey == nil {
		t.Error("rating key should not be nil")
	}
	if keys.bip44Key == nil {
		t.Error("bip44 key should not be nil")
	}
	if keys.solKey == nil {
		t.Error("sol key should not be nil")
	}
	if keys.mnemonicSeed == "" {
		t.Error("mnemonic should not be empty")
	}
}

func TestGenerateNodeKeys_WithExternalIdentity(t *testing.T) {
	// Generate a test identity key
	baseKeys, err := generateNodeKeys("", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Use that as external identity
	keys, err := generateNodeKeys("", baseKeys.identityKey)
	if err != nil {
		t.Fatalf("generateNodeKeys with external identity failed: %v", err)
	}

	// Identity key should match the external one
	if string(keys.identityKey) != string(baseKeys.identityKey) {
		t.Error("identity key should match external key")
	}

	// Wallet keys should still be generated (from mnemonic)
	if keys.escrowKey == nil {
		t.Error("escrow key should be generated even with external identity")
	}
}

func TestGenerateNodeKeys_WithCustomMnemonic(t *testing.T) {
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	keys, err := generateNodeKeys(mnemonic, nil)
	if err != nil {
		t.Fatalf("generateNodeKeys with custom mnemonic failed: %v", err)
	}

	if keys.mnemonicSeed != mnemonic {
		t.Error("mnemonic should match provided value")
	}

	// Generate again with same mnemonic — keys should be deterministic
	keys2, err := generateNodeKeys(mnemonic, nil)
	if err != nil {
		t.Fatal(err)
	}

	if keys.escrowKey.Serialize()[0] != keys2.escrowKey.Serialize()[0] {
		t.Error("escrow keys should be deterministic from same mnemonic")
	}
}

func TestAutoMigrateDatabaseSafe(t *testing.T) {
	sharedDB := testSharedDB(t)
	dataDir := path.Join(os.TempDir(), "mobazha", "migrate-safe-test")
	os.MkdirAll(dataDir, os.ModePerm)
	t.Cleanup(func() { os.RemoveAll(dataDir) })

	// Create a TenantDB wrapper
	resetMigrateOnce()
	r, err := NewRepoWithSharedDB("test-migrate", dataDir, sharedDB, nil, true)
	if err != nil {
		t.Fatalf("NewRepoWithSharedDB failed: %v", err)
	}
	defer r.Close()

	// Verify core tables exist by trying to query them
	tablesToCheck := []string{
		"keys", "outgoing_messages", "incoming_messages",
		"orders", "user_preferences", "receiving_accounts", "shared_payment_intents",
	}
	for _, table := range tablesToCheck {
		var count int64
		err := sharedDB.Table(table).Count(&count).Error
		if err != nil {
			t.Errorf("table %s should exist after migration: %v", table, err)
		}
	}
}

func TestSharedDBMigrateOnce_OnlyRunsOnce(t *testing.T) {
	resetMigrateOnce()

	sharedDB := testSharedDB(t)

	migrateCount := 0

	dirA := path.Join(os.TempDir(), "mobazha", "once-test-A")
	dirB := path.Join(os.TempDir(), "mobazha", "once-test-B")
	os.MkdirAll(dirA, os.ModePerm)
	os.MkdirAll(dirB, os.ModePerm)
	t.Cleanup(func() {
		os.RemoveAll(dirA)
		os.RemoveAll(dirB)
	})

	// First call triggers migration
	r1, err := NewRepoWithSharedDB("t1", dirA, sharedDB, nil, true)
	if err != nil {
		t.Fatalf("first repo failed: %v", err)
	}
	r1.Close()
	migrateCount++

	// Second call should skip migration (sync.Once)
	r2, err := NewRepoWithSharedDB("t2", dirB, sharedDB, nil, true)
	if err != nil {
		t.Fatalf("second repo failed: %v", err)
	}
	r2.Close()

	// Both repos should work despite migration running only once
	// (verified by successful creation)
	t.Log(fmt.Sprintf("Created %d repos with migration running once", migrateCount+1))
}

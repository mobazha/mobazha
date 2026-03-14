package database

import (
	"reflect"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type testModel struct {
	TenantID string `gorm:"primaryKey"`
	ID       int    `gorm:"primaryKey;autoIncrement:false"`
	Name     string
	BizKey   string `gorm:"uniqueIndex:idx_biz"`
}

type mockReadSaver struct {
	db *gorm.DB
}

func (m *mockReadSaver) Read() *gorm.DB          { return m.db }
func (m *mockReadSaver) Save(i interface{}) error { return m.db.Save(i).Error }

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&testModel{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSaveByBusinessKey_Insert(t *testing.T) {
	db := setupTestDB(t)
	tx := &mockReadSaver{db: db}

	m := &testModel{TenantID: "t1", ID: 1, Name: "first", BizKey: "bk1"}
	if err := SaveByBusinessKey(tx, m, "biz_key = ?", "bk1"); err != nil {
		t.Fatal(err)
	}

	var count int64
	db.Model(&testModel{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 record, got %d", count)
	}
}

func TestSaveByBusinessKey_UpdateExisting(t *testing.T) {
	db := setupTestDB(t)
	tx := &mockReadSaver{db: db}

	original := &testModel{TenantID: "t1", ID: 1, Name: "original", BizKey: "bk1"}
	if err := db.Create(original).Error; err != nil {
		t.Fatal(err)
	}

	updated := &testModel{TenantID: "t1", Name: "updated", BizKey: "bk1"}
	if err := SaveByBusinessKey(tx, updated, "biz_key = ?", "bk1"); err != nil {
		t.Fatal(err)
	}

	if updated.ID != 1 {
		t.Errorf("expected ID=1 (from existing), got %d", updated.ID)
	}

	var count int64
	db.Model(&testModel{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 record after upsert, got %d", count)
	}

	var result testModel
	db.First(&result, "biz_key = ?", "bk1")
	if result.Name != "updated" {
		t.Errorf("expected Name=updated, got %s", result.Name)
	}
}

func TestSaveByBusinessKey_NoExisting_NewInsert(t *testing.T) {
	db := setupTestDB(t)
	tx := &mockReadSaver{db: db}

	m := &testModel{TenantID: "t1", ID: 5, Name: "new", BizKey: "bk_new"}
	if err := SaveByBusinessKey(tx, m, "biz_key = ?", "bk_nonexistent"); err != nil {
		t.Fatal(err)
	}

	var count int64
	db.Model(&testModel{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 record, got %d", count)
	}
}

func TestSaveByBusinessKey_CopiesIDField(t *testing.T) {
	m := &testModel{ID: 0}
	existing := &testModel{ID: 42}

	srcID := reflect.ValueOf(existing).Elem().FieldByName("ID")
	dstID := reflect.ValueOf(m).Elem().FieldByName("ID")
	if srcID.IsValid() && dstID.IsValid() && dstID.CanSet() {
		dstID.Set(srcID)
	}
	if m.ID != 42 {
		t.Errorf("expected ID=42, got %d", m.ID)
	}
}

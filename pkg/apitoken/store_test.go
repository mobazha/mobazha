package apitoken

import (
	"testing"

	"github.com/mobazha/mobazha/pkg/database/sqlitedialect"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	return db
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(newTestDB(t))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestStore_CreateAndFindByPrefix(t *testing.T) {
	store := newTestStore(t)

	raw, tok, err := Generate("test1", []string{"listings:read"}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := store.Create(tok); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tok.ID == 0 {
		t.Error("expected non-zero ID after Create")
	}

	found, err := store.FindByPrefix(tok.TokenPrefix)
	if err != nil {
		t.Fatalf("FindByPrefix: %v", err)
	}
	if found.Name != "test1" {
		t.Errorf("expected name 'test1', got %q", found.Name)
	}
	if !Verify(raw, found.TokenHash) {
		t.Error("hash should match original raw token")
	}
}

func TestStore_FindByHash(t *testing.T) {
	store := newTestStore(t)

	raw, tok, err := Generate("hash-test", []string{"ai:use"}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := store.Create(tok); err != nil {
		t.Fatalf("Create: %v", err)
	}

	hash := HashRaw(raw)
	found, err := store.FindByHash(hash)
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if found.ID != tok.ID {
		t.Errorf("expected ID %d, got %d", tok.ID, found.ID)
	}
}

func TestStore_List(t *testing.T) {
	store := newTestStore(t)

	for i := 0; i < 3; i++ {
		_, tok, err := Generate("list-test", []string{"ai:use"}, nil)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if err := store.Create(tok); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	tokens, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tokens) != 3 {
		t.Errorf("expected 3 tokens, got %d", len(tokens))
	}
}

func TestStore_Revoke(t *testing.T) {
	store := newTestStore(t)

	_, tok, err := Generate("revoke-test", []string{"ai:use"}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := store.Create(tok); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Revoke(tok.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	// FindByPrefix should not find revoked token
	_, err = store.FindByPrefix(tok.TokenPrefix)
	if err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound after revoke, got %v", err)
	}

	// Token should still appear in List
	tokens, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token in list, got %d", len(tokens))
	}
	if tokens[0].IsActive() {
		t.Error("revoked token should not be active")
	}
}

func TestStore_CountActive(t *testing.T) {
	store := newTestStore(t)

	for i := 0; i < 3; i++ {
		_, tok, err := Generate("count-test", []string{"ai:use"}, nil)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if err := store.Create(tok); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	count, err := store.CountActive()
	if err != nil {
		t.Fatalf("CountActive: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 active, got %d", count)
	}

	tokens, _ := store.List()
	if err := store.Revoke(tokens[0].ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	count, err = store.CountActive()
	if err != nil {
		t.Fatalf("CountActive: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 active after revoke, got %d", count)
	}
}

func TestStore_RevokeNotFound(t *testing.T) {
	store := newTestStore(t)
	if err := store.Revoke(99999); err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestStore_TouchUsage(t *testing.T) {
	store := newTestStore(t)

	_, tok, err := Generate("touch-test", []string{"ai:use"}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := store.Create(tok); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tok.LastUsedAt != nil {
		t.Error("LastUsedAt should be nil initially")
	}

	store.TouchUsage(tok.ID)

	found, err := store.FindByHash(tok.TokenHash)
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if found.LastUsedAt == nil {
		t.Error("LastUsedAt should be set after TouchUsage")
	}
}

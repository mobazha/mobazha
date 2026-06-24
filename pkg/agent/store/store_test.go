package store

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database/dbstore"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/database/sqlitedialect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRuntimeStore_CRUD(t *testing.T) {
	rs := NewRuntimeStore()

	thread := &Thread{
		ID:         "th_1",
		TenantID:   "tenant_a",
		Persona:    "selection",
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}

	rs.UpdateThread(thread)

	got, ok := rs.GetThread("tenant_a", "th_1")
	if !ok {
		t.Fatal("expected to find thread")
	}
	if got.TenantID != "tenant_a" {
		t.Errorf("expected tenant_a, got %s", got.TenantID)
	}

	_, ok = rs.GetThread("tenant_a", "nonexistent")
	if ok {
		t.Error("should not find nonexistent thread")
	}
}

func TestRuntimeStore_Remove(t *testing.T) {
	rs := NewRuntimeStore()
	rs.UpdateThread(&Thread{ID: "th_1", TenantID: "t1", LastActive: time.Now()})
	rs.RemoveThread("t1", "th_1")
	if rs.Count() != 0 {
		t.Errorf("expected 0 threads, got %d", rs.Count())
	}
}

func TestRuntimeStore_CleanupIdle(t *testing.T) {
	rs := NewRuntimeStore()

	rs.UpdateThread(&Thread{
		ID:         "old",
		TenantID:   "t1",
		LastActive: time.Now().Add(-2 * time.Hour),
	})
	rs.UpdateThread(&Thread{
		ID:         "fresh",
		TenantID:   "t1",
		LastActive: time.Now(),
	})

	removed := rs.CleanupIdle(1 * time.Hour)
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if rs.Count() != 1 {
		t.Errorf("expected 1 remaining, got %d", rs.Count())
	}
	_, ok := rs.GetThread("t1", "fresh")
	if !ok {
		t.Error("fresh thread should remain")
	}
}

func TestRuntimeStore_TenantIsolation(t *testing.T) {
	rs := NewRuntimeStore()

	rs.UpdateThread(&Thread{ID: "th_shared", TenantID: "tenant_1", Persona: "sel_1", LastActive: time.Now()})
	rs.UpdateThread(&Thread{ID: "th_shared", TenantID: "tenant_2", Persona: "sel_2", LastActive: time.Now()})

	a, ok := rs.GetThread("tenant_1", "th_shared")
	if !ok || a.TenantID != "tenant_1" || a.Persona != "sel_1" {
		t.Error("tenant_1 thread mismatch or overwritten by tenant_2")
	}
	b, ok := rs.GetThread("tenant_2", "th_shared")
	if !ok || b.TenantID != "tenant_2" || b.Persona != "sel_2" {
		t.Error("tenant_2 thread mismatch")
	}
	if rs.Count() != 2 {
		t.Errorf("expected 2 threads, got %d", rs.Count())
	}

	// Wrong tenant cannot find the thread
	_, ok = rs.GetThread("tenant_3", "th_shared")
	if ok {
		t.Error("tenant_3 should not find th_shared")
	}
}

func TestRuntimeStore_MessageHistory(t *testing.T) {
	rs := NewRuntimeStore()

	rs.AppendMessage("t1", "th1", &Message{ID: "m1", TenantID: "t1", Role: "user", Content: "hello"})
	rs.AppendMessage("t1", "th1", &Message{ID: "m2", TenantID: "t1", Role: "assistant", Content: "hi there"})
	rs.AppendMessage("t1", "th1", &Message{ID: "m3", TenantID: "t1", Role: "user", Content: "how are you"})

	msgs := rs.GetMessages("t1", "th1")
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" {
		t.Errorf("expected first message 'hello', got %q", msgs[0].Content)
	}
	if msgs[2].Content != "how are you" {
		t.Errorf("expected last message 'how are you', got %q", msgs[2].Content)
	}
}

func TestRuntimeStore_MessageTenantIsolation(t *testing.T) {
	rs := NewRuntimeStore()

	rs.AppendMessage("t1", "th1", &Message{ID: "m1", TenantID: "t1", Role: "user", Content: "tenant1 msg"})
	rs.AppendMessage("t2", "th1", &Message{ID: "m2", TenantID: "t2", Role: "user", Content: "tenant2 msg"})

	t1Msgs := rs.GetMessages("t1", "th1")
	if len(t1Msgs) != 1 || t1Msgs[0].Content != "tenant1 msg" {
		t.Error("tenant1 should only see its own messages")
	}

	t2Msgs := rs.GetMessages("t2", "th1")
	if len(t2Msgs) != 1 || t2Msgs[0].Content != "tenant2 msg" {
		t.Error("tenant2 should only see its own messages")
	}
}

func TestRuntimeStore_TruncateMessages(t *testing.T) {
	rs := NewRuntimeStore()

	for i := 0; i < 10; i++ {
		rs.AppendMessage("t1", "th1", &Message{
			ID:       "m" + string(rune('0'+i)),
			TenantID: "t1",
			Role:     "user",
			Content:  "msg " + string(rune('0'+i)),
		})
	}

	rs.TruncateMessages("t1", "th1", 3)
	msgs := rs.GetMessages("t1", "th1")
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages after truncation, got %d", len(msgs))
	}
}

func TestRuntimeStore_GetThread_ReturnsDefensiveCopy(t *testing.T) {
	rs := NewRuntimeStore()
	rs.UpdateThread(&Thread{ID: "th_1", TenantID: "t1", Persona: "original", LastActive: time.Now()})

	got, ok := rs.GetThread("t1", "th_1")
	if !ok {
		t.Fatal("expected to find thread")
	}
	got.Persona = "mutated"

	fresh, ok := rs.GetThread("t1", "th_1")
	if !ok {
		t.Fatal("expected to find thread again")
	}
	if fresh.Persona != "original" {
		t.Error("GetThread should return defensive copy; internal state was modified")
	}
}

func TestRuntimeStore_TouchThread(t *testing.T) {
	rs := NewRuntimeStore()
	oldTime := time.Now().Add(-1 * time.Hour)
	rs.UpdateThread(&Thread{ID: "th_1", TenantID: "t1", LastActive: oldTime})

	rs.TouchThread("t1", "th_1")

	got, ok := rs.GetThread("t1", "th_1")
	if !ok {
		t.Fatal("expected to find thread")
	}
	if got.LastActive.Before(time.Now().Add(-1 * time.Second)) {
		t.Error("TouchThread should have updated LastActive to near now")
	}

	// Touch nonexistent thread should not panic
	rs.TouchThread("t1", "nonexistent")
}

func TestRuntimeStore_GetMessages_ReturnsDefensiveCopy(t *testing.T) {
	rs := NewRuntimeStore()
	rs.AppendMessage("t1", "th1", &Message{ID: "m1", TenantID: "t1", Role: "user", Content: "original"})

	msgs := rs.GetMessages("t1", "th1")
	msgs[0].Content = "modified"

	fresh := rs.GetMessages("t1", "th1")
	if fresh[0].Content != "original" {
		t.Error("GetMessages should return a defensive copy; internal state was modified")
	}
}

func TestGormPersistence_CRUDAndTenantIsolation(t *testing.T) {
	sharedDB, err := gorm.Open(sqlitedialect.Open(t.TempDir()+"/agent-store.db"), &gorm.Config{})
	require.NoError(t, err)
	dbA := newAgentStoreTestTenantDB(t, sharedDB, "tenant_a")
	dbB := newAgentStoreTestTenantDB(t, sharedDB, "tenant_b")
	require.NoError(t, MigrateModels(dbA))

	persist := NewGormPersistence(dbA)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, persist.SaveThread(ctx, &Thread{
		ID:         "shared-thread",
		TenantID:   "tenant_a",
		Persona:    "seller",
		Title:      "Tenant A",
		CreatedAt:  now,
		LastActive: now,
	}))
	require.NoError(t, persist.SaveMessage(ctx, &Message{
		ID:        "msg_a",
		TenantID:  "tenant_a",
		ThreadID:  "shared-thread",
		Role:      "user",
		Content:   "hello a",
		CreatedAt: now,
	}))

	persistB := NewGormPersistence(dbB)
	require.NoError(t, persistB.SaveThread(ctx, &Thread{
		ID:         "shared-thread",
		TenantID:   "tenant_b",
		Persona:    "seller",
		Title:      "Tenant B",
		CreatedAt:  now,
		LastActive: now.Add(time.Minute),
	}))
	require.NoError(t, persistB.SaveMessage(ctx, &Message{
		ID:        "msg_b",
		TenantID:  "tenant_b",
		ThreadID:  "shared-thread",
		Role:      "user",
		Content:   "hello b",
		CreatedAt: now,
	}))

	gotB, err := persistB.LoadThread(ctx, "tenant_b", "shared-thread")
	require.NoError(t, err)
	require.Equal(t, "Tenant B", gotB.Title)
	msgsB, err := persistB.LoadMessages(ctx, "tenant_b", "shared-thread")
	require.NoError(t, err)
	require.Len(t, msgsB, 1)
	require.Equal(t, "hello b", msgsB[0].Content)

	gotA, err := persist.LoadThread(ctx, "tenant_a", "shared-thread")
	require.NoError(t, err)
	require.Equal(t, "Tenant A", gotA.Title)
	msgsA, err := persist.LoadMessages(ctx, "tenant_a", "shared-thread")
	require.NoError(t, err)
	require.Len(t, msgsA, 1)
	require.Equal(t, "hello a", msgsA[0].Content)
}

func newAgentStoreTestTenantDB(t *testing.T, sharedDB *gorm.DB, tenantID string) database.Database {
	t.Helper()
	db, err := dbstore.NewTenantDBWithPublicData(sharedDB, tenantID, dbstore.NewDBPublicData(sharedDB, tenantID))
	require.NoError(t, err)
	return db
}

func TestGormPersistence_DeleteThread(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, MigrateModels(db))

	persist := NewGormPersistence(db)
	ctx := context.Background()
	require.NoError(t, persist.SaveThread(ctx, &Thread{ID: "th", TenantID: "tenant", LastActive: time.Now()}))
	require.NoError(t, persist.SaveTurn(ctx, &Turn{ID: "turn", TenantID: "tenant", ThreadID: "th"}))
	require.NoError(t, persist.SaveMessage(ctx, &Message{ID: "msg", TenantID: "tenant", ThreadID: "th", Role: "user", Content: "hello"}))

	require.NoError(t, persist.DeleteThread(ctx, "tenant", "th"))
	_, err = persist.LoadThread(ctx, "tenant", "th")
	require.ErrorIs(t, err, ErrThreadNotFound)
	msgs, err := persist.LoadMessages(ctx, "tenant", "th")
	require.NoError(t, err)
	require.Empty(t, msgs)
}

func TestGormPersistence_RedactsSensitiveJSON(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, MigrateModels(db))

	persist := NewGormPersistence(db)
	ctx := context.Background()
	require.NoError(t, persist.SaveThread(ctx, &Thread{ID: "th", TenantID: "tenant", LastActive: time.Now()}))
	require.NoError(t, persist.SaveMessage(ctx, &Message{
		ID:        "msg",
		TenantID:  "tenant",
		ThreadID:  "th",
		Role:      "tool",
		Content:   `{"token":"secret-token","value":"safe"}`,
		ToolCalls: `[{"name":"x","arguments":"{\"api_key\":\"secret-key\",\"query\":\"safe\"}"}]`,
	}))

	msgs, err := persist.LoadMessages(ctx, "tenant", "th")
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Contains(t, msgs[0].Content, `"token":"[REDACTED]"`)
	require.NotContains(t, msgs[0].Content, "secret-token")
	require.NotContains(t, msgs[0].ToolCalls, "secret-key")
	require.Contains(t, msgs[0].ToolCalls, `[REDACTED]`)
}

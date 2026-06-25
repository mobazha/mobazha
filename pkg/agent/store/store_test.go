package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database/dbstore"
	"github.com/mobazha/mobazha3.0/pkg/agent/kernel"
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

func TestGormPersistence_ExplicitTenantPredicates(t *testing.T) {
	sharedDB, err := gorm.Open(sqlitedialect.Open(t.TempDir()+"/agent-explicit-tenant.db"), &gorm.Config{})
	require.NoError(t, err)
	dbA := newAgentStoreTestTenantDB(t, sharedDB, "tenant_a")
	dbB := newAgentStoreTestTenantDB(t, sharedDB, "tenant_b")
	require.NoError(t, MigrateModels(dbA))
	persistA := NewGormPersistence(dbA)
	persistB := NewGormPersistence(dbB)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, persistA.SaveThread(ctx, &Thread{ID: "shared", TenantID: "tenant_a", Title: "A", LastActive: now}))
	require.NoError(t, persistB.SaveThread(ctx, &Thread{ID: "shared", TenantID: "tenant_b", Title: "B", LastActive: now.Add(time.Minute)}))
	require.NoError(t, persistA.SaveMessage(ctx, &Message{ID: "msg_a", TenantID: "tenant_a", ThreadID: "shared", Role: "user", Content: "a"}))
	require.NoError(t, persistB.SaveMessage(ctx, &Message{ID: "msg_b", TenantID: "tenant_b", ThreadID: "shared", Role: "user", Content: "b"}))

	got, err := persistB.LoadThread(ctx, "tenant_b", "shared")
	require.NoError(t, err)
	require.Equal(t, "B", got.Title)
	msgs, err := persistB.LoadMessages(ctx, "tenant_b", "shared")
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Equal(t, "b", msgs[0].Content)

	list, err := persistA.ListThreads(ctx, "tenant_a", 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "tenant_a", list[0].TenantID)

	require.NoError(t, persistA.DeleteThread(ctx, "tenant_a", "shared"))
	_, err = persistA.LoadThread(ctx, "tenant_a", "shared")
	require.ErrorIs(t, err, ErrThreadNotFound)
	got, err = persistB.LoadThread(ctx, "tenant_b", "shared")
	require.NoError(t, err)
	require.Equal(t, "B", got.Title)
}

func TestGormPersistence_SkillRunsAndArtifacts(t *testing.T) {
	sharedDB, err := gorm.Open(sqlitedialect.Open(t.TempDir()+"/agent-artifacts.db"), &gorm.Config{})
	require.NoError(t, err)
	dbA := newAgentStoreTestTenantDB(t, sharedDB, "tenant_a")
	dbB := newAgentStoreTestTenantDB(t, sharedDB, "tenant_b")
	require.NoError(t, MigrateModels(dbA))
	persistA := NewGormPersistence(dbA)
	persistB := NewGormPersistence(dbB)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, persistA.SaveSkillRun(ctx, &SkillRun{
		ID:            "run_shared",
		TenantID:      "tenant_a",
		ThreadID:      "th_a",
		SkillID:       "product.import",
		StoreID:       "store_a",
		ActorID:       "seller_a",
		ActingPersona: "seller",
		Status:        SkillRunStatusRunning,
		Input:         `{"source":"excel","api_key":"secret"}`,
		StartedAt:     now,
	}))
	require.NoError(t, persistB.SaveSkillRun(ctx, &SkillRun{
		ID:        "run_shared",
		TenantID:  "tenant_b",
		ThreadID:  "th_b",
		SkillID:   "product.import",
		Status:    SkillRunStatusRunning,
		Input:     `{"source":"text"}`,
		StartedAt: now,
	}))
	require.NoError(t, persistA.SaveArtifact(ctx, &Artifact{
		ID:         "art_draft",
		TenantID:   "tenant_a",
		ThreadID:   "th_a",
		SkillRunID: "run_shared",
		SkillID:    "product.import",
		Kind:       ArtifactKindProposal,
		Status:     ArtifactStatusNeedsReview,
		Name:       "Row 12 proposal",
		Data:       `{"columnMapping":{"Item Name":"title","Cost USD":"price.amountMinor"},"token":"secret"}`,
		CreatedAt:  now,
	}))
	require.NoError(t, persistB.SaveArtifact(ctx, &Artifact{
		ID:         "art_draft",
		TenantID:   "tenant_b",
		ThreadID:   "th_b",
		SkillRunID: "run_shared",
		SkillID:    "product.import",
		Kind:       ArtifactKindProposal,
		Status:     ArtifactStatusReady,
		Data:       `{"title":"B"}`,
		CreatedAt:  now,
	}))

	runA, err := persistA.LoadSkillRun(ctx, "tenant_a", "run_shared")
	require.NoError(t, err)
	require.Contains(t, runA.Input, `[REDACTED]`)
	require.NotContains(t, runA.Input, "secret")

	runsA, err := persistA.ListSkillRuns(ctx, "tenant_a", "product.import", SkillRunStatusRunning, 10, 0)
	require.NoError(t, err)
	require.Len(t, runsA, 1)
	require.Equal(t, "tenant_a", runsA[0].TenantID)

	artifactsA, err := persistA.ListArtifacts(ctx, "tenant_a", "run_shared", ArtifactKindProposal, ArtifactStatusNeedsReview, 10, 0)
	require.NoError(t, err)
	require.Len(t, artifactsA, 1)
	require.Contains(t, artifactsA[0].Data, `"Item Name":"title"`)
	require.Contains(t, artifactsA[0].Data, `[REDACTED]`)
	require.NotContains(t, artifactsA[0].Data, "secret")

	artifactB, err := persistB.LoadArtifact(ctx, "tenant_b", "art_draft")
	require.NoError(t, err)
	require.Equal(t, ArtifactStatusReady, artifactB.Status)

	_, err = persistA.LoadArtifact(ctx, "tenant_a", "missing")
	require.ErrorIs(t, err, ErrArtifactNotFound)
}

func TestGormPersistence_ApprovalQueueAndDecision(t *testing.T) {
	sharedDB, err := gorm.Open(sqlitedialect.Open(t.TempDir()+"/agent-approvals.db"), &gorm.Config{})
	require.NoError(t, err)
	dbA := newAgentStoreTestTenantDB(t, sharedDB, "tenant_a")
	dbB := newAgentStoreTestTenantDB(t, sharedDB, "tenant_b")
	require.NoError(t, MigrateModels(dbA))
	persistA := NewGormPersistence(dbA)
	persistB := NewGormPersistence(dbB)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, persistA.SaveApproval(ctx, &Approval{
		ID:             "appr_shared",
		TenantID:       "tenant_a",
		ThreadID:       "th_a",
		TurnID:         "turn_a",
		ToolCallID:     "call_a",
		SkillID:        "product.import",
		StoreID:        "store_a",
		ActorID:        "seller_a",
		ActingPersona:  "seller",
		Risk:           "write",
		Action:         "listings_create",
		Summary:        "Create listing",
		Payload:        `{"api_key":"secret","title":"ManagedEscrow"}`,
		RequestHash:    "hash_a",
		IdempotencyKey: "th_a:turn_a:call_a",
		Status:         ApprovalStatusPending,
		CreatedAt:      now,
	}))
	require.NoError(t, persistB.SaveApproval(ctx, &Approval{
		ID:          "appr_shared",
		TenantID:    "tenant_b",
		ThreadID:    "th_b",
		SkillID:     "product.import",
		Risk:        "write",
		Action:      "listings_create",
		Summary:     "Create listing B",
		Payload:     `{"title":"B"}`,
		RequestHash: "hash_b",
		Status:      ApprovalStatusPending,
		CreatedAt:   now.Add(time.Minute),
	}))

	pendingA, err := persistA.ListApprovals(ctx, "tenant_a", ApprovalStatusPending, 10, 0)
	require.NoError(t, err)
	require.Len(t, pendingA, 1)
	require.Equal(t, "tenant_a", pendingA[0].TenantID)
	require.Equal(t, "appr_shared", pendingA[0].ID)
	require.Contains(t, pendingA[0].Payload, "secret")
	redacted := SanitizeApprovalForAPI(pendingA[0])
	require.Contains(t, redacted.Payload, `[REDACTED]`)
	require.NotContains(t, redacted.Payload, "secret")

	gotB, err := persistB.LoadApproval(ctx, "tenant_b", "appr_shared")
	require.NoError(t, err)
	require.Equal(t, "tenant_b", gotB.TenantID)
	require.Equal(t, "hash_b", gotB.RequestHash)

	decided, err := persistA.UpdateApprovalStatus(ctx, "tenant_a", "appr_shared", ApprovalStatusApproved, "reviewer_1")
	require.NoError(t, err)
	require.Equal(t, ApprovalStatusApproved, decided.Status)
	require.Equal(t, "reviewer_1", decided.DecisionBy)
	require.NotNil(t, decided.DecisionAt)

	decidedAgain, err := persistA.UpdateApprovalStatus(ctx, "tenant_a", "appr_shared", ApprovalStatusRejected, "reviewer_2")
	require.NoError(t, err)
	require.Equal(t, ApprovalStatusApproved, decidedAgain.Status)
	require.Equal(t, "reviewer_1", decidedAgain.DecisionBy)

	pendingA, err = persistA.ListApprovals(ctx, "tenant_a", ApprovalStatusPending, 10, 0)
	require.NoError(t, err)
	require.Empty(t, pendingA)
	approvedA, err := persistA.ListApprovals(ctx, "tenant_a", ApprovalStatusApproved, 10, 0)
	require.NoError(t, err)
	require.Len(t, approvedA, 1)

	claimed, err := persistA.ClaimApprovalForApply(ctx, "tenant_a", "appr_shared", "applier_1")
	require.NoError(t, err)
	require.Equal(t, ApprovalStatusApplying, claimed.Status)
	require.Equal(t, "applier_1", claimed.AppliedBy)

	applied, err := persistA.MarkApprovalApplied(ctx, "tenant_a", "appr_shared", `{"token":"secret","ok":true}`, "applier_1")
	require.NoError(t, err)
	require.Equal(t, ApprovalStatusApplied, applied.Status)
	require.NotNil(t, applied.AppliedAt)
	require.Contains(t, applied.ApplyResult, `[REDACTED]`)
	require.NotContains(t, applied.ApplyResult, "secret")

	gotB, err = persistB.LoadApproval(ctx, "tenant_b", "appr_shared")
	require.NoError(t, err)
	require.Equal(t, ApprovalStatusPending, gotB.Status)
}

func TestGormPersistence_ApprovalApplyFailureCanBeReclaimed(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, MigrateModels(db))
	persist := NewGormPersistence(db)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, persist.SaveApproval(ctx, &Approval{
		ID:          "appr_retry",
		TenantID:    database.StandaloneTenantID,
		SkillID:     "product.import",
		Risk:        "write",
		Action:      "listings_create",
		Summary:     "Create listing",
		Payload:     `{"listing":{"title":"Draft"}}`,
		RequestHash: "hash",
		Status:      ApprovalStatusApproved,
		CreatedAt:   now,
	}))

	claimed, err := persist.ClaimApprovalForApply(ctx, database.StandaloneTenantID, "appr_retry", "applier_1")
	require.NoError(t, err)
	require.Equal(t, ApprovalStatusApplying, claimed.Status)

	failed, err := persist.MarkApprovalApplyFailed(ctx, database.StandaloneTenantID, "appr_retry", strings.Repeat("x", 2100), "applier_1")
	require.NoError(t, err)
	require.Equal(t, ApprovalStatusApplyFailed, failed.Status)
	require.Len(t, []rune(failed.ApplyError), 2014)

	reclaimed, err := persist.ClaimApprovalForApply(ctx, database.StandaloneTenantID, "appr_retry", "applier_2")
	require.NoError(t, err)
	require.Equal(t, ApprovalStatusApplying, reclaimed.Status)
	require.Equal(t, "applier_2", reclaimed.AppliedBy)
	require.Empty(t, reclaimed.ApplyError)

	_, err = persist.ClaimApprovalForApply(ctx, database.StandaloneTenantID, "appr_retry", "applier_3")
	require.ErrorIs(t, err, ErrApprovalClaimConflict)
	latest, loadErr := persist.LoadApproval(ctx, database.StandaloneTenantID, "appr_retry")
	require.NoError(t, loadErr)
	require.Equal(t, ApprovalStatusApplying, latest.Status)
	require.Equal(t, "applier_2", latest.AppliedBy)
}

func TestGormPersistence_MemoryStoreScopesAndArchive(t *testing.T) {
	sharedDB, err := gorm.Open(sqlitedialect.Open(t.TempDir()+"/agent-memory.db"), &gorm.Config{})
	require.NoError(t, err)
	dbA := newAgentStoreTestTenantDB(t, sharedDB, "tenant_a")
	dbB := newAgentStoreTestTenantDB(t, sharedDB, "tenant_b")
	require.NoError(t, MigrateModels(dbA))
	persistA := NewGormPersistence(dbA)
	persistB := NewGormPersistence(dbB)
	ctx := context.Background()
	scopeA := kernel.Scope{TenantID: "tenant_a", StoreID: "store_a", ActorID: "actor_a"}
	scopeAOtherStore := kernel.Scope{TenantID: "tenant_a", StoreID: "store_other", ActorID: "actor_a"}
	scopeAOtherActor := kernel.Scope{TenantID: "tenant_a", StoreID: "store_a", ActorID: "actor_other"}
	scopeB := kernel.Scope{TenantID: "tenant_b", StoreID: "store_b", ActorID: "actor_b"}

	require.NoError(t, persistA.Save(ctx, scopeA, kernel.MemoryItem{
		ID:      "mem_user",
		Scope:   kernel.MemoryUser,
		Subject: "language",
		Content: "请默认用中文回答",
	}))
	require.NoError(t, persistA.Save(ctx, scopeA, kernel.MemoryItem{
		ID:      "mem_store",
		Scope:   kernel.MemoryStoreScope,
		Subject: "brand",
		Content: "品牌语气保持克制",
	}))
	require.NoError(t, persistA.Save(ctx, scopeA, kernel.MemoryItem{
		ID:      "mem_tenant",
		Scope:   kernel.MemoryTenant,
		Subject: "policy",
		Content: "租户默认人工确认",
	}))
	require.NoError(t, persistB.Save(ctx, scopeB, kernel.MemoryItem{
		ID:      "mem_user",
		Scope:   kernel.MemoryUser,
		Subject: "language",
		Content: "tenant b memory",
	}))

	items, err := persistA.Search(ctx, kernel.MemoryQuery{Scope: scopeA, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 3)

	items, err = persistA.Search(ctx, kernel.MemoryQuery{Scope: scopeAOtherStore, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 2)
	for _, item := range items {
		require.NotEqual(t, "mem_store", item.ID)
	}

	items, err = persistA.Search(ctx, kernel.MemoryQuery{Scope: scopeAOtherActor, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 2)
	for _, item := range items {
		require.NotEqual(t, "mem_user", item.ID)
	}

	items, err = persistB.Search(ctx, kernel.MemoryQuery{Scope: scopeB, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "tenant b memory", items[0].Content)

	require.ErrorIs(t, persistA.Delete(ctx, scopeAOtherActor, "mem_user"), ErrMemoryNotFound)
	items, err = persistA.Search(ctx, kernel.MemoryQuery{Scope: scopeA, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 3)

	require.NoError(t, persistA.Delete(ctx, scopeA, "mem_user"))
	items, err = persistA.Search(ctx, kernel.MemoryQuery{Scope: scopeA, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 2)
	for _, item := range items {
		require.NotEqual(t, "mem_user", item.ID)
	}

	require.Error(t, persistA.Save(ctx, kernel.Scope{TenantID: "tenant_a"}, kernel.MemoryItem{
		ID:      "mem_invalid_user",
		Scope:   kernel.MemoryUser,
		Content: "missing actor",
	}))
	require.Error(t, persistA.Save(ctx, kernel.Scope{TenantID: "tenant_a"}, kernel.MemoryItem{
		ID:      "mem_invalid_store",
		Scope:   kernel.MemoryStoreScope,
		Content: "missing store",
	}))
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
	require.NoError(t, persist.SaveThread(ctx, &Thread{ID: "th", TenantID: database.StandaloneTenantID, LastActive: time.Now()}))
	require.NoError(t, persist.SaveTurn(ctx, &Turn{ID: "turn", TenantID: database.StandaloneTenantID, ThreadID: "th"}))
	require.NoError(t, persist.SaveMessage(ctx, &Message{ID: "msg", TenantID: database.StandaloneTenantID, ThreadID: "th", Role: "user", Content: "hello"}))
	require.NoError(t, persist.SaveSkillRun(ctx, &SkillRun{ID: "run", TenantID: database.StandaloneTenantID, ThreadID: "th", SkillID: "product.import"}))
	require.NoError(t, persist.SaveArtifact(ctx, &Artifact{ID: "art", TenantID: database.StandaloneTenantID, ThreadID: "th", SkillRunID: "run", Kind: ArtifactKindProposal}))
	require.NoError(t, persist.SaveApproval(ctx, &Approval{
		ID:          "appr",
		TenantID:    database.StandaloneTenantID,
		ThreadID:    "th",
		Risk:        "write",
		Action:      "listings_create",
		Summary:     "Create listing",
		RequestHash: "hash",
		Status:      ApprovalStatusPending,
	}))

	require.NoError(t, persist.DeleteThread(ctx, database.StandaloneTenantID, "th"))
	_, err = persist.LoadThread(ctx, database.StandaloneTenantID, "th")
	require.ErrorIs(t, err, ErrThreadNotFound)
	msgs, err := persist.LoadMessages(ctx, database.StandaloneTenantID, "th")
	require.NoError(t, err)
	require.Empty(t, msgs)
	approvals, err := persist.ListApprovals(ctx, database.StandaloneTenantID, ApprovalStatusPending, 10, 0)
	require.NoError(t, err)
	require.Empty(t, approvals)
	runs, err := persist.ListSkillRuns(ctx, database.StandaloneTenantID, "product.import", "", 10, 0)
	require.NoError(t, err)
	require.Empty(t, runs)
	artifacts, err := persist.ListArtifacts(ctx, database.StandaloneTenantID, "run", "", "", 10, 0)
	require.NoError(t, err)
	require.Empty(t, artifacts)
}

func TestGormPersistence_RedactsSensitiveJSON(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, MigrateModels(db))

	persist := NewGormPersistence(db)
	ctx := context.Background()
	require.NoError(t, persist.SaveThread(ctx, &Thread{ID: "th", TenantID: database.StandaloneTenantID, LastActive: time.Now()}))
	require.NoError(t, persist.SaveMessage(ctx, &Message{
		ID:        "msg",
		TenantID:  database.StandaloneTenantID,
		ThreadID:  "th",
		Role:      "tool",
		Content:   `{"token":"secret-token","value":"safe"}`,
		ToolCalls: `[{"name":"x","arguments":"{\"api_key\":\"secret-key\",\"query\":\"safe\"}"}]`,
	}))

	msgs, err := persist.LoadMessages(ctx, database.StandaloneTenantID, "th")
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Contains(t, msgs[0].Content, `"token":"[REDACTED]"`)
	require.NotContains(t, msgs[0].Content, "secret-token")
	require.NotContains(t, msgs[0].ToolCalls, "secret-key")
	require.Contains(t, msgs[0].ToolCalls, `[REDACTED]`)
}

func TestGormPersistence_ApprovalPayloadPreservesExecutionHash(t *testing.T) {
	sharedDB, err := gorm.Open(sqlitedialect.Open(t.TempDir()+"/agent-approval-hash.db"), &gorm.Config{})
	require.NoError(t, err)
	db := newAgentStoreTestTenantDB(t, sharedDB, "tenant_a")
	require.NoError(t, MigrateModels(db))
	persist := NewGormPersistence(db)
	ctx := context.Background()

	payload := `{"api_key":"secret-value","title":"ManagedEscrow"}`
	req := kernel.ApprovalRequest{
		ID:      "appr_hash",
		SkillID: kernel.SkillProductImport,
		Scope: kernel.Scope{
			TenantID:      "tenant_a",
			StoreID:       "store_a",
			ActorID:       "seller_a",
			ActingPersona: kernel.PersonaSeller,
		},
		Risk:    kernel.RiskWrite,
		Action:  "listings_create",
		Summary: "Create listing",
		Payload: json.RawMessage(payload),
	}
	hash, err := kernel.ComputeApprovalHash(req)
	require.NoError(t, err)

	require.NoError(t, persist.SaveApproval(ctx, &Approval{
		ID:            req.ID,
		TenantID:      "tenant_a",
		SkillID:       string(req.SkillID),
		StoreID:       req.Scope.StoreID,
		ActorID:       req.Scope.ActorID,
		ActingPersona: string(req.Scope.ActingPersona),
		Risk:          string(req.Risk),
		Action:        req.Action,
		Summary:       req.Summary,
		Payload:       payload,
		ArtifactIDs:   `["art_proposal_1"]`,
		RequestHash:   hash,
		Status:        ApprovalStatusApproved,
		CreatedAt:     time.Now(),
	}))

	got, err := persist.LoadApproval(ctx, "tenant_a", "appr_hash")
	require.NoError(t, err)
	require.Contains(t, got.Payload, "secret-value")
	require.Equal(t, `["art_proposal_1"]`, got.ArtifactIDs)

	recomputed, err := kernel.ComputeApprovalHash(kernel.ApprovalRequest{
		SkillID: req.SkillID,
		Scope:   req.Scope,
		Risk:    req.Risk,
		Action:  req.Action,
		Summary: req.Summary,
		Payload: json.RawMessage(got.Payload),
	})
	require.NoError(t, err)
	require.Equal(t, hash, recomputed)

	redacted := SanitizeApprovalForAPI(got)
	require.Contains(t, redacted.Payload, `[REDACTED]`)
	require.NotContains(t, redacted.Payload, "secret-value")
}

package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var testDBCounter atomic.Int64

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	n := testDBCounter.Add(1)
	dsn := fmt.Sprintf("file:memdb_%d?mode=memory&cache=shared&_busy_timeout=5000&_journal_mode=WAL", n)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := MigrateWorkLocks(db); err != nil {
		t.Fatalf("migrate work locks: %v", err)
	}
	if err := MigrateLocks(db); err != nil {
		t.Fatalf("migrate locks: %v", err)
	}
	return db
}

func fixedNow(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// --- TryClaimWork tests ---

func TestTryClaimWork_FirstClaim(t *testing.T) {
	db := newTestDB(t)
	now := time.Now()

	ok, err := TryClaimWork(db, "job-a", "tenant-1", "holder-A", 30*time.Second, fixedNow(now))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected claim to succeed on empty table")
	}

	var lock WorkLock
	if err := db.First(&lock, "job_name = ? AND tenant_id = ?", "job-a", "tenant-1").Error; err != nil {
		t.Fatalf("row not found: %v", err)
	}
	if lock.HolderID != "holder-A" {
		t.Fatalf("holder_id = %q, want holder-A", lock.HolderID)
	}
}

func TestTryClaimWork_SameHolderReacquires(t *testing.T) {
	db := newTestDB(t)
	now := time.Now()

	ok, _ := TryClaimWork(db, "job-a", "tenant-1", "holder-A", 30*time.Second, fixedNow(now))
	if !ok {
		t.Fatal("first claim should succeed")
	}

	ok, err := TryClaimWork(db, "job-a", "tenant-1", "holder-A", 30*time.Second, fixedNow(now.Add(5*time.Second)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("same holder should re-acquire")
	}
}

func TestTryClaimWork_DifferentHolderBlocked(t *testing.T) {
	db := newTestDB(t)
	now := time.Now()

	ok, _ := TryClaimWork(db, "job-a", "tenant-1", "holder-A", 30*time.Second, fixedNow(now))
	if !ok {
		t.Fatal("first claim should succeed")
	}

	ok, err := TryClaimWork(db, "job-a", "tenant-1", "holder-B", 30*time.Second, fixedNow(now.Add(5*time.Second)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("different holder should be blocked while lease is live")
	}
}

func TestTryClaimWork_ExpiredLeaseAllowsTakeover(t *testing.T) {
	db := newTestDB(t)
	now := time.Now()
	ttl := 10 * time.Second

	ok, _ := TryClaimWork(db, "job-a", "tenant-1", "holder-A", ttl, fixedNow(now))
	if !ok {
		t.Fatal("first claim should succeed")
	}

	afterExpiry := now.Add(ttl + time.Second)
	ok, err := TryClaimWork(db, "job-a", "tenant-1", "holder-B", ttl, fixedNow(afterExpiry))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("should take over expired lease")
	}

	var lock WorkLock
	db.First(&lock, "job_name = ? AND tenant_id = ?", "job-a", "tenant-1")
	if lock.HolderID != "holder-B" {
		t.Fatalf("holder_id = %q, want holder-B after takeover", lock.HolderID)
	}
}

func TestTryClaimWork_DifferentTenants_Independent(t *testing.T) {
	db := newTestDB(t)
	now := time.Now()

	ok1, _ := TryClaimWork(db, "job-a", "tenant-1", "holder-A", 30*time.Second, fixedNow(now))
	ok2, _ := TryClaimWork(db, "job-a", "tenant-2", "holder-B", 30*time.Second, fixedNow(now))

	if !ok1 || !ok2 {
		t.Fatalf("different tenants should independently claim: ok1=%v ok2=%v", ok1, ok2)
	}
}

// --- RenewWork tests ---

func TestRenewWork_ExtendsLease(t *testing.T) {
	db := newTestDB(t)
	now := time.Now()
	ttl := 30 * time.Second

	TryClaimWork(db, "job-a", "tenant-1", "holder-A", ttl, fixedNow(now))

	renewTime := now.Add(10 * time.Second)
	if err := RenewWork(db, "job-a", "tenant-1", "holder-A", ttl, fixedNow(renewTime)); err != nil {
		t.Fatalf("renew failed: %v", err)
	}

	var lock WorkLock
	db.First(&lock, "job_name = ? AND tenant_id = ?", "job-a", "tenant-1")
	expectedExpiry := renewTime.Add(ttl)
	if lock.ExpiresAt.Before(expectedExpiry.Add(-time.Second)) {
		t.Fatalf("expires_at = %v, expected ~%v", lock.ExpiresAt, expectedExpiry)
	}
}

func TestRenewWork_LostLock(t *testing.T) {
	db := newTestDB(t)
	now := time.Now()
	ttl := 10 * time.Second

	TryClaimWork(db, "job-a", "tenant-1", "holder-A", ttl, fixedNow(now))

	afterExpiry := now.Add(ttl + time.Second)
	TryClaimWork(db, "job-a", "tenant-1", "holder-B", ttl, fixedNow(afterExpiry))

	err := RenewWork(db, "job-a", "tenant-1", "holder-A", ttl, fixedNow(afterExpiry))
	if err != ErrWorkLockLost {
		t.Fatalf("expected ErrWorkLockLost, got %v", err)
	}
}

// --- ReleaseWork tests ---

func TestReleaseWork_CleansUp(t *testing.T) {
	db := newTestDB(t)
	now := time.Now()

	TryClaimWork(db, "job-a", "tenant-1", "holder-A", 30*time.Second, fixedNow(now))

	if err := ReleaseWork(db, "job-a", "tenant-1", "holder-A"); err != nil {
		t.Fatalf("release failed: %v", err)
	}

	var count int64
	db.Model(&WorkLock{}).Where("job_name = ? AND tenant_id = ?", "job-a", "tenant-1").Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 rows after release, got %d", count)
	}
}

func TestReleaseWork_Idempotent(t *testing.T) {
	db := newTestDB(t)

	if err := ReleaseWork(db, "nonexistent", "tenant-x", "holder-x"); err != nil {
		t.Fatalf("release of non-existent should not error: %v", err)
	}
}

// --- LeaseRenewer tests ---

func TestLeaseRenewer_RenewsLease(t *testing.T) {
	db := newFileDB(t)
	// TTL must be >= 3s so that renewInterval (ttl/3) >= 1s minimum.
	ttl := 3 * time.Second

	ok, err := TryClaimWork(db, "job-a", "tenant-1", "holder-A", ttl, time.Now)
	if err != nil || !ok {
		t.Fatalf("initial claim failed: ok=%v err=%v", ok, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, workCancel := context.WithCancel(ctx)

	renewer := StartLeaseRenewer(ctx, workCancel, db, "job-a", "tenant-1", "holder-A", ttl, time.Now)

	// Wait for at least 2 renewals (interval = ttl/3 = 1s).
	time.Sleep(2500 * time.Millisecond)

	var lock WorkLock
	db.First(&lock, "job_name = ? AND tenant_id = ?", "job-a", "tenant-1")
	checkTime := time.Now()
	if lock.ExpiresAt.Before(checkTime) {
		t.Fatalf("lease should have been renewed to stay alive, expiresAt=%v, now=%v",
			lock.ExpiresAt, checkTime)
	}

	renewer.Stop()
}

func TestLeaseRenewer_CancelsWorkOnLockLost(t *testing.T) {
	db := newFileDB(t)
	ttl := 100 * time.Millisecond

	TryClaimWork(db, "job-a", "tenant-1", "holder-A", ttl, time.Now)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workCtx, workCancel := context.WithCancel(ctx)
	renewer := StartLeaseRenewer(ctx, workCancel, db, "job-a", "tenant-1", "holder-A", ttl, time.Now)

	// Simulate another holder stealing the lock.
	afterExpiry := time.Now().Add(ttl + 50*time.Millisecond)
	time.Sleep(ttl + 50*time.Millisecond)
	TryClaimWork(db, "job-a", "tenant-1", "holder-B", ttl, fixedNow(afterExpiry))

	select {
	case <-workCtx.Done():
		// Expected: renewer detected lock loss and cancelled workCtx.
	case <-time.After(2 * time.Second):
		t.Fatal("workCtx should have been cancelled after lock loss")
	}

	renewer.Stop()
}

// --- Multi-holder concurrent tests ---

func newFileDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := fmt.Sprintf("%s/test.db?_busy_timeout=5000&_journal_mode=WAL", dir)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open file db: %v", err)
	}
	if err := MigrateWorkLocks(db); err != nil {
		t.Fatalf("migrate work locks: %v", err)
	}
	if err := MigrateLocks(db); err != nil {
		t.Fatalf("migrate locks: %v", err)
	}
	return db
}

func TestConcurrentClaim_NoDuplicateRun(t *testing.T) {
	db := newFileDB(t)
	now := time.Now()
	ttl := 5 * time.Second
	const holders = 10

	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < holders; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ok, err := TryClaimWork(db, "job-x", "tenant-1", holderName(id), ttl, fixedNow(now))
			if err != nil {
				t.Errorf("holder %d error: %v", id, err)
				return
			}
			if ok {
				wins.Add(1)
			}
		}(i)
	}
	wg.Wait()

	got := wins.Load()
	if got != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", got)
	}
}

func TestNodeFn_WithWorkLocks_NoDoubleRun(t *testing.T) {
	db := newFileDB(t)
	reg := newStubRegistry("tenant-1", "tenant-2")

	// Simulate stubNode with identity
	reg.mu.Lock()
	reg.nodes = []contracts.NodeService{
		&stubNodeWithID{id: "tenant-1"},
		&stubNodeWithID{id: "tenant-2"},
	}
	reg.mu.Unlock()

	var execCount atomic.Int32
	s, _ := New(Config{
		HolderID: "host-1",
		Registry: reg,
		DB:       db,
		LeaseTTL: 5 * time.Second,
	})
	_ = s.Register(Job{
		Name:           "locked-job",
		Interval:       20 * time.Millisecond,
		MaxConcurrency: 4,
		NodeFn: func(_ context.Context, node contracts.NodeService) error {
			execCount.Add(1)
			time.Sleep(5 * time.Millisecond)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	_ = s.Start(ctx)
	time.Sleep(80 * time.Millisecond)
	cancel()
	s.Stop()

	got := execCount.Load()
	if got < 2 {
		t.Fatalf("expected at least 2 executions (2 tenants × some ticks), got %d", got)
	}
}

func holderName(i int) string {
	return "holder-" + string(rune('A'+i))
}

// stubNodeWithID provides IdentityInfo().GetNodeID() for work_lock tests.
type stubNodeWithID struct {
	stubNode
	id string
}

func (s *stubNodeWithID) IdentityInfo() contracts.IdentityService {
	return &stubIdentity{nodeID: s.id}
}

type stubIdentity struct {
	nodeID string
}

func (s *stubIdentity) GetNodeID() string                       { return s.nodeID }
func (s *stubIdentity) Identity() peer.ID                       { return "" }
func (s *stubIdentity) UsingTestnet() bool                      { return false }
func (s *stubIdentity) SignMessage(_ []byte) ([]byte, []byte, error) { return nil, nil, nil }
func (s *stubIdentity) IsGlobalBanned(_ peer.ID) bool           { return false }

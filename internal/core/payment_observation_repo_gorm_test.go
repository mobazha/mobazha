//go:build !private_distribution

package core

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ─────────────────────────────────────────────────────────────────────────────
// PaymentObservation repo test infrastructure
// ─────────────────────────────────────────────────────────────────────────────
//
// We reuse the testDatabase / testTx pair defined in order_repo_gorm_test.go
// (same package) so the database.Tx surface stays in one place. The only
// thing we add here is a constructor that AutoMigrates PaymentObservation
// instead of Order — newTestDatabase only migrates Order, and adding a
// second model there would broaden its blast radius.

// newPaymentObservationDB opens an in-memory SQLite, AutoMigrates
// PaymentObservation, and wraps it in the existing testDatabase so a
// GormPaymentObservationRepo can be wired against database.Database.
func newPaymentObservationDB(t *testing.T) *testDatabase {
	t.Helper()
	db, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PaymentObservation{}))
	return &testDatabase{gormDB: db}
}

// makeObservation produces a fully-populated PaymentObservation suitable for
// insertion. Tests override only the fields they want to vary so the dedupe
// tuple semantics stay obvious at the call site.
func makeObservation(id string) *models.PaymentObservation {
	return &models.PaymentObservation{
		TenantID:       database.StandaloneTenantID,
		ID:             id,
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		EventIndex:     0,
		EventType:      models.PaymentEventManagedEscrowReceived,
		FromAddress:    "0xfeedfeedfeedfeedfeedfeedfeedfeedfeedfeed",
		ToAddress:      "0x111122223333444455556666777788889999aaaa",
		TokenAddress:   "",
		Amount:         "1000000000000000000", // 1 ETH wei
		BlockNumber:    100,
		BlockTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Confirmations:  0,
		Source:         models.PaymentObservationSourceMonitor,
		Observer:       "monitor:eip155:1:worker-A",
		Status:         models.PaymentObservationStatusPending,
	}
}

// newPaymentObservationTestRepo constructs a wired repo plus a handle to
// the underlying *gorm.DB so tests can both poke directly (Create, Select)
// and drive the repo through its public surface.
func newPaymentObservationTestRepo(t *testing.T) (*GormPaymentObservationRepo, *gorm.DB) {
	t.Helper()
	tdb := newPaymentObservationDB(t)
	return NewGormPaymentObservationRepo(tdb, tdb.gormDB), tdb.gormDB
}

// ═══════════════════════════════════════════════════════════════════════════
// InsertObservation
// ═══════════════════════════════════════════════════════════════════════════

func TestGormPaymentObservationRepo_InsertObservation_Success(t *testing.T) {
	repo, db := newPaymentObservationTestRepo(t)

	obs := makeObservation("obs-1")
	require.NoError(t, repo.InsertObservation(context.Background(), obs))

	var count int64
	require.NoError(t, db.Model(&models.PaymentObservation{}).Count(&count).Error)
	assert.Equal(t, int64(1), count, "expected exactly one row after a single Insert")

	var stored models.PaymentObservation
	require.NoError(t, db.Where("id = ?", "obs-1").First(&stored).Error)
	assert.Equal(t, obs.TxHash, stored.TxHash)
	assert.Equal(t, obs.Source, stored.Source)
	assert.Equal(t, obs.Observer, stored.Observer)
}

func TestGormPaymentObservationRepo_InsertObservation_DuplicateReturnsSentinel(t *testing.T) {
	// Contract: an observer that re-inserts the same chain event (same
	// tenant, chain, tx hash, event index, observer string) MUST receive
	// ErrDuplicateObservation. The worker treats this as success, which is
	// what makes "process-restart safety" trivially correct.
	repo, _ := newPaymentObservationTestRepo(t)

	first := makeObservation("obs-first")
	require.NoError(t, repo.InsertObservation(context.Background(), first))

	replay := makeObservation("obs-second-id-but-same-tuple")
	err := repo.InsertObservation(context.Background(), replay)

	require.Error(t, err)
	assert.True(t, errors.Is(err, contracts.ErrDuplicateObservation),
		"expected ErrDuplicateObservation, got %v", err)
}

func TestGormPaymentObservationRepo_InsertObservation_DistinctObserversCoexist(t *testing.T) {
	// Two observers (typically a monitor and a buyer envelope) seeing the
	// same on-chain event each get their own row by design — the aggregation
	// layer downstream picks the highest-priority source.
	repo, db := newPaymentObservationTestRepo(t)

	mon := makeObservation("obs-mon")
	require.NoError(t, repo.InsertObservation(context.Background(), mon))

	buyer := makeObservation("obs-buyer")
	buyer.Source = models.PaymentObservationSourceBuyerReported
	buyer.Observer = "buyer:12D3KooWFakePeerID"
	require.NoError(t, repo.InsertObservation(context.Background(), buyer))

	var count int64
	require.NoError(t, db.Model(&models.PaymentObservation{}).Count(&count).Error)
	assert.Equal(t, int64(2), count)
}

func TestGormPaymentObservationRepo_InsertObservation_RejectsMissingFields(t *testing.T) {
	// Defensive: an obs with empty TenantID or ID is a programming error
	// (the caller must allocate a UUID and route the right tenant). The
	// repo refuses early instead of letting the DB return an opaque NOT
	// NULL error.
	repo, _ := newPaymentObservationTestRepo(t)

	// nil
	err := repo.InsertObservation(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "obs must not be nil")

	// missing ID
	missingID := makeObservation("")
	err = repo.InsertObservation(context.Background(), missingID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ID must be set")

	// missing TenantID
	missingTenant := makeObservation("obs-x")
	missingTenant.TenantID = ""
	err = repo.InsertObservation(context.Background(), missingTenant)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TenantID must be set")
}

// ═══════════════════════════════════════════════════════════════════════════
// ListDeduplicatedConfirmed
// ═══════════════════════════════════════════════════════════════════════════

func TestGormPaymentObservationRepo_ListDeduplicatedConfirmed_FiltersPendingAndReverted(t *testing.T) {
	// Only confirmed rows feed the aggregator. Pending and reverted rows
	// must NOT appear in the dedupe output.
	repo, db := newPaymentObservationTestRepo(t)

	pending := makeObservation("obs-pending")
	pending.TxHash = "0xpending"
	pending.Status = models.PaymentObservationStatusPending
	require.NoError(t, db.Create(pending).Error)

	reverted := makeObservation("obs-reverted")
	reverted.TxHash = "0xreverted"
	reverted.Status = models.PaymentObservationStatusReverted
	require.NoError(t, db.Create(reverted).Error)

	confirmed := makeObservation("obs-confirmed")
	confirmed.TxHash = "0xconfirmed"
	confirmed.Status = models.PaymentObservationStatusConfirmed
	require.NoError(t, db.Create(confirmed).Error)

	got, err := repo.ListDeduplicatedConfirmed(context.Background(),
		database.StandaloneTenantID, "order-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "0xconfirmed", got[0].TxHash)
}

func TestGormPaymentObservationRepo_ListDeduplicatedConfirmed_PrefersMonitorOverBuyer(t *testing.T) {
	// Same chain event observed by both monitor and buyer envelope: the
	// monitor row wins because its Source has higher priority.
	repo, db := newPaymentObservationTestRepo(t)

	monitorRow := makeObservation("obs-monitor")
	monitorRow.Source = models.PaymentObservationSourceMonitor
	monitorRow.Observer = "monitor:eip155:1:w-A"
	monitorRow.Status = models.PaymentObservationStatusConfirmed
	monitorRow.BlockTime = time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC) // later
	require.NoError(t, db.Create(monitorRow).Error)

	buyerRow := makeObservation("obs-buyer")
	buyerRow.Source = models.PaymentObservationSourceBuyerReported
	buyerRow.Observer = "buyer:peer-x"
	buyerRow.Status = models.PaymentObservationStatusConfirmed
	buyerRow.BlockTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) // earlier
	require.NoError(t, db.Create(buyerRow).Error)

	got, err := repo.ListDeduplicatedConfirmed(context.Background(),
		database.StandaloneTenantID, "order-1")
	require.NoError(t, err)
	require.Len(t, got, 1, "two rows for the same event tuple must dedupe to one")
	assert.Equal(t, models.PaymentObservationSourceMonitor, got[0].Source,
		"monitor must outrank buyer_reported even when buyer row has earlier BlockTime")
	assert.Equal(t, "obs-monitor", got[0].ID)
}

func TestGormPaymentObservationRepo_ListDeduplicatedConfirmed_TieBreaksByBlockTime(t *testing.T) {
	// When two rows share the same priority (e.g. two monitor workers
	// observed the same event), the earlier block_time wins. This matches
	// the design's "earliest observation" rule for same-source ties.
	repo, db := newPaymentObservationTestRepo(t)

	first := makeObservation("obs-first")
	first.Observer = "monitor:eip155:1:w-A"
	first.Status = models.PaymentObservationStatusConfirmed
	first.BlockTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(first).Error)

	second := makeObservation("obs-second")
	second.Observer = "monitor:eip155:1:w-B"
	second.Status = models.PaymentObservationStatusConfirmed
	second.BlockTime = time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC) // 1 second later
	require.NoError(t, db.Create(second).Error)

	got, err := repo.ListDeduplicatedConfirmed(context.Background(),
		database.StandaloneTenantID, "order-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "obs-first", got[0].ID)
}

func TestGormPaymentObservationRepo_ListDeduplicatedConfirmed_KeepsDistinctEventTuples(t *testing.T) {
	// Three confirmed events that differ in (chain, tx, event_index)
	// must all be returned — dedupe is per-tuple, not global.
	repo, db := newPaymentObservationTestRepo(t)

	a := makeObservation("obs-a")
	a.TxHash = "0xa"
	a.EventIndex = 0
	a.Status = models.PaymentObservationStatusConfirmed
	require.NoError(t, db.Create(a).Error)

	b := makeObservation("obs-b")
	b.TxHash = "0xa"
	b.EventIndex = 1 // different log within same tx
	b.Status = models.PaymentObservationStatusConfirmed
	require.NoError(t, db.Create(b).Error)

	c := makeObservation("obs-c")
	c.TxHash = "0xb"
	c.Status = models.PaymentObservationStatusConfirmed
	require.NoError(t, db.Create(c).Error)

	got, err := repo.ListDeduplicatedConfirmed(context.Background(),
		database.StandaloneTenantID, "order-1")
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestGormPaymentObservationRepo_ListDeduplicatedConfirmed_IsTenantScoped(t *testing.T) {
	// Confirmed observations for a different tenant must NOT bleed into
	// the result. This guards the soft-tenant boundary the testDatabase
	// emulates (a real tenantTx would add WHERE tenant_id automatically).
	repo, db := newPaymentObservationTestRepo(t)

	mine := makeObservation("obs-mine")
	mine.TxHash = "0xmine"
	mine.TenantID = "tenant-A"
	mine.Status = models.PaymentObservationStatusConfirmed
	require.NoError(t, db.Create(mine).Error)

	other := makeObservation("obs-other")
	other.TxHash = "0xother"
	other.TenantID = "tenant-B"
	other.Status = models.PaymentObservationStatusConfirmed
	require.NoError(t, db.Create(other).Error)

	got, err := repo.ListDeduplicatedConfirmed(context.Background(),
		"tenant-A", "order-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "obs-mine", got[0].ID)
}

func TestGormPaymentObservationRepo_ListDeduplicatedConfirmed_RejectsEmptyArgs(t *testing.T) {
	repo, _ := newPaymentObservationTestRepo(t)

	_, err := repo.ListDeduplicatedConfirmed(context.Background(), "", "order-1")
	require.Error(t, err)
	_, err = repo.ListDeduplicatedConfirmed(context.Background(), "tenant-A", "")
	require.Error(t, err)
}

// ═══════════════════════════════════════════════════════════════════════════
// ListByOrder
// ═══════════════════════════════════════════════════════════════════════════

func TestGormPaymentObservationRepo_ListByOrder_ReturnsAllStatuses(t *testing.T) {
	// Audit/dispute review cares about the full observation history,
	// including pending and reverted rows. ListByOrder must NOT filter
	// by status.
	repo, db := newPaymentObservationTestRepo(t)

	for i, st := range []string{
		models.PaymentObservationStatusPending,
		models.PaymentObservationStatusConfirmed,
		models.PaymentObservationStatusReverted,
	} {
		o := makeObservation("obs-" + strconv.Itoa(i))
		o.TxHash = "0x" + strconv.Itoa(i)
		o.EventIndex = i
		o.Status = st
		require.NoError(t, db.Create(o).Error)
	}

	got, err := repo.ListByOrder(context.Background(),
		database.StandaloneTenantID, "order-1")
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestGormPaymentObservationRepo_ListByOrder_OrdersByCreatedAtAscending(t *testing.T) {
	// Stable order is a documented contract — auditors and tests rely on
	// it. SQLite resolves "ORDER BY created_at ASC, id ASC" deterministically.
	repo, db := newPaymentObservationTestRepo(t)

	first := makeObservation("obs-first")
	first.TxHash = "0xa"
	first.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(first).Error)

	second := makeObservation("obs-second")
	second.TxHash = "0xb"
	second.CreatedAt = time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(second).Error)

	got, err := repo.ListByOrder(context.Background(),
		database.StandaloneTenantID, "order-1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "obs-first", got[0].ID)
	assert.Equal(t, "obs-second", got[1].ID)
}

// ═══════════════════════════════════════════════════════════════════════════
// RefreshConfirmations
// ═══════════════════════════════════════════════════════════════════════════

func TestGormPaymentObservationRepo_RefreshConfirmations_TransitionsBuriedRows(t *testing.T) {
	// A pending row buried by exactly the required depth must transition.
	repo, db := newPaymentObservationTestRepo(t)

	o := makeObservation("obs-1")
	o.BlockNumber = 100
	o.Status = models.PaymentObservationStatusPending
	require.NoError(t, db.Create(o).Error)

	// Chain head at 112; required = 12 → threshold = 100 → row qualifies.
	refs, err := repo.RefreshConfirmations(context.Background(), "eip155", "1", 112, 12)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, contracts.OrderRef{
		TenantID: database.StandaloneTenantID,
		OrderID:  "order-1",
	}, refs[0])

	var stored models.PaymentObservation
	require.NoError(t, db.Where("id = ?", "obs-1").First(&stored).Error)
	assert.Equal(t, models.PaymentObservationStatusConfirmed, stored.Status)
	assert.Equal(t, 12, stored.Confirmations)
}

func TestGormPaymentObservationRepo_RefreshConfirmations_LeavesShallowRowsAlone(t *testing.T) {
	// Same row, but chain head only 110 (depth = 10). Threshold = 98 — the
	// row at block 100 fails the predicate and stays pending.
	repo, db := newPaymentObservationTestRepo(t)

	o := makeObservation("obs-1")
	o.BlockNumber = 100
	o.Status = models.PaymentObservationStatusPending
	require.NoError(t, db.Create(o).Error)

	refs, err := repo.RefreshConfirmations(context.Background(), "eip155", "1", 110, 12)
	require.NoError(t, err)
	assert.Empty(t, refs)

	var stored models.PaymentObservation
	require.NoError(t, db.Where("id = ?", "obs-1").First(&stored).Error)
	assert.Equal(t, models.PaymentObservationStatusPending, stored.Status,
		"row at depth 10 must NOT confirm when depth requirement is 12")
}

func TestGormPaymentObservationRepo_RefreshConfirmations_IgnoresOtherChains(t *testing.T) {
	// A run for ETH (eip155:1) must not promote a BSC (eip155:56) row even
	// when the depth would qualify. The chain tuple is part of the WHERE.
	repo, db := newPaymentObservationTestRepo(t)

	bsc := makeObservation("obs-bsc")
	bsc.ChainReference = "56"
	bsc.BlockNumber = 100
	bsc.Status = models.PaymentObservationStatusPending
	require.NoError(t, db.Create(bsc).Error)

	refs, err := repo.RefreshConfirmations(context.Background(), "eip155", "1", 112, 12)
	require.NoError(t, err)
	assert.Empty(t, refs, "ETH refresh must not touch BSC rows")

	var stored models.PaymentObservation
	require.NoError(t, db.Where("id = ?", "obs-bsc").First(&stored).Error)
	assert.Equal(t, models.PaymentObservationStatusPending, stored.Status)
}

func TestGormPaymentObservationRepo_RefreshConfirmations_DeduplicatesAffectedTuples(t *testing.T) {
	// Multiple rows on the same (tenant, order) — for example, two events
	// from a multisend tx — must collapse into a single OrderRef in the
	// return value so the caller doesn't trigger N redundant aggregation
	// runs.
	repo, db := newPaymentObservationTestRepo(t)

	for i := 0; i < 3; i++ {
		o := makeObservation("obs-" + strconv.Itoa(i))
		o.EventIndex = i
		o.BlockNumber = 100
		o.Status = models.PaymentObservationStatusPending
		require.NoError(t, db.Create(o).Error)
	}

	refs, err := repo.RefreshConfirmations(context.Background(), "eip155", "1", 112, 12)
	require.NoError(t, err)
	require.Len(t, refs, 1, "three rows for the same order must dedupe to one OrderRef")
	assert.Equal(t, "order-1", refs[0].OrderID)
}

func TestGormPaymentObservationRepo_RefreshConfirmations_DoesNotRevertOrTouchConfirmed(t *testing.T) {
	// Confirmed rows are immutable from the worker's perspective — re-running
	// the sweep must NOT re-write them or include them in the affected set.
	repo, db := newPaymentObservationTestRepo(t)

	already := makeObservation("obs-already")
	already.BlockNumber = 50
	already.Status = models.PaymentObservationStatusConfirmed
	already.Confirmations = 12
	require.NoError(t, db.Create(already).Error)

	refs, err := repo.RefreshConfirmations(context.Background(), "eip155", "1", 200, 12)
	require.NoError(t, err)
	assert.Empty(t, refs, "already-confirmed row must not appear in refs")

	var stored models.PaymentObservation
	require.NoError(t, db.Where("id = ?", "obs-already").First(&stored).Error)
	assert.Equal(t, 12, stored.Confirmations,
		"Confirmations must NOT be rewritten by a sweep that didn't transition the row")
}

func TestGormPaymentObservationRepo_RefreshConfirmations_NoOpWhenChainHeadShallow(t *testing.T) {
	// Devnet edge: requiredConfirmations exceeds the chain head. Threshold
	// would be negative — we explicitly return early to avoid matching all
	// rows ("block_number ≤ -2" matches nothing in practice but the code
	// must not even issue the UPDATE).
	repo, db := newPaymentObservationTestRepo(t)

	o := makeObservation("obs-dev")
	o.BlockNumber = 1
	o.Status = models.PaymentObservationStatusPending
	require.NoError(t, db.Create(o).Error)

	refs, err := repo.RefreshConfirmations(context.Background(), "eip155", "1", 5, 12)
	require.NoError(t, err)
	assert.Empty(t, refs)

	var stored models.PaymentObservation
	require.NoError(t, db.Where("id = ?", "obs-dev").First(&stored).Error)
	assert.Equal(t, models.PaymentObservationStatusPending, stored.Status)
}

func TestGormPaymentObservationRepo_RefreshConfirmations_RejectsInvalidArgs(t *testing.T) {
	repo, _ := newPaymentObservationTestRepo(t)

	_, err := repo.RefreshConfirmations(context.Background(), "", "1", 100, 12)
	require.Error(t, err)
	_, err = repo.RefreshConfirmations(context.Background(), "eip155", "", 100, 12)
	require.Error(t, err)
	_, err = repo.RefreshConfirmations(context.Background(), "eip155", "1", 100, -1)
	require.Error(t, err)
}

// ═══════════════════════════════════════════════════════════════════════════
// Sequential dedupe-stress: simulate the worker-loop pattern of a chain
// monitor that walks through a list of observed events, retrying on RPC
// replays. The combination of (same observer, repeated inserts) plus
// (different observers, same event) is the exact pattern the production
// monitor must survive without double-counting.
// ═══════════════════════════════════════════════════════════════════════════

func TestGormPaymentObservationRepo_InsertObservation_RetryStormSemantics(t *testing.T) {
	// Concrete scenario: a monitor restart causes the worker to replay
	// 10 events, each "seen" twice. A second observer (a buyer envelope)
	// sees 5 of those 10 events. Expected end state:
	//   - 10 monitor rows (no duplicates from the replay)
	//   - 5 buyer rows  (independent observer)
	//   - 15 total rows
	//   - the 10 replay attempts each return ErrDuplicateObservation
	//
	// We run sequentially because :memory: SQLite uses one connection
	// per Open() and concurrent goroutines would see a phantom "no such
	// table" error. The semantics tested here are about the UNIQUE
	// constraint behaviour, not about real parallelism — the latter is
	// covered by the schema-level tests in internal/repo/ which do the
	// same UNIQUE assertion.
	repo, db := newPaymentObservationTestRepo(t)

	const numEvents = 10
	for i := 0; i < numEvents; i++ {
		// First observation by monitor.
		mon := makeObservation("obs-monitor-" + strconv.Itoa(i))
		mon.TxHash = "0xtx" + strconv.Itoa(i)
		require.NoError(t, repo.InsertObservation(context.Background(), mon))

		// Replay by the SAME monitor → ErrDuplicateObservation.
		replay := makeObservation("obs-monitor-replay-" + strconv.Itoa(i))
		replay.TxHash = "0xtx" + strconv.Itoa(i) // same dedupe tuple
		err := repo.InsertObservation(context.Background(), replay)
		assert.True(t, errors.Is(err, contracts.ErrDuplicateObservation),
			"event %d: replay should be ErrDuplicateObservation, got %v", i, err)
	}

	// Second observer (buyer envelope) sees the first 5 events.
	for i := 0; i < 5; i++ {
		buyer := makeObservation("obs-buyer-" + strconv.Itoa(i))
		buyer.TxHash = "0xtx" + strconv.Itoa(i)
		buyer.Source = models.PaymentObservationSourceBuyerReported
		buyer.Observer = "buyer:peer-" + strconv.Itoa(i)
		require.NoError(t, repo.InsertObservation(context.Background(), buyer))
	}

	var total int64
	require.NoError(t, db.Model(&models.PaymentObservation{}).Count(&total).Error)
	assert.Equal(t, int64(numEvents+5), total,
		"expected %d monitor + 5 buyer = %d total, got %d",
		numEvents, numEvents+5, total)
}

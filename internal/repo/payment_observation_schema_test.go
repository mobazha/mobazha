package repo

import (
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// schemaDB opens an in-memory SQLite DB and AutoMigrates only the
// PaymentObservation model. This keeps schema-shape tests isolated from the
// (much heavier) full-model migration covered by TestAutoMigrateDatabasemanaged EVM,
// while still exercising the same dialect and GORM index parser used in
// production.
func schemaDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite in-memory: %v", err)
	}
	if err := db.AutoMigrate(&models.PaymentObservation{}); err != nil {
		t.Fatalf("AutoMigrate PaymentObservation: %v", err)
	}
	return db
}

// makeObs returns a populated PaymentObservation with sensible defaults so
// individual tests can override only the fields they care about (typically
// the dedupe-tuple components).
func makeObs(id string) models.PaymentObservation {
	return models.PaymentObservation{
		TenantID:       "_default",
		ID:             id,
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xfeedfacecafebeefdeadbeef00000000000000000000000000000000000000000",
		EventIndex:     0,
		EventType:      models.PaymentEventManagedEscrowReceived,
		FromAddress:    "0xfromfromfromfromfromfromfromfromfromfrom",
		ToAddress:      "0x111122223333444455556666777788889999aaaa",
		TokenAddress:   "",
		Amount:         "1000000000000000000", // 1 ETH in wei
		BlockNumber:    100,
		BlockTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Confirmations:  6,
		Source:         models.PaymentObservationSourceMonitor,
		Observer:       "monitor:eip155:1:worker-A",
		Status:         models.PaymentObservationStatusConfirmed,
	}
}

// isUniqueViolation matches both SQLite ("UNIQUE constraint failed") and
// PostgreSQL ("duplicate key value violates unique constraint") wordings;
// we keep the matcher symmetric with internal/core/supply_chain_app_service.go
// so future PG runs of this test don't need a separate path.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key value violates unique constraint")
}

// TestPaymentObservation_AutoMigrate_CreatesTable verifies that AutoMigrate
// produces the expected table — guards against a model-rename refactor that
// would silently drop production data because GORM cannot rename tables.
func TestPaymentObservation_AutoMigrate_CreatesTable(t *testing.T) {
	db := schemaDB(t)

	// HasTable / Migrator.HasTable returns true only if the named table
	// exists; we compare against the model's TableName() to also catch
	// accidental TableName overrides.
	want := (models.PaymentObservation{}).TableName()
	if !db.Migrator().HasTable(want) {
		t.Fatalf("AutoMigrate did not create table %q", want)
	}
}

// TestPaymentObservation_DedupeUnique_SameObserverRejected covers the core
// idempotency contract: a single observer (same monitor worker, or same
// buyer peer) re-inserting the same chain event MUST hit UNIQUE and fail.
// This is what lets the worker recover from RPC replay / process restart
// without duplicating accounting rows.
func TestPaymentObservation_DedupeUnique_SameObserverRejected(t *testing.T) {
	db := schemaDB(t)

	a := makeObs("obs-A")
	if err := db.Create(&a).Error; err != nil {
		t.Fatalf("first insert should succeed: %v", err)
	}

	// Second row with a DIFFERENT primary key (id) but identical dedupe
	// tuple — observer included — must be rejected.
	b := makeObs("obs-A-replay")
	err := db.Create(&b).Error
	if err == nil {
		t.Fatal("duplicate (tenant, chain, tx, event_index, observer) tuple was accepted; UNIQUE index missing")
	}
	if !isUniqueViolation(err) {
		t.Fatalf("expected UNIQUE violation, got: %v", err)
	}
}

// TestPaymentObservation_DedupeUnique_DifferentObserverAccepted documents
// the design choice that monitor + buyer_reported each get their own row
// for the same chain event — the aggregator picks the highest-priority one
// at SELECT time. If this test fails, the aggregation layer (DISTINCT ON
// observer priority) becomes pointless because there is at most one row.
func TestPaymentObservation_DedupeUnique_DifferentObserverAccepted(t *testing.T) {
	db := schemaDB(t)

	monitorRow := makeObs("obs-monitor")
	if err := db.Create(&monitorRow).Error; err != nil {
		t.Fatalf("monitor insert failed: %v", err)
	}

	buyerRow := makeObs("obs-buyer")
	buyerRow.Source = models.PaymentObservationSourceBuyerReported
	buyerRow.Observer = "buyer:12D3KooWFakePeerIDForTestOnlyDoNotUseInProduction"

	if err := db.Create(&buyerRow).Error; err != nil {
		t.Fatalf("buyer-reported row should NOT collide with monitor row "+
			"(by design — aggregator picks highest priority at SELECT time): %v", err)
	}

	var count int64
	db.Model(&models.PaymentObservation{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 rows after distinct-observer inserts, got %d", count)
	}
}

// TestPaymentObservation_DedupeUnique_CrossTenantIsolation guarantees that
// two tenants observing the same chain event each get their own row. Without
// tenant_id at priority:1 in the UNIQUE index, Tenant B's monitor would be
// blocked from recording any payment that Tenant A already saw — a hard
// multi-tenant correctness bug.
func TestPaymentObservation_DedupeUnique_CrossTenantIsolation(t *testing.T) {
	db := schemaDB(t)

	tenantA := makeObs("obs-tenant-A")
	tenantA.TenantID = "tenant-A"
	if err := db.Create(&tenantA).Error; err != nil {
		t.Fatalf("tenant A insert failed: %v", err)
	}

	tenantB := makeObs("obs-tenant-B")
	tenantB.TenantID = "tenant-B"
	// Same chain/tx/event/observer — only tenant differs.
	if err := db.Create(&tenantB).Error; err != nil {
		t.Fatalf("tenant B insert MUST succeed (cross-tenant isolation broken): %v", err)
	}

	var count int64
	db.Model(&models.PaymentObservation{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected 1 row per tenant (= 2 total), got %d", count)
	}
}

// TestPaymentObservation_DedupeUnique_DifferentEventIndexAccepted covers the
// real-world case of a single tx emitting multiple ERC-20 Transfer logs
// (e.g. a multisend ERC-20 distribution to several escrows). Each log must
// be recordable independently.
func TestPaymentObservation_DedupeUnique_DifferentEventIndexAccepted(t *testing.T) {
	db := schemaDB(t)

	log0 := makeObs("obs-log0")
	log0.EventType = models.PaymentEventERC20Transfer
	log0.EventIndex = 0
	if err := db.Create(&log0).Error; err != nil {
		t.Fatalf("log index 0 insert failed: %v", err)
	}

	log1 := makeObs("obs-log1")
	log1.EventType = models.PaymentEventERC20Transfer
	log1.EventIndex = 1
	if err := db.Create(&log1).Error; err != nil {
		t.Fatalf("log index 1 with same tx hash MUST succeed: %v", err)
	}
}

// TestPaymentObservation_DedupeUnique_DifferentChainAccepted guards the
// CAIP-2 unification: the same tx hash on, say, ETH (eip155:1) and BSC
// (eip155:56) is theoretically possible (collision-free guarantees only hold
// per-chain), and even more so across CAIP-2 namespaces. Each must be its
// own row.
func TestPaymentObservation_DedupeUnique_DifferentChainAccepted(t *testing.T) {
	db := schemaDB(t)

	eth := makeObs("obs-eth")
	eth.ChainNamespace = "eip155"
	eth.ChainReference = "1"
	if err := db.Create(&eth).Error; err != nil {
		t.Fatalf("ETH insert failed: %v", err)
	}

	bsc := makeObs("obs-bsc")
	bsc.ChainNamespace = "eip155"
	bsc.ChainReference = "56"
	if err := db.Create(&bsc).Error; err != nil {
		t.Fatalf("BSC insert with same tx hash MUST succeed: %v", err)
	}

	xmr := makeObs("obs-xmr")
	xmr.ChainNamespace = "xmr"
	xmr.ChainReference = "mainnet"
	xmr.EventType = models.PaymentEventXMRDeposit
	if err := db.Create(&xmr).Error; err != nil {
		t.Fatalf("XMR insert across CAIP-2 namespaces MUST succeed: %v", err)
	}
}

// TestPaymentObservation_AutoCreateTime confirms that omitting CreatedAt
// causes GORM to fill it from autoCreateTime (the spec says "fact rows are
// append-only and timestamped at creation"). If autoCreateTime is dropped
// from the tag, the worker would have to fill it manually and inconsistency
// across observers would creep in.
func TestPaymentObservation_AutoCreateTime(t *testing.T) {
	db := schemaDB(t)

	obs := makeObs("obs-time")
	obs.CreatedAt = time.Time{} // zero on purpose
	before := time.Now().Add(-time.Second)
	if err := db.Create(&obs).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}

	var got models.PaymentObservation
	if err := db.Where("id = ?", obs.ID).First(&got).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("CreatedAt was not filled by autoCreateTime")
	}
	if got.CreatedAt.Before(before) {
		t.Fatalf("CreatedAt %v earlier than test start %v", got.CreatedAt, before)
	}
}

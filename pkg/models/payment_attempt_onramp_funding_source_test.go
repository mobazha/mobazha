// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"fmt"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newOnrampSourceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&PaymentAttemptOnrampFundingSource{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func activeOnrampSource(tenant, attempt, onrampOrder string) *PaymentAttemptOnrampFundingSource {
	s := &PaymentAttemptOnrampFundingSource{
		TenantID:       tenant,
		AttemptID:      attempt,
		OnrampOrderID:  onrampOrder,
		OrderID:        "order-" + attempt,
		ProviderID:     "mock-onramp",
		IdempotencyKey: "primary",
	}
	s.SetStatus(OnrampSourceStatusAwaitingPayment)
	return s
}

// The at-most-one-active invariant is per attempt. Scoped to the active column
// alone the partial unique index degrades into one global row for the entire
// table: the first buyer with an in-flight purchase makes every other buyer's
// initiate fail with a duplicate-key error, across every tenant on the node.
func TestOnrampFundingSourceAllowsOneActivePurchasePerAttempt(t *testing.T) {
	db := newOnrampSourceTestDB(t)

	first := activeOnrampSource("tenant-a", "attempt-1", "mock-onramp-1")
	if err := db.Create(first).Error; err != nil {
		t.Fatalf("first buyer's purchase: %v", err)
	}

	// A different attempt, same tenant — a second buyer checking out concurrently.
	second := activeOnrampSource("tenant-a", "attempt-2", "mock-onramp-2")
	if err := db.Create(second).Error; err != nil {
		t.Fatalf("second attempt blocked by another attempt's in-flight purchase: %v", err)
	}

	// A different tenant entirely — must never collide.
	other := activeOnrampSource("tenant-b", "attempt-1", "mock-onramp-3")
	if err := db.Create(other).Error; err != nil {
		t.Fatalf("second tenant blocked by another tenant's in-flight purchase: %v", err)
	}
}

// The invariant it does enforce: one attempt cannot have two purchases in
// flight at once, so a stuck purchase can't be silently orphaned by a new one.
func TestOnrampFundingSourceRejectsSecondActivePurchaseForSameAttempt(t *testing.T) {
	db := newOnrampSourceTestDB(t)

	if err := db.Create(activeOnrampSource("tenant-a", "attempt-1", "mock-onramp-1")).Error; err != nil {
		t.Fatalf("first purchase: %v", err)
	}
	if err := db.Create(activeOnrampSource("tenant-a", "attempt-1", "mock-onramp-2")).Error; err == nil {
		t.Fatal("a second in-flight purchase for the same attempt must be rejected")
	}
}

// Inactive rows are outside the partial index, so an attempt may accumulate
// finished purchases and still start a new one.
func TestOnrampFundingSourceAllowsNewPurchaseAfterPreviousSettles(t *testing.T) {
	db := newOnrampSourceTestDB(t)

	done := activeOnrampSource("tenant-a", "attempt-1", "mock-onramp-1")
	done.SetStatus(OnrampSourceStatusFailed)
	if done.Active {
		t.Fatal("failed purchase must not stay active")
	}
	if err := db.Create(done).Error; err != nil {
		t.Fatalf("settled purchase: %v", err)
	}
	// A retry is a new idempotency claim; reusing the key would (correctly)
	// collide on idx_onramp_source_idem instead, which is a different invariant.
	retry := activeOnrampSource("tenant-a", "attempt-1", "mock-onramp-2")
	retry.IdempotencyKey = "retry"
	if err := db.Create(retry).Error; err != nil {
		t.Fatalf("retry after a failed purchase: %v", err)
	}
}

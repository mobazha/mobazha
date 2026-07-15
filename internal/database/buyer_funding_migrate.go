// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package database

import (
	pkgdb "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
)

// MigrateBuyerFundingModels creates/updates the buyer-funding tables
// (RFC-0012 / ADR-019). Owned by initBuyerFundingSubsystem, deliberately
// separate from MigrateFiatModels: onramp funding attaches to crypto attempts
// and must exist even when the fiat PSP subsystem is disabled by the
// distribution capability policy.
func MigrateBuyerFundingModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		if err := dropMisScopedOnrampActiveIndex(tx); err != nil {
			return err
		}
		return tx.Migrate(&models.PaymentAttemptOnrampFundingSource{})
	})
}

// dropMisScopedOnrampActiveIndex removes idx_onramp_source_active when it still
// carries its original scope of (active) alone — a single partial-unique row for
// the whole table, i.e. one in-flight onramp purchase per node rather than per
// attempt. AutoMigrate creates a missing index but never re-scopes one that
// already exists under the same name, so the corrected model definition cannot
// take effect until the old index is gone. Dropping it is safe: the invariant it
// enforced is strictly weaker than the one Migrate recreates.
func dropMisScopedOnrampActiveIndex(tx pkgdb.Tx) error {
	db := tx.Read()
	migrator := db.Migrator()
	if !migrator.HasTable(&models.PaymentAttemptOnrampFundingSource{}) {
		return nil // fresh install: Migrate builds the correct index outright
	}
	if !migrator.HasIndex(&models.PaymentAttemptOnrampFundingSource{}, "idx_onramp_source_active") {
		return nil
	}
	var indexed int64
	if err := db.Raw(`
		SELECT COUNT(*) FROM pg_index i
		JOIN pg_class c ON c.oid = i.indexrelid
		WHERE c.relname = 'idx_onramp_source_active'`).Scan(&indexed).Error; err != nil {
		// Not Postgres (SQLite in tests) — the model definition is authoritative
		// there and Migrate rebuilds from scratch.
		return nil //nolint:nilerr
	}
	var columns int64
	if err := db.Raw(`
		SELECT COALESCE(MAX(array_length(i.indkey, 1)), 0) FROM pg_index i
		JOIN pg_class c ON c.oid = i.indexrelid
		WHERE c.relname = 'idx_onramp_source_active'`).Scan(&columns).Error; err != nil {
		return err
	}
	if columns >= 3 {
		return nil // already scoped to (tenant_id, attempt_id, active)
	}
	return migrator.DropIndex(&models.PaymentAttemptOnrampFundingSource{}, "idx_onramp_source_active")
}

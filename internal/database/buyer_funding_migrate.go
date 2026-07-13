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
		return tx.Migrate(&models.PaymentAttemptOnrampFundingSource{})
	})
}

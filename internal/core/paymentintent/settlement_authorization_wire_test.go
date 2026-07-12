// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"fmt"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSettlementAuthorizationWire_RoundTripsCanonicalSnapshot(t *testing.T) {
	_, _, terms, _, _, bundle, target := cryptoAttemptFixture(t)
	authorization := models.PaymentAttemptSettlementAuthorization{
		Version: models.SettlementAuthorizationVersion,
		Terms:   terms, Target: target, Authorization: bundle,
	}
	wire, err := SettlementAuthorizationToProto(authorization)
	require.NoError(t, err)
	roundTrip, err := SettlementAuthorizationFromProto(wire)
	require.NoError(t, err)
	require.Equal(t, authorization, roundTrip)

	wire.AttemptID = "another-attempt"
	_, err = SettlementAuthorizationFromProto(wire)
	require.ErrorIs(t, err, models.ErrPaymentAttemptSettlementTermsConflict)
}

func TestRetainReceivedSettlementAuthorization_IsIdempotentAndImmutable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:settlement-authorization-inbox-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.PaymentAttempt{}, &models.PaymentRouteBinding{},
		&models.PaymentAttemptSettlementOffer{}, &models.PaymentAttemptSettlementAuthorizationRecord{},
	))
	attempt, route, terms, _, _, bundle, target := cryptoAttemptFixture(t)
	attempt, err = CreateCryptoPaymentAttemptDraft(db, attempt, route)
	require.NoError(t, err)
	authorization := models.PaymentAttemptSettlementAuthorization{
		Version: models.SettlementAuthorizationVersion,
		Terms:   terms, Target: target, Authorization: bundle,
	}
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return RetainReceivedSettlementAuthorizationInTransaction(tx, attempt.TenantID, authorization)
	}))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return RetainReceivedSettlementAuthorizationInTransaction(tx, attempt.TenantID, authorization)
	}))
	loaded, err := LoadRetainedSettlementAuthorization(db, attempt.TenantID, attempt.AttemptID)
	require.NoError(t, err)
	require.Equal(t, authorization, loaded)

	mutated := authorization
	mutated.Terms.SellerAddress = "0x9999999999999999999999999999999999999999"
	require.Error(t, db.Transaction(func(tx *gorm.DB) error {
		return RetainReceivedSettlementAuthorizationInTransaction(tx, attempt.TenantID, mutated)
	}))
}

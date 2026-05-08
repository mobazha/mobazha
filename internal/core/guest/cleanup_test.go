package guest

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCleanupSvc(db *testDatabase) *GuestOrderAppService {
	return NewGuestOrderAppService(GuestOrderAppServiceConfig{
		DB:     db,
		NodeID: "node-test",
	})
}

// ── CleanupExpiredOrders ────────────────────────────────────────

func TestCleanupExpiredOrders_TransitionsAwaitingToExpired(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	past := time.Now().Add(-time.Hour)
	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken: "gst_expired",
		State:      models.GuestOrderAwaitingPayment,
		ExpiresAt:  past,
	})
	seedReservation(t, db, 1, models.InventoryReservation{
		OrderRef:    "gst_expired",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "sku-1",
		Quantity:    1,
		ReservedAt:  time.Now().Add(-2 * time.Hour),
		ExpiresAt:   past,
	})

	svc.CleanupExpiredOrders(context.Background())

	got := loadGuestOrder(t, db, "gst_expired")
	assert.Equal(t, models.GuestOrderExpired, got.State,
		"expired awaiting order should transition to EXPIRED")

	res := loadReservation(t, db, 1)
	assert.NotNil(t, res.ReleasedAt,
		"reservation linked to expired order should be released")
}

func TestCleanupExpiredOrders_SkipsDetectedOrders(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken: "gst_detected_expired",
		State:      models.GuestOrderPaymentDetected,
		ExpiresAt:  time.Now().Add(-time.Minute),
	})

	svc.CleanupExpiredOrders(context.Background())

	got := loadGuestOrder(t, db, "gst_detected_expired")
	assert.Equal(t, models.GuestOrderPaymentDetected, got.State,
		"PAYMENT_DETECTED orders must not be expired by cleanup — confirmation polling manages their lifecycle")
}

func TestCleanupExpiredOrders_SkipsFundedOrders(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken: "gst_funded",
		State:      models.GuestOrderFunded,
		ExpiresAt:  time.Now().Add(-time.Hour),
	})

	svc.CleanupExpiredOrders(context.Background())

	got := loadGuestOrder(t, db, "gst_funded")
	assert.Equal(t, models.GuestOrderFunded, got.State,
		"funded orders must not be expired by the cleanup loop")
}

func TestCleanupExpiredOrders_SkipsNotYetExpired(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken: "gst_future",
		State:      models.GuestOrderAwaitingPayment,
		ExpiresAt:  time.Now().Add(time.Hour),
	})

	svc.CleanupExpiredOrders(context.Background())

	got := loadGuestOrder(t, db, "gst_future")
	assert.Equal(t, models.GuestOrderAwaitingPayment, got.State,
		"orders whose expires_at is in the future must not be expired")
}

// ── AutoCompleteOrders ───────────────────────────────────────────

func TestAutoCompleteOrders_TransitionsOldShippedToCompleted(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	oldShipped := time.Now().Add(-defaultAutoCompletePeriod - time.Hour)
	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken: "gst_autocomplete",
		State:      models.GuestOrderShipped,
		ExpiresAt:  time.Now().Add(-defaultAutoCompletePeriod),
		ShippedAt:  &oldShipped,
	})

	svc.AutoCompleteOrders(context.Background())

	got := loadGuestOrder(t, db, "gst_autocomplete")
	assert.Equal(t, models.GuestOrderCompleted, got.State)
	require.NotNil(t, got.CompletedAt)
}

func TestAutoCompleteOrders_SkipsRecentlyShipped(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	recent := time.Now().Add(-time.Hour)
	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken: "gst_recent_shipped",
		State:      models.GuestOrderShipped,
		ShippedAt:  &recent,
	})

	svc.AutoCompleteOrders(context.Background())

	got := loadGuestOrder(t, db, "gst_recent_shipped")
	assert.Equal(t, models.GuestOrderShipped, got.State,
		"orders within the auto-complete window must not be completed yet")
}

// ── releaseExpiredReservations ──────────────────────────────────

func TestReleaseExpiredReservations_OnlyReleasesExpiredUnconfirmed(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	now := time.Now()
	seedReservation(t, db, 1, models.InventoryReservation{
		OrderRef:    "gst_A",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "sku-1",
		Quantity:    1,
		ReservedAt:  now.Add(-2 * time.Hour),
		ExpiresAt:   now.Add(-time.Minute),
	})
	seedReservation(t, db, 2, models.InventoryReservation{
		OrderRef:    "gst_B",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "sku-2",
		Quantity:    1,
		ReservedAt:  now,
		ExpiresAt:   now.Add(time.Hour),
	})
	seedReservation(t, db, 3, models.InventoryReservation{
		OrderRef:    "gst_C",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "sku-3",
		Quantity:    2,
		ReservedAt:  now.Add(-3 * time.Hour),
		ExpiresAt:   now.Add(-time.Hour),
		Confirmed:   true,
	})

	svc.releaseExpiredReservations(context.Background())

	got1 := loadReservation(t, db, 1)
	assert.NotNil(t, got1.ReleasedAt, "id=1 expired+unconfirmed should be released")

	got2 := loadReservation(t, db, 2)
	assert.Nil(t, got2.ReleasedAt, "id=2 not yet expired should remain active")

	got3 := loadReservation(t, db, 3)
	assert.Nil(t, got3.ReleasedAt, "id=3 confirmed reservation must not be released")
}

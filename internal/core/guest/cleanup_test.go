package guest

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
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

	// Past the default (UTXO) grace period (1h) so cleanup fires
	// regardless of per-coin grace dispatch.
	past := time.Now().Add(-2 * time.Hour)
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
		ReservedAt:  time.Now().Add(-3 * time.Hour),
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

func TestCleanupExpiredOrders_RespectsXMRGrace(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	past := time.Now().Add(-30 * time.Minute) // expired, but well within 2h XMR grace
	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken:  "gst_xmr_in_grace",
		State:       models.GuestOrderAwaitingPayment,
		PaymentCoin: "crypto:monero:mainnet:native",
		ExpiresAt:   past,
	})

	svc.CleanupExpiredOrders(context.Background())

	got := loadGuestOrder(t, db, "gst_xmr_in_grace")
	assert.Equal(t, models.GuestOrderAwaitingPayment, got.State,
		"XMR order within grace window must not be expired — watcher still owns it")
}

// TestCleanupExpiredOrders_RespectsUTXOGrace mirrors the XMR test for UTXO
// chains. The grace-aware cleanup applies uniformly across chain families,
// not just Monero.
func TestCleanupExpiredOrders_RespectsUTXOGrace(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	past := time.Now().Add(-15 * time.Minute) // expired, but within 1h UTXO grace
	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken:  "gst_btc_in_grace",
		State:       models.GuestOrderAwaitingPayment,
		PaymentCoin: "crypto:bitcoin:mainnet:native",
		ExpiresAt:   past,
	})

	svc.CleanupExpiredOrders(context.Background())

	got := loadGuestOrder(t, db, "gst_btc_in_grace")
	assert.Equal(t, models.GuestOrderAwaitingPayment, got.State,
		"UTXO order within grace window must not be expired — watcher still owns it")
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

func TestAutoCompleteOrders_UsesDigitalGoodSnapshot(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	shippedSixDaysAgo := time.Now().Add(-6 * 24 * time.Hour)
	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken: "gst_digital_snapshot",
		State:      models.GuestOrderShipped,
		ShippedAt:  &shippedSixDaysAgo,
		// Digital guest orders snapshot the store review window at creation.
		// Six days after shipment should complete a 5-day digital order,
		// even though the legacy guest default is 14 days.
		AutoCompleteAfterShipDaysOverride: 5,
	})

	svc.AutoCompleteOrders(context.Background())

	got := loadGuestOrder(t, db, "gst_digital_snapshot")
	assert.Equal(t, models.GuestOrderCompleted, got.State)
}

func TestAutoCompleteOrders_SkipsDigitalSnapshotBeforeDeadline(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	shippedFourDaysAgo := time.Now().Add(-4 * 24 * time.Hour)
	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken:                        "gst_digital_recent",
		State:                             models.GuestOrderShipped,
		ShippedAt:                         &shippedFourDaysAgo,
		AutoCompleteAfterShipDaysOverride: 5,
	})

	svc.AutoCompleteOrders(context.Background())

	got := loadGuestOrder(t, db, "gst_digital_recent")
	assert.Equal(t, models.GuestOrderShipped, got.State)
}

func TestDigitalGoodReviewWindowDays(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	assert.Equal(t, uint32(3), svc.digitalGoodReviewWindowDays())

	require.NoError(t, db.gormDB.Create(&models.UserPreferences{
		TenantMixin:                 models.TenantMixin{TenantID: testTenantID},
		ID:                          1,
		DigitalGoodReviewWindowDays: 7,
	}).Error)

	assert.Equal(t, uint32(7), svc.digitalGoodReviewWindowDays())
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

// TestReservationExpiresAtForOrder_IncludesPerCoinGrace locks the formula
// CreateGuestOrder uses to compute reservation.ExpiresAt. If a future
// refactor reverts to the bare order.ExpiresAt, this test fails — and the
// "release runs before watcher gives up" race that motivated the
// per-coin-grace cleanup fix would silently re-emerge in production.
func TestReservationExpiresAtForOrder_IncludesPerCoinGrace(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		coin string
		want time.Duration
	}{
		{"crypto:bitcoin:mainnet:native", utxoGracePeriod},
		{"crypto:litecoin:mainnet:native", utxoGracePeriod},
		{"crypto:ethereum:mainnet:native", evmGracePeriod},
		{"crypto:solana:mainnet:native", solanaGracePeriod},
		{"crypto:unknown:mainnet:native", utxoGracePeriod}, // default branch
	}
	for _, tc := range cases {
		t.Run(tc.coin, func(t *testing.T) {
			got := reservationExpiresAtForOrder(base, tc.coin)
			want := base.Add(tc.want)
			assert.True(t, got.Equal(want),
				"coin %q: got %s, want %s", tc.coin, got, want)
		})
	}
}

// TestReleaseExpiredReservations_RespectsGuestOrderGrace is the regression
// test for the cleanup-vs-watcher race: a reservation created via the
// production formula (reservationExpiresAtForOrder) for an XMR order that
// is past its order.ExpiresAt but still inside the per-coin grace window
// must NOT be released. Without this, an in-grace fund would leave the
// order in PAYMENT_DETECTED while its inventory had already been resold.
func TestReleaseExpiredReservations_RespectsGuestOrderGrace(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	// Order's payment window expired 30min ago; XMR grace is 2h, so
	// reservation.ExpiresAt is ~90min in the future.
	orderExpires := time.Now().Add(-30 * time.Minute)
	resvExpires := reservationExpiresAtForOrder(orderExpires, "crypto:monero:mainnet:native")
	seedReservation(t, db, 1, models.InventoryReservation{
		OrderRef:    "gst_xmr_in_grace",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "sku-1",
		Quantity:    1,
		ReservedAt:  time.Now().Add(-time.Hour),
		ExpiresAt:   resvExpires,
	})

	svc.releaseExpiredReservations(context.Background())

	got := loadReservation(t, db, 1)
	assert.Nil(t, got.ReleasedAt,
		"reservation for an XMR order still in its grace window must not be released")
}

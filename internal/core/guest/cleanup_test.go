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

// TestCleanupExpiredOrders_SweepsPoolOnlyEvicted is the regression test for
// the architectural fix that moved EXTERNAL_PAYMENT pool detection out of the
// PAYMENT_DETECTED state. Before the fix, an EXTERNAL_PAYMENT pool tx that never mined
// (e.g. evicted from the mempool) would land the order in PAYMENT_DETECTED
// and CleanupExpiredOrders — which intentionally skips PAYMENT_DETECTED —
// would leave it stuck forever. The fix makes pool a pure UX hint on top
// of AWAITING_PAYMENT, so cleanup naturally sweeps it.
//
// ExpiresAt is set well past external_paymentGracePeriod (2h) to confirm cleanup
// fires once the watcher's grace window has fully elapsed. The in-grace
// behavior is covered by TestCleanupExpiredOrders_RespectsEXTERNAL_PAYMENTGrace.
func TestCleanupExpiredOrders_SweepsPoolOnlyEvicted(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	past := time.Now().Add(-3 * time.Hour) // > external_paymentGracePeriod (2h)
	poolDetectedAt := time.Now().Add(-3*time.Hour - 30*time.Minute)
	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken:     "gst_external_payment_pool_evicted",
		State:          models.GuestOrderAwaitingPayment, // critical: still AWAITING despite pool observation
		PaymentCoin:    "crypto:external_payment:mainnet:native",
		ExpiresAt:      past,
		PoolTxHash:     "external_paymenttxevicted",
		PoolAmount:     30_000_000_000,
		PoolDetectedAt: &poolDetectedAt,
	})

	svc.CleanupExpiredOrders(context.Background())

	got := loadGuestOrder(t, db, "gst_external_payment_pool_evicted")
	assert.Equal(t, models.GuestOrderExpired, got.State,
		"pool-only EXTERNAL_PAYMENT order whose tx never mined must be swept by CleanupExpiredOrders")
}

// TestCleanupExpiredOrders_RespectsEXTERNAL_PAYMENTGrace covers the cleanup-vs-watcher
// race fix: an EXTERNAL_PAYMENT order whose ExpiresAt has passed but is still inside
// external_paymentGracePeriod must NOT be expired by cleanup, so the watcher can
// still call HandlePaymentDetected if a pool tx mines during grace.
//
// Pre-fix bug: cleanup sees `expires_at < now` and flips state to EXPIRED;
// monitor's confirmed event then fails the AWAITING→PAYMENT_DETECTED
// transition (state mismatch) → funds stranded.
func TestCleanupExpiredOrders_RespectsEXTERNAL_PAYMENTGrace(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	past := time.Now().Add(-30 * time.Minute) // expired, but well within 2h EXTERNAL_PAYMENT grace
	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken:  "gst_external_payment_in_grace",
		State:       models.GuestOrderAwaitingPayment,
		PaymentCoin: "crypto:external_payment:mainnet:native",
		ExpiresAt:   past,
	})

	svc.CleanupExpiredOrders(context.Background())

	got := loadGuestOrder(t, db, "gst_external_payment_in_grace")
	assert.Equal(t, models.GuestOrderAwaitingPayment, got.State,
		"EXTERNAL_PAYMENT order within grace window must not be expired — watcher still owns it")
}

// TestCleanupExpiredOrders_RespectsUTXOGrace mirrors the EXTERNAL_PAYMENT test for UTXO
// chains. The grace-aware cleanup applies uniformly across chain families,
// not just ExternalPayment.
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
		{"crypto:external_payment:mainnet:native", external_paymentGracePeriod},
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
// production formula (reservationExpiresAtForOrder) for an EXTERNAL_PAYMENT order that
// is past its order.ExpiresAt but still inside the per-coin grace window
// must NOT be released. Without this, an in-grace fund would leave the
// order in PAYMENT_DETECTED while its inventory had already been resold.
func TestReleaseExpiredReservations_RespectsGuestOrderGrace(t *testing.T) {
	db := newGuestTestDB(t)
	svc := newCleanupSvc(db)

	// Order's payment window expired 30min ago; EXTERNAL_PAYMENT grace is 2h, so
	// reservation.ExpiresAt is ~90min in the future.
	orderExpires := time.Now().Add(-30 * time.Minute)
	resvExpires := reservationExpiresAtForOrder(orderExpires, "crypto:external_payment:mainnet:native")
	seedReservation(t, db, 1, models.InventoryReservation{
		OrderRef:    "gst_external_payment_in_grace",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "sku-1",
		Quantity:    1,
		ReservedAt:  time.Now().Add(-time.Hour),
		ExpiresAt:   resvExpires,
	})

	svc.releaseExpiredReservations(context.Background())

	got := loadReservation(t, db, 1)
	assert.Nil(t, got.ReleasedAt,
		"reservation for an EXTERNAL_PAYMENT order still in its grace window must not be released")
}

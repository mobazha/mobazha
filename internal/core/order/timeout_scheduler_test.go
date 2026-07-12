package order

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestOrderAppServiceForTimeout(t *testing.T) (*OrderAppService, *testDatabase, *mockEventBus) {
	t.Helper()
	db := newTestDatabase(t)
	require.NoError(t, db.gormDB.AutoMigrate(&models.OutboxEvent{}))
	bus := &mockEventBus{}
	shutdown := make(chan struct{})
	svc := &OrderAppService{
		db:       db,
		eventBus: bus,
		nodeID:   "test-node",
		shutdown: shutdown,
	}
	return svc, db, bus
}

func TestOrderTimeout_ExpiredOrderIsCanceled(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	past := time.Now().Add(-10 * time.Minute)
	insertTestOrder(t, db, "order-expired-1", models.OrderState_AWAITING_PAYMENT, true, &past)

	svc.expireTimedOutOrders()
	svc.deliverOutboxEvents()

	var order models.Order
	err := db.gormDB.Where("id = ?", "order-expired-1").First(&order).Error
	require.NoError(t, err)
	assert.Equal(t, models.OrderState_CANCELED, order.State)
	assert.False(t, order.Open)

	require.Len(t, bus.emitted, 1)
	evt, ok := bus.emitted[0].(*events.OrderExpired)
	require.True(t, ok)
	assert.Equal(t, "order-expired-1", evt.OrderID)
	assert.Equal(t, "payment_timeout", evt.Reason)
}

func TestOrderTimeout_NotExpiredOrderIsUntouched(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	future := time.Now().Add(1 * time.Hour)
	insertTestOrder(t, db, "order-active-1", models.OrderState_AWAITING_PAYMENT, true, &future)

	svc.expireTimedOutOrders()

	var order models.Order
	err := db.gormDB.Where("id = ?", "order-active-1").First(&order).Error
	require.NoError(t, err)
	assert.Equal(t, models.OrderState_AWAITING_PAYMENT, order.State)
	assert.True(t, order.Open)
	assert.Empty(t, bus.emitted)
}

func TestOrderTimeout_NilExpiresAtIsIgnored(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	insertTestOrder(t, db, "order-nil-exp", models.OrderState_AWAITING_PAYMENT, true, nil)

	svc.expireTimedOutOrders()

	var order models.Order
	err := db.gormDB.Where("id = ?", "order-nil-exp").First(&order).Error
	require.NoError(t, err)
	assert.Equal(t, models.OrderState_AWAITING_PAYMENT, order.State)
	assert.True(t, order.Open)
	assert.Empty(t, bus.emitted)
}

func TestOrderTimeout_AlreadyCanceledIsSkipped(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	past := time.Now().Add(-10 * time.Minute)
	insertTestOrder(t, db, "order-already-canceled", models.OrderState_CANCELED, false, &past)

	svc.expireTimedOutOrders()

	assert.Empty(t, bus.emitted)
}

func TestOrderTimeout_PendingStateIsNotExpired(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	past := time.Now().Add(-10 * time.Minute)
	insertTestOrder(t, db, "order-pending", models.OrderState_PENDING, true, &past)

	svc.expireTimedOutOrders()

	var order models.Order
	err := db.gormDB.Where("id = ?", "order-pending").First(&order).Error
	require.NoError(t, err)
	assert.Equal(t, models.OrderState_PENDING, order.State)
	assert.True(t, order.Open)
	assert.Empty(t, bus.emitted)
}

func TestOrderTimeout_ExpiredOrderWithPaymentSentIsNotCanceled(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	past := time.Now().Add(-10 * time.Minute)
	insertTestOrder(t, db, "order-paid-late", models.OrderState_AWAITING_PAYMENT, true, &past)
	err := db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().Where("id = ?", "order-paid-late").First(&order).Error; err != nil {
			return err
		}
		if err := order.SetPaymentSent(&pb.PaymentSent{
			TransactionID: "tx-paid-late",
			Coin:          "crypto:bip122:000000000019d6689c085ae165831e93:native",
			Amount:        "29838",
		}); err != nil {
			return err
		}
		return tx.Save(&order)
	})
	require.NoError(t, err)
	require.NoError(t, db.gormDB.Exec(
		"UPDATE orders SET state = ?, open = ? WHERE id = ?",
		int32(models.OrderState_AWAITING_PAYMENT), true, "order-paid-late",
	).Error)

	svc.expireTimedOutOrders()

	var order models.Order
	err = db.gormDB.Where("id = ?", "order-paid-late").First(&order).Error
	require.NoError(t, err)
	assert.Equal(t, models.OrderState_AWAITING_PAYMENT, order.State)
	assert.True(t, order.Open)
	assert.NotNil(t, order.SerializedPaymentSent)
	assert.Empty(t, bus.emitted)
}

func TestOrderTimeout_VerifiedExpiredOrderIsNotCanceled(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	past := time.Now().Add(-10 * time.Minute)
	insertTestOrder(t, db, "order-verified-late", models.OrderState_AWAITING_PAYMENT, true, &past)
	err := db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().Where("id = ?", "order-verified-late").First(&order).Error; err != nil {
			return err
		}
		order.MarkPaymentVerified()
		return tx.Save(&order)
	})
	require.NoError(t, err)

	svc.expireTimedOutOrders()

	var order models.Order
	err = db.gormDB.Where("id = ?", "order-verified-late").First(&order).Error
	require.NoError(t, err)
	assert.Equal(t, models.OrderState_AWAITING_PAYMENT, order.State)
	assert.True(t, order.Open)
	assert.Equal(t, models.PaymentVerificationStatusVerified, order.CurrentPaymentVerificationStatus())
	assert.Empty(t, bus.emitted)
}

func TestOrderTimeout_MultipleExpiredOrders(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	past := time.Now().Add(-10 * time.Minute)
	insertTestOrder(t, db, "order-exp-a", models.OrderState_AWAITING_PAYMENT, true, &past)
	insertTestOrder(t, db, "order-exp-b", models.OrderState_AWAITING_PAYMENT, true, &past)

	future := time.Now().Add(1 * time.Hour)
	insertTestOrder(t, db, "order-active", models.OrderState_AWAITING_PAYMENT, true, &future)

	svc.expireTimedOutOrders()
	svc.deliverOutboxEvents()

	var orderA, orderB, orderC models.Order
	require.NoError(t, db.gormDB.Where("id = ?", "order-exp-a").First(&orderA).Error)
	require.NoError(t, db.gormDB.Where("id = ?", "order-exp-b").First(&orderB).Error)
	require.NoError(t, db.gormDB.Where("id = ?", "order-active").First(&orderC).Error)

	assert.Equal(t, models.OrderState_CANCELED, orderA.State)
	assert.Equal(t, models.OrderState_CANCELED, orderB.State)
	assert.Equal(t, models.OrderState_AWAITING_PAYMENT, orderC.State)
	assert.Len(t, bus.emitted, 2)
}

func insertTestOrder(t *testing.T, db *testDatabase, id string, state models.OrderState, open bool, expiresAt *time.Time) {
	t.Helper()
	insertTestOrderWithStateAge(t, db, id, state, open, expiresAt, nil)
}

func insertTestOrderWithStateAge(t *testing.T, db *testDatabase, id string, state models.OrderState, open bool, expiresAt *time.Time, stateChangedAt *time.Time) {
	t.Helper()
	order := &models.Order{
		ID:   models.OrderID(id),
		Open: open,
		OrderTimeoutState: models.OrderTimeoutState{
			ExpiresAt: expiresAt,
		},
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	order.TenantID = database.StandaloneTenantID
	order.SetFSMState(state)
	if stateChangedAt != nil {
		order.LastStateChangeAt = stateChangedAt
	}
	err := db.gormDB.Create(order).Error
	require.NoError(t, err)
}

// ── Extended timeout tests: PENDING ─────────────────────────────────────

func TestStaleOrder_AwaitingPaymentVerificationWarnAfter7d(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	stateChange := time.Now().Add(-8 * 24 * time.Hour) // 8 days ago
	insertTestOrderWithStateAge(t, db, "verify-8d", models.OrderState_AWAITING_PAYMENT_VERIFICATION, true, nil, &stateChange)

	svc.processStaleOrders()
	svc.deliverOutboxEvents()

	var order models.Order
	require.NoError(t, db.gormDB.Where("id = ?", "verify-8d").First(&order).Error)
	assert.Equal(t, models.OrderState_AWAITING_PAYMENT_VERIFICATION, order.State)
	assert.True(t, order.Open)
	assert.NotNil(t, order.TimeoutWarnedAt)

	require.Len(t, bus.emitted, 1)
	evt, ok := bus.emitted[0].(*events.OrderStaleWarning)
	require.True(t, ok)
	assert.Equal(t, "verify-8d", evt.OrderID)
}

func TestStaleOrder_AwaitingPaymentVerificationCancelAfter14d(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	stateChange := time.Now().Add(-15 * 24 * time.Hour) // 15 days ago
	insertTestOrderWithStateAge(t, db, "verify-15d", models.OrderState_AWAITING_PAYMENT_VERIFICATION, true, nil, &stateChange)

	svc.processStaleOrders()
	svc.deliverOutboxEvents()

	var order models.Order
	require.NoError(t, db.gormDB.Where("id = ?", "verify-15d").First(&order).Error)
	assert.Equal(t, models.OrderState_CANCELED, order.State)
	assert.False(t, order.Open)

	require.Len(t, bus.emitted, 1)
	evt, ok := bus.emitted[0].(*events.OrderExpired)
	require.True(t, ok)
	assert.Equal(t, "verify-15d", evt.OrderID)
	assert.Equal(t, "payment_verification_timeout", evt.Reason)
}

func TestStaleOrder_PendingWarnAfter7d(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	stateChange := time.Now().Add(-8 * 24 * time.Hour) // 8 days ago
	insertTestOrderWithStateAge(t, db, "pending-8d", models.OrderState_PENDING, true, nil, &stateChange)

	svc.processStaleOrders()
	svc.deliverOutboxEvents()

	var order models.Order
	require.NoError(t, db.gormDB.Where("id = ?", "pending-8d").First(&order).Error)
	assert.Equal(t, models.OrderState_PENDING, order.State, "should still be PENDING (warn, not expire)")
	assert.True(t, order.Open)
	assert.NotNil(t, order.TimeoutWarnedAt)

	require.Len(t, bus.emitted, 1)
	evt, ok := bus.emitted[0].(*events.OrderStaleWarning)
	require.True(t, ok)
	assert.Equal(t, "pending-8d", evt.OrderID)
}

func TestStaleOrder_PendingCancelAfter14d(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	stateChange := time.Now().Add(-15 * 24 * time.Hour) // 15 days ago
	insertTestOrderWithStateAge(t, db, "pending-15d", models.OrderState_PENDING, true, nil, &stateChange)

	svc.processStaleOrders()
	svc.deliverOutboxEvents()

	var order models.Order
	require.NoError(t, db.gormDB.Where("id = ?", "pending-15d").First(&order).Error)
	assert.Equal(t, models.OrderState_CANCELED, order.State)
	assert.False(t, order.Open)

	require.Len(t, bus.emitted, 1)
	evt, ok := bus.emitted[0].(*events.OrderExpired)
	require.True(t, ok)
	assert.Equal(t, "pending-15d", evt.OrderID)
	assert.Equal(t, "pending_unconfirmed", evt.Reason)
}

func TestStaleOrder_PendingFreshIsUntouched(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	stateChange := time.Now().Add(-2 * 24 * time.Hour) // 2 days ago
	insertTestOrderWithStateAge(t, db, "pending-2d", models.OrderState_PENDING, true, nil, &stateChange)

	svc.processStaleOrders()

	var order models.Order
	require.NoError(t, db.gormDB.Where("id = ?", "pending-2d").First(&order).Error)
	assert.Equal(t, models.OrderState_PENDING, order.State)
	assert.True(t, order.Open)
	assert.Empty(t, bus.emitted)
}

// ── Extended timeout tests: AWAITING_SHIPMENT ───────────────────────────

func TestStaleOrder_AwaitingShipmentWarnAfter14d(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	stateChange := time.Now().Add(-16 * 24 * time.Hour)
	insertTestOrderWithStateAge(t, db, "ship-16d", models.OrderState_AWAITING_SHIPMENT, true, nil, &stateChange)

	svc.processStaleOrders()
	svc.deliverOutboxEvents()

	var order models.Order
	require.NoError(t, db.gormDB.Where("id = ?", "ship-16d").First(&order).Error)
	assert.Equal(t, models.OrderState_AWAITING_SHIPMENT, order.State)
	assert.NotNil(t, order.TimeoutWarnedAt)

	require.Len(t, bus.emitted, 1)
	_, ok := bus.emitted[0].(*events.OrderStaleWarning)
	require.True(t, ok)
}

func TestStaleOrder_AwaitingShipmentNoWarnDuplicate(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	stateChange := time.Now().Add(-16 * 24 * time.Hour)
	insertTestOrderWithStateAge(t, db, "ship-warned", models.OrderState_AWAITING_SHIPMENT, true, nil, &stateChange)

	svc.processStaleOrders()
	svc.deliverOutboxEvents()
	require.Len(t, bus.emitted, 1)

	bus.emitted = nil
	svc.processStaleOrders()
	svc.deliverOutboxEvents()
	assert.Empty(t, bus.emitted, "should not warn twice")
}

// ── Extended timeout tests: DISPUTED ────────────────────────────────────

func TestStaleOrder_DisputedWarnAfter7d(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	stateChange := time.Now().Add(-8 * 24 * time.Hour)
	insertTestOrderWithStateAge(t, db, "dispute-8d", models.OrderState_DISPUTED, true, nil, &stateChange)

	svc.processStaleOrders()
	svc.deliverOutboxEvents()

	var order models.Order
	require.NoError(t, db.gormDB.Where("id = ?", "dispute-8d").First(&order).Error)
	assert.Equal(t, models.OrderState_DISPUTED, order.State, "disputed orders are warned, not auto-resolved")

	require.Len(t, bus.emitted, 1)
	_, ok := bus.emitted[0].(*events.OrderStaleWarning)
	require.True(t, ok)
}

func TestStaleOrder_NilLastStateChangeIsIgnored(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	insertTestOrderWithStateAge(t, db, "pending-nil", models.OrderState_PENDING, true, nil, nil)

	svc.processStaleOrders()
	assert.Empty(t, bus.emitted)
}

// ── Fiat payment race condition tests ───────────────────────────────────

func fiatMetadataJSON(t *testing.T, providerID, sessionID string) []byte {
	t.Helper()
	m := map[string]string{"fiat_provider": providerID, "fiat_session_id": sessionID}
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return b
}

func insertTestFiatOrder(t *testing.T, db *testDatabase, id string, state models.OrderState, open bool, expiresAt *time.Time, providerID, sessionID string) {
	t.Helper()
	order := &models.Order{
		ID:   models.OrderID(id),
		Open: open,
		OrderTimeoutState: models.OrderTimeoutState{
			ExpiresAt: expiresAt,
		},
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				FiatMetadata: fiatMetadataJSON(t, providerID, sessionID),
			},
		},
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	order.TenantID = database.StandaloneTenantID
	order.SetFSMState(state)
	require.NoError(t, db.gormDB.Create(order).Error)
}

// mockFiatOps implements contracts.FiatPaymentOperations for testing.
type mockFiatOps struct {
	cancelErr    error
	cancelCalled bool
	statusResult string
	statusErr    error
	refundResult *contracts.RefundResult
	refundErr    error
}

func (m *mockFiatOps) RefundPayment(_ context.Context, _ string, _ contracts.RefundParams) (*contracts.RefundResult, error) {
	return m.refundResult, m.refundErr
}

func (m *mockFiatOps) DisbursePayment(_ context.Context, _ string, _ contracts.DisbursePaymentParams) (*contracts.DisbursePaymentResult, error) {
	return nil, nil
}

func (m *mockFiatOps) ProviderCapabilities(context.Context, string) (contracts.FiatProviderCapabilities, error) {
	return contracts.FiatProviderCapabilities{}, nil
}

func (m *mockFiatOps) CancelPayment(_ context.Context, _ string, _ string) error {
	m.cancelCalled = true
	return m.cancelErr
}

func (m *mockFiatOps) GetPaymentStatus(_ context.Context, _ string, _ string) (string, error) {
	return m.statusResult, m.statusErr
}

func TestOrderTimeout_FiatCancelSucceeds_OrderCanceled(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	mock := &mockFiatOps{}
	svc.SetFiatOps(mock)

	past := time.Now().Add(-10 * time.Minute)
	insertTestFiatOrder(t, db, "fiat-cancel-ok", models.OrderState_AWAITING_PAYMENT, true, &past, "stripe", "pi_pending")

	svc.expireTimedOutOrders()
	svc.deliverOutboxEvents()

	var order models.Order
	require.NoError(t, db.gormDB.Where("id = ?", "fiat-cancel-ok").First(&order).Error)
	assert.Equal(t, models.OrderState_CANCELED, order.State)
	assert.False(t, order.Open)
	assert.True(t, mock.cancelCalled)
	assert.Len(t, bus.emitted, 1)
}

func TestOrderTimeout_FiatCancelFails_PaymentSucceeded_SkipsCancellation(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	svc.SetFiatOps(&mockFiatOps{
		cancelErr:    errors.New("stripe: payment intent has already succeeded"),
		statusResult: "succeeded",
	})

	past := time.Now().Add(-10 * time.Minute)
	insertTestFiatOrder(t, db, "fiat-succeeded", models.OrderState_AWAITING_PAYMENT, true, &past, "stripe", "pi_succeeded")

	svc.expireTimedOutOrders()

	var order models.Order
	require.NoError(t, db.gormDB.Where("id = ?", "fiat-succeeded").First(&order).Error)
	assert.Equal(t, models.OrderState_AWAITING_PAYMENT, order.State, "order should NOT be canceled")
	assert.True(t, order.Open, "order should remain open for reconciliation")
	assert.Empty(t, bus.emitted, "no events should be emitted")
}

func TestOrderTimeout_FiatCancelFails_StatusCheckFails_ProceedsWithCancel(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	svc.SetFiatOps(&mockFiatOps{
		cancelErr: errors.New("stripe API timeout"),
		statusErr: errors.New("stripe API unreachable"),
	})

	past := time.Now().Add(-10 * time.Minute)
	insertTestFiatOrder(t, db, "fiat-both-fail", models.OrderState_AWAITING_PAYMENT, true, &past, "stripe", "pi_unreachable")

	svc.expireTimedOutOrders()
	svc.deliverOutboxEvents()

	var order models.Order
	require.NoError(t, db.gormDB.Where("id = ?", "fiat-both-fail").First(&order).Error)
	assert.Equal(t, models.OrderState_CANCELED, order.State, "conservative: cancel order when Stripe unreachable")
	assert.False(t, order.Open)
	assert.Len(t, bus.emitted, 1)
}

func TestOrderTimeout_FiatCancelFails_StatusPending_ProceedsWithCancel(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	svc.SetFiatOps(&mockFiatOps{
		cancelErr:    errors.New("some transient error"),
		statusResult: "requires_payment_method",
	})

	past := time.Now().Add(-10 * time.Minute)
	insertTestFiatOrder(t, db, "fiat-cancel-fail-pending", models.OrderState_AWAITING_PAYMENT, true, &past, "stripe", "pi_not_paid")

	svc.expireTimedOutOrders()
	svc.deliverOutboxEvents()

	var order models.Order
	require.NoError(t, db.gormDB.Where("id = ?", "fiat-cancel-fail-pending").First(&order).Error)
	assert.Equal(t, models.OrderState_CANCELED, order.State, "cancel order when payment not succeeded")
	assert.False(t, order.Open)
	assert.Len(t, bus.emitted, 1)
}

func TestOrderTimeout_NoFiatMetadata_NormalCancelFlow(t *testing.T) {
	svc, db, bus := newTestOrderAppServiceForTimeout(t)

	past := time.Now().Add(-10 * time.Minute)
	insertTestOrder(t, db, "crypto-expired", models.OrderState_AWAITING_PAYMENT, true, &past)

	svc.expireTimedOutOrders()
	svc.deliverOutboxEvents()

	var order models.Order
	require.NoError(t, db.gormDB.Where("id = ?", "crypto-expired").First(&order).Error)
	assert.Equal(t, models.OrderState_CANCELED, order.State, "crypto order should cancel normally")
	assert.False(t, order.Open)
	assert.Len(t, bus.emitted, 1)
}

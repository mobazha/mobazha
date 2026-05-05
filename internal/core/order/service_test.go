//go:build !private_distribution

package order

import (
	"testing"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ── test helpers ────────────────────────────────────────────────────────

func newTestOrderAppService(t *testing.T, cfg OrderAppServiceConfig) *OrderAppService {
	t.Helper()
	if cfg.DB == nil {
		db, err := repo.MockDB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		cfg.DB = db
	}
	if cfg.EventBus == nil {
		cfg.EventBus = events.NewBus()
	}
	if cfg.NodeID == "" {
		cfg.NodeID = "test-order-svc"
	}
	return NewOrderAppService(cfg)
}

func seedOrder(t *testing.T, svc *OrderAppService, id string, role string, state models.OrderState) {
	t.Helper()
	order := &models.Order{
		ID:     models.OrderID(id),
		MyRole: role,
	}
	order.SetFSMState(state)
	err := svc.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	})
	require.NoError(t, err)
}

func seedCase(t *testing.T, svc *OrderAppService, id string) {
	t.Helper()
	c := &models.Case{
		ID: models.OrderID(id),
	}
	err := svc.db.Update(func(tx database.Tx) error {
		return tx.Save(c)
	})
	require.NoError(t, err)
}

// ── Constructor & Registry ──────────────────────────────────────────────

func TestOrderAppService_NewOrderAppService(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	assert.NotNil(t, svc)
	assert.Equal(t, "test-order-svc", svc.nodeID)
}

func TestOrderAppService_SetRegistry(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	assert.Nil(t, svc.paymentRegistry)

	reg := payment.NewRegistry()
	svc.SetRegistry(reg)
	assert.Same(t, reg, svc.paymentRegistry)
}

// ── GetOrder ────────────────────────────────────────────────────────────

func TestOrderAppService_GetOrder_NotFound(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	order, err := svc.GetOrder("nonexistent")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "GetOrder uses First: not-found returns ErrRecordNotFound")
	assert.Nil(t, order)
}

func TestOrderAppService_GetOrder_Found(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	seedOrder(t, svc, "order-get-1", "buyer", models.OrderState_PENDING)

	order, err := svc.GetOrder("order-get-1")
	require.NoError(t, err)
	assert.Equal(t, models.OrderID("order-get-1"), order.ID)
}

// ── GetPurchases ────────────────────────────────────────────────────────

func TestOrderAppService_GetPurchases_Empty(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	orders, count, err := svc.GetPurchases(nil, "", false, false, 100, nil)
	require.NoError(t, err)
	assert.Empty(t, orders)
	assert.Equal(t, int64(0), count)
}

func TestOrderAppService_GetPurchases_ReturnsOnlyBuyerOrders(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	seedOrder(t, svc, "purchase-1", "buyer", models.OrderState_PENDING)
	seedOrder(t, svc, "sale-1", "vendor", models.OrderState_PENDING)
	seedOrder(t, svc, "purchase-2", "buyer", models.OrderState_SHIPPED)

	orders, count, err := svc.GetPurchases(nil, "", false, false, 100, nil)
	require.NoError(t, err)
	assert.Len(t, orders, 2)
	assert.Equal(t, int64(2), count)
	for _, o := range orders {
		assert.Equal(t, "buyer", o.MyRole, "GetPurchases should only return buyer orders")
	}
}

func TestOrderAppService_GetPurchases_StateFilter(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	seedOrder(t, svc, "p-pending", "buyer", models.OrderState_PENDING)
	seedOrder(t, svc, "p-shipped", "buyer", models.OrderState_SHIPPED)
	seedOrder(t, svc, "p-completed", "buyer", models.OrderState_COMPLETED)

	orders, count, err := svc.GetPurchases([]models.OrderState{models.OrderState_SHIPPED}, "", false, false, 100, nil)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, int64(1), count)
	assert.Equal(t, models.OrderState_SHIPPED, orders[0].State)
}

// ── GetSales ────────────────────────────────────────────────────────────

func TestOrderAppService_GetSales_Empty(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	orders, count, err := svc.GetSales(nil, "", false, false, 100, nil)
	require.NoError(t, err)
	assert.Empty(t, orders)
	assert.Equal(t, int64(0), count)
}

func TestOrderAppService_GetSales_ReturnsOnlyVendorOrders(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	seedOrder(t, svc, "sale-v1", "vendor", models.OrderState_PENDING)
	seedOrder(t, svc, "purchase-b1", "buyer", models.OrderState_PENDING)
	seedOrder(t, svc, "sale-v2", "vendor", models.OrderState_AWAITING_SHIPMENT)

	orders, count, err := svc.GetSales(nil, "", false, false, 100, nil)
	require.NoError(t, err)
	assert.Len(t, orders, 2)
	assert.Equal(t, int64(2), count)
	for _, o := range orders {
		assert.Equal(t, "vendor", o.MyRole, "GetSales should only return vendor orders")
	}
}

func TestOrderAppService_GetSales_StateFilter(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	seedOrder(t, svc, "s-pending", "vendor", models.OrderState_PENDING)
	seedOrder(t, svc, "s-shipped", "vendor", models.OrderState_SHIPPED)

	orders, count, err := svc.GetSales([]models.OrderState{models.OrderState_SHIPPED}, "", false, false, 100, nil)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, int64(1), count)
	assert.Equal(t, models.OrderState_SHIPPED, orders[0].State)
}

// ── GetCase ─────────────────────────────────────────────────────────────

func TestOrderAppService_GetCase_NotFound(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	c, err := svc.GetCase("nonexistent-case")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "GetCase uses First: not-found returns ErrRecordNotFound")
	assert.Nil(t, c)
}

func TestOrderAppService_GetCase_Found(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	seedCase(t, svc, "case-1")

	c, err := svc.GetCase("case-1")
	require.NoError(t, err)
	assert.Equal(t, models.OrderID("case-1"), c.ID)
}

// ── GetCases ────────────────────────────────────────────────────────────

func TestOrderAppService_GetCases_Empty(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	cases, count, err := svc.GetCases(nil, "", false, false, 100, nil)
	require.NoError(t, err)
	assert.Empty(t, cases)
	assert.Equal(t, int64(0), count)
}

func TestOrderAppService_GetCases_Found(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	seedCase(t, svc, "case-a")
	seedCase(t, svc, "case-b")

	cases, count, err := svc.GetCases(nil, "", false, false, 100, nil)
	require.NoError(t, err)
	assert.Len(t, cases, 2)
	assert.Equal(t, int64(2), count)
}

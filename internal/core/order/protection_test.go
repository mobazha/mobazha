package order

import (
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
)

func seedProtectionOrder(t *testing.T, db *testDatabase, order *models.Order) {
	t.Helper()
	if err := db.gormDB.Create(order).Error; err != nil {
		t.Fatalf("seedProtectionOrder: %v", err)
	}
}

func TestExtendProtection_HappyPath(t *testing.T) {
	db := newTestDatabase(t)
	order := &models.Order{
		ID:   "order-ext-1",
		Open: true,
	}
	order.SetRole(models.RoleBuyer)
	order.SetFSMState(models.OrderState_SHIPPED)
	seedProtectionOrder(t, db, order)

	svc := &OrderAppService{db: db}
	info, err := svc.ExtendProtection("order-ext-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil protection info")
	}

	var updated models.Order
	db.gormDB.First(&updated, "id = ?", "order-ext-1")
	if updated.ProtectionExtendedAt == nil {
		t.Error("ProtectionExtendedAt should be set after extension")
	}
}

func TestExtendProtection_VendorRejected(t *testing.T) {
	db := newTestDatabase(t)
	order := &models.Order{
		ID:   "order-ext-2",
		Open: true,
	}
	order.SetRole(models.RoleVendor)
	order.SetFSMState(models.OrderState_SHIPPED)
	seedProtectionOrder(t, db, order)

	svc := &OrderAppService{db: db}
	_, err := svc.ExtendProtection("order-ext-2")
	if !errors.Is(err, ErrNotBuyerOrder) {
		t.Errorf("expected ErrNotBuyerOrder, got: %v", err)
	}
}

func TestExtendProtection_WrongState(t *testing.T) {
	states := []models.OrderState{
		models.OrderState_AWAITING_SHIPMENT,
		models.OrderState_COMPLETED,
		models.OrderState_CANCELED,
		models.OrderState_DISPUTED,
	}
	for _, state := range states {
		db := newTestDatabase(t)
		order := &models.Order{
			ID:   models.OrderID("order-ext-state-" + state.String()),
			Open: true,
		}
		order.SetRole(models.RoleBuyer)
		order.SetFSMState(state)
		seedProtectionOrder(t, db, order)

		svc := &OrderAppService{db: db}
		_, err := svc.ExtendProtection(order.ID)
		if !errors.Is(err, ErrOrderNotInProtectionPeriod) {
			t.Errorf("state %v: expected ErrOrderNotInProtectionPeriod, got: %v", state, err)
		}
	}
}

func TestExtendProtection_AlreadyExtended(t *testing.T) {
	db := newTestDatabase(t)
	now := time.Now()
	order := &models.Order{
		ID:   "order-ext-dup",
		Open: true,
		OrderLifecycle: models.OrderLifecycle{
			ProtectionExtendedAt: &now,
		},
	}
	order.SetRole(models.RoleBuyer)
	order.SetFSMState(models.OrderState_SHIPPED)
	seedProtectionOrder(t, db, order)

	svc := &OrderAppService{db: db}
	_, err := svc.ExtendProtection("order-ext-dup")
	if !errors.Is(err, ErrProtectionAlreadyExtended) {
		t.Errorf("expected ErrProtectionAlreadyExtended, got: %v", err)
	}
}

func TestExtendProtection_OrderNotFound(t *testing.T) {
	db := newTestDatabase(t)
	svc := &OrderAppService{db: db}
	_, err := svc.ExtendProtection("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent order")
	}
}

// --- OpenAfterSaleDispute validation tests ---

func TestOpenAfterSaleDispute_EmptyReason(t *testing.T) {
	svc := &OrderAppService{}
	err := svc.OpenAfterSaleDispute("order-1", "", "some description")
	if !errors.Is(err, ErrAfterSaleReasonEmpty) {
		t.Errorf("expected ErrAfterSaleReasonEmpty, got: %v", err)
	}
}

func TestOpenAfterSaleDispute_WrongState(t *testing.T) {
	db := newTestDatabase(t)
	order := &models.Order{
		ID:   "order-dispute-1",
		Open: true,
	}
	order.SetRole(models.RoleBuyer)
	order.SetFSMState(models.OrderState_SHIPPED)
	seedProtectionOrder(t, db, order)

	svc := &OrderAppService{db: db}
	err := svc.OpenAfterSaleDispute("order-dispute-1", "NOT_RECEIVED", "didn't get it")
	if err == nil {
		t.Error("expected error for non-COMPLETED order")
	}
}

func TestOpenAfterSaleDispute_AlreadyOpened(t *testing.T) {
	db := newTestDatabase(t)
	completedAt := time.Now().Add(-2 * 24 * time.Hour)
	disputeAt := time.Now().Add(-1 * 24 * time.Hour)
	order := &models.Order{
		ID:   "order-dispute-dup",
		Open: false,
		OrderLifecycle: models.OrderLifecycle{
			CompletedAt: &completedAt,
		},
		AfterSaleDispute: models.AfterSaleDispute{
			OpenedAt: &disputeAt,
		},
	}
	order.SetRole(models.RoleBuyer)
	order.SetFSMState(models.OrderState_COMPLETED)
	seedProtectionOrder(t, db, order)

	svc := &OrderAppService{db: db}
	err := svc.OpenAfterSaleDispute("order-dispute-dup", "QUALITY_ISSUE", "broken item")
	if !errors.Is(err, ErrAfterSaleDisputeAlreadyOpen) {
		t.Errorf("expected ErrAfterSaleDisputeAlreadyOpen, got: %v", err)
	}
}

func TestOpenAfterSaleDispute_WindowExpired(t *testing.T) {
	db := newTestDatabase(t)
	completedAt := time.Now().Add(-10 * 24 * time.Hour)
	order := &models.Order{
		ID:   "order-dispute-exp",
		Open: false,
		OrderLifecycle: models.OrderLifecycle{
			CompletedAt: &completedAt,
		},
	}
	order.SetRole(models.RoleBuyer)
	order.SetFSMState(models.OrderState_COMPLETED)
	seedProtectionOrder(t, db, order)

	svc := &OrderAppService{db: db}
	err := svc.OpenAfterSaleDispute("order-dispute-exp", "NOT_RECEIVED", "never arrived")
	if err == nil {
		t.Error("expected error for expired after-sale window")
	}
}

func TestOpenAfterSaleDispute_VendorRejected(t *testing.T) {
	db := newTestDatabase(t)
	completedAt := time.Now().Add(-2 * 24 * time.Hour)
	order := &models.Order{
		ID:   "order-dispute-vendor",
		Open: false,
		OrderLifecycle: models.OrderLifecycle{
			CompletedAt: &completedAt,
		},
	}
	order.SetRole(models.RoleVendor)
	order.SetFSMState(models.OrderState_COMPLETED)
	seedProtectionOrder(t, db, order)

	svc := &OrderAppService{db: db}
	err := svc.OpenAfterSaleDispute("order-dispute-vendor", "NOT_RECEIVED", "didn't get it")
	if err == nil {
		t.Error("vendor should not be allowed to open after-sale dispute")
	}
}

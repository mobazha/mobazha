package core

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// --- mock implementations ---

type mockStandardFetcher struct {
	orders []models.Order
	total  int64
	err    error
}

func (m *mockStandardFetcher) GetSales(stateFilter []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error) {
	return m.orders, m.total, m.err
}

type mockGuestSvc struct {
	orders []models.GuestOrder
	total  int64
	err    error
}

func (m *mockGuestSvc) CreateGuestOrder(ctx context.Context, req contracts.CreateGuestOrderRequest) (*contracts.GuestOrderResponse, error) {
	return nil, nil
}
func (m *mockGuestSvc) GetGuestOrderStatus(ctx context.Context, token string) (*contracts.GuestOrderStatusResponse, error) {
	return nil, nil
}
func (m *mockGuestSvc) ListGuestOrders(ctx context.Context, filter contracts.GuestOrderFilter) ([]models.GuestOrder, int64, error) {
	return m.orders, m.total, m.err
}
func (m *mockGuestSvc) ShipGuestOrder(ctx context.Context, token string, tracking, carrier string) error {
	return nil
}
func (m *mockGuestSvc) CompleteGuestOrder(ctx context.Context, token string) error {
	return nil
}
func (m *mockGuestSvc) HandlePaymentDetected(orderToken string, txHash string) error {
	return nil
}
func (m *mockGuestSvc) HandleConfirmationUpdate(orderToken string, confs int) error {
	return nil
}
func (m *mockGuestSvc) CleanupExpiredOrders(ctx context.Context) {}
func (m *mockGuestSvc) AutoCompleteOrders(ctx context.Context)   {}
func (m *mockGuestSvc) StartCleanupLoop()                        {}
func (m *mockGuestSvc) GetGuestCheckoutConfig(ctx context.Context) (*models.GuestCheckoutConfig, error) {
	return nil, nil
}
func (m *mockGuestSvc) SaveGuestCheckoutConfig(ctx context.Context, cfg *models.GuestCheckoutConfig) error {
	return nil
}
func (m *mockGuestSvc) IsEnabled(ctx context.Context) bool { return true }

// --- helpers ---

func makeGuestOrder(token, email, coin string, state models.GuestOrderState, createdAt time.Time) models.GuestOrder {
	return models.GuestOrder{
		OrderToken:   token,
		ContactEmail: email,
		PaymentCoin:  coin,
		PaymentAmount: "1000000",
		State:        state,
		CreatedAt:    createdAt,
		Items: []models.GuestOrderItem{
			{ListingTitle: "Item for " + token, Quantity: 1},
		},
	}
}

// --- U-01: MergesGuestAndStandard ---

func TestUnifiedOrderView_MergesGuestAndStandard(t *testing.T) {
	now := time.Now()
	guestOrders := []models.GuestOrder{
		makeGuestOrder("tok1", "a@b.com", "BTC", models.GuestOrderAwaitingPayment, now.Add(-1*time.Hour)),
		makeGuestOrder("tok2", "c@d.com", "ETH", models.GuestOrderFunded, now.Add(-30*time.Minute)),
	}

	v := NewUnifiedOrderView(
		&mockStandardFetcher{},
		&mockGuestSvc{orders: guestOrders, total: 2},
	)

	results, meta, err := v.ListOrders(context.Background(), contracts.OrderListFilter{
		View: "all", PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(results))
	}

	if results[0].CreatedAt.Before(results[1].CreatedAt) {
		t.Error("expected descending order by createdAt")
	}

	if meta.Total != 2 {
		t.Errorf("expected total=2, got %d", meta.Total)
	}
}

// --- U-02: PaginationFirstPage ---

func TestUnifiedOrderView_PaginationFirstPage(t *testing.T) {
	now := time.Now()
	guests := make([]models.GuestOrder, 5)
	for i := 0; i < 5; i++ {
		guests[i] = makeGuestOrder("t"+string(rune('a'+i)), "", "BTC",
			models.GuestOrderAwaitingPayment, now.Add(-time.Duration(i)*time.Hour))
	}

	v := NewUnifiedOrderView(nil, &mockGuestSvc{orders: guests, total: 5})

	results, meta, err := v.ListOrders(context.Background(), contracts.OrderListFilter{
		View: "guest", Page: 0, PageSize: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results on first page, got %d", len(results))
	}
	if meta.Page != 0 {
		t.Errorf("expected page=0, got %d", meta.Page)
	}
}

// --- U-03: PaginationSecondPage ---

func TestUnifiedOrderView_PaginationSecondPage(t *testing.T) {
	now := time.Now()
	guests := make([]models.GuestOrder, 5)
	for i := 0; i < 5; i++ {
		guests[i] = makeGuestOrder("t"+string(rune('a'+i)), "", "BTC",
			models.GuestOrderAwaitingPayment, now.Add(-time.Duration(i)*time.Hour))
	}

	v := NewUnifiedOrderView(nil, &mockGuestSvc{orders: guests, total: 5})

	results, meta, err := v.ListOrders(context.Background(), contracts.OrderListFilter{
		View: "guest", Page: 1, PageSize: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results on second page, got %d", len(results))
	}
	if meta.Page != 1 {
		t.Errorf("expected page=1, got %d", meta.Page)
	}
}

// --- U-04: FilterByState ---

func TestUnifiedOrderView_FilterByState(t *testing.T) {
	now := time.Now()
	funded := models.GuestOrderFunded
	guests := []models.GuestOrder{
		makeGuestOrder("tok1", "", "BTC", models.GuestOrderAwaitingPayment, now),
		makeGuestOrder("tok2", "", "BTC", models.GuestOrderFunded, now.Add(-time.Hour)),
	}

	v := NewUnifiedOrderView(nil, &mockGuestSvc{orders: []models.GuestOrder{guests[1]}, total: 1})

	results, _, err := v.ListOrders(context.Background(), contracts.OrderListFilter{
		View: "guest", State: funded.String(), PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range results {
		if r.State != funded.String() {
			t.Errorf("expected state %s, got %s", funded.String(), r.State)
		}
	}
}

// --- U-05: EmptyGuest ---

func TestUnifiedOrderView_EmptyGuest(t *testing.T) {
	v := NewUnifiedOrderView(
		&mockStandardFetcher{},
		&mockGuestSvc{},
	)

	results, meta, err := v.ListOrders(context.Background(), contracts.OrderListFilter{
		View: "all", PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
	if meta.Total != 0 {
		t.Errorf("expected total=0, got %d", meta.Total)
	}
}

// --- U-06: EmptyStandard ---

func TestUnifiedOrderView_EmptyStandard(t *testing.T) {
	now := time.Now()
	guests := []models.GuestOrder{
		makeGuestOrder("tok1", "x@y.com", "ETH", models.GuestOrderFunded, now),
	}

	v := NewUnifiedOrderView(nil, &mockGuestSvc{orders: guests, total: 1})

	results, _, err := v.ListOrders(context.Background(), contracts.OrderListFilter{
		View: "all", PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 guest result, got %d", len(results))
	}
	if results[0].Type != "guest" {
		t.Errorf("expected type=guest, got %s", results[0].Type)
	}
}

// --- U-07: BothEmpty ---

func TestUnifiedOrderView_BothEmpty(t *testing.T) {
	v := NewUnifiedOrderView(nil, nil)

	results, meta, err := v.ListOrders(context.Background(), contracts.OrderListFilter{
		View: "all", PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
	if meta.Total != 0 {
		t.Errorf("expected total=0, got %d", meta.Total)
	}
}

// --- convertGuestOrder unit tests ---

func TestConvertGuestOrder_BasicFields(t *testing.T) {
	now := time.Now()
	g := models.GuestOrder{
		OrderToken:    "abc123",
		ContactEmail:  "buyer@test.com",
		PaymentCoin:   "BTC",
		PaymentAmount: "50000",
		State:         models.GuestOrderFunded,
		CreatedAt:     now,
		Items: []models.GuestOrderItem{
			{ListingTitle: "Widget", Quantity: 2},
			{ListingTitle: "Gadget", Quantity: 1},
		},
	}

	s := convertGuestOrder(g)

	if s.ID != "gst_abc123" {
		t.Errorf("expected ID=gst_abc123, got %s", s.ID)
	}
	if s.Type != "guest" {
		t.Errorf("expected type=guest, got %s", s.Type)
	}
	if s.BuyerName != "buyer@test.com" {
		t.Errorf("expected buyerName=buyer@test.com, got %s", s.BuyerName)
	}
	if len(s.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(s.Items))
	}
	if s.Items[0].Title != "Widget" || s.Items[0].Quantity != 2 {
		t.Errorf("item 0 mismatch: %+v", s.Items[0])
	}
	if s.Total.Amount != "50000" || s.Total.CurrencyCode != "BTC" {
		t.Errorf("total mismatch: %+v", s.Total)
	}
	if s.PaymentCoin != "BTC" {
		t.Errorf("paymentCoin mismatch: %s", s.PaymentCoin)
	}
}

func TestConvertGuestOrder_NoEmail_DefaultsToGuest(t *testing.T) {
	g := models.GuestOrder{
		OrderToken: "xyz",
		State:      models.GuestOrderAwaitingPayment,
		CreatedAt:  time.Now(),
	}

	s := convertGuestOrder(g)
	if s.BuyerName != "Guest" {
		t.Errorf("expected buyerName=Guest, got %s", s.BuyerName)
	}
}

func TestConvertGuestOrder_SweepStatus(t *testing.T) {
	g := models.GuestOrder{
		OrderToken:     "sw1",
		SweepToAddress: "0xabc",
		State:          models.GuestOrderCompleted,
		CreatedAt:      time.Now(),
	}

	s := convertGuestOrder(g)
	if s.SweepStatus != "pending" {
		t.Errorf("expected sweepStatus=pending, got %s", s.SweepStatus)
	}
}

func TestConvertGuestOrder_UpdatedAtUsesLatestTimestamp(t *testing.T) {
	now := time.Now()
	funded := now.Add(1 * time.Hour)
	completed := now.Add(2 * time.Hour)

	g := models.GuestOrder{
		OrderToken:  "ts1",
		CreatedAt:   now,
		FundedAt:    &funded,
		CompletedAt: &completed,
		State:       models.GuestOrderCompleted,
	}

	s := convertGuestOrder(g)
	if !s.UpdatedAt.Equal(completed) {
		t.Errorf("expected updatedAt=%v, got %v", completed, s.UpdatedAt)
	}
}

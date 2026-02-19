package contracts

import (
	"context"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

// OrderFilter holds query parameters for listing orders.
type OrderFilter struct {
	StateFilter   []models.OrderState
	SearchTerm    string
	SearchColumns []string
	Exclude       []string
	SortByRead    bool
	SortAscending bool
	Limit         int
}

// OrderRepo abstracts order-specific database access.
//
// Implementations wrap the underlying database layer (GORM, PostgreSQL, etc.)
// and provide tenant-scoped operations on models.Order. This port decouples
// business logic from storage technology, enabling:
//   - Unit testing with in-memory stubs
//   - Future PostgreSQL migration without touching app services
//   - SaaS multi-tenant isolation via scoped implementations
//
// All methods accept context.Context for cancellation, tracing, and
// (optionally) transaction propagation. Implementations that need
// transactional grouping should use context-carried transactions.
type OrderRepo interface {
	// FindByID loads an order by ID.
	FindByID(ctx context.Context, orderID string) (*models.Order, error)

	// FindPurchases returns orders where my_role = "buyer", filtered by the given criteria.
	FindPurchases(ctx context.Context, filter OrderFilter) ([]models.Order, int64, error)

	// FindSales returns orders where my_role = "vendor", filtered by the given criteria.
	FindSales(ctx context.Context, filter OrderFilter) ([]models.Order, int64, error)

	// FindUnverifiedPaymentOrders returns vendor orders with a serialized
	// PaymentSent but payment_verified = false and open = true.
	FindUnverifiedPaymentOrders(ctx context.Context) ([]models.Order, error)

	// Save persists an order (insert or upsert).
	Save(ctx context.Context, order *models.Order) error

	// MarkAsRead sets read = true for the given order.
	MarkAsRead(ctx context.Context, orderID string) error

	// UpdateState sets the order state.
	UpdateState(ctx context.Context, orderID string, state models.OrderState) error

	// UpdateLastCheckTime updates the last_check_for_payments timestamp.
	UpdateLastCheckTime(ctx context.Context, orderID string, t time.Time) error

	// ExpirePaymentVerification marks an order's payment as expired
	// (sets open = false and last_check_for_payments to the given reason marker).
	ExpirePaymentVerification(ctx context.Context, orderID string, marker time.Time) error
}

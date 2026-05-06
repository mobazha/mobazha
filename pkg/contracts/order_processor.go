// Package contracts defines contract interfaces between core and implementations.
package contracts

import (
	"context"

	"github.com/mobazha/mobazha3.0/pkg/orders"
)

// OrderProcessor is the contract interface for processing order state transitions.
// This interface defines how order events are handled and persisted.
type OrderProcessor interface {
	// ProcessEvent processes an order event and returns the new state.
	// The implementation is responsible for:
	// 1. Validating the event is valid for the current state
	// 2. Persisting the state change
	// 3. Triggering any side effects (notifications, payments, etc.)
	ProcessEvent(ctx context.Context, orderID string, event orders.OrderEvent) (orders.OrderState, error)

	// GetState returns the current state of an order.
	GetState(ctx context.Context, orderID string) (orders.OrderState, error)

	// ValidateTransition checks if a transition is valid without applying it.
	ValidateTransition(ctx context.Context, orderID string, event orders.OrderEvent) error
}

// OrderStore is the contract interface for order persistence.
// Implementations handle storage in local DB (node) or multi-tenant DB (cloud).
//
// The Order type is intentionally kept as interface{} to allow each implementation
// to use its own model (e.g., models.Order in node, tenant-scoped model in cloud).
// The contract enforces the pattern, not the exact data structure.
type OrderStore interface {
	// SaveOrder saves or updates an order.
	SaveOrder(ctx context.Context, order interface{}) error

	// GetOrder retrieves an order by ID.
	GetOrder(ctx context.Context, orderID string) (interface{}, error)

	// ListOrders returns orders matching the filter criteria.
	ListOrders(ctx context.Context, filter OrderFilter) (interface{}, error)

	// UpdateState updates only the state of an order.
	UpdateState(ctx context.Context, orderID string, state orders.OrderState) error
}

// NOTE: OrderFilter is defined in order_repo.go (the production-ready version).
// The core's original OrderFilter (with BuyerPeerID/VendorPeerID fields) has been
// merged into the existing definition. Add peer-based filters there if needed.

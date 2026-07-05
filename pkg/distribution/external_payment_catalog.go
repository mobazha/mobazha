// SPDX-License-Identifier: MPL-2.0

package distribution

import (
	"errors"
	"fmt"
	"sync"

	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ErrExternalPaymentRuntimeUnavailable means the exact implementation bound
// to durable payment work is not available in the current composition.
var ErrExternalPaymentRuntimeUnavailable = errors.New("direct observed runtime unavailable")

// ExternalPaymentRuntimeRegistration binds one complete durable route identity
// to the runtime that implements it.
type ExternalPaymentRuntimeRegistration struct {
	Route            payment.RouteIdentity
	Runtime          ExternalPaymentRuntime
	ActiveForNewWork bool
}

// ExternalPaymentRuntimeCatalog selects the active implementation for new
// work while retaining exact historical implementations for recovery.
type ExternalPaymentRuntimeCatalog struct {
	mu      sync.RWMutex
	byRoute map[payment.RouteIdentity]ExternalPaymentRuntime
	active  map[string]payment.RouteIdentity
}

func NewExternalPaymentRuntimeCatalog() *ExternalPaymentRuntimeCatalog {
	return &ExternalPaymentRuntimeCatalog{
		byRoute: make(map[payment.RouteIdentity]ExternalPaymentRuntime),
		active:  make(map[string]payment.RouteIdentity),
	}
}

// Register makes registration active for its concrete asset and retains all
// other route identities for historical recovery.
func (c *ExternalPaymentRuntimeCatalog) Register(registration ExternalPaymentRuntimeRegistration) error {
	if c == nil {
		return fmt.Errorf("%w: catalog is nil", ErrExternalPaymentRuntimeUnavailable)
	}
	if registration.Runtime == nil {
		return fmt.Errorf("%w: runtime is nil", ErrExternalPaymentRuntimeUnavailable)
	}
	if err := registration.Route.Validate(); err != nil {
		return err
	}
	if registration.Route.RailKind != string(PaymentRailDirectObserved) {
		return fmt.Errorf("direct observed runtime route has rail %q", registration.Route.RailKind)
	}
	if registration.Route.AssetID == string(PaymentAssetAny) {
		return errors.New("direct observed runtime route requires a concrete asset")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.byRoute[registration.Route]; exists {
		return fmt.Errorf("direct observed runtime route %q is already registered", registration.Route.ContributionID)
	}
	if registration.ActiveForNewWork {
		if active, exists := c.active[registration.Route.AssetID]; exists && active != registration.Route {
			return fmt.Errorf("direct observed asset %q already has an active route", registration.Route.AssetID)
		}
	}
	c.byRoute[registration.Route] = registration.Runtime
	if registration.ActiveForNewWork {
		c.active[registration.Route.AssetID] = registration.Route
	}
	return nil
}

// Unregister removes only the exact implementation generation. If it was the
// active route, new work fails closed instead of silently selecting an older
// generation.
func (c *ExternalPaymentRuntimeCatalog) Unregister(route payment.RouteIdentity) {
	if c == nil || route.IsZero() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.byRoute, route)
	if active, ok := c.active[route.AssetID]; ok && active == route {
		delete(c.active, route.AssetID)
	}
}

// Active returns the implementation selected for new work on asset.
func (c *ExternalPaymentRuntimeCatalog) Active(asset iwallet.CoinType) (ExternalPaymentRuntimeRegistration, error) {
	if c == nil {
		return ExternalPaymentRuntimeRegistration{}, ErrExternalPaymentRuntimeUnavailable
	}
	c.mu.RLock()
	route, ok := c.active[string(asset)]
	runtime := c.byRoute[route]
	c.mu.RUnlock()
	if !ok || runtime == nil {
		return ExternalPaymentRuntimeRegistration{}, fmt.Errorf("%w for asset %q", ErrExternalPaymentRuntimeUnavailable, asset)
	}
	return ExternalPaymentRuntimeRegistration{Route: route, Runtime: runtime, ActiveForNewWork: true}, nil
}

// Resolve returns the exact historical implementation captured by route.
func (c *ExternalPaymentRuntimeCatalog) Resolve(route payment.RouteIdentity) (ExternalPaymentRuntime, error) {
	if c == nil || route.IsZero() {
		return nil, ErrExternalPaymentRuntimeUnavailable
	}
	c.mu.RLock()
	runtime := c.byRoute[route]
	c.mu.RUnlock()
	if runtime == nil {
		return nil, fmt.Errorf("%w for contribution %q generation %q", ErrExternalPaymentRuntimeUnavailable, route.ContributionID, route.ImplementationGeneration)
	}
	return runtime, nil
}

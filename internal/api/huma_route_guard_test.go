//go:build !private_distribution

package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// newTestGatewayForRouting creates a minimal Gateway that can register
// routes without nil panics. No real node manager or auth is wired —
// handler functions will panic if actually invoked, but route
// registration itself is managed_escrow.
func newTestGatewayForRouting() *Gateway {
	return &Gateway{
		config:            &GatewayConfig{},
		guestOrderLimiter: newRateLimiter(100, time.Minute),
	}
}

// activatedHumaDomains lists path prefixes whose legacy routes have
// been removed. Collisions in these domains are hard failures.
// AH-1.4: All domains with Huma counterparts have been activated.
var activatedHumaDomains = []string{
	"/v1/wallet/",
	"/v1/carts/",
	"/v1/chat/",
	"/v1/notifications/",
	"/v1/orders/",
	"/v1/orders",
	"/v1/purchases",
	"/v1/sales",
	"/v1/cases",
	"/v1/disputes/",
	"/v1/crypto/",
	"/v1/system/",
	"/v1/auth/",
	"/v1/admin/",
	"/v1/exchange-rates",
	"/v1/moderators",
	"/v1/blocklist/",
	"/v1/peers",
	"/v1/config",
	"/v1/fiat/",
	"/v1/listings",
	"/v1/media/",
	"/v1/profiles",
	"/v1/posts/",
	"/v1/posts",
	"/v1/followers",
	"/v1/following",
	"/v1/ratings/",
	"/v1/webhooks",
	"/v1/fulfillment/",
	"/v1/preferences",
	"/v1/wishlists",
	"/v1/settings/",
	"/v1/features",
	"/v1/shipping/",
	"/v1/discounts/",
	"/v1/discounts",
	"/v1/collections",
	"/v1/store-policy",
	"/v1/ai/",
	"/v1/guest/",
	"/v1/payment-methods/",
	"/v1/analytics/",
}

func isActivatedDomain(path string) bool {
	for _, prefix := range activatedHumaDomains {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// collectRouteCollisions walks the chi router and returns all method+path
// pairs that are registered more than once.
func collectRouteCollisions(r chi.Router) []string {
	type routeKey struct {
		method   string
		template string
	}

	seen := map[routeKey]int{}
	var collisions []string

	_ = chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		key := routeKey{method: method, template: route}
		seen[key]++
		if seen[key] == 2 {
			collisions = append(collisions, fmt.Sprintf("%s %s", method, route))
		}
		return nil
	})
	sort.Strings(collisions)
	return collisions
}

// TestAH14_NoRouteCollision_ActivatedDomains fails if any Huma-activated
// domain still has a collision with legacy routes.
func TestAH14_NoRouteCollision_ActivatedDomains(t *testing.T) {
	g := newTestGatewayForRouting()
	r := chi.NewMux()
	r.Use(maxBodySizeMiddleware(defaultMaxBodySize))
	g.registerHumaAPI(r)

	collisions := collectRouteCollisions(r)

	var activated, pending []string
	for _, c := range collisions {
		parts := strings.SplitN(c, " ", 2)
		if len(parts) == 2 && isActivatedDomain(parts[1]) {
			activated = append(activated, c)
		} else {
			pending = append(pending, c)
		}
	}

	if len(activated) > 0 {
		t.Errorf("Route collision in activated Huma domain — legacy route "+
			"not removed:\n  %s", strings.Join(activated, "\n  "))
	}

	if len(pending) > 0 {
		t.Logf("Pending domain collisions (expected until migration):\n  %s",
			strings.Join(pending, "\n  "))
	}
}

// TestAH14_ActivatedRoutesServedByHuma verifies that after legacy routes
// are removed, representative endpoints from each activated domain are
// still matched by the router (served by Huma handlers).
func TestAH14_ActivatedRoutesServedByHuma(t *testing.T) {
	g := newTestGatewayForRouting()
	r := chi.NewMux()
	r.Use(maxBodySizeMiddleware(defaultMaxBodySize))
	g.registerHumaAPI(r)

	cases := []struct {
		method string
		path   string
	}{
		// Wallet
		{"POST", "/v1/wallet/spend"},
		{"GET", "/v1/wallet/mnemonic"},
		{"GET", "/v1/wallet/currencies"},
		{"GET", "/v1/wallet/receiving-accounts"},
		// Cart
		{"GET", "/v1/carts"},
		{"POST", "/v1/carts/some-peer/items"},
		// Chat
		{"GET", "/v1/chat/rooms"},
		{"POST", "/v1/chat/rooms/room1/messages"},
		// Notifications
		{"GET", "/v1/notifications"},
		{"GET", "/v1/notifications/count"},
		// Orders
		{"POST", "/v1/orders"},
		{"POST", "/v1/orders/supply-quote"},
		{"GET", "/v1/orders/order123"},
		{"POST", "/v1/orders/order123/confirm"},
		{"GET", "/v1/purchases"},
		{"GET", "/v1/sales"},
		// Disputes
		{"POST", "/v1/disputes/order123/open"},
		// Fiat (auth'd)
		{"GET", "/v1/fiat/providers"},
		{"GET", "/v1/fiat/stripe/config"},
		// Listings
		{"POST", "/v1/listings"},
		{"POST", "/v1/listings/supply-summary"},
		{"GET", "/v1/listings/peer1/my-product"},
		// Media
		{"POST", "/v1/media/images"},
		{"GET", "/v1/media/images/img123"},
		// Profiles
		{"GET", "/v1/profiles/peer1"},
		{"PUT", "/v1/profiles"},
		// Social
		{"GET", "/v1/followers/peer1"},
		{"GET", "/v1/following"},
		{"GET", "/v1/ratings/rating1"},
		// Posts
		{"POST", "/v1/posts"},
		{"GET", "/v1/posts/my-post"},
		// Webhooks
		{"GET", "/v1/webhooks"},
		{"POST", "/v1/webhooks"},
		// Fulfillment
		{"GET", "/v1/fulfillment/providers"},
		// Collections
		{"GET", "/v1/collections"},
		// Store policy
		{"GET", "/v1/store-policy"},
		{"GET", "/v1/store-policy/moderators"},
		{"PUT", "/v1/store-policy/moderators"},
		{"GET", "/v1/store-policy/peer1/published"},
		// Shipping
		{"GET", "/v1/shipping/profiles"},
		// Discounts
		{"GET", "/v1/discounts"},
		// Settings
		{"GET", "/v1/preferences"},
		{"GET", "/v1/wishlists"},
		{"GET", "/v1/features"},
		// AI
		{"GET", "/v1/ai/status"},
		{"POST", "/v1/agent/product-import/ingest"},
		{"GET", "/v1/agent/product-import/runs/skillrun_1/workbench"},
		{"POST", "/v1/agent/product-import/runs/skillrun_1/approvals"},
		{"POST", "/v1/agent/product-import/runs/skillrun_1/approval-decisions"},
		{"POST", "/v1/agent/product-import/runs/skillrun_1/approval-applications"},
		{"POST", "/v1/agent/artifacts/art_1/approval"},
		// Guest checkout
		{"POST", "/v1/guest/orders"},
		{"POST", "/v1/guest/orders/quote"},
		// Payment methods
		{"GET", "/v1/payment-methods/peer1"},
		// Batch 5: misc / system / auth
		{"POST", "/v1/crypto/sign"},
		{"GET", "/v1/system/info"},
		{"GET", "/v1/auth/tokens"},
		{"POST", "/v1/admin/password"},
		{"GET", "/v1/exchange-rates"},
		{"GET", "/v1/moderators"},
		{"PUT", "/v1/blocklist/peer1"},
		{"GET", "/v1/peers"},
		{"GET", "/v1/config"},
		// AH-1.5: Multipart passthrough (migrated to Huma)
		{"POST", "/v1/chat/rooms/room1/avatar"},
		{"POST", "/v1/chat/media/upload"},
		{"POST", "/v1/listings/import"},
		{"POST", "/v1/listings/import/gumroad"},
		{"POST", "/v1/media/files"},
		// AH-1.6: Public discount helpers (migrated to Huma)
		{"POST", "/v1/discounts/peer1/validate"},
		{"GET", "/v1/discounts/peer1/applicable"},
		{"POST", "/v1/discounts/peer1/calculate"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s_%s", tc.method, tc.path), func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			func() {
				defer func() { recover() }()
				r.ServeHTTP(rr, req)
			}()
			if rr.Code == http.StatusNotFound || rr.Code == http.StatusMethodNotAllowed {
				t.Fatalf("no route matched %s %s (status %d) — Huma handler missing?",
					tc.method, tc.path, rr.Code)
			}
		})
	}
}

package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
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

// collectRouteCollisions walks the router and returns all method+path
// pairs that are registered more than once.
func collectRouteCollisions(r *mux.Router) []string {
	type routeKey struct {
		method   string
		template string
	}

	seen := map[routeKey]int{}
	var collisions []string

	_ = r.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		tmpl, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}
		methods, err := route.GetMethods()
		if err != nil {
			return nil
		}
		for _, m := range methods {
			key := routeKey{method: m, template: tmpl}
			seen[key]++
			if seen[key] == 2 {
				collisions = append(collisions, fmt.Sprintf("%s %s", m, tmpl))
			}
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
	r := mux.NewRouter()
	r.Use(maxBodySizeMiddleware(defaultMaxBodySize))
	g.registerBusinessRoutes(r)
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

// TestAH14_MuxPassthroughRoutes_Documented ensures that every route in
// registerBusinessRoutes() (routes.go) is explicitly documented here.
// These routes are NOT in the Huma OpenAPI surface. If you add or remove
// a mux-only route you must update this list, preventing silent drift
// between the live API and the generated spec.
func TestAH14_MuxPassthroughRoutes_Documented(t *testing.T) {
	g := newTestGatewayForRouting()

	muxRouter := mux.NewRouter()
	g.registerBusinessRoutes(muxRouter)

	var actual []string
	_ = muxRouter.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		tmpl, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}
		methods, err := route.GetMethods()
		if err != nil {
			return nil
		}
		for _, m := range methods {
			actual = append(actual, fmt.Sprintf("%s %s", m, tmpl))
		}
		return nil
	})
	sort.Strings(actual)

	// Exhaustive documented set. Each entry is a route that cannot use the
	// Huma JSON bridge and must remain as a direct mux handler:
	//
	//  multipart/form-data routes: Huma bridge wraps bodies as json.RawMessage,
	//    which is incompatible with multipart uploads (avatars, media, ZIP import, files).
	//
	//  public storefront discount helpers: {peerID} path param conflicts with
	//    {discountID} under /v1/discounts/ in gorilla/mux; chi migration will resolve.
	documented := []string{
		"GET /v1/discounts/{peerID}/applicable",
		"POST /v1/chat/media/upload",
		"POST /v1/chat/rooms/{roomID}/avatar",
		"POST /v1/discounts/{peerID}/calculate",
		"POST /v1/discounts/{peerID}/validate",
		"POST /v1/listings/import",
		"POST /v1/media/files",
	}

	if len(actual) != len(documented) {
		t.Fatalf("mux-only route count mismatch: got %d, documented %d\n"+
			"  got:        %v\n  documented: %v",
			len(actual), len(documented), actual, documented)
	}
	for i := range documented {
		if actual[i] != documented[i] {
			t.Errorf("mux-only route mismatch at [%d]: got %q, documented %q",
				i, actual[i], documented[i])
		}
	}
}

// TestAH14_ActivatedRoutesServedByHuma verifies that after legacy routes
// are removed, representative endpoints from each activated domain are
// still matched by the router (served by Huma handlers).
func TestAH14_ActivatedRoutesServedByHuma(t *testing.T) {
	g := newTestGatewayForRouting()
	r := mux.NewRouter()
	r.Use(maxBodySizeMiddleware(defaultMaxBodySize))
	g.registerBusinessRoutes(r)
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
		// Guest checkout
		{"POST", "/v1/guest/orders"},
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
		// Multipart passthrough (mux, not Huma)
		{"POST", "/v1/chat/rooms/room1/avatar"},
		{"POST", "/v1/chat/media/upload"},
		{"POST", "/v1/listings/import"},
		{"POST", "/v1/media/files"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s_%s", tc.method, tc.path), func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, tc.path, nil)
			var match mux.RouteMatch
			if !r.Match(req, &match) {
				t.Fatalf("no route matched %s %s — Huma handler missing?", tc.method, tc.path)
			}
		})
	}
}

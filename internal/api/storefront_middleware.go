// Package api — storefront_middleware.go
//
// MS-Phase-2a · MS2a.2c · MS2a.5 — Node-side storefront resolver.
//
// The hosting Gateway resolves a storefront subdomain (`{slug}.app.mobazha.org`)
// into a Storefront.Filter + Storefront.PriceRule and injects request headers:
//
//   X-Storefront-ID                     — storefront identity (e.g. "spring-sale")
//   X-Storefront-Filter-Collections     — comma-separated collection IDs
//   X-Storefront-Filter-Tags            — comma-separated include tags
//   X-Storefront-Filter-ExcludeTags     — comma-separated exclude tags
//   X-Storefront-Price-Rule             — compact JSON (MS2a.5)
//
// This middleware parses those headers into a StorefrontContext and stores
// it in request.Context for downstream handlers (listing/profile/order) to
// scope their queries. When no X-Storefront-ID header is present (main host
// / API traffic), the middleware is a no-op — handlers see no storefront
// context and return the full unfiltered view, preserving backward compat.
//
// Tag-based filtering is NOT applied yet — ListingMetadata currently lacks
// a Tags field (TD-033). Collection filtering is the primary MVP surface.
//
// The price rule is applied only to list-view DTOs (ListingMetadata) so we
// never mutate a SignedListing (whose price field is cryptographically
// signed by the seller). Detail / checkout flows deliberately ignore it —
// see MULTI_STORE_ROADMAP.md MS2a.5 for the follow-up plan.
package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/storefront"
)

const (
	headerStorefrontID                = "X-Storefront-ID"
	headerStorefrontFilterCollections = "X-Storefront-Filter-Collections"
	headerStorefrontFilterTags        = "X-Storefront-Filter-Tags"
	headerStorefrontFilterExcludeTags = "X-Storefront-Filter-ExcludeTags"
	headerStorefrontPriceRule         = "X-Storefront-Price-Rule"
)

// DefaultStorefrontID matches hosting's `db.DefaultStorefrontID` — the
// reserved ID for the implicit main-store storefront. Handlers treat this
// as "no filter" (show the full main-store catalog).
const DefaultStorefrontID = "default"

// StorefrontFilter mirrors hosting's db.StorefrontFilter in the wire format
// we consume from request headers. Empty slices mean "no restriction on
// this axis" (rather than "allow nothing").
type StorefrontFilter struct {
	CollectionIDs []string
	Tags          []string
	ExcludeTags   []string
}

// IsEmpty reports whether all filter axes are empty. Callers use this to
// short-circuit — an empty filter means "show everything the main store
// would show" and handlers skip the filtering loop entirely.
func (f *StorefrontFilter) IsEmpty() bool {
	if f == nil {
		return true
	}
	return len(f.CollectionIDs) == 0 && len(f.Tags) == 0 && len(f.ExcludeTags) == 0
}

// StorefrontContext captures the storefront routing info for a single
// request. ID is always set when the context is present; Filter/PriceRule
// may be nil — a non-empty ID without either means "storefront exists but
// has no restrictions and no pricing adjustment" (treated as "show
// everything with base prices" at the listing layer).
type StorefrontContext struct {
	ID        string
	Filter    *StorefrontFilter
	PriceRule *storefront.PriceRule
}

// storefrontCtxKey is the private context key used to stash the parsed
// storefront context. Exported helpers below read/write through it so
// handlers never touch the raw key.
type storefrontCtxKeyType struct{}

var storefrontCtxKey = storefrontCtxKeyType{}

// StorefrontMiddleware parses storefront headers injected by the hosting
// Gateway (MS2a.2b) and stashes a StorefrontContext on the request. When
// no X-Storefront-ID header is present, the middleware is a no-op.
//
// Unknown / malformed header values are tolerated — empty or whitespace
// tokens are dropped and the rest are passed through. Handlers are
// expected to verify filter semantics against their local data.
func (g *Gateway) StorefrontMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sfID := strings.TrimSpace(r.Header.Get(headerStorefrontID))
		if sfID == "" {
			next.ServeHTTP(w, r)
			return
		}

		sfCtx := &StorefrontContext{
			ID:        sfID,
			Filter:    parseStorefrontFilterFromHeaders(r),
			PriceRule: parseStorefrontPriceRuleFromHeader(r),
		}
		ctx := context.WithValue(r.Context(), storefrontCtxKey, sfCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// StorefrontFromContext returns the StorefrontContext injected by
// StorefrontMiddleware, or nil when the request does not carry storefront
// routing headers (main host traffic / internal API calls).
func StorefrontFromContext(ctx context.Context) *StorefrontContext {
	if ctx == nil {
		return nil
	}
	sc, _ := ctx.Value(storefrontCtxKey).(*StorefrontContext)
	return sc
}

// StorefrontFilterFromContext is a convenience that returns the filter
// portion or nil. Handlers that only care about filtering (not storefront
// identity) can use this and skip the wrapper struct.
func StorefrontFilterFromContext(ctx context.Context) *StorefrontFilter {
	sc := StorefrontFromContext(ctx)
	if sc == nil {
		return nil
	}
	return sc.Filter
}

// StorefrontPriceRuleFromContext is a convenience that returns the price
// rule or nil. Handlers applying the rule to list-view DTOs call this
// right before emitting the response.
func StorefrontPriceRuleFromContext(ctx context.Context) *storefront.PriceRule {
	sc := StorefrontFromContext(ctx)
	if sc == nil {
		return nil
	}
	return sc.PriceRule
}

// parseStorefrontFilterFromHeaders reads the three X-Storefront-Filter-*
// headers and returns a StorefrontFilter. Returns nil when all axes are
// empty so callers can cheaply short-circuit via Filter == nil.
func parseStorefrontFilterFromHeaders(r *http.Request) *StorefrontFilter {
	collections := parseCSVHeader(r.Header.Get(headerStorefrontFilterCollections))
	tags := parseCSVHeader(r.Header.Get(headerStorefrontFilterTags))
	excludeTags := parseCSVHeader(r.Header.Get(headerStorefrontFilterExcludeTags))

	if len(collections) == 0 && len(tags) == 0 && len(excludeTags) == 0 {
		return nil
	}
	return &StorefrontFilter{
		CollectionIDs: collections,
		Tags:          tags,
		ExcludeTags:   excludeTags,
	}
}

// parseStorefrontPriceRuleFromHeader reads X-Storefront-Price-Rule and
// returns the decoded rule. Parse failures are tolerated — they log a
// warning and return nil so a malformed hosting header can never 500 a
// listing endpoint. A nil rule means "use base prices" at the listing
// layer.
func parseStorefrontPriceRuleFromHeader(r *http.Request) *storefront.PriceRule {
	raw := r.Header.Get(headerStorefrontPriceRule)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	rule, err := storefront.ParsePriceRule(raw)
	if err != nil {
		log.Warningf("storefront middleware: invalid %s header: %v", headerStorefrontPriceRule, err)
		return nil
	}
	return rule
}

// parseCSVHeader splits a comma-separated header value into cleaned tokens.
// Empty / whitespace-only tokens are dropped. Returns nil for an empty
// input so callers can nil-check instead of len-check.
func parseCSVHeader(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

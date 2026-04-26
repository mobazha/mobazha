package contracts

import (
	"sort"
	"strings"
)

// Scope represents a fine-grained permission for API access.
// Format: {domain}:{action}
//
// Domains map to API route groups. Actions are:
//   - read   — GET/HEAD operations
//   - write  — POST/PUT/PATCH/DELETE operations
//   - manage — superset of read+write with additional admin-level operations
type Scope string

const (
	// Listings
	ScopeListingsRead  Scope = "listings:read"
	ScopeListingsWrite Scope = "listings:write"

	// Orders (seller perspective: confirm, fulfill, refund)
	ScopeOrdersRead   Scope = "orders:read"
	ScopeOrdersManage Scope = "orders:manage"

	// Purchases (buyer perspective)
	ScopePurchasesRead Scope = "purchases:read"

	// Wallet
	ScopeWalletRead   Scope = "wallet:read"
	ScopeWalletSpend  Scope = "wallet:spend"
	ScopeWalletManage Scope = "wallet:manage"

	// Chat (Matrix)
	ScopeChatRead  Scope = "chat:read"
	ScopeChatWrite Scope = "chat:write"

	// Profiles
	ScopeProfilesRead  Scope = "profiles:read"
	ScopeProfilesWrite Scope = "profiles:write"

	// Notifications
	ScopeNotificationsRead   Scope = "notifications:read"
	ScopeNotificationsManage Scope = "notifications:manage"

	// Media (images, files)
	ScopeMediaRead  Scope = "media:read"
	ScopeMediaWrite Scope = "media:write"

	// Ratings
	ScopeRatingsRead Scope = "ratings:read"

	// Settings / Preferences
	ScopeSettingsRead  Scope = "settings:read"
	ScopeSettingsWrite Scope = "settings:write"

	// Webhooks
	ScopeWebhooksManage Scope = "webhooks:manage"

	// Discounts
	ScopeDiscountsRead  Scope = "discounts:read"
	ScopeDiscountsWrite Scope = "discounts:write"

	// Collections
	ScopeCollectionsRead  Scope = "collections:read"
	ScopeCollectionsWrite Scope = "collections:write"

	// Shipping profiles
	ScopeShippingRead  Scope = "shipping:read"
	ScopeShippingWrite Scope = "shipping:write"

	// Fiat payment
	ScopeFiatRead   Scope = "fiat:read"
	ScopeFiatManage Scope = "fiat:manage"

	// AI features
	ScopeAIUse Scope = "ai:use"

	// Analytics (future: store stats, reports)
	ScopeAnalyticsRead Scope = "analytics:read"

	// Wishlists
	ScopeWishlistsRead  Scope = "wishlists:read"
	ScopeWishlistsWrite Scope = "wishlists:write"

	// Carts
	ScopeCartsRead  Scope = "carts:read"
	ScopeCartsWrite Scope = "carts:write"

	// Disputes
	ScopeDisputesRead   Scope = "disputes:read"
	ScopeDisputesManage Scope = "disputes:manage"

	// ScopeAny is a sentinel value used in route → scope maps to mark a route
	// as readable by ANY authenticated identity (including low-privilege API
	// tokens). It is intentionally the empty string so that:
	//   - IsValid() returns false → token-creation paths (which reject unknown
	//     scopes via ValidateScopes) cannot accidentally accept it from clients.
	//   - Scope-enforcement middleware can treat "" as "skip the HasScope check"
	//     without conflating it with a real permission.
	// Use this for global/metadata endpoints like /v1/auth/identity and
	// /v1/auth/scopes that every authenticated caller needs.
	ScopeAny Scope = ""
)

// allScopes is the canonical registry of valid scopes.
var allScopes = map[Scope]bool{
	ScopeListingsRead: true, ScopeListingsWrite: true,
	ScopeOrdersRead: true, ScopeOrdersManage: true,
	ScopePurchasesRead: true,
	ScopeWalletRead: true, ScopeWalletSpend: true, ScopeWalletManage: true,
	ScopeChatRead: true, ScopeChatWrite: true,
	ScopeProfilesRead: true, ScopeProfilesWrite: true,
	ScopeNotificationsRead: true, ScopeNotificationsManage: true,
	ScopeMediaRead: true, ScopeMediaWrite: true,
	ScopeRatingsRead: true,
	ScopeSettingsRead: true, ScopeSettingsWrite: true,
	ScopeWebhooksManage: true,
	ScopeDiscountsRead: true, ScopeDiscountsWrite: true,
	ScopeCollectionsRead: true, ScopeCollectionsWrite: true,
	ScopeShippingRead: true, ScopeShippingWrite: true,
	ScopeFiatRead: true, ScopeFiatManage: true,
	ScopeAIUse: true,
	ScopeAnalyticsRead: true,
	ScopeWishlistsRead: true, ScopeWishlistsWrite: true,
	ScopeCartsRead: true, ScopeCartsWrite: true,
	ScopeDisputesRead: true, ScopeDisputesManage: true,
}

// IsValid returns whether s is a recognized scope.
func (s Scope) IsValid() bool {
	return allScopes[s]
}

// Domain returns the part before the colon (e.g. "listings").
func (s Scope) Domain() string {
	if i := strings.IndexByte(string(s), ':'); i >= 0 {
		return string(s)[:i]
	}
	return string(s)
}

// Action returns the part after the colon (e.g. "read").
func (s Scope) Action() string {
	if i := strings.IndexByte(string(s), ':'); i >= 0 {
		return string(s)[i+1:]
	}
	return ""
}

// ValidateScopes checks that all scopes in the list are recognized.
// Returns the first invalid scope, or "" if all are valid.
func ValidateScopes(scopes []Scope) Scope {
	for _, s := range scopes {
		if !s.IsValid() {
			return s
		}
	}
	return ""
}

// ParseScopes converts a slice of strings to Scopes.
func ParseScopes(strs []string) []Scope {
	scopes := make([]Scope, len(strs))
	for i, s := range strs {
		scopes[i] = Scope(s)
	}
	return scopes
}

// ScopeStrings converts Scopes to a string slice.
func ScopeStrings(scopes []Scope) []string {
	strs := make([]string, len(scopes))
	for i, s := range scopes {
		strs[i] = string(s)
	}
	return strs
}

// scopeParents defines the hierarchy: child → parent.
// Having the parent in a ScopeSet automatically grants the child.
var scopeParents = map[Scope]Scope{
	ScopeListingsRead:      ScopeListingsWrite,
	ScopeOrdersRead:        ScopeOrdersManage,
	ScopeWalletRead:        ScopeWalletManage,
	ScopeWalletSpend:       ScopeWalletManage,
	ScopeChatRead:          ScopeChatWrite,
	ScopeProfilesRead:      ScopeProfilesWrite,
	ScopeNotificationsRead: ScopeNotificationsManage,
	ScopeMediaRead:         ScopeMediaWrite,
	ScopeSettingsRead:      ScopeSettingsWrite,
	ScopeDiscountsRead:     ScopeDiscountsWrite,
	ScopeCollectionsRead:   ScopeCollectionsWrite,
	ScopeShippingRead:      ScopeShippingWrite,
	ScopeFiatRead:          ScopeFiatManage,
	ScopeDisputesRead:      ScopeDisputesManage,
	ScopeWishlistsRead:     ScopeWishlistsWrite,
	ScopeCartsRead:         ScopeCartsWrite,
}

// ScopeSet is a set of scopes for fast lookup.
type ScopeSet map[Scope]struct{}

// NewScopeSet builds a ScopeSet from a list of scopes.
func NewScopeSet(scopes []Scope) ScopeSet {
	ss := make(ScopeSet, len(scopes))
	for _, s := range scopes {
		ss[s] = struct{}{}
	}
	return ss
}

// Has returns whether the set contains the given scope,
// respecting scope hierarchy: :manage implies :read and :write.
func (ss ScopeSet) Has(s Scope) bool {
	if _, ok := ss[s]; ok {
		return true
	}
	if parent, ok := scopeParents[s]; ok {
		_, ok = ss[parent]
		return ok
	}
	return false
}

// HasAny returns whether the set contains any of the given scopes,
// respecting scope hierarchy (same as Has).
func (ss ScopeSet) HasAny(scopes ...Scope) bool {
	for _, s := range scopes {
		if ss.Has(s) {
			return true
		}
	}
	return false
}

// AllScopes returns all recognized scopes in stable sorted order.
func AllScopes() []Scope {
	scopes := make([]Scope, 0, len(allScopes))
	for s := range allScopes {
		scopes = append(scopes, s)
	}
	sort.Slice(scopes, func(i, j int) bool { return scopes[i] < scopes[j] })
	return scopes
}

// SellerScopes returns the default scope set for a seller (store owner).
func SellerScopes() []Scope {
	return []Scope{
		ScopeListingsRead, ScopeListingsWrite,
		ScopeOrdersRead, ScopeOrdersManage,
		ScopeWalletRead, ScopeWalletSpend,
		ScopeChatRead, ScopeChatWrite,
		ScopeProfilesRead, ScopeProfilesWrite,
		ScopeNotificationsRead, ScopeNotificationsManage,
		ScopeMediaRead, ScopeMediaWrite,
		ScopeRatingsRead,
		ScopeSettingsRead, ScopeSettingsWrite,
		ScopeWebhooksManage,
		ScopeDiscountsRead, ScopeDiscountsWrite,
		ScopeCollectionsRead, ScopeCollectionsWrite,
		ScopeShippingRead, ScopeShippingWrite,
		ScopeFiatRead, ScopeFiatManage,
		ScopeAIUse,
		ScopeAnalyticsRead,
	}
}

// BuyerScopes returns the default scope set for a buyer.
//
// Includes ScopeAIUse so a buyer:* token can drive the local MCP server (the
// /v1/mcp endpoint enforces ai:use). The MCP routing plan explicitly states
// that MCP is role-agnostic — sellers manage listings, buyers query
// purchases/chat — through the same endpoint, gated only by scopes.
func BuyerScopes() []Scope {
	return []Scope{
		ScopeListingsRead,
		ScopePurchasesRead,
		ScopeWalletRead, ScopeWalletSpend,
		ScopeChatRead, ScopeChatWrite,
		ScopeProfilesRead, ScopeProfilesWrite,
		ScopeNotificationsRead,
		ScopeMediaRead,
		ScopeRatingsRead,
		ScopeWishlistsRead, ScopeWishlistsWrite,
		ScopeCartsRead, ScopeCartsWrite,
		ScopeAIUse,
	}
}

// ExpandScopePresets replaces role-based wildcard scopes with their concrete
// scope lists. Recognized presets are kept in this single place so both the
// SaaS gateway and standalone node speak the same vocabulary.
//
// Supported presets:
//   - "seller:*" → SellerScopes()
//   - "buyer:*"  → BuyerScopes()
//
// Unknown values pass through unchanged so they can be rejected by the regular
// validation pass (ValidateScopes / ParseScopes) with a precise error.
func ExpandScopePresets(raw []string) []string {
	if len(raw) == 0 {
		return raw
	}
	expanded := make([]string, 0, len(raw))
	for _, s := range raw {
		switch s {
		case "seller:*":
			expanded = append(expanded, ScopeStrings(SellerScopes())...)
		case "buyer:*":
			expanded = append(expanded, ScopeStrings(BuyerScopes())...)
		default:
			expanded = append(expanded, s)
		}
	}
	return expanded
}

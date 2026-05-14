package api

import "github.com/mobazha/mobazha3.0/pkg/contracts"

// routeScopeMap defines the required scope for Node API (/v1/*) route prefixes
// in standalone mode.
//
// This list is the standalone counterpart of mobazha_hosting's routeScopeMap;
// it includes only the routes that this binary actually serves (no SaaS-only
// platform routes). The middleware checks if "{METHOD} {path}" starts with one
// of the patterns and requires the corresponding scope.
//
// Important policy (deny-by-default):
//   - JWT / Basic Auth users have Scopes == nil → bypass scope checks entirely
//     (full access). They are validated by AuthenticationMiddleware.
//   - API tokens (mbz_*) carry an explicit ScopeSet. ScopeEnforcementMiddleware
//     denies any request that does not match an entry in this map, so newly
//     added private routes are managed_escrow-by-default until they are mapped here.
//
// Order matters: more-specific paths must come before their less-specific
// prefix counterparts (e.g. "POST /v1/wallet/spend" before "GET /v1/wallet").
var routeScopeMap = []routeScope{
	// Listings
	{"GET /v1/listings", contracts.ScopeListingsRead},
	{"POST /v1/listings", contracts.ScopeListingsWrite},
	{"PUT /v1/listings", contracts.ScopeListingsWrite},
	{"DELETE /v1/listings", contracts.ScopeListingsWrite},

	// Orders
	{"GET /v1/orders", contracts.ScopeOrdersRead},
	{"POST /v1/orders", contracts.ScopeOrdersManage},
	// orders payment watch tear-down: an order management action that
	// happens to use DELETE; without an explicit entry the deny-by-default
	// branch in matchRouteScope would block tokens carrying ScopeOrdersManage.
	{"DELETE /v1/orders", contracts.ScopeOrdersManage},

	// Guest checkout (sellers receive guest orders; tokens with the orders
	// scope are a legitimate use case — e.g. fulfillment automation).
	{"GET /v1/guest/orders", contracts.ScopeOrdersRead},
	{"PUT /v1/guest/orders", contracts.ScopeOrdersManage},

	// Sales (seller view of orders)
	{"GET /v1/sales", contracts.ScopeOrdersRead},
	{"POST /v1/sales", contracts.ScopeOrdersRead},

	// Data exports (DG-1.10 — "Your store, your data" portability).
	// Listings export reads the public catalogue and only needs listings
	// scope; sales/customers expose buyer info and require orders scope.
	// Specific entries come before any future broader prefix so deny-by-
	// default routes API tokens correctly.
	{"GET /v1/exports/listings", contracts.ScopeListingsRead},
	{"GET /v1/exports/sales", contracts.ScopeOrdersRead},
	{"GET /v1/exports/customers", contracts.ScopeOrdersRead},

	// Purchases (buyer perspective)
	{"GET /v1/purchases", contracts.ScopePurchasesRead},
	{"POST /v1/purchases", contracts.ScopePurchasesRead},

	// Wallet (specific routes first)
	{"GET /v1/wallet/mnemonic", contracts.ScopeWalletManage},
	{"POST /v1/wallet/spend", contracts.ScopeWalletSpend},
	{"POST /v1/wallet/receiving-accounts", contracts.ScopeWalletSpend},
	{"PUT /v1/wallet/receiving-accounts", contracts.ScopeWalletSpend},
	{"DELETE /v1/wallet/receiving-accounts", contracts.ScopeWalletSpend},
	{"GET /v1/wallet", contracts.ScopeWalletRead},

	// Profiles
	{"GET /v1/profiles", contracts.ScopeProfilesRead},
	{"POST /v1/profiles", contracts.ScopeProfilesWrite},
	{"PUT /v1/profiles", contracts.ScopeProfilesWrite},
	{"PATCH /v1/profiles", contracts.ScopeProfilesWrite},

	// Notifications + channels
	{"GET /v1/notifications", contracts.ScopeNotificationsRead},
	{"POST /v1/notifications", contracts.ScopeNotificationsManage},
	{"PUT /v1/notifications", contracts.ScopeNotificationsManage},
	{"DELETE /v1/notifications", contracts.ScopeNotificationsManage},

	// Media
	{"GET /v1/media", contracts.ScopeMediaRead},
	{"POST /v1/media", contracts.ScopeMediaWrite},

	// Ratings
	{"GET /v1/ratings", contracts.ScopeRatingsRead},
	{"POST /v1/ratings", contracts.ScopeRatingsRead},

	// Settings + Preferences
	{"GET /v1/settings", contracts.ScopeSettingsRead},
	{"PUT /v1/settings", contracts.ScopeSettingsWrite},
	{"POST /v1/settings", contracts.ScopeSettingsWrite},
	{"PATCH /v1/settings", contracts.ScopeSettingsWrite},
	{"GET /v1/preferences", contracts.ScopeSettingsRead},
	{"PUT /v1/preferences", contracts.ScopeSettingsWrite},
	{"POST /v1/preferences", contracts.ScopeSettingsWrite},

	// Webhooks
	{"GET /v1/webhooks", contracts.ScopeWebhooksManage},
	{"POST /v1/webhooks", contracts.ScopeWebhooksManage},
	{"PATCH /v1/webhooks", contracts.ScopeWebhooksManage},
	{"PUT /v1/webhooks", contracts.ScopeWebhooksManage},
	{"DELETE /v1/webhooks", contracts.ScopeWebhooksManage},

	// Discounts
	{"GET /v1/discounts", contracts.ScopeDiscountsRead},
	{"POST /v1/discounts", contracts.ScopeDiscountsWrite},
	{"PUT /v1/discounts", contracts.ScopeDiscountsWrite},
	{"DELETE /v1/discounts", contracts.ScopeDiscountsWrite},

	// Collections
	{"GET /v1/collections", contracts.ScopeCollectionsRead},
	{"POST /v1/collections", contracts.ScopeCollectionsWrite},
	{"PUT /v1/collections", contracts.ScopeCollectionsWrite},
	{"DELETE /v1/collections", contracts.ScopeCollectionsWrite},

	// Shipping
	{"GET /v1/shipping", contracts.ScopeShippingRead},
	{"POST /v1/shipping", contracts.ScopeShippingWrite},
	{"PUT /v1/shipping", contracts.ScopeShippingWrite},
	{"PATCH /v1/shipping", contracts.ScopeShippingWrite},
	{"DELETE /v1/shipping", contracts.ScopeShippingWrite},

	// Fiat
	{"GET /v1/fiat", contracts.ScopeFiatRead},
	{"POST /v1/fiat", contracts.ScopeFiatManage},
	{"PUT /v1/fiat", contracts.ScopeFiatManage},
	{"DELETE /v1/fiat", contracts.ScopeFiatManage},

	// Fulfillment / supply chain
	{"GET /v1/fulfillment", contracts.ScopeFulfillmentRead},
	{"POST /v1/fulfillment", contracts.ScopeFulfillmentManage},
	{"DELETE /v1/fulfillment", contracts.ScopeFulfillmentManage},

	// AI
	{"POST /v1/ai", contracts.ScopeAIUse},
	{"GET /v1/ai", contracts.ScopeAIUse},
	{"PUT /v1/ai", contracts.ScopeAIUse},
	{"DELETE /v1/ai", contracts.ScopeAIUse},

	// Wishlists
	{"GET /v1/wishlists", contracts.ScopeWishlistsRead},
	{"POST /v1/wishlists", contracts.ScopeWishlistsWrite},
	{"DELETE /v1/wishlists", contracts.ScopeWishlistsWrite},

	// Carts
	{"GET /v1/carts", contracts.ScopeCartsRead},
	{"POST /v1/carts", contracts.ScopeCartsWrite},
	{"PUT /v1/carts", contracts.ScopeCartsWrite},
	{"DELETE /v1/carts", contracts.ScopeCartsWrite},

	// Disputes / cases
	{"GET /v1/disputes", contracts.ScopeDisputesRead},
	{"POST /v1/disputes", contracts.ScopeDisputesManage},
	{"GET /v1/cases", contracts.ScopeDisputesRead},
	{"POST /v1/cases", contracts.ScopeDisputesManage},

	// Exchange rates (commonly needed alongside wallet operations)
	{"GET /v1/exchange-rates", contracts.ScopeWalletRead},

	// Followers / following (associated with profile)
	{"GET /v1/followers", contracts.ScopeProfilesRead},
	{"POST /v1/followers", contracts.ScopeProfilesWrite},
	{"PUT /v1/following", contracts.ScopeProfilesWrite},
	{"GET /v1/following", contracts.ScopeProfilesRead},
	{"DELETE /v1/following", contracts.ScopeProfilesWrite},

	// Posts
	{"GET /v1/posts", contracts.ScopeListingsRead},
	{"POST /v1/posts", contracts.ScopeListingsWrite},
	{"PUT /v1/posts", contracts.ScopeListingsWrite},
	{"DELETE /v1/posts", contracts.ScopeListingsWrite},

	// Payment methods (public read; tokens with wallet scope can call)
	{"GET /v1/payment-methods", contracts.ScopeWalletRead},

	// Matrix chat
	{"GET /v1/chat", contracts.ScopeChatRead},
	{"POST /v1/chat", contracts.ScopeChatWrite},
	{"PUT /v1/chat", contracts.ScopeChatWrite},
	{"DELETE /v1/chat", contracts.ScopeChatWrite},

	// Config (read-only)
	{"GET /v1/config", contracts.ScopeSettingsRead},
	// Feature flags catalogue: pure metadata, any authenticated identity
	// (including low-privilege API tokens) needs to query "what is on?"
	// to render correctly. Marked ScopeAny so the middleware skips the
	// HasScope check while still requiring auth.
	{"GET /v1/features", contracts.ScopeAny},

	// Blocklist
	{"PUT /v1/blocklist", contracts.ScopeProfilesWrite},
	{"DELETE /v1/blocklist", contracts.ScopeProfilesWrite},

	// Crypto utilities (sign / verify / hash with the node key)
	{"POST /v1/crypto", contracts.ScopeWalletManage},

	// Analytics
	{"GET /v1/analytics", contracts.ScopeAnalyticsRead},
	{"POST /v1/analytics", contracts.ScopeAnalyticsRead},

	// Digital assets (seller management shares listings scope)
	{"GET /v1/digital-assets", contracts.ScopeListingsRead},
	{"POST /v1/digital-assets", contracts.ScopeListingsWrite},
	{"PATCH /v1/digital-assets", contracts.ScopeListingsWrite},
	{"PUT /v1/digital-assets", contracts.ScopeListingsWrite},
	{"DELETE /v1/digital-assets", contracts.ScopeListingsWrite},

	// Moderators (profile-related actions)
	{"GET /v1/moderators", contracts.ScopeProfilesRead},
	{"POST /v1/moderators", contracts.ScopeProfilesWrite},
	{"DELETE /v1/moderators", contracts.ScopeProfilesWrite},

	// Auth identity / scopes are global metadata endpoints. Any authenticated
	// identity (admin JWT/Basic OR low-privilege API token) needs to read
	// these to discover "who am I" and "what can I do". We mark them with
	// ScopeAny so the middleware skips the HasScope check while still
	// requiring the route to be authenticated.
	{"GET /v1/auth/identity", contracts.ScopeAny},
	{"GET /v1/auth/scopes", contracts.ScopeAny},
	// /v1/auth/tokens management is intentionally NOT in this map:
	// API tokens cannot manage tokens. Only JWT/Basic admins can.
}

type routeScope struct {
	pattern string
	scope   contracts.Scope
}

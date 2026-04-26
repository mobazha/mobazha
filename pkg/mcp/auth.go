package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

// IdentityResponse matches the GET /platform/v1/auth/identity response envelope.
type IdentityResponse struct {
	Data IdentityData `json:"data"`
}

type IdentityData struct {
	UserID     string   `json:"user_id"`
	PeerID     string   `json:"peer_id"`
	IsAPIToken bool     `json:"is_api_token"`
	Scopes     []string `json:"scopes"`
}

// FetchIdentityFromPath calls the given identity endpoint path to resolve the
// caller's scopes. The path is deployment-specific and MUST be supplied by the
// caller — there is no default. Hosting (SaaS) uses /platform/v1/auth/identity;
// standalone nodes use /v1/auth/identity.
func FetchIdentityFromPath(ctx context.Context, bridge Bridge, identityPath string) (*IdentityData, error) {
	if identityPath == "" {
		return nil, fmt.Errorf("identity path is required")
	}
	code, body, err := bridge.Call(ctx, "GET", identityPath, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch identity: %w", err)
	}
	if code == 401 {
		return nil, fmt.Errorf("authentication failed: check your API token")
	}
	if code != 200 {
		return nil, fmt.Errorf("fetch identity returned HTTP %d: %s", code, truncate(body, 200))
	}

	var resp IdentityResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse identity response: %w", err)
	}
	return &resp.Data, nil
}

// ScopeSet provides fast scope lookup.
type ScopeSet map[string]struct{}

// NewScopeSet builds a ScopeSet from a string slice.
func NewScopeSet(scopes []string) ScopeSet {
	ss := make(ScopeSet, len(scopes))
	for _, s := range scopes {
		ss[s] = struct{}{}
	}
	return ss
}

// Has returns true if the set contains the given scope.
func (ss ScopeSet) Has(scope string) bool {
	_, ok := ss[scope]
	return ok
}

// toolScopeRequirement maps a tool name to its required scope. Empty value
// means the tool is public-readable and requires only authentication.
//
// All values are sourced from contracts.Scope* constants so that any rename
// in the canonical vocabulary is caught by the compiler instead of silently
// drifting (see pkg/contracts/scopes.go).
var toolScopeRequirement = map[string]string{
	// Read tools
	"listings_list_mine":            string(contracts.ScopeListingsRead),
	"listings_get":                  string(contracts.ScopeListingsRead),
	"listings_get_template":         string(contracts.ScopeListingsRead),
	"orders_get_sales":              string(contracts.ScopeOrdersRead),
	"orders_get_purchases":          string(contracts.ScopePurchasesRead),
	"orders_get_detail":             string(contracts.ScopeOrdersRead),
	"orders_get_cases":              string(contracts.ScopeDisputesRead),
	"wallet_get_receiving_accounts": string(contracts.ScopeWalletRead),
	"profile_get":                   string(contracts.ScopeProfilesRead),
	"chat_get_conversations":        string(contracts.ScopeChatRead),
	"chat_get_messages":             string(contracts.ScopeChatRead),
	"notifications_list":            string(contracts.ScopeNotificationsRead),
	"notifications_unread_count":    string(contracts.ScopeNotificationsRead),
	"exchange_rates_get":            "",
	"search_listings":               "",
	"search_profiles":               "",
	"discounts_list":                string(contracts.ScopeDiscountsRead),
	"collections_list":              string(contracts.ScopeCollectionsRead),
	"settings_get_storefront":       string(contracts.ScopeSettingsRead),
	"fiat_get_providers":            string(contracts.ScopeFiatRead),
	"fiat_get_provider_config":      string(contracts.ScopeFiatRead),

	// Write tools
	"listings_import_json":     string(contracts.ScopeListingsWrite),
	"listings_create":          string(contracts.ScopeListingsWrite),
	"listings_update":          string(contracts.ScopeListingsWrite),
	"listings_delete":          string(contracts.ScopeListingsWrite),
	"orders_confirm":           string(contracts.ScopeOrdersManage),
	"orders_decline":           string(contracts.ScopeOrdersManage),
	"orders_fulfill":           string(contracts.ScopeOrdersManage),
	"orders_refund":            string(contracts.ScopeOrdersManage),
	"orders_cancel":            string(contracts.ScopeOrdersManage),
	"orders_complete":          string(contracts.ScopeOrdersManage),
	"chat_send_message":        string(contracts.ScopeChatWrite),
	"profile_update":           string(contracts.ScopeProfilesWrite),
	"notifications_mark_read":  string(contracts.ScopeNotificationsManage),
	"discounts_create":         string(contracts.ScopeDiscountsWrite),
	"discounts_update":         string(contracts.ScopeDiscountsWrite),
	"discounts_delete":         string(contracts.ScopeDiscountsWrite),
	"collections_create":       string(contracts.ScopeCollectionsWrite),
	"collections_add_products": string(contracts.ScopeCollectionsWrite),
}

// FilterToolsByScopes returns tool names that the given scopes permit.
func FilterToolsByScopes(scopes ScopeSet) []string {
	var allowed []string
	for tool, requiredScope := range toolScopeRequirement {
		if requiredScope == "" || scopes.Has(requiredScope) {
			allowed = append(allowed, tool)
		}
	}
	return allowed
}

// FetchPeerIDFromProfile calls GET /v1/profiles to get the current node's peerID.
// Used when the token doesn't include peerID (API tokens resolved from platform).
func FetchPeerIDFromProfile(ctx context.Context, bridge Bridge) (string, error) {
	code, body, err := bridge.Call(ctx, "GET", "/v1/profiles", url.Values{}, nil)
	if err != nil {
		return "", fmt.Errorf("fetch profile: %w", err)
	}
	if code != 200 {
		return "", fmt.Errorf("fetch profile returned HTTP %d", code)
	}

	var resp struct {
		Data struct {
			PeerID string `json:"peerID"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse profile: %w", err)
	}
	return resp.Data.PeerID, nil
}

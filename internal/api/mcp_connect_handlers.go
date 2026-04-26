package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/internal/mcpconnect"
	"github.com/mobazha/mobazha3.0/pkg/apitoken"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

type mcpConnectResponse struct {
	Token   string                     `json:"token,omitempty"`
	Clients []mcpconnect.ConnectResult `json:"clients"`
}

// handlePOSTMCPConnect configures MCP for all detected AI clients.
// Standalone-only: auto-mints a long-lived local API token (mbz_*) and writes
// AI client config files pointing at the node's own /v1/mcp endpoint.
func (g *Gateway) handlePOSTMCPConnect(w http.ResponseWriter, r *http.Request) {
	if err := g.requireLocalTokenStore(w); err != nil {
		return
	}

	opts, rawToken, err := g.buildMCPConnectOpts(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	results := mcpconnect.ConnectAll(opts)
	resp := mcpConnectResponse{
		Token:   rawToken,
		Clients: results,
	}
	response.Success(w, resp)
}

// handlePOSTMCPConnectClient configures MCP for a specific AI client.
func (g *Gateway) handlePOSTMCPConnectClient(w http.ResponseWriter, r *http.Request) {
	if err := g.requireLocalTokenStore(w); err != nil {
		return
	}

	clientName := mux.Vars(r)["client"]
	if clientName == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "client name required")
		return
	}

	opts, rawToken, err := g.buildMCPConnectOpts(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	result, err := mcpconnect.ConnectByName(clientName, opts)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	resp := mcpConnectResponse{
		Token:   rawToken,
		Clients: []mcpconnect.ConnectResult{result},
	}
	response.Success(w, resp)
}

// handleGETMCPClients lists detected AI clients and their configuration status.
func (g *Gateway) handleGETMCPClients(w http.ResponseWriter, r *http.Request) {
	clients := mcpconnect.DetectAll()
	response.Success(w, clients)
}

// handlePOSTMCPDisconnect removes MCP configuration from all clients.
func (g *Gateway) handlePOSTMCPDisconnect(w http.ResponseWriter, r *http.Request) {
	results := mcpconnect.DisconnectAll()
	response.Success(w, results)
}

// handlePOSTMCPDisconnectClient removes MCP configuration from a specific client.
func (g *Gateway) handlePOSTMCPDisconnectClient(w http.ResponseWriter, r *http.Request) {
	clientName := mux.Vars(r)["client"]
	if clientName == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "client name required")
		return
	}

	result, err := mcpconnect.DisconnectByName(clientName)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(w, result)
}

// requireLocalTokenStore blocks execution when no API token store is wired
// into the gateway (e.g. PublicOnly mode, or DataDir + GormDB both unset).
// MCP auto-connect needs to mint a token locally; without a store there is
// nowhere to persist it.
func (g *Gateway) requireLocalTokenStore(w http.ResponseWriter) error {
	if g.getTokenStore() == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"MCP auto-connect requires a local API token store; this node is running without persistent token storage")
		return fmt.Errorf("no token store")
	}
	return nil
}

// buildMCPConnectOpts constructs the connection options. It auto-mints a
// long-lived API token using the local token store (no round-trip to the SaaS
// platform — the standalone node owns its own credentials) and returns the raw
// token string for one-time display to the user.
//
// Architectural note (Fix B): the previous implementation called
// `${SaaSAPIURL}/platform/v1/auth/tokens` over HTTP using the caller's JWT,
// which had two structural problems:
//
//  1. The token returned by SaaS is validated against SaaS's user table, but
//     the standalone node's AuthenticationMiddleware only knows how to verify
//     local mbz_ tokens. The minted token would have been silently rejected
//     on every subsequent /v1/mcp request.
//  2. It coupled standalone "MCP works" to "SaaS reachable", defeating the
//     whole point of running standalone.
//
// We now mint locally, persist via apitoken.Store, and return the same
// mbz_<id>_<secret> shape the manual /v1/auth/tokens flow produces.
func (g *Gateway) buildMCPConnectOpts(r *http.Request) (mcpconnect.ConnectOpts, string, error) {
	gatewayURL := g.resolveGatewayURL()
	mcpURL := gatewayURL + "/v1/mcp"

	var body struct {
		Token string `json:"token"`
		Force bool   `json:"force"`
	}
	if r.Body != nil {
		defer r.Body.Close()
		json.NewDecoder(r.Body).Decode(&body)
	}

	// If an explicit API token was provided in the body, use it directly.
	// This path keeps the door open for "user generated their own token in
	// the Admin UI and wants to wire it up" without re-minting.
	if body.Token != "" {
		binPath, _ := os.Executable()
		return mcpconnect.ConnectOpts{
			MCPURL:        mcpURL,
			Token:         body.Token,
			BridgeBinPath: binPath,
			Force:         body.Force,
		}, "", nil
	}

	// Defense-in-depth: only an admin (JWT / Basic Auth) can auto-mint. We
	// already enforce this at the route level (the /v1/mcp/* handlers are
	// not in routeScopeMap so API tokens hit 403 in ScopeEnforcementMiddleware),
	// but mirroring the explicit check here gives a precise error message
	// and survives any future routeScopeMap regression.
	if id := GetAuthIdentity(r.Context()); id != nil && id.IsAPIToken {
		return mcpconnect.ConnectOpts{}, "", fmt.Errorf(
			"MCP auto-connect must be triggered by an admin session; api tokens cannot mint new tokens")
	}

	apiToken, err := g.mintMCPAPIToken()
	if err != nil {
		return mcpconnect.ConnectOpts{}, "", fmt.Errorf("failed to create MCP API token: %w", err)
	}

	binPath, _ := os.Executable()
	return mcpconnect.ConnectOpts{
		MCPURL:        mcpURL,
		Token:         apiToken,
		BridgeBinPath: binPath,
		Force:         body.Force,
	}, apiToken, nil
}

// mintMCPAPIToken generates a long-lived local API token suitable for MCP
// usage and persists it in the gateway's token store. The scope set is the
// canonical "seller:*" preset (contracts.SellerScopes) — i.e. functionally
// identical to the token a user would obtain via Quick Connect with the
// seller preset in the web UI. We deliberately do NOT use defaultTokenScopes
// here because that helper is the manual "create token" form's fallback and
// trims optional scopes (wallet/chat/notifications/discounts/collections/
// shipping/fiat/analytics) that the AI agent typically needs.
func (g *Gateway) mintMCPAPIToken() (string, error) {
	store := g.getTokenStore()
	if store == nil {
		return "", fmt.Errorf("token store unavailable")
	}

	count, err := store.CountActive()
	if err != nil {
		return "", fmt.Errorf("counting tokens: %w", err)
	}
	if count >= int64(apitoken.MaxPerUser) {
		return "", fmt.Errorf("maximum number of API tokens reached (%d); revoke an existing token before re-running auto-connect", apitoken.MaxPerUser)
	}

	scopes := contracts.ScopeStrings(contracts.SellerScopes())
	rawToken, record, err := apitoken.Generate("mcp-auto-connect", scopes, nil)
	if err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}
	if err := store.Create(record); err != nil {
		return "", fmt.Errorf("persisting token: %w", err)
	}
	return rawToken, nil
}

// resolveGatewayURL returns the base URL for the node's HTTP gateway, written
// into the AI client config (Cursor / Claude Desktop / etc.) by the
// auto-connect flow.
//
// The listener address can be a wildcard host (":15104", "0.0.0.0:15104",
// "[::]:15104") which is fine for the server itself but not routable from a
// client. We normalize it to a loopback host with the same helper used for the
// in-process MCP bridge (gateway.go::normalizeLoopbackAddr) so the address
// written to the client config is always something the local AI client can
// actually reach. Production deployments that want a public URL should
// configure that explicitly upstream — this function only guarantees a
// reachable default.
func (g *Gateway) resolveGatewayURL() string {
	addr := normalizeLoopbackAddr(g.listener.Addr().String())
	scheme := "http"
	if g.config.UseSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, addr)
}

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/internal/mcpconnect"
	"github.com/mobazha/mobazha3.0/pkg/apitoken"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// readAndRestoreBody reads the entire request body into a buffer and replaces
// r.Body with a fresh reader so subsequent decoders see the same payload.
// Bounded by a 1 MiB cap to defend against accidental large bodies (the MCP
// connect flow only ever carries a token + force flag, well under 1 KiB).
func readAndRestoreBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	const maxBody = 1 << 20
	buf, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	return buf, nil
}

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

	// Hard pre-flight: containerized nodes cannot reach the host's filesystem
	// to write AI client configs (~/.cursor/mcp.json etc.), so auto-connect
	// is structurally impossible — not just "no clients found right now".
	// Refuse even with force=true; the user must use the manual Quick Connect
	// flow (copy token + paste config) instead.
	if isContainerized() {
		response.Error(w, http.StatusPreconditionFailed, response.CodeBadRequest,
			"this node is running in a container and cannot write to the host's AI client config files; use Quick Connect to copy the token manually")
		return
	}

	// Soft pre-flight: avoid burning a token slot when no AI clients are
	// installed. We peek at the request body to honour the `force` override
	// before buildMCPConnectOpts consumes it. Body is small; we re-use the
	// parsed shape inside buildMCPConnectOpts via a closure-friendly path is
	// not trivial here, so we accept one extra Detect call (cheap: filesystem
	// stats on a handful of paths).
	if !readForceFlag(r) {
		any := false
		for _, c := range mcpconnect.DetectAll() {
			if c.Installed {
				any = true
				break
			}
		}
		if !any {
			response.Error(w, http.StatusPreconditionFailed, response.CodeBadRequest,
				"no supported AI clients were detected on this host; install Cursor / Claude Desktop / etc., or pass force=true to override")
			return
		}
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

// readForceFlag peeks at the optional `force` boolean in the JSON body without
// consuming it for downstream readers. We wrap the body in a tee-style buffer
// so buildMCPConnectOpts can re-decode the same payload.
func readForceFlag(r *http.Request) bool {
	if r.Body == nil {
		return false
	}
	// We can't read-then-rewind http.Request.Body without buffering. Buffer
	// the (small) JSON body once and replace r.Body with a fresh reader so
	// buildMCPConnectOpts still sees the original payload.
	buf, err := readAndRestoreBody(r)
	if err != nil {
		return false
	}
	var body struct {
		Force bool `json:"force"`
	}
	_ = json.Unmarshal(buf, &body)
	return body.Force
}

// handlePOSTMCPConnectClient configures MCP for a specific AI client.
func (g *Gateway) handlePOSTMCPConnectClient(w http.ResponseWriter, r *http.Request) {
	if err := g.requireLocalTokenStore(w); err != nil {
		return
	}

	clientName := chi.URLParam(r, "client")
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

// mcpCapabilityResponse summarises whether MCP auto-connect is viable on this
// host before the user clicks "Connect All". It is read-only and never mints
// tokens or writes config files.
type mcpCapabilityResponse struct {
	// Supported is true when auto-connect is likely to succeed: a token store
	// exists, at least one slot is free, and at least one AI client is installed
	// in a detectable location on this machine.
	Supported bool `json:"supported"`
	// Reason is a stable machine-readable code explaining the most blocking
	// issue when Supported is false: "ok" / "containerized" /
	// "no_token_store" / "token_slots_exhausted" / "no_clients".
	Reason string `json:"reason"`
	// Containerized indicates the node is running inside a Docker container,
	// where filesystem writes for AI client configs cannot reach the host.
	// When true, Supported is forced to false and DetectedClients is empty
	// (scanning the container's filesystem yields meaningless results — the
	// host's Cursor / Claude install is invisible from in here).
	Containerized bool `json:"containerized"`
	// HasTokenStore is true when the gateway can persist API tokens. Without
	// it, auto-connect cannot mint a credential.
	HasTokenStore bool `json:"hasTokenStore"`
	// TokenSlotsLeft is the remaining quota under apitoken.MaxPerUser.
	TokenSlotsLeft int `json:"tokenSlotsLeft"`
	// DetectedClients is the full client matrix (installed + configured),
	// suitable for UI rendering. Frontend filters by installed=true to
	// decide what to show in the "we found these" preview.
	DetectedClients []mcpconnect.ClientStatus `json:"detectedClients"`
}

// handleGETMCPCapability is a read-only probe: tells the frontend whether
// auto-connect can plausibly succeed (token store available, slots left, any
// client installed) without minting a token or writing files. The Admin UI
// uses this to decide between showing the "Connect All" button or a hint
// like "no AI clients detected on this host".
//
// Why we need this: the previous flow happily minted a token even when zero
// clients were detected, burning one of the 20 token slots for nothing.
// Worse, when the standalone node runs inside Docker, mcpconnect.DetectAll
// scans the container's filesystem (which has no AI client configs) and
// auto-connect produces a "100% success" response that wrote nothing the
// host can use. Surfacing both signals up-front avoids both pitfalls.
func (g *Gateway) handleGETMCPCapability(w http.ResponseWriter, r *http.Request) {
	resp := mcpCapabilityResponse{
		Containerized:   isContainerized(),
		DetectedClients: []mcpconnect.ClientStatus{},
	}

	// Token store status is reported in both modes — even when containerized,
	// the user still needs to know if manual token creation will work.
	store := g.getTokenStore()
	resp.HasTokenStore = store != nil
	if store != nil {
		count, err := store.CountActive()
		if err != nil {
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
				fmt.Sprintf("counting tokens: %v", err))
			return
		}
		left := apitoken.MaxPerUser - int(count)
		if left < 0 {
			left = 0
		}
		resp.TokenSlotsLeft = left
	}

	// Containerized short-circuit: scanning the container's filesystem for
	// AI client configs is meaningless — the host's Cursor / Claude install
	// is invisible from in here, and even if we found something to write,
	// the host wouldn't see it. Stop here with a stable reason code so the
	// frontend can render a manual-only path.
	if resp.Containerized {
		resp.Reason = "containerized"
		response.Success(w, resp)
		return
	}

	resp.DetectedClients = mcpconnect.DetectAll()
	installedCount := 0
	for _, c := range resp.DetectedClients {
		if c.Installed {
			installedCount++
		}
	}

	switch {
	case !resp.HasTokenStore:
		resp.Reason = "no_token_store"
	case resp.TokenSlotsLeft <= 0:
		resp.Reason = "token_slots_exhausted"
	case installedCount == 0:
		resp.Reason = "no_clients"
	default:
		resp.Reason = "ok"
		resp.Supported = true
	}

	response.Success(w, resp)
}

// isContainerized reports whether this process is likely running inside a
// Docker container. We check `/.dockerenv` (created by `docker run`) which
// is the lowest-effort, false-positive-free signal. Other heuristics
// (cgroup parsing, kubernetes env) add complexity for marginal value at
// this stage.
func isContainerized() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// handlePOSTMCPDisconnect removes MCP configuration from all clients.
func (g *Gateway) handlePOSTMCPDisconnect(w http.ResponseWriter, r *http.Request) {
	results := mcpconnect.DisconnectAll()
	response.Success(w, results)
}

// handlePOSTMCPDisconnectClient removes MCP configuration from a specific client.
func (g *Gateway) handlePOSTMCPDisconnectClient(w http.ResponseWriter, r *http.Request) {
	clientName := chi.URLParam(r, "client")
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
			return mcpconnect.ConnectOpts{}, "", fmt.Errorf("invalid request body: %w", err)
		}
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

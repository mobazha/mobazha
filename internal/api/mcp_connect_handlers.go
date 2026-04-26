package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/internal/mcpconnect"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

type mcpConnectResponse struct {
	Token   string                     `json:"token,omitempty"`
	Clients []mcpconnect.ConnectResult `json:"clients"`
}

// handlePOSTMCPConnect configures MCP for all detected AI clients.
// Standalone-only: auto-mints a long-lived API token via SaaS platform.
func (g *Gateway) handlePOSTMCPConnect(w http.ResponseWriter, r *http.Request) {
	if err := g.requireStandaloneWithSaaS(w); err != nil {
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
	if err := g.requireStandaloneWithSaaS(w); err != nil {
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

// requireStandaloneWithSaaS blocks execution when not running as a standalone
// node connected to a SaaS platform. Writes the error response and returns
// a non-nil error to signal the caller should return immediately.
func (g *Gateway) requireStandaloneWithSaaS(w http.ResponseWriter) error {
	if g.config.SaaSAPIURL == "" {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"MCP auto-connect is only available on standalone nodes linked to the platform")
		return fmt.Errorf("not standalone")
	}
	return nil
}

// buildMCPConnectOpts constructs the connection options. It auto-mints a
// long-lived API token via the SaaS platform using the caller's JWT, so
// that AI clients get a stable credential instead of a short-lived JWT.
// Returns the raw API token string for one-time display to the user.
func (g *Gateway) buildMCPConnectOpts(r *http.Request) (mcpconnect.ConnectOpts, string, error) {
	gatewayURL := g.resolveGatewayURL()
	mcpURL := gatewayURL + "/platform/v1/mcp"

	var body struct {
		Token string `json:"token"`
		Force bool   `json:"force"`
	}
	if r.Body != nil {
		defer r.Body.Close()
		json.NewDecoder(r.Body).Decode(&body)
	}

	// If an explicit API token was provided in the body, use it directly.
	if body.Token != "" {
		binPath, _ := os.Executable()
		return mcpconnect.ConnectOpts{
			MCPURL:        mcpURL,
			Token:         body.Token,
			BridgeBinPath: binPath,
			Force:         body.Force,
		}, "", nil
	}

	// Otherwise, auto-mint a long-lived API token via SaaS platform.
	jwt := extractBearerToken(r)
	if jwt == "" {
		return mcpconnect.ConnectOpts{}, "", fmt.Errorf(
			"JWT Bearer token required for auto-connect; use the Admin UI or pass an explicit token in the request body")
	}

	apiToken, err := g.mintMCPAPIToken(jwt)
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

// mintMCPAPIToken calls the SaaS platform's token creation API to produce a
// long-lived API token with seller scopes suitable for MCP usage.
func (g *Gateway) mintMCPAPIToken(jwt string) (string, error) {
	payload, _ := json.Marshal(map[string]interface{}{
		"name":   "mcp-auto-connect",
		"scopes": []string{"listings:read", "listings:write", "orders:read", "orders:manage", "profiles:read", "profiles:write", "media:read", "media:write", "settings:read", "ai:use"},
	})

	url := g.config.SaaSAPIURL + "/platform/v1/auth/tokens"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	if g.config.StandaloneAPIKey != "" {
		req.Header.Set("X-Standalone-Store-Key", g.config.StandaloneAPIKey)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling platform: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("platform returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}
	if result.Data.Token == "" {
		return "", fmt.Errorf("platform returned empty token")
	}

	return result.Data.Token, nil
}

// resolveGatewayURL returns the base URL for the node's HTTP gateway.
func (g *Gateway) resolveGatewayURL() string {
	addr := g.listener.Addr().String()
	scheme := "http"
	if g.config.UseSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, addr)
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

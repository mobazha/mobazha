// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package cdp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Authenticator signs one CDP API request. CDP requires a short-lived JWT
// minted from the developer API key on every call.
//
// TODO(sandbox): implement the JWT authenticator once dev keys exist — CDP
// issues Ed25519 (v2) and EC/ES256 (legacy) API keys with different signing
// flows, and picking blind risks a subtly wrong implementation. Until then
// the provider registers only when credentials are configured, and every
// call fails with the authenticator's error: fail-closed, never fail-wrong.
// https://docs.cdp.coinbase.com/api-reference/rest-api/onramp-offramp/create-session-token
type Authenticator interface {
	Authorize(req *http.Request) error
}

// HTTPClient is the production Client over the CDP Onramp REST API.
type HTTPClient struct {
	baseURL    string
	auth       Authenticator
	httpClient *http.Client
}

// NewHTTPClient builds the production client. baseURL defaults to the
// production API host.
func NewHTTPClient(auth Authenticator, baseURL string) *HTTPClient {
	if baseURL == "" {
		baseURL = "https://api.developer.coinbase.com"
	}
	return &HTTPClient{
		baseURL:    baseURL,
		auth:       auth,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// sessionTokenBody mirrors the Session Token API request shape.
type sessionTokenBody struct {
	Addresses []sessionTokenAddress `json:"addresses"`
	Assets    []string              `json:"assets,omitempty"`
	ClientIP  string                `json:"clientIp"`
}

type sessionTokenAddress struct {
	Address     string   `json:"address"`
	Blockchains []string `json:"blockchains"`
}

type sessionTokenResponse struct {
	Token string `json:"token"`
}

// CreateSessionToken implements Client.
func (c *HTTPClient) CreateSessionToken(ctx context.Context, req SessionTokenRequest) (string, error) {
	body, err := json.Marshal(sessionTokenBody{
		Addresses: []sessionTokenAddress{{Address: req.Address, Blockchains: req.Networks}},
		Assets:    req.Assets,
		ClientIP:  req.ClientIP,
	})
	if err != nil {
		return "", fmt.Errorf("cdp: encode session token request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/onramp/v1/token", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("cdp: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if err := c.auth.Authorize(httpReq); err != nil {
		return "", fmt.Errorf("cdp: authorize: %w", err)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("cdp: create session token: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("cdp: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cdp: create session token: status %d: %s", resp.StatusCode, truncate(payload))
	}
	var out sessionTokenResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		return "", fmt.Errorf("cdp: decode session token: %w", err)
	}
	if out.Token == "" {
		return "", fmt.Errorf("cdp: session token response carried no token")
	}
	return out.Token, nil
}

type transactionsResponse struct {
	Transactions []Transaction `json:"transactions"`
}

// BuyTransactionsByPartnerUser implements Client.
func (c *HTTPClient) BuyTransactionsByPartnerUser(ctx context.Context, partnerUserID string) ([]Transaction, error) {
	endpoint := c.baseURL + "/onramp/v1/buy/user/" + url.PathEscape(partnerUserID) + "/transactions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("cdp: build request: %w", err)
	}
	if err := c.auth.Authorize(httpReq); err != nil {
		return nil, fmt.Errorf("cdp: authorize: %w", err)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("cdp: buy transactions: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("cdp: read response: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // no purchase completed for this partnerUserId yet
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cdp: buy transactions: status %d: %s", resp.StatusCode, truncate(payload))
	}
	var out transactionsResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("cdp: decode transactions: %w", err)
	}
	return out.Transactions, nil
}

func truncate(b []byte) string {
	const max = 256
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}

var _ Client = (*HTTPClient)(nil)

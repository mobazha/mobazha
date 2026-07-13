// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Package privy is the Privy embedded-wallet provider module (RFC-0012). The
// client here speaks Privy's REST API; the provider in provider.go maps it onto
// the contracts.EmbeddedWalletProvider contract and enforces RFC-0012's custody
// and signing-surface rules.
package privy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is Privy's production REST base. Override in Config for tests.
const DefaultBaseURL = "https://api.privy.io/v1"

// Client is a minimal Privy REST client. It authenticates every call with the
// app id + secret (HTTP Basic plus the privy-app-id header) exactly as Privy
// requires; credentials are never logged.
type Client struct {
	appID      string
	appSecret  string
	baseURL    string
	httpClient *http.Client
}

// NewClient builds a Privy REST client. appID/appSecret authenticate the
// application to Privy; baseURL and httpClient are optional (sensible defaults
// are used when empty/nil).
func NewClient(appID, appSecret, baseURL string, httpClient *http.Client) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		appID:      appID,
		appSecret:  appSecret,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

func (c *Client) authHeaders(req *http.Request) {
	basic := base64.StdEncoding.EncodeToString([]byte(c.appID + ":" + c.appSecret))
	req.Header.Set("Authorization", "Basic "+basic)
	req.Header.Set("privy-app-id", c.appID)
	req.Header.Set("Content-Type", "application/json")
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("privy: encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("privy: build request: %w", err)
	}
	c.authHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("privy: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("privy: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The response body may echo request fields but never the app secret;
		// still, keep it bounded and untrusted.
		return fmt.Errorf("privy: %s %s returned %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	if out != nil {
		if err := json.Unmarshal(payload, out); err != nil {
			return fmt.Errorf("privy: decode response: %w", err)
		}
	}
	return nil
}

// walletResponse is the subset of Privy's wallet object we consume.
type walletResponse struct {
	ID      string `json:"id"`
	Address string `json:"address"`
}

// CreateServerWallet provisions an application-owned ("server") wallet. This is
// the Phase 0 reproduction vehicle: an app-custodied key used only to prove
// on-chain signature acceptance. It is NOT the production buyer-custody path
// (RFC-0012 Proposal 2 forbids a standing platform signer for real funds).
func (c *Client) CreateServerWallet(ctx context.Context, chainType string) (id, address string, err error) {
	if strings.TrimSpace(chainType) == "" {
		chainType = "ethereum"
	}
	var out walletResponse
	if err := c.do(ctx, http.MethodPost, "/wallets", map[string]string{"chain_type": chainType}, &out); err != nil {
		return "", "", err
	}
	if out.ID == "" || out.Address == "" {
		return "", "", fmt.Errorf("privy: create wallet returned empty id/address")
	}
	return out.ID, out.Address, nil
}

// privyTypedData mirrors Privy's expected typed_data shape. Privy uses
// snake_case primary_type where the EIP-712 JSON standard uses primaryType.
type privyTypedData struct {
	Types       json.RawMessage `json:"types"`
	Domain      json.RawMessage `json:"domain"`
	Message     json.RawMessage `json:"message"`
	PrimaryType string          `json:"primary_type"`
}

// standardTypedData is the standard EIP-712 typed-data JSON shape used across
// the contract (matching eth_signTypedData_v4 input).
type standardTypedData struct {
	Types       json.RawMessage `json:"types"`
	Domain      json.RawMessage `json:"domain"`
	Message     json.RawMessage `json:"message"`
	PrimaryType string          `json:"primaryType"`
}

type rpcResponse struct {
	Data struct {
		Signature string `json:"signature"`
	} `json:"data"`
}

// SignTypedDataV4WithServerWallet signs standard EIP-712 typed-data JSON with an
// app-owned wallet via Privy's rpc endpoint, returning the hex signature. Used
// only by the gated server-wallet fixture path.
func (c *Client) SignTypedDataV4WithServerWallet(ctx context.Context, walletID string, standardEIP712 []byte) (string, error) {
	var std standardTypedData
	if err := json.Unmarshal(standardEIP712, &std); err != nil {
		return "", fmt.Errorf("privy: typed-data payload is not standard EIP-712 JSON: %w", err)
	}
	body := map[string]any{
		"method": "eth_signTypedData_v4",
		"params": map[string]any{
			"typed_data": privyTypedData{
				Types:       std.Types,
				Domain:      std.Domain,
				Message:     std.Message,
				PrimaryType: std.PrimaryType,
			},
		},
	}
	var out rpcResponse
	if err := c.do(ctx, http.MethodPost, "/wallets/"+walletID+"/rpc", body, &out); err != nil {
		return "", err
	}
	if out.Data.Signature == "" {
		return "", fmt.Errorf("privy: rpc returned empty signature")
	}
	return out.Data.Signature, nil
}

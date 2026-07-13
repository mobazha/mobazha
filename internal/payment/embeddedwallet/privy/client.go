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
	return c.doAs(ctx, method, path, "", body, out)
}

// doAs is do with an optional buyer access token. When buyerAccessToken is
// non-empty it is carried as a bearer credential in addition to the app auth,
// so Privy scopes the call to the buyer's own authority (user-authorized
// signing). The app Basic auth still identifies the calling application.
func (c *Client) doAs(ctx context.Context, method, path, buyerAccessToken string, body any, out any) error {
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
	if strings.TrimSpace(buyerAccessToken) != "" {
		req.Header.Set("privy-access-token", buyerAccessToken)
	}

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

// signTypedDataV4Body builds the shared rpc body for eth_signTypedData_v4,
// translating standard EIP-712 JSON into Privy's snake_case typed_data shape.
func signTypedDataV4Body(standardEIP712 []byte) (map[string]any, error) {
	var std standardTypedData
	if err := json.Unmarshal(standardEIP712, &std); err != nil {
		return nil, fmt.Errorf("privy: typed-data payload is not standard EIP-712 JSON: %w", err)
	}
	return map[string]any{
		"method": "eth_signTypedData_v4",
		"params": map[string]any{
			"typed_data": privyTypedData{
				Types:       std.Types,
				Domain:      std.Domain,
				Message:     std.Message,
				PrimaryType: std.PrimaryType,
			},
		},
	}, nil
}

// SignTypedDataV4WithServerWallet signs standard EIP-712 typed-data JSON with an
// app-owned wallet via Privy's rpc endpoint, returning the hex signature. Used
// only by the gated server-wallet fixture path.
func (c *Client) SignTypedDataV4WithServerWallet(ctx context.Context, walletID string, standardEIP712 []byte) (string, error) {
	body, err := signTypedDataV4Body(standardEIP712)
	if err != nil {
		return "", err
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

// SignTypedDataV4WithUserWallet signs standard EIP-712 typed-data JSON with a
// buyer-owned embedded wallet, authorized by the buyer's Privy access token.
// Unlike the server-wallet path (app authority), this carries the buyer's token
// so Privy produces the signature only on the buyer's own authority — RFC-0012
// Proposal 2 forbids any platform-authority-only signing of real buyer funds.
//
// The user-authorization transport (access token as bearer) and rpc path are
// confirmed against a live dev app by the env-gated identity test, not asserted
// offline.
func (c *Client) SignTypedDataV4WithUserWallet(ctx context.Context, walletID, buyerAccessToken string, standardEIP712 []byte) (string, error) {
	body, err := signTypedDataV4Body(standardEIP712)
	if err != nil {
		return "", err
	}
	var out rpcResponse
	if err := c.doAs(ctx, http.MethodPost, "/wallets/"+walletID+"/rpc", buyerAccessToken, body, &out); err != nil {
		return "", err
	}
	if out.Data.Signature == "" {
		return "", fmt.Errorf("privy: rpc returned empty signature")
	}
	return out.Data.Signature, nil
}

// linkedAccount is the subset of a Privy user's linked_accounts we read. A user
// may link a custom_auth account (its custom_user_id is the Casdoor sub) and one
// or more wallets (embedded wallets have wallet_client_type "privy").
type linkedAccount struct {
	Type             string `json:"type"`
	CustomUserID     string `json:"custom_user_id"`
	ID               string `json:"id"`
	Address          string `json:"address"`
	ChainType        string `json:"chain_type"`
	WalletClientType string `json:"wallet_client_type"`
}

// userResponse is the subset of Privy's user object we consume.
type userResponse struct {
	ID             string          `json:"id"`
	LinkedAccounts []linkedAccount `json:"linked_accounts"`
}

// toPrivyUser projects a Privy user object onto the identity-link view: its
// custom-auth subject and its embedded wallets.
func (u userResponse) toPrivyUser() *PrivyUser {
	out := &PrivyUser{DID: u.ID}
	for _, a := range u.LinkedAccounts {
		switch a.Type {
		case "custom_auth":
			if out.CustomAuthSubject == "" {
				out.CustomAuthSubject = a.CustomUserID
			}
		case "wallet":
			// Only embedded (privy-client) wallets are buyer-owned keys we admit;
			// an externally-linked MetaMask account is not provisioned by us.
			if a.WalletClientType == "privy" && a.Address != "" {
				out.Wallets = append(out.Wallets, PrivyWallet{ID: a.ID, Address: a.Address, ChainType: a.ChainType})
			}
		}
	}
	return out
}

// restUserDirectory reads Privy user records over the REST API. Its endpoint
// and field specifics are confirmed against a live dev app by the env-gated
// identity test; the security logic that consumes it (identity.go) is proven
// offline against a fake directory.
type restUserDirectory struct{ client *Client }

// UserByDID resolves a user by DID via GET /v1/users/{did}.
func (d restUserDirectory) UserByDID(ctx context.Context, did string) (*PrivyUser, error) {
	if strings.TrimSpace(did) == "" {
		return nil, fmt.Errorf("privy: user lookup requires a did")
	}
	var out userResponse
	if err := d.client.do(ctx, http.MethodGet, "/users/"+did, nil, &out); err != nil {
		return nil, err
	}
	if out.ID == "" {
		return nil, nil
	}
	return out.toPrivyUser(), nil
}

// UserByCustomAuthSubject resolves the user whose linked custom-auth account has
// custom_user_id == subject, via Privy's user-search endpoint.
func (d restUserDirectory) UserByCustomAuthSubject(ctx context.Context, subject string) (*PrivyUser, error) {
	if strings.TrimSpace(subject) == "" {
		return nil, fmt.Errorf("privy: user lookup requires a subject")
	}
	// Privy user search returns the matching user by custom auth id. The request
	// shape is confirmed by the live identity test.
	reqBody := map[string]string{"custom_user_id": subject}
	var out struct {
		Data []userResponse `json:"data"`
	}
	if err := d.client.do(ctx, http.MethodPost, "/users/search", reqBody, &out); err != nil {
		return nil, err
	}
	for _, u := range out.Data {
		user := u.toPrivyUser()
		if user.CustomAuthSubject == subject {
			return user, nil
		}
	}
	return nil, nil
}

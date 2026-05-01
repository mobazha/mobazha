package cj

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://developers.cjdropshipping.com/api2.0/v1"

	// CJ enforces a strict 1 QPS (1 request per second) rate limit.
	defaultQPS        = 1
	rateLimitInterval = time.Second
)

// Client is the HTTP client for CJ Dropshipping API v2.0.
// It manages OAuth-style token refresh and enforces 1 QPS rate limiting.
type Client struct {
	baseURL    string
	httpClient *http.Client

	mu          sync.Mutex
	accessToken string
	// apiKey is the CJ API key used to obtain/refresh access tokens.
	apiKey string

	lastRequest time.Time
	qps         int
}

// ClientOption configures the CJ client.
type ClientOption func(*Client)

func WithBaseURL(url string) ClientOption {
	return func(c *Client) { c.baseURL = url }
}

func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

func WithQPS(qps int) ClientOption {
	return func(c *Client) { c.qps = qps }
}

// NewClient creates a CJ API client. The apiKey is used to obtain access tokens.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    defaultBaseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		qps:        defaultQPS,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SetAccessToken sets a pre-obtained access token (for testing or pre-auth).
func (c *Client) SetAccessToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = token
}

// APIError represents a non-2xx response from CJ API.
type APIError struct {
	StatusCode int
	Code       int    `json:"code"`
	Message    string `json:"message"`
	Result     bool   `json:"result"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("cj: HTTP %d (code %d): %s", e.StatusCode, e.Code, e.Message)
}

func (e *APIError) IsRetryable() bool {
	return e.StatusCode >= 500 || e.StatusCode == 429
}

// AuthError indicates invalid or expired credentials.
type AuthError struct{ Message string }

func (e *AuthError) Error() string { return "cj: auth error: " + e.Message }

// RateLimitError indicates 1 QPS exceeded.
type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("cj: rate limited, retry after %s", e.RetryAfter)
}

// throttle enforces the 1 QPS limit by sleeping if needed.
func (c *Client) throttle(ctx context.Context) error {
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	interval := rateLimitInterval / time.Duration(c.qps)
	if elapsed < interval {
		wait := interval - elapsed
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		c.mu.Lock()
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()
	return nil
}

// ObtainAccessToken exchanges the API key for an access token.
func (c *Client) ObtainAccessToken(ctx context.Context) error {
	if err := c.throttle(ctx); err != nil {
		return err
	}

	url := c.baseURL + "/authentication/getAccessToken"
	body, _ := json.Marshal(map[string]string{"email": c.apiKey})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cj: create auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CJ-Access-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cj: auth request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("cj: read auth response: %w", err)
	}

	var result struct {
		Result  bool   `json:"result"`
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			AccessToken        string `json:"accessToken"`
			AccessTokenExpiryDate string `json:"accessTokenExpiryDate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("cj: decode auth response: %w", err)
	}
	if !result.Result || result.Data.AccessToken == "" {
		return &AuthError{Message: result.Message}
	}

	c.mu.Lock()
	c.accessToken = result.Data.AccessToken
	c.mu.Unlock()
	return nil
}

// do executes an HTTP request with rate limiting and auth header. When the
// upstream returns a 401 (or business-level auth-error envelope), the client
// will transparently refresh its access token using the API key and retry the
// original request once. This lets long-running nodes survive CJ's token
// expiry without manual re-init.
func (c *Client) do(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	return c.doWithRetry(ctx, method, path, body, out, true)
}

func (c *Client) doWithRetry(ctx context.Context, method, path string, body interface{}, out interface{}, allowRefresh bool) error {
	if err := c.throttle(ctx); err != nil {
		return err
	}

	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("cj: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("cj: create request: %w", err)
	}

	c.mu.Lock()
	token := c.accessToken
	c.mu.Unlock()

	req.Header.Set("CJ-Access-Token", token)
	req.Header.Set("User-Agent", "Mobazha/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cj: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("cj: read response: %w", err)
	}

	if resp.StatusCode == 401 {
		if allowRefresh {
			if refreshErr := c.refreshIfPossible(ctx, token); refreshErr == nil {
				return c.doWithRetry(ctx, method, path, body, out, false)
			}
		}
		return &AuthError{Message: string(respBody)}
	}
	if resp.StatusCode == 429 {
		return &RateLimitError{RetryAfter: 2 * time.Second}
	}
	if resp.StatusCode >= 400 {
		return &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}

	// CJ wraps all responses in { result, code, message, data }.
	// Parse the envelope and check for business-level errors.
	var envelope struct {
		Result  bool            `json:"result"`
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("cj: decode envelope: %w", err)
	}

	if !envelope.Result {
		// 1600100/1600101 are CJ's "auth invalid / expired token" codes.
		if envelope.Code == 1600100 || envelope.Code == 1600101 {
			if allowRefresh {
				if refreshErr := c.refreshIfPossible(ctx, token); refreshErr == nil {
					return c.doWithRetry(ctx, method, path, body, out, false)
				}
			}
			return &AuthError{Message: envelope.Message}
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Code:       envelope.Code,
			Message:    envelope.Message,
		}
	}

	if out != nil && len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("cj: decode data: %w", err)
		}
	}
	return nil
}

// refreshIfPossible re-obtains an access token, but only when the API key is
// available and the token we tried with is still the current one (i.e. another
// goroutine has not already refreshed it for us).
func (c *Client) refreshIfPossible(ctx context.Context, attemptedToken string) error {
	if c.apiKey == "" {
		return fmt.Errorf("cj: no api key configured for refresh")
	}
	c.mu.Lock()
	if c.accessToken != attemptedToken {
		// Another caller already refreshed the token; no-op.
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	return c.ObtainAccessToken(ctx)
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// Post performs a POST request.
func (c *Client) Post(ctx context.Context, path string, body, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

// Patch performs a PATCH request.
func (c *Client) Patch(ctx context.Context, path string, body, out interface{}) error {
	return c.do(ctx, http.MethodPatch, path, body, out)
}

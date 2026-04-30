package printful

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL    = "https://api.printful.com"
	defaultRateLimit  = 120         // requests per minute
	rateLimitInterval = time.Minute // sliding window
)

// Client is a rate-limited HTTP client for the Printful API.
type Client struct {
	baseURL    string
	token      string
	storeID    string
	httpClient *http.Client

	mu        sync.Mutex
	tokens    int
	lastReset time.Time
	rateLimit int
}

// ClientOption configures a Printful Client.
type ClientOption func(*Client)

// WithBaseURL overrides the API base URL (for testing).
func WithBaseURL(url string) ClientOption {
	return func(c *Client) { c.baseURL = strings.TrimRight(url, "/") }
}

// WithHTTPClient injects a custom *http.Client (for testing).
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// WithRateLimit overrides the per-minute request cap.
func WithRateLimit(rpm int) ClientOption {
	return func(c *Client) { c.rateLimit = rpm }
}

// WithStoreID sets the Printful store ID for store-scoped API calls.
func WithStoreID(id string) ClientOption {
	return func(c *Client) { c.storeID = id }
}

// NewClient creates a Printful API client with token-bucket rate limiting.
func NewClient(token string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    defaultBaseURL,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		rateLimit:  defaultRateLimit,
		lastReset:  time.Now(),
	}
	c.tokens = c.rateLimit
	for _, o := range opts {
		o(c)
	}
	return c
}

// apiResponse is the Printful envelope: {"code": 200, "result": ...}
type apiResponse struct {
	Code   int             `json:"code"`
	Result json.RawMessage `json:"result"`
	Error  *apiError       `json:"error,omitempty"`
}

type apiError struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// Get performs a rate-limited GET request.
func (c *Client) Get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// Post performs a rate-limited POST request with a JSON body.
func (c *Client) Post(ctx context.Context, path string, body interface{}, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

// Delete performs a rate-limited DELETE request.
func (c *Client) Delete(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodDelete, path, nil, out)
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	if err := c.waitForToken(ctx); err != nil {
		return err
	}

	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("printful: marshal body: %w", err)
		}
		bodyReader = strings.NewReader(string(b))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("printful: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if c.storeID != "" {
		req.Header.Set("X-PF-Store-Id", c.storeID)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("printful: request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("printful: read response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return &RateLimitError{RetryAfter: resp.Header.Get("Retry-After")}
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return &AuthError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var envelope apiResponse
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		if resp.StatusCode >= 400 {
			return &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
		}
		return fmt.Errorf("printful: decode response: %w", err)
	}

	if envelope.Code >= 400 || resp.StatusCode >= 400 {
		msg := ""
		if envelope.Error != nil {
			msg = envelope.Error.Message
		}
		return &APIError{StatusCode: envelope.Code, Message: msg, Body: string(respBody)}
	}

	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return fmt.Errorf("printful: decode result: %w", err)
		}
	}
	return nil
}

// waitForToken blocks until a rate-limit token is available.
func (c *Client) waitForToken(ctx context.Context) error {
	for {
		c.mu.Lock()
		elapsed := time.Since(c.lastReset)
		if elapsed >= rateLimitInterval {
			c.tokens = c.rateLimit
			c.lastReset = time.Now()
		}
		if c.tokens > 0 {
			c.tokens--
			c.mu.Unlock()
			return nil
		}
		wait := rateLimitInterval - elapsed
		c.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

// DiscoverStoreID calls GET /stores and sets the first store's ID.
// Printful API tokens are typically scoped to a single store.
func (c *Client) DiscoverStoreID(ctx context.Context) error {
	if c.storeID != "" {
		return nil
	}
	var stores []struct {
		ID int `json:"id"`
	}
	if err := c.Get(ctx, "/stores", &stores); err != nil {
		return fmt.Errorf("discover store ID: %w", err)
	}
	if len(stores) == 0 {
		return fmt.Errorf("no stores found for this API token")
	}
	c.storeID = fmt.Sprintf("%d", stores[0].ID)
	return nil
}

// Error types for callers to match on.

// RateLimitError indicates 429 Too Many Requests.
type RateLimitError struct {
	RetryAfter string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("printful: rate limited (retry-after: %s)", e.RetryAfter)
}

// AuthError indicates 401 Unauthorized.
type AuthError struct {
	StatusCode int
	Body       string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("printful: unauthorized (%d)", e.StatusCode)
}

// APIError is a generic Printful API error.
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("printful: API error %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("printful: API error %d", e.StatusCode)
}

// IsRetryable returns true for server errors (5xx) that are worth retrying.
func (e *APIError) IsRetryable() bool {
	return e.StatusCode >= 500
}

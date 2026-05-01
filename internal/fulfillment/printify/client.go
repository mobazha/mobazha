package printify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	defaultBaseURL   = "https://api.printify.com/v1"
	defaultRateLimit = 600
	catalogRateLimit = 100

	rateLimitInterval = time.Minute
)

type Client struct {
	baseURL    string
	token      string
	shopID     string
	httpClient *http.Client

	mu        sync.Mutex
	tokens    int
	lastReset time.Time
	rateLimit int
}

type ClientOption func(*Client)

func WithBaseURL(url string) ClientOption {
	return func(c *Client) { c.baseURL = url }
}

func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

func WithRateLimit(limit int) ClientOption {
	return func(c *Client) {
		c.rateLimit = limit
		c.tokens = limit
	}
}

func WithShopID(id string) ClientOption {
	return func(c *Client) { c.shopID = id }
}

func NewClient(token string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    defaultBaseURL,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		rateLimit:  defaultRateLimit,
		tokens:     defaultRateLimit,
		lastReset:  time.Now(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("printify: HTTP %d: %s", e.StatusCode, e.Message)
}

func (e *APIError) IsRetryable() bool { return e.StatusCode >= 500 }

type AuthError struct{ Message string }

func (e *AuthError) Error() string { return "printify: auth error: " + e.Message }

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("printify: rate limited, retry after %s", e.RetryAfter)
}

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

func (c *Client) do(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	if err := c.waitForToken(ctx); err != nil {
		return err
	}

	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("printify: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("printify: create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", "Mobazha/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json;charset=utf-8")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("printify: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("printify: read response: %w", err)
	}

	switch {
	case resp.StatusCode == 401:
		return &AuthError{Message: string(respBody)}
	case resp.StatusCode == 429:
		retryAfter := 60 * time.Second
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				retryAfter = time.Duration(secs) * time.Second
			}
		}
		return &RateLimitError{RetryAfter: retryAfter}
	case resp.StatusCode >= 400:
		return &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("printify: decode response: %w", err)
		}
	}

	return nil
}

func (c *Client) Get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) Post(ctx context.Context, path string, body, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

func (c *Client) Put(ctx context.Context, path string, body, out interface{}) error {
	return c.do(ctx, http.MethodPut, path, body, out)
}

func (c *Client) Delete(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodDelete, path, nil, out)
}

func (c *Client) shopPath(suffix string) string {
	return fmt.Sprintf("/shops/%s%s", c.shopID, suffix)
}

func (c *Client) DiscoverShopID(ctx context.Context) error {
	var shops []pyShop
	if err := c.Get(ctx, "/shops.json", &shops); err != nil {
		return fmt.Errorf("printify: discover shop: %w", err)
	}
	if len(shops) == 0 {
		return fmt.Errorf("printify: no shops found")
	}
	c.shopID = strconv.Itoa(shops[0].ID)
	return nil
}

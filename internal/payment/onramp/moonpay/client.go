// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package moonpay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// HTTPClient is the production Client over MoonPay's REST API.
//
// Endpoints (verify against current docs during the sandbox pass):
//   - GET /v1/transactions/ext/{externalTransactionId} — buy transactions by
//     our correlation id, authenticated with the secret key.
//     https://dev.moonpay.com/api-reference
//   - GET /v3/currencies/{code}/buy_quote — fiat cost of a fixed receive
//     amount, publishable-key authenticated.
type HTTPClient struct {
	secretKey      string
	publishableKey string
	baseURL        string
	httpClient     *http.Client
}

// NewHTTPClient builds the production client. baseURL defaults to the
// production API; point it at the sandbox API for sandbox runs.
func NewHTTPClient(secretKey, publishableKey, baseURL string) *HTTPClient {
	if baseURL == "" {
		baseURL = "https://api.moonpay.com"
	}
	return &HTTPClient{
		secretKey:      secretKey,
		publishableKey: publishableKey,
		baseURL:        baseURL,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
}

// TransactionsByExternalID implements Client.
func (c *HTTPClient) TransactionsByExternalID(ctx context.Context, externalTransactionID string) ([]Transaction, error) {
	endpoint := c.baseURL + "/v1/transactions/ext/" + url.PathEscape(externalTransactionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("moonpay: build request: %w", err)
	}
	req.Header.Set("Authorization", "Api-Key "+c.secretKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("moonpay: transactions by external id: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("moonpay: read response: %w", err)
	}
	// MoonPay returns 404 for an id it has never seen: the buyer simply has
	// not completed the widget yet. That is an empty history, not an error.
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("moonpay: transactions by external id: status %d: %s", resp.StatusCode, truncate(body))
	}
	var txs []Transaction
	if err := json.Unmarshal(body, &txs); err != nil {
		return nil, fmt.Errorf("moonpay: decode transactions: %w", err)
	}
	return txs, nil
}

// BuyQuote implements Client.
func (c *HTTPClient) BuyQuote(ctx context.Context, currencyCode, fiatCurrency, quoteAmount string) (BuyQuote, error) {
	query := url.Values{}
	query.Set("apiKey", c.publishableKey)
	query.Set("baseCurrencyCode", fiatCurrency)
	query.Set("quoteCurrencyAmount", quoteAmount)
	endpoint := c.baseURL + "/v3/currencies/" + url.PathEscape(currencyCode) + "/buy_quote?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return BuyQuote{}, fmt.Errorf("moonpay: build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return BuyQuote{}, fmt.Errorf("moonpay: buy quote: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return BuyQuote{}, fmt.Errorf("moonpay: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return BuyQuote{}, fmt.Errorf("moonpay: buy quote: status %d: %s", resp.StatusCode, truncate(body))
	}
	var quote BuyQuote
	if err := json.Unmarshal(body, &quote); err != nil {
		return BuyQuote{}, fmt.Errorf("moonpay: decode buy quote: %w", err)
	}
	return quote, nil
}

func truncate(b []byte) string {
	const max = 256
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}

var _ Client = (*HTTPClient)(nil)

package tron

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"
)

const (
	defaultTimeout        = 10 * time.Second
	defaultMaxRetries     = 3
	defaultInitialBackoff = 500 * time.Millisecond
	defaultMaxBackoff     = 5 * time.Second
	solidBlockConfirms    = 19
)

// RetryConfig controls the retry/backoff behavior for TRON RPC calls.
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// DefaultRetryConfig returns the standard retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     defaultMaxRetries,
		InitialBackoff: defaultInitialBackoff,
		MaxBackoff:     defaultMaxBackoff,
	}
}

// TronClient provides resilient access to the TRON HTTP API.
// It supports multiple endpoints with automatic failover.
type TronClient struct {
	endpoints   []string
	httpClient  *http.Client
	retryConfig RetryConfig
	currentIdx  int
	mu          sync.Mutex
}

// NewTronClient creates a client with the given endpoints and retry configuration.
func NewTronClient(endpoints []string, retryConfig RetryConfig) *TronClient {
	if len(endpoints) == 0 {
		panic("tron: at least one endpoint is required")
	}
	return &TronClient{
		endpoints: endpoints,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		retryConfig: retryConfig,
	}
}

// TronTransaction represents the response from /wallet/gettransactionbyid.
type TronTransaction struct {
	TxID    string          `json:"txID"`
	RawData json.RawMessage `json:"raw_data"`
	Ret     []struct {
		ContractRet string `json:"contractRet"`
	} `json:"ret"`
}

// TronTransactionInfo represents the response from /wallet/gettransactioninfobyid.
type TronTransactionInfo struct {
	ID              string `json:"id"`
	BlockNumber     int64  `json:"blockNumber"`
	BlockTimeStamp  int64  `json:"blockTimeStamp"`
	ContractResult  []string `json:"contractResult"`
	Receipt         TronReceipt `json:"receipt"`
	Log             []TronEventLog `json:"log"`
	Result          string `json:"result,omitempty"`
	ResMessage      string `json:"resMessage,omitempty"`
}

// TronReceipt represents the receipt portion of transaction info.
type TronReceipt struct {
	EnergyUsage      int64  `json:"energy_usage"`
	EnergyUsageTotal int64  `json:"energy_usage_total"`
	NetUsage         int64  `json:"net_usage"`
	Result           string `json:"result"`
}

// TronEventLog represents an event log entry.
type TronEventLog struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}

// TronBlock represents a block response.
type TronBlock struct {
	BlockHeader struct {
		RawData struct {
			Number    int64 `json:"number"`
			Timestamp int64 `json:"timestamp"`
		} `json:"raw_data"`
	} `json:"block_header"`
}

// ConstantResult represents the response from /wallet/triggerconstantcontract.
type ConstantResult struct {
	Result struct {
		Result  bool   `json:"result"`
		Message string `json:"message,omitempty"`
	} `json:"result"`
	ConstantResult []string `json:"constant_result"`
	Transaction    json.RawMessage `json:"transaction"`
}

// GetTransaction queries a transaction by ID.
func (c *TronClient) GetTransaction(ctx context.Context, txID string) (*TronTransaction, error) {
	body := map[string]string{"value": txID}
	var result TronTransaction
	if err := c.doWithRetry(ctx, "/wallet/gettransactionbyid", body, &result); err != nil {
		return nil, err
	}
	if result.TxID == "" {
		return nil, fmt.Errorf("tron: transaction %s not found", txID)
	}
	return &result, nil
}

// GetTransactionInfo queries the transaction receipt/info by ID.
func (c *TronClient) GetTransactionInfo(ctx context.Context, txID string) (*TronTransactionInfo, error) {
	body := map[string]string{"value": txID}
	var result TronTransactionInfo
	if err := c.doWithRetry(ctx, "/wallet/gettransactioninfobyid", body, &result); err != nil {
		return nil, err
	}
	if result.ID == "" {
		return nil, fmt.Errorf("tron: transaction info for %s not found", txID)
	}
	return &result, nil
}

// GetNowBlock returns the latest block.
func (c *TronClient) GetNowBlock(ctx context.Context) (*TronBlock, error) {
	var result TronBlock
	if err := c.doWithRetry(ctx, "/wallet/getnowblock", struct{}{}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TriggerConstantContract performs a read-only contract call.
func (c *TronClient) TriggerConstantContract(ctx context.Context, ownerAddr, contractAddr, functionSelector, parameter string) (*ConstantResult, error) {
	body := map[string]interface{}{
		"owner_address":     ownerAddr,
		"contract_address":  contractAddr,
		"function_selector": functionSelector,
		"parameter":         parameter,
	}
	var result ConstantResult
	if err := c.doWithRetry(ctx, "/wallet/triggerconstantcontract", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BroadcastTransaction broadcasts a signed transaction.
func (c *TronClient) BroadcastTransaction(ctx context.Context, signedTx json.RawMessage) error {
	var result struct {
		Result  bool   `json:"result"`
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
		TxID    string `json:"txid,omitempty"`
	}
	if err := c.doWithRetry(ctx, "/wallet/broadcasttransaction", signedTx, &result); err != nil {
		return err
	}
	if !result.Result {
		return fmt.Errorf("tron: broadcast failed: code=%s message=%s", result.Code, result.Message)
	}
	return nil
}

// doWithRetry executes an HTTP POST with exponential backoff and endpoint failover.
func (c *TronClient) doWithRetry(ctx context.Context, path string, body interface{}, result interface{}) error {
	var lastErr error

	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := c.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		endpoint := c.currentEndpoint()
		url := endpoint + path

		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("tron: marshal request body: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
		if err != nil {
			return fmt.Errorf("tron: create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("tron: endpoint %s: %w", endpoint, err)
			c.switchEndpoint()
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("tron: read response: %w", err)
			c.switchEndpoint()
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("tron: HTTP %d from %s%s: %s", resp.StatusCode, endpoint, path, string(respBody))
			c.switchEndpoint()
			continue
		}

		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("tron: unmarshal response: %w", err)
		}
		return nil
	}

	return fmt.Errorf("tron: all retries exhausted: %w", lastErr)
}

func (c *TronClient) currentEndpoint() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.endpoints[c.currentIdx]
}

func (c *TronClient) switchEndpoint() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.endpoints) > 1 {
		c.currentIdx = (c.currentIdx + 1) % len(c.endpoints)
	}
}

func (c *TronClient) calculateBackoff(attempt int) time.Duration {
	backoff := c.retryConfig.InitialBackoff * time.Duration(math.Pow(2, float64(attempt-1)))
	if backoff > c.retryConfig.MaxBackoff {
		backoff = c.retryConfig.MaxBackoff
	}
	return backoff
}

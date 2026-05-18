package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// HTTPPlatformRelay calls hosting POST /platform/v1/relay/execute for standalone
// nodes that configure RelayAPIURL (+ optional Bearer JWT).
type HTTPPlatformRelay struct {
	baseURL string // e.g. https://app.example.com (no trailing path)
	token   string
	http    *http.Client

	mu           sync.RWMutex
	byChainID    map[uint64]string // from GET /platform/v1/relay/status evmChains
	registryOnce sync.Once
}

var _ EVMRelayService = (*HTTPPlatformRelay)(nil)

// NewHTTPPlatformRelay constructs a client; baseURL is trimmed of trailing slashes.
func NewHTTPPlatformRelay(baseURL string, bearerToken string) *HTTPPlatformRelay {
	return &HTTPPlatformRelay{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(bearerToken),
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// defaultRelayChainTypeFromID is used only when status.evmChains is unavailable
// (legacy hosting or fetch failure). Must stay aligned with app*.yaml relay.chains keys.
func defaultRelayChainTypeFromID(chainID uint64) (string, error) {
	switch chainID {
	case 1:
		return "eth", nil
	case 56:
		return "bsc", nil
	case 137:
		return "polygon", nil
	case 8453:
		return "base", nil
	case 10:
		return "optimism", nil
	case 100:
		return "gnosis", nil
	case 324:
		return "zksync", nil
	case 5000:
		return "mantle", nil
	case 42161:
		return "arbitrum", nil
	case 42220:
		return "celo", nil
	case 43114:
		return "avalanche", nil
	case 59144:
		return "linea", nil
	case 534352:
		return "scroll", nil
	default:
		return "", fmt.Errorf("chain id %d has no relay key mapping", chainID)
	}
}

func (c *HTTPPlatformRelay) loadRegistryFromStatus(ctx context.Context) error {
	u := c.baseURL + "/platform/v1/relay/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	c.applyAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("relay status HTTP %d", resp.StatusCode)
	}
	var wrapped successEnvelope[relayStatusData]
	if err := json.Unmarshal(raw, &wrapped); err != nil || wrapped.Data == nil {
		return fmt.Errorf("relay status: malformed envelope")
	}
	next := make(map[uint64]string)
	for _, row := range wrapped.Data.EvmChains {
		if row.ChainID == 0 || row.ChainType == "" {
			continue
		}
		next[row.ChainID] = strings.ToLower(strings.TrimSpace(row.ChainType))
	}
	c.mu.Lock()
	c.byChainID = next
	c.mu.Unlock()
	if len(next) == 0 {
		return fmt.Errorf("relay status: evmChains empty")
	}
	return nil
}

// ChainTypeForID implements EVMRelayService — prefers hosting GET /relay/status evmChains.
func (c *HTTPPlatformRelay) ChainTypeForID(chainID uint64) (string, error) {
	c.tryFillRegistry()

	c.mu.RLock()
	if c.byChainID != nil {
		if ct, ok := c.byChainID[chainID]; ok && ct != "" {
			c.mu.RUnlock()
			return ct, nil
		}
	}
	c.mu.RUnlock()

	return defaultRelayChainTypeFromID(chainID)
}

func (c *HTTPPlatformRelay) tryFillRegistry() {
	c.registryOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = c.loadRegistryFromStatus(ctx)
	})
}

// IsAvailable implements EVMRelayService.
func (c *HTTPPlatformRelay) IsAvailable() bool {
	return c != nil && c.baseURL != ""
}

func (c *HTTPPlatformRelay) applyAuth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

type relayExecHTTPBody struct {
	ChainType        string `json:"chainType"`
	To               string `json:"to"`
	Data             string `json:"data"`
	OrderID          string `json:"orderId,omitempty"`
	SettlementAction string `json:"settlementAction,omitempty"`
	ClientActionID   string `json:"clientActionId,omitempty"`
}

type successEnvelope[T any] struct {
	Data *T `json:"data,omitempty"`
}

type relayExecDataPayload struct {
	Success bool   `json:"success"`
	TxHash  string `json:"txHash,omitempty"`
	TaskID  string `json:"taskId,omitempty"`
	Error   string `json:"error,omitempty"`
}

type errEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Detail  string `json:"detail,omitempty"`
	} `json:"error"`
}

type relayStatusData struct {
	Enabled         bool     `json:"enabled"`
	GasWalletAddr   string   `json:"gasWalletAddress,omitempty"`
	SupportedChains []string `json:"supportedChains,omitempty"`
	EvmChains       []struct {
		ChainType string `json:"chainType"`
		ChainID   uint64 `json:"chainId"`
	} `json:"evmChains"`
}

type gasWalletData struct {
	Address      string `json:"address"`
	Balance      string `json:"balance"`
	LowWatermark string `json:"lowWatermark,omitempty"`
	Available    bool   `json:"available"`
	Reason       string `json:"reason,omitempty"`
}

// Execute implements EVMRelayService.
func (c *HTTPPlatformRelay) Execute(ctx context.Context, req *EVMRelayRequest) (*EVMRelayResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("nil relay request")
	}
	payload := relayExecHTTPBody{
		ChainType:        req.ChainType,
		To:               req.To,
		Data:             req.Data,
		OrderID:          req.OrderID,
		SettlementAction: req.SettlementAction,
		ClientActionID:   req.ClientActionID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	u := c.baseURL + "/platform/v1/relay/execute"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.applyAuth(httpReq)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var ee errEnvelope
		if json.Unmarshal(raw, &ee) == nil && ee.Error.Message != "" {
			return nil, fmt.Errorf("relay HTTP %d: %s (%s)", resp.StatusCode, ee.Error.Message, ee.Error.Code)
		}
		return nil, fmt.Errorf("relay HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var wrapped successEnvelope[relayExecDataPayload]
	if err := json.Unmarshal(raw, &wrapped); err != nil || wrapped.Data == nil {
		return nil, fmt.Errorf("relay execute: malformed success envelope")
	}
	d := wrapped.Data
	if !d.Success || d.TxHash == "" {
		if d.Error != "" {
			return nil, fmt.Errorf("relay declined: %s", d.Error)
		}
		return nil, fmt.Errorf("relay response missing txHash")
	}
	if !common.IsHexHash(strings.TrimSpace(d.TxHash)) {
		return nil, fmt.Errorf("relay response returned invalid txHash %q", d.TxHash)
	}
	return &EVMRelayResponse{TxHash: d.TxHash, TaskID: d.TaskID}, nil
}

// GetSupportedChains implements EVMRelayService.
func (c *HTTPPlatformRelay) GetSupportedChains() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	u := c.baseURL + "/platform/v1/relay/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")
	c.applyAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	var wrapped successEnvelope[relayStatusData]
	if json.Unmarshal(raw, &wrapped) != nil || wrapped.Data == nil {
		return nil
	}
	return wrapped.Data.SupportedChains
}

// GetGasWalletAddress implements EVMRelayService.
func (c *HTTPPlatformRelay) GetGasWalletAddress(ctx context.Context, chainID uint64) (string, error) {
	st, err := c.GetGasWalletStatus(ctx, chainID)
	if err != nil {
		return "", err
	}
	if st == nil || strings.TrimSpace(st.Address) == "" {
		return "", fmt.Errorf("empty gas wallet address")
	}
	return st.Address, nil
}

// GetGasWalletStatus implements EVMRelayService.
func (c *HTTPPlatformRelay) GetGasWalletStatus(ctx context.Context, chainID uint64) (*EVMGasWalletStatus, error) {
	u := c.baseURL + "/platform/v1/relay/gas-wallet"
	q := url.Values{}
	q.Set("chain_id", strconv.FormatUint(chainID, 10))

	reqURL := u + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	c.applyAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var ee errEnvelope
		if json.Unmarshal(raw, &ee) == nil && ee.Error.Message != "" {
			return nil, fmt.Errorf("gas-wallet %d: %s", resp.StatusCode, ee.Error.Message)
		}
		return nil, fmt.Errorf("gas-wallet HTTP %d", resp.StatusCode)
	}
	var wrapped successEnvelope[gasWalletData]
	if err := json.Unmarshal(raw, &wrapped); err != nil || wrapped.Data == nil {
		return nil, fmt.Errorf("gas-wallet: malformed envelope")
	}
	d := wrapped.Data
	bal := big.NewInt(0)
	if d.Balance != "" {
		if bi, ok := new(big.Int).SetString(d.Balance, 10); ok {
			bal = bi
		}
	}
	return &EVMGasWalletStatus{
		Address: d.Address,
		Balance: bal,
		Healthy: d.Available,
		Reason:  d.Reason,
	}, nil
}

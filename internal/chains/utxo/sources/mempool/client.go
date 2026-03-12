package mempool

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("mempool")

// WebSocket reconnection settings
const (
	wsReconnectInterval = 5 * time.Second
	wsMaxReconnectTries = 10
	wsPingInterval      = 25 * time.Second // Send ping every 25s to keep connection alive
	wsWriteTimeout      = 10 * time.Second
	wsReadTimeout       = 90 * time.Second // Allow 90s between messages (3x ping interval)
)

// Source implements utxo.PaymentSource using mempool.space API
// Supports both REST API and WebSocket for real-time transaction notifications
type Source struct {
	baseURL    string
	wsURL      string
	httpClient *http.Client
	chain      iwallet.ChainType
	testnet    bool
	mu         sync.RWMutex
	healthy    bool
	lastCheck  time.Time

	// WebSocket connection
	ws          *websocket.Conn
	wsMu        sync.Mutex
	wsConnected bool
	wsShutdown  chan struct{}
	wsReconnect chan struct{}
	wsDone      sync.WaitGroup

	// Subscription management
	subscriptions   map[string]func(tx *iwallet.Transaction) // address -> callback
	subscriptionsMu sync.RWMutex
}

// API response types
type AddressInfo struct {
	Address      string       `json:"address"`
	ChainStats   AddressStats `json:"chain_stats"`
	MempoolStats AddressStats `json:"mempool_stats"`
}

type AddressStats struct {
	FundedTxoCount uint64 `json:"funded_txo_count"`
	FundedTxoSum   uint64 `json:"funded_txo_sum"`
	SpentTxoCount  uint64 `json:"spent_txo_count"`
	SpentTxoSum    uint64 `json:"spent_txo_sum"`
	TxCount        uint64 `json:"tx_count"`
}

type TransactionAPI struct {
	Txid     string    `json:"txid"`
	Version  int       `json:"version"`
	Locktime int       `json:"locktime"`
	Size     int       `json:"size"`
	Weight   int       `json:"weight"`
	Fee      uint64    `json:"fee"`
	Vin      []VinAPI  `json:"vin"`
	Vout     []VoutAPI `json:"vout"`
	Status   StatusAPI `json:"status"`
}

type VinAPI struct {
	Txid         string   `json:"txid"`
	Vout         int      `json:"vout"`
	Prevout      *VoutAPI `json:"prevout,omitempty"`
	Scriptsig    string   `json:"scriptsig"`
	ScriptsigAsm string   `json:"scriptsig_asm"`
	Sequence     uint32   `json:"sequence"`
}

type VoutAPI struct {
	Scriptpubkey        string `json:"scriptpubkey"`
	ScriptpubkeyAsm     string `json:"scriptpubkey_asm"`
	ScriptpubkeyType    string `json:"scriptpubkey_type"`
	ScriptpubkeyAddress string `json:"scriptpubkey_address,omitempty"`
	Value               uint64 `json:"value"`
}

type StatusAPI struct {
	Confirmed   bool   `json:"confirmed"`
	BlockHeight int64  `json:"block_height,omitempty"`
	BlockHash   string `json:"block_hash,omitempty"`
	BlockTime   int64  `json:"block_time,omitempty"`
}

// NewSource creates a new mempool.space payment source
// Set testnet=true for testnet (testnet4 for BTC), false for mainnet
// Returns nil if the chain is not supported
func NewSource(chain iwallet.ChainType, testnet bool) *Source {
	baseURL := getBaseURL(chain, testnet)
	if baseURL == "" {
		log.Warningf("Mempool source not available for chain %s (testnet=%v)", chain, testnet)
		return nil
	}

	wsURL := getWebSocketURL(chain, testnet)

	return &Source{
		baseURL: baseURL,
		wsURL:   wsURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		chain:         chain,
		testnet:       testnet,
		healthy:       true,
		wsShutdown:    make(chan struct{}),
		wsReconnect:   make(chan struct{}, 1),
		subscriptions: make(map[string]func(tx *iwallet.Transaction)),
	}
}

func getBaseURL(chain iwallet.ChainType, testnet bool) string {
	switch chain {
	case iwallet.ChainBitcoin:
		if testnet {
			// Bitcoin Testnet4 (the latest testnet, replacing testnet3)
			return "https://mempool.space/testnet4/api"
		}
		return "https://mempool.space/api"
	case iwallet.ChainLitecoin:
		if testnet {
			// Litecoin testnet not supported by litecoinspace.org
			return ""
		}
		// litecoinspace.org is a mempool.space fork for LTC
		return "https://litecoinspace.org/api"
	case iwallet.ChainBitcoinCash:
		// No reliable public REST API for BCH, return empty to disable
		return ""
	case iwallet.ChainZCash:
		// No reliable public REST API for ZEC, return empty to disable
		return ""
	default:
		return ""
	}
}

func getWebSocketURL(chain iwallet.ChainType, testnet bool) string {
	switch chain {
	case iwallet.ChainBitcoin:
		if testnet {
			// Bitcoin Testnet4 WebSocket
			return "wss://mempool.space/testnet4/api/v1/ws"
		}
		return "wss://mempool.space/api/v1/ws"
	case iwallet.ChainLitecoin:
		if testnet {
			// Litecoin testnet WebSocket not available
			return ""
		}
		return "wss://litecoinspace.org/api/v1/ws"
	default:
		return ""
	}
}

// Subscribe subscribes to payment notifications for an address via WebSocket
func (s *Source) Subscribe(ctx context.Context, address string, scriptPubKey []byte, callback func(tx *iwallet.Transaction)) error {
	if s.wsURL == "" {
		return fmt.Errorf("WebSocket not available for chain %s", s.chain)
	}

	// Store the callback
	s.subscriptionsMu.Lock()
	s.subscriptions[address] = callback
	s.subscriptionsMu.Unlock()

	// Ensure WebSocket is connected
	if err := s.ensureWSConnected(); err != nil {
		return fmt.Errorf("failed to connect WebSocket: %w", err)
	}

	// Send track-address message
	return s.trackAddress(address)
}

// Unsubscribe unsubscribes from payment notifications for an address
func (s *Source) Unsubscribe(ctx context.Context, address string) error {
	s.subscriptionsMu.Lock()
	delete(s.subscriptions, address)
	remaining := len(s.subscriptions)
	s.subscriptionsMu.Unlock()

	// If no more subscriptions, we could close the WebSocket
	// But we'll keep it open for potential future subscriptions
	if remaining == 0 {
		log.Debugf("No more subscriptions for chain %s, WebSocket kept alive", s.chain)
	}

	return nil
}

// GetTransactions gets transactions for an address
func (s *Source) GetTransactions(ctx context.Context, address string, scriptPubKey []byte) ([]*iwallet.Transaction, error) {
	url := fmt.Sprintf("%s/address/%s/txs", s.baseURL, address)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.markUnhealthy()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.markUnhealthy()
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var txs []TransactionAPI
	if err := json.Unmarshal(body, &txs); err != nil {
		return nil, err
	}

	s.markHealthy()

	result := make([]*iwallet.Transaction, 0, len(txs))
	for _, tx := range txs {
		result = append(result, s.convertTransaction(&tx))
	}

	return result, nil
}

// GetTransaction gets a specific transaction by ID
func (s *Source) GetTransaction(ctx context.Context, txid string) (*iwallet.Transaction, error) {
	url := fmt.Sprintf("%s/tx/%s", s.baseURL, txid)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.markUnhealthy()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.markUnhealthy()
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tx TransactionAPI
	if err := json.Unmarshal(body, &tx); err != nil {
		return nil, err
	}

	s.markHealthy()

	return s.convertTransaction(&tx), nil
}

func (s *Source) convertTransaction(tx *TransactionAPI) *iwallet.Transaction {
	result := &iwallet.Transaction{
		ID: iwallet.TransactionID(tx.Txid),
	}

	if tx.Status.Confirmed {
		result.Height = uint64(tx.Status.BlockHeight)
		result.Timestamp = time.Unix(tx.Status.BlockTime, 0)
	} else {
		result.Height = 0
		// For unconfirmed transactions, use current time as timestamp
		result.Timestamp = time.Now()
	}

	// Calculate total value from outputs
	var totalValue uint64
	for i, vout := range tx.Vout {
		totalValue += vout.Value
		if vout.ScriptpubkeyAddress != "" {
			// Create outpoint ID (txid:vout) for UTXO tracking
			outpointID := makeOutpointID(tx.Txid, uint32(i))
			result.To = append(result.To, iwallet.SpendInfo{
				ID:      outpointID,
				Address: iwallet.NewAddress(vout.ScriptpubkeyAddress, iwallet.CoinType(s.chain)),
				Amount:  iwallet.NewAmount(int64(vout.Value)),
			})
		}
	}
	result.Value = iwallet.NewAmount(int64(totalValue))

	// Extract from addresses with outpoint IDs
	for _, vin := range tx.Vin {
		if vin.Prevout != nil && vin.Prevout.ScriptpubkeyAddress != "" {
			// Create outpoint ID for the input (previous output being spent)
			outpointID := makeOutpointID(vin.Txid, uint32(vin.Vout))
			result.From = append(result.From, iwallet.SpendInfo{
				ID:      outpointID,
				Address: iwallet.NewAddress(vin.Prevout.ScriptpubkeyAddress, iwallet.CoinType(s.chain)),
				Amount:  iwallet.NewAmount(int64(vin.Prevout.Value)),
			})
		}
	}

	return result
}

// makeOutpointID creates a serialized outpoint ID (txid:vout)
// Returns nil if txid is invalid or cannot be decoded
func makeOutpointID(txid string, vout uint32) []byte {
	if txid == "" {
		return nil
	}
	txidBytes, err := hex.DecodeString(txid)
	if err != nil {
		log.Warningf("Failed to decode txid %s: %v", txid, err)
		return nil
	}
	if len(txidBytes) != 32 {
		log.Warningf("Invalid txid length %d (expected 32): %s", len(txidBytes), txid)
		return nil
	}
	// Reverse txid bytes (Bitcoin uses little-endian internally)
	for i, j := 0, len(txidBytes)-1; i < j; i, j = i+1, j-1 {
		txidBytes[i], txidBytes[j] = txidBytes[j], txidBytes[i]
	}
	outpoint := make([]byte, 36)
	copy(outpoint[:32], txidBytes)
	binary.LittleEndian.PutUint32(outpoint[32:], vout)
	return outpoint
}

// IsHealthy returns true if the source is healthy
func (s *Source) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Consider unhealthy if last failure was less than 1 minute ago
	if !s.healthy && time.Since(s.lastCheck) < time.Minute {
		return false
	}

	return true
}

func (s *Source) markHealthy() {
	s.mu.Lock()
	s.healthy = true
	s.lastCheck = time.Now()
	s.mu.Unlock()
}

func (s *Source) markUnhealthy() {
	s.mu.Lock()
	s.healthy = false
	s.lastCheck = time.Now()
	s.mu.Unlock()
}

// Chain returns the chain type this source supports
func (s *Source) Chain() iwallet.ChainType {
	return s.chain
}

// Close closes the source and WebSocket connection
func (s *Source) Close() error {
	s.closeWebSocket()
	return nil
}

// WebSocket connection management

// ensureWSConnected ensures the WebSocket connection is established
func (s *Source) ensureWSConnected() error {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()

	if s.wsConnected && s.ws != nil {
		return nil
	}

	return s.connectWebSocket()
}

// connectWebSocket establishes a WebSocket connection
// Must be called with wsMu held
func (s *Source) connectWebSocket() error {
	if s.wsURL == "" {
		return fmt.Errorf("WebSocket URL not configured")
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(s.wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	s.ws = conn
	s.wsConnected = true

	// Start message handler
	s.wsDone.Add(1)
	go s.handleWebSocketMessages()

	// Start ping loop
	s.wsDone.Add(1)
	go s.pingLoop()

	log.Infof("[%s] Connected to mempool.space WebSocket: %s", s.chain, s.wsURL)

	// Re-subscribe all existing addresses
	s.subscriptionsMu.RLock()
	addresses := make([]string, 0, len(s.subscriptions))
	for addr := range s.subscriptions {
		addresses = append(addresses, addr)
	}
	s.subscriptionsMu.RUnlock()

	for _, addr := range addresses {
		if err := s.trackAddressLocked(addr); err != nil {
			log.Warningf("[%s] Failed to re-subscribe address %s: %v", s.chain, addr, err)
		}
	}

	return nil
}

// closeWebSocket closes the WebSocket connection
func (s *Source) closeWebSocket() {
	s.wsMu.Lock()
	if s.ws != nil {
		s.ws.Close()
		s.ws = nil
	}
	s.wsConnected = false
	s.wsMu.Unlock()

	// Signal shutdown
	select {
	case <-s.wsShutdown:
		// Already closed
	default:
		close(s.wsShutdown)
	}

	s.wsDone.Wait()
}

// trackAddress sends a track-address message to the WebSocket
func (s *Source) trackAddress(address string) error {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	return s.trackAddressLocked(address)
}

// trackAddressLocked sends a track-address message (must be called with wsMu held)
func (s *Source) trackAddressLocked(address string) error {
	if s.ws == nil {
		return fmt.Errorf("WebSocket not connected")
	}

	// mempool.space WebSocket API: {"action": "track-address", "address": "..."}
	msg := map[string]interface{}{
		"action":  "track-address",
		"address": address,
	}

	s.ws.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
	if err := s.ws.WriteJSON(msg); err != nil {
		return fmt.Errorf("failed to send track-address: %w", err)
	}

	log.Debugf("[%s] Tracking address: %s", s.chain, address)
	return nil
}

// handleWebSocketMessages handles incoming WebSocket messages
func (s *Source) handleWebSocketMessages() {
	defer s.wsDone.Done()

	for {
		select {
		case <-s.wsShutdown:
			return
		default:
		}

		s.wsMu.Lock()
		ws := s.ws
		s.wsMu.Unlock()

		if ws == nil {
			return
		}

		ws.SetReadDeadline(time.Now().Add(wsReadTimeout))
		_, message, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Warningf("[%s] WebSocket read error: %v", s.chain, err)
			}
			s.handleDisconnection()
			return
		}

		s.processWebSocketMessage(message)
	}
}

// WebSocket message types from mempool.space
type wsAddressTransaction struct {
	Address      string           `json:"address"`
	Transactions []TransactionAPI `json:"address-transactions,omitempty"`
}

type wsMessage struct {
	// For address-transactions event
	AddressTransactions []TransactionAPI `json:"address-transactions,omitempty"`
}

// processWebSocketMessage processes an incoming WebSocket message
func (s *Source) processWebSocketMessage(message []byte) {
	// mempool.space sends different message types
	// For address tracking, it sends: {"address-transactions": [...]}

	var msg map[string]json.RawMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Warningf("[%s] Failed to parse WebSocket message: %v", s.chain, err)
		return
	}

	// Handle address-transactions event
	if txData, ok := msg["address-transactions"]; ok {
		var txs []TransactionAPI
		if err := json.Unmarshal(txData, &txs); err != nil {
			log.Warningf("[%s] Failed to parse address-transactions: %v", s.chain, err)
			return
		}

		for _, tx := range txs {
			s.handleIncomingTransaction(&tx)
		}
	}
}

// handleIncomingTransaction processes an incoming transaction notification
func (s *Source) handleIncomingTransaction(tx *TransactionAPI) {
	converted := s.convertTransaction(tx)

	log.Infof("[%s] Received transaction via WebSocket: %s", s.chain, tx.Txid)

	// Find matching subscriptions based on transaction addresses
	s.subscriptionsMu.RLock()
	defer s.subscriptionsMu.RUnlock()

	// Check all output addresses
	for _, vout := range tx.Vout {
		if callback, ok := s.subscriptions[vout.ScriptpubkeyAddress]; ok {
			callback(converted)
			return // Only call once per transaction
		}
	}

	// Check all input addresses
	for _, vin := range tx.Vin {
		if vin.Prevout != nil && vin.Prevout.ScriptpubkeyAddress != "" {
			if callback, ok := s.subscriptions[vin.Prevout.ScriptpubkeyAddress]; ok {
				callback(converted)
				return // Only call once per transaction
			}
		}
	}
}

// handleDisconnection handles WebSocket disconnection and triggers reconnection
func (s *Source) handleDisconnection() {
	s.wsMu.Lock()
	s.wsConnected = false
	if s.ws != nil {
		s.ws.Close()
		s.ws = nil
	}
	s.wsMu.Unlock()

	log.Warningf("[%s] WebSocket disconnected, attempting reconnection...", s.chain)

	// Trigger reconnection in a goroutine
	go s.reconnectWebSocket()
}

// reconnectWebSocket attempts to reconnect the WebSocket with exponential backoff
func (s *Source) reconnectWebSocket() {
	for i := 0; i < wsMaxReconnectTries; i++ {
		select {
		case <-s.wsShutdown:
			return
		default:
		}

		// Wait before reconnecting (exponential backoff)
		waitTime := wsReconnectInterval * time.Duration(1<<uint(i))
		if waitTime > 2*time.Minute {
			waitTime = 2 * time.Minute
		}

		log.Infof("[%s] Reconnecting WebSocket in %v (attempt %d/%d)", s.chain, waitTime, i+1, wsMaxReconnectTries)

		select {
		case <-s.wsShutdown:
			return
		case <-time.After(waitTime):
		}

		s.wsMu.Lock()
		err := s.connectWebSocket()
		s.wsMu.Unlock()

		if err == nil {
			log.Infof("[%s] WebSocket reconnected successfully", s.chain)
			return
		}

		log.Warningf("[%s] Reconnection attempt %d failed: %v", s.chain, i+1, err)
	}

	log.Errorf("[%s] Failed to reconnect WebSocket after %d attempts", s.chain, wsMaxReconnectTries)
	s.markUnhealthy()
}

// pingLoop sends periodic pings to keep the WebSocket connection alive
func (s *Source) pingLoop() {
	defer s.wsDone.Done()

	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.wsShutdown:
			return
		case <-ticker.C:
			s.wsMu.Lock()
			ws := s.ws
			s.wsMu.Unlock()

			if ws == nil {
				return
			}

			// mempool.space WebSocket: send JSON ping to keep connection alive
			// Format: {"action": "ping"}
			ws.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			pingMsg := map[string]string{"action": "ping"}
			if err := ws.WriteJSON(pingMsg); err != nil {
				log.Warningf("[%s] Failed to send ping: %v", s.chain, err)
				s.handleDisconnection()
				return
			}
		}
	}
}

// GetSubscriptionCount returns the number of active subscriptions
func (s *Source) GetSubscriptionCount() int {
	s.subscriptionsMu.RLock()
	defer s.subscriptionsMu.RUnlock()
	return len(s.subscriptions)
}

// IsWebSocketConnected returns true if the WebSocket is connected
func (s *Source) IsWebSocketConnected() bool {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	return s.wsConnected
}

// FeeEstimate represents the fee estimation response from mempool.space
type FeeEstimate struct {
	FastestFee  uint64 `json:"fastestFee"`  // 1 block
	HalfHourFee uint64 `json:"halfHourFee"` // 3 blocks
	HourFee     uint64 `json:"hourFee"`     // 6 blocks
	EconomyFee  uint64 `json:"economyFee"`  // 36+ blocks
	MinimumFee  uint64 `json:"minimumFee"`
}

// BroadcastTransaction broadcasts a raw transaction to the network
// Returns the transaction ID if successful
func (s *Source) BroadcastTransaction(ctx context.Context, txHex string) (string, error) {
	url := fmt.Sprintf("%s/tx", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString(txHex))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("broadcast failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Response is the txid
	txid := strings.TrimSpace(string(body))
	return txid, nil
}

// EstimateFee estimates the fee for confirmation in numBlocks
// Returns fee in satoshis per vbyte
func (s *Source) EstimateFee(ctx context.Context, numBlocks int) (uint64, error) {
	url := fmt.Sprintf("%s/v1/fees/recommended", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var feeEstimate FeeEstimate
	if err := json.NewDecoder(resp.Body).Decode(&feeEstimate); err != nil {
		return 0, err
	}

	// Select appropriate fee based on target blocks
	var feeRate uint64
	switch {
	case numBlocks <= 1:
		feeRate = feeEstimate.FastestFee
	case numBlocks <= 3:
		feeRate = feeEstimate.HalfHourFee
	case numBlocks <= 6:
		feeRate = feeEstimate.HourFee
	default:
		feeRate = feeEstimate.EconomyFee
	}

	// Ensure minimum fee
	if feeRate < 1 {
		feeRate = 1
	}

	return feeRate, nil
}
